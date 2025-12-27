// Package cli provides the command-line interface for Patrol.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/profile"
	"github.com/xabinapal/patrol/internal/utils"
)

// CLI holds the application state for the CLI.
type CLI struct {
	Config  *config.Config
	Keyring keyring.Store
	rootCmd *cobra.Command

	// Flags
	profileFlag string
	verboseFlag bool
	outputFlag  string
}

// New creates a new CLI instance.
func New() *CLI {
	cli := &CLI{
		Keyring: keyring.DefaultStore(),
	}

	cli.rootCmd = &cobra.Command{
		Use:   "patrol [command]",
		Short: "Patrol - Vault/OpenBao token manager",
		Long: `Patrol is a CLI utility that manages HashiCorp Vault and OpenBao
authentication tokens, providing secure persistent storage and automatic renewal.

Patrol can be used in two ways:
  1. As a CLI proxy: Use 'patrol' in place of 'vault' or 'bao' commands
  2. As a token helper: Configure Vault to use Patrol for token storage

Any command not recognized as a Patrol command will be passed through to the
underlying Vault/OpenBao CLI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.initialize(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args, show help
			if len(args) == 0 {
				return cmd.Help()
			}
			// Otherwise, proxy to vault
			return cli.proxyCommand(cmd.Context(), args)
		},
	}

	// Configure to accept unknown commands for proxying
	cli.rootCmd.FParseErrWhitelist.UnknownFlags = true

	// Global flags
	cli.rootCmd.PersistentFlags().StringVarP(&cli.profileFlag, "profile", "p", "", "Use a specific profile")
	cli.rootCmd.PersistentFlags().BoolVarP(&cli.verboseFlag, "verbose", "v", false, "Enable verbose output")
	cli.rootCmd.PersistentFlags().StringVarP(&cli.outputFlag, "output", "o", "text", "Output format (text, json)")

	// Add commands
	cli.addCommands()

	return cli
}

// addCommands adds all subcommands to the root command.
func (cli *CLI) addCommands() {
	cli.rootCmd.AddCommand(
		cli.newVersionCmd(),
		cli.newLoginCmd(),
		cli.newLogoutCmd(),
		cli.newProfileCmd(),
		cli.newConfigCmd(),
		cli.newDoctorCmd(),
		cli.newDaemonCmd(),
		cli.newTokenHelperCmd(),
		cli.newCompletionCmd(),
	)
}

// initialize loads configuration and sets up the CLI.
func (cli *CLI) initialize(cmd *cobra.Command) error {
	// Skip initialization for certain commands
	if IsTokenHelperCommand(cmd.Name()) {
		return nil
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	cli.Config = cfg

	// Override current profile if flag is set
	if cli.profileFlag != "" {
		if err := cli.Config.SetCurrent(cli.profileFlag); err != nil {
			return fmt.Errorf("invalid profile: %w", err)
		}
	}

	// Check environment variable for profile override
	if envProfile := os.Getenv("PATROL_PROFILE"); envProfile != "" && cli.profileFlag == "" {
		// Security: Validate profile name format before using in error messages
		if !utils.IsValidProfileName(envProfile) {
			if cli.verboseFlag {
				fmt.Fprintf(os.Stderr, "Warning: PATROL_PROFILE contains invalid profile name format\n")
			}
		} else if err := cli.Config.SetCurrent(envProfile); err != nil {
			// Don't fail, just warn
			if cli.verboseFlag {
				fmt.Fprintf(os.Stderr, "Warning: PATROL_PROFILE profile %q not found\n", envProfile)
			}
		}
	}

	return nil
}

// Execute runs the CLI.
// It detects the command type and routes to the appropriate handler.
func (cli *CLI) Execute(ctx context.Context) error {
	// Route based on command type
	args := os.Args[1:]

	// 1. Check if this is a token helper invocation
	if cli.ShouldHandleAsTokenHelper(args) {
		return cli.handleTokenHelper(ctx, args[0])
	}

	// 2. Check if this should be proxied to vault/bao
	if shouldProxy, proxyArgs := cli.ShouldProxy(); shouldProxy {
		if err := cli.InitializeForProxy(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return cli.proxyCommand(ctx, proxyArgs)
	}

	// 3. Handle as a regular Patrol command
	return cli.rootCmd.ExecuteContext(ctx)
}

// GetCurrentProfile returns the current profile, considering flags and env vars.
func (cli *CLI) GetCurrentProfile() (*profile.Profile, error) {
	return profile.GetCurrent(cli.Config)
}
