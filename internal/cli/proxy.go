package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/utils"
)

// patrolCommands is the single source of truth for all built-in Patrol commands.
// This prevents bugs where new commands are added to addCommands() but not here.
var patrolCommands = map[string]bool{
	// Core commands
	"login": true, "logout": true, "daemon": true,
	// Profile management
	"profile": true,
	// Token helper commands (used by Vault)
	"get": true, "store": true, "erase": true,
	// Additional commands
	"config": true, "doctor": true,
	"help": true, "version": true, "completion": true,
}

// ShouldProxy checks if the command should be proxied to vault/bao.
// Returns true and the args if we should proxy, false otherwise.
func (cli *CLI) ShouldProxy() (bool, []string) {
	if len(os.Args) < 2 {
		return false, nil
	}

	// Extract args, skipping patrol-specific flags
	args := extractVaultArgs()
	if len(args) == 0 {
		return false, nil
	}

	// Check if the first non-flag arg is a patrol command
	firstArg := args[0]
	if !isPatrolCommand(firstArg) && !strings.HasPrefix(firstArg, "-") {
		return true, args
	}

	return false, nil
}

// InitializeForProxy initializes CLI state for proxy operations.
func (cli *CLI) InitializeForProxy() error {
	// Parse flags manually for proxy commands
	for i, arg := range os.Args[1:] {
		if arg == "-p" || arg == "--profile" {
			if i+2 < len(os.Args) {
				cli.profileFlag = os.Args[i+2]
			}
		} else if strings.HasPrefix(arg, "-p=") {
			cli.profileFlag = strings.TrimPrefix(arg, "-p=")
		} else if strings.HasPrefix(arg, "--profile=") {
			cli.profileFlag = strings.TrimPrefix(arg, "--profile=")
		} else if arg == "-v" || arg == "--verbose" {
			cli.verboseFlag = true
		}
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
			if cli.verboseFlag {
				fmt.Fprintf(os.Stderr, "Warning: PATROL_PROFILE profile %q not found\n", envProfile)
			}
		}
	}

	return nil
}

// proxyCommand proxies a command to the Vault/OpenBao CLI.
func (cli *CLI) proxyCommand(ctx context.Context, args []string) error {
	// Get the current profile
	prof, err := cli.GetCurrentProfile()
	if err != nil {
		return err
	}

	// Get the stored token (optional - token is not required for proxy)
	token, _ := prof.GetToken(cli.Keyring) //nolint:errcheck // token is optional

	// Create the executor
	exec := proxy.NewExecutor(prof.Connection,
		proxy.WithToken(token),
		proxy.WithStdin(os.Stdin),
		proxy.WithStdout(os.Stdout),
		proxy.WithStderr(os.Stderr),
	)

	// Execute the command
	exitCode, err := exec.Execute(ctx, args, nil)
	if err != nil {
		return err
	}

	// Exit with the same code as the vault command
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// extractVaultArgs extracts arguments meant for the Vault CLI.
func extractVaultArgs() []string {
	args := os.Args[1:]

	// Skip patrol-specific flags
	vaultArgs := make([]string, 0, len(args))
	skipNext := false
	foundCommand := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Skip patrol flags
		if arg == "-p" || arg == "--profile" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "-p=") || strings.HasPrefix(arg, "--profile=") {
			continue
		}
		if arg == "-v" || arg == "--verbose" {
			continue
		}

		// Only check the FIRST non-flag argument to see if it's a patrol command
		// Subsequent args like "get" in "kv get" should not be checked
		if !foundCommand && !strings.HasPrefix(arg, "-") {
			if isPatrolCommand(arg) {
				return nil
			}
			foundCommand = true
		}

		vaultArgs = append(vaultArgs, arg)
	}

	return vaultArgs
}

// isPatrolCommand checks if the given command is a built-in Patrol command.
func isPatrolCommand(name string) bool {
	return patrolCommands[name]
}
