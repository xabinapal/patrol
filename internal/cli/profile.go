package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/profile"
	"github.com/xabinapal/patrol/internal/token"
	"github.com/xabinapal/patrol/internal/utils"
	"github.com/xabinapal/patrol/internal/vault"
)

// ProfileListOutput represents profile list output for JSON.
type ProfileListOutput struct {
	Current  string         `json:"current"`
	Profiles []profile.Info `json:"profiles"`
}

// newProfileCmd creates the profile command group.
func (cli *CLI) newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profile",
		Aliases: []string{"profiles", "config"},
		Short:   "Manage Vault/OpenBao connection profiles",
		Long: `Manage connection profiles for different Vault/OpenBao servers.

Profiles allow you to easily switch between multiple Vault servers
without having to remember addresses or reconfigure environment variables.

Examples:
  # List all profiles
  patrol profile list

  # Add a new profile
  patrol profile add prod --address=https://vault.example.com:8200

  # Remove a profile
  patrol profile remove old-profile

  # Show profile status
  patrol profile status prod`,
	}

	cmd.AddCommand(
		cli.newProfileListCmd(),
		cli.newProfileAddCmd(),
		cli.newProfileRemoveCmd(),
		cli.newProfileEditCmd(),
		cli.newProfileRenewCmd(),
		cli.newProfileRevokeCmd(),
		cli.newProfileStatusCmd(),
		cli.newProfileUseCmd(),
	)

	return cmd
}

// newProfileListCmd creates the profile list command.
func (cli *CLI) newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}
			return cli.runProfileList(format)
		},
	}
}

// runProfileList displays all configured profiles.
func (cli *CLI) runProfileList(format OutputFormat) error {
	output := NewOutputWriter(format)

	// Get profiles from config
	profiles := profile.List(cli.Config)

	profileList := ProfileListOutput{
		Current:  cli.Config.Current,
		Profiles: profiles,
	}

	if len(cli.Config.Connections) == 0 {
		return output.Write(profileList, func() {
			fmt.Println("No profiles configured.")
			fmt.Println()
			fmt.Println("Add a profile with: patrol profile add <name> --address=<vault-address>")
		})
	}

	return output.Write(profileList, func() {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tADDRESS\tTYPE\tLOGGED IN")

		for _, prof := range profiles {
			current := ""
			if prof.Current {
				current = "* "
			}

			// Check keyring to see if logged in (CLI layer responsibility)
			loggedInStr := "no"
			profProfile, err := profile.Get(cli.Config, prof.Name)
			if err == nil {
				if profProfile.HasToken(cli.Keyring) {
					loggedInStr = "yes"
				}
			}

			connType := prof.Type
			if connType == "" {
				connType = "vault"
			}

			fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\n", current, prof.Name, prof.Address, connType, loggedInStr)
		}

		// #nosec G104 - Flush error on stdout; if write fails, user will see incomplete output
		_ = w.Flush()

		if cli.Config.Current != "" {
			fmt.Printf("\n* = current profile (%s)\n", cli.Config.Current)
		}
	})
}

// newProfileAddCmd creates the profile add command.
func (cli *CLI) newProfileAddCmd() *cobra.Command {
	var (
		address       string
		binaryType    string
		binaryPath    string
		namespace     string
		tlsSkipVerify bool
		caCert        string
		caPath        string
		clientCert    string
		clientKey     string
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new connection profile",
		Long: `Add a new Vault/OpenBao connection profile.

Examples:
  # Add a basic Vault profile
  patrol profile add prod --address=https://vault.example.com:8200

  # Add an OpenBao profile
  patrol profile add openbao-dev --address=https://openbao.example.com:8200 --type=openbao

  # Add with custom binary path
  patrol profile add custom --address=https://vault.local:8200 --binary=/opt/vault/bin/vault

  # Add with namespace (Vault Enterprise)
  patrol profile add team1 --address=https://vault.example.com:8200 --namespace=admin/team1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if address == "" {
				return errors.New("--address is required")
			}

			bt := config.BinaryTypeVault
			if binaryType == "openbao" || binaryType == "bao" {
				bt = config.BinaryTypeOpenBao
			}

			conn := config.Connection{
				Name:          name,
				Address:       address,
				Type:          bt,
				BinaryPath:    binaryPath,
				Namespace:     namespace,
				TLSSkipVerify: tlsSkipVerify,
				CACert:        caCert,
				CAPath:        caPath,
				ClientCert:    clientCert,
				ClientKey:     clientKey,
			}

			if err := cli.Config.AddConnection(conn); err != nil {
				return err
			}

			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Added profile %q\n", name)
			if cli.Config.Current == name {
				fmt.Println("This profile is now active.")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&address, "address", "a", "", "Vault/OpenBao server address (required)")
	cmd.Flags().StringVarP(&binaryType, "type", "t", "vault", "Binary type: vault or openbao")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Custom path to vault/bao binary")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Vault namespace (Enterprise)")
	cmd.Flags().BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification")
	cmd.Flags().StringVar(&caCert, "ca-cert", "", "Path to CA certificate file")
	cmd.Flags().StringVar(&caPath, "ca-path", "", "Path to directory of CA certificates")
	cmd.Flags().StringVar(&clientCert, "client-cert", "", "Path to client certificate file")
	cmd.Flags().StringVar(&clientKey, "client-key", "", "Path to client key file")

	if err := cmd.MarkFlagRequired("address"); err != nil {
		return nil
	}

	return cmd
}

// newProfileRemoveCmd creates the profile remove command.
func (cli *CLI) newProfileRemoveCmd() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a connection profile",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			prof, err := profile.Get(cli.Config, name)
			if err != nil {
				return err
			}

			hasToken := prof.HasToken(cli.Keyring)

			if hasToken && !forceFlag {
				return fmt.Errorf("profile %q has a stored token. Use --force to remove anyway, or logout first", name)
			}

			// Remove token if exists
			if hasToken {
				if err := prof.DeleteToken(cli.Keyring); err != nil {
					return fmt.Errorf("failed to remove token: %w", err)
				}
			}

			// Remove profile
			if err := cli.Config.RemoveConnection(name); err != nil {
				return err
			}

			// Save the updated configuration
			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Removed profile %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force removal even if token exists")

	return cmd
}

// newProfileEditCmd creates the profile edit command.
func (cli *CLI) newProfileEditCmd() *cobra.Command {
	var (
		address       string
		binaryType    string
		binaryPath    string
		namespace     string
		tlsSkipVerify *bool
		caCert        string
		caPath        string
		clientCert    string
		clientKey     string
	)

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an existing profile",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			prof, err := profile.Get(cli.Config, name)
			if err != nil {
				return err
			}
			conn := prof.Connection

			// Update fields that were specified
			if address != "" {
				conn.Address = address
			}
			if binaryType != "" {
				if binaryType == "openbao" || binaryType == "bao" {
					conn.Type = config.BinaryTypeOpenBao
				} else {
					conn.Type = config.BinaryTypeVault
				}
			}
			if cmd.Flags().Changed("binary") {
				conn.BinaryPath = binaryPath
			}
			if cmd.Flags().Changed("namespace") {
				conn.Namespace = namespace
			}
			if tlsSkipVerify != nil {
				conn.TLSSkipVerify = *tlsSkipVerify
			}
			if cmd.Flags().Changed("ca-cert") {
				conn.CACert = caCert
			}
			if cmd.Flags().Changed("ca-path") {
				conn.CAPath = caPath
			}
			if cmd.Flags().Changed("client-cert") {
				conn.ClientCert = clientCert
			}
			if cmd.Flags().Changed("client-key") {
				conn.ClientKey = clientKey
			}

			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Updated profile %q\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&address, "address", "a", "", "Vault/OpenBao server address")
	cmd.Flags().StringVarP(&binaryType, "type", "t", "", "Binary type: vault or openbao")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Custom path to vault/bao binary")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Vault namespace (Enterprise)")

	var skipVerify bool
	cmd.Flags().BoolVar(&skipVerify, "tls-skip-verify", false, "Skip TLS certificate verification")
	tlsSkipVerify = &skipVerify

	cmd.Flags().StringVar(&caCert, "ca-cert", "", "Path to CA certificate file")
	cmd.Flags().StringVar(&caPath, "ca-path", "", "Path to directory of CA certificates")
	cmd.Flags().StringVar(&clientCert, "client-cert", "", "Path to client certificate file")
	cmd.Flags().StringVar(&clientKey, "client-key", "", "Path to client key file")

	return cmd
}

// newProfileUseCmd creates the profile use command for switching profiles.
func (cli *CLI) newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <profile>",
		Aliases: []string{"switch"},
		Short:   "Switch to a different profile",
		Long: `Switch the active Vault/OpenBao profile.

The active profile determines which Vault server commands are sent to
and which stored token is used for authentication.

Examples:
  # Switch to production profile
  patrol profile use prod

  # Switch to development profile
  patrol profile use dev`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := cli.Config.SetCurrent(name); err != nil {
				return err
			}
			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			prof, err := profile.Get(cli.Config, name)
			if err != nil {
				// Profile not found, but continue with address from args
				fmt.Printf("Switched to profile %q\n", name)
				return nil
			}
			fmt.Printf("Switched to profile %q (%s)\n", name, prof.Address)

			// Check if logged in
			if !prof.HasToken(cli.Keyring) {
				fmt.Println("Note: You are not logged in to this profile. Run 'patrol login' to authenticate.")
			}

			return nil
		},
	}
}

// getProfileNames returns a list of all profile names for completion.
func (cli *CLI) getProfileNames() []string {
	profiles := profile.List(cli.Config)
	if profiles == nil {
		return nil
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return names
}

// newProfileRenewCmd creates the profile renew command.
func (cli *CLI) newProfileRenewCmd() *cobra.Command {
	var increment string

	cmd := &cobra.Command{
		Use:   "renew [name]",
		Short: "Manually renew the stored token",
		Long: `Manually trigger renewal of the token stored for a profile.

This is useful when you want to extend your token's TTL immediately
without waiting for the daemon to do it automatically.

If no profile name is given, renews the token for the current profile.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			var prof *profile.Profile
			if len(args) > 0 {
				var err error
				prof, err = profile.Get(cli.Config, args[0])
				if err != nil {
					return err
				}
			} else {
				var err error
				prof, err = profile.GetCurrent(cli.Config)
				if err != nil {
					return err
				}
			}

			// Get current token for comparison
			storedToken, err := prof.GetToken(cli.Keyring)
			if err != nil {
				return fmt.Errorf("no token stored for profile %q; run 'patrol login' first", prof.Name)
			}

			// Renew token
			tok, err := prof.RenewToken(ctx, cli.Keyring, increment)
			if err != nil {
				return err
			}

			if tok.ClientToken != storedToken {
				fmt.Println("Token renewed and updated in keyring")
			} else {
				fmt.Println("Token renewed successfully")
			}

			// Show new TTL
			if tok.LeaseDuration > 0 {
				ttl := time.Duration(tok.LeaseDuration) * time.Second
				fmt.Printf("New TTL: %s\n", utils.FormatDuration(ttl))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&increment, "increment", "", "Request a specific TTL increment (e.g., '1h', '24h')")

	return cmd
}

// newProfileRevokeCmd creates the profile revoke command.
func (cli *CLI) newProfileRevokeCmd() *cobra.Command {
	var skipRevoke bool

	cmd := &cobra.Command{
		Use:   "revoke [name]",
		Short: "Revoke and clear the stored token",
		Long: `Revoke the token with Vault and remove it from the keyring.

By default, this command will:
1. Revoke the token with the Vault server
2. Remove the token from the keyring

Use --skip-revoke to only remove from keyring without revoking.

If no profile name is given, revokes the token for the current profile.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			var prof *profile.Profile
			if len(args) > 0 {
				var err error
				prof, err = profile.Get(cli.Config, args[0])
				if err != nil {
					return err
				}
			} else {
				var err error
				prof, err = profile.GetCurrent(cli.Config)
				if err != nil {
					return err
				}
			}

			// Check if token exists
			if !prof.HasToken(cli.Keyring) {
				fmt.Printf("No token stored for profile %q\n", prof.Name)
				return nil
			}

			// Revoke token with Vault (unless skipped)
			if !skipRevoke {
				if err := prof.RevokeToken(ctx, cli.Keyring); err != nil {
					fmt.Printf("Warning: failed to revoke token with Vault: %v\n", err)
					fmt.Println("The token will be removed from the keyring anyway.")
				} else {
					fmt.Println("Token revoked with Vault")
				}
			}

			// Remove from keyring
			if err := prof.DeleteToken(cli.Keyring); err != nil {
				return fmt.Errorf("failed to remove token from keyring: %w", err)
			}

			fmt.Println("Token removed from keyring")
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipRevoke, "skip-revoke", false, "Only remove from keyring without revoking")

	return cmd
}

// ProfileStatusOutput represents profile status output for JSON.
type ProfileStatusOutput struct {
	Profile *profile.Status     `json:"profile,omitempty"`
	Token   *token.Status       `json:"token,omitempty"`
	Server  *vault.HealthStatus `json:"server,omitempty"`
}

// newProfileStatusCmd creates the profile status command.
func (cli *CLI) newProfileStatusCmd() *cobra.Command {
	var showToken bool

	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show comprehensive profile status and information",
		Long: `Display comprehensive status information for a profile, including:
- Profile configuration details
- Server connectivity and health
- Token validity and metadata

By default, the full token is masked. Use --show-token to display it.

If no profile name is given, shows status for the current profile.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			var prof *profile.Profile
			if len(args) > 0 {
				var err error
				prof, err = profile.Get(cli.Config, args[0])
				if err != nil {
					return err
				}
			} else {
				var err error
				prof, err = profile.GetCurrent(cli.Config)
				if err != nil {
					return err
				}
			}

			return cli.runProfileStatus(cmd.Context(), prof, format, showToken)
		},
	}

	cmd.Flags().BoolVar(&showToken, "show-token", false, "Show the full token (not masked)")

	return cmd
}

// runProfileStatus displays comprehensive status for a profile.
func (cli *CLI) runProfileStatus(ctx context.Context, prof *profile.Profile, format OutputFormat, showToken bool) error {
	output := NewOutputWriter(format)

	// Initialize status output for JSON
	status := &ProfileStatusOutput{}

	// Get profile status
	profileStatus, err := profile.GetStatus(cli.Config, prof.Name)
	if err != nil {
		return err
	}
	status.Profile = profileStatus

	// Test server connectivity
	serverStatus := vault.CheckHealth(ctx, prof.Connection)
	status.Server = serverStatus

	// Get token status
	tokenStatus, storedToken, err := prof.GetTokenStatus(ctx, cli.Keyring)
	if err != nil {
		return err
	}
	status.Token = tokenStatus

	// Get lookup data for display if token is valid (optional - we already have status)
	var lookupData *token.VaultTokenLookupData
	if tokenStatus.Valid && storedToken != "" {
		// Try to get full lookup data, but don't fail if it doesn't work
		if data, lookupErr := prof.LookupToken(ctx, cli.Keyring); lookupErr == nil {
			lookupData = data
		}
	}

	// Single unified output function
	return output.Write(status, func() {
		cli.printProfileStatusHeader(prof)
		fmt.Println()
		cli.printServerConnectivity(status.Server)
		cli.printTokenInformation(status.Token, storedToken, lookupData, showToken, prof.Connection)
	})
}

// printProfileStatusHeader prints the profile configuration header.
func (cli *CLI) printProfileStatusHeader(prof *profile.Profile) {
	fmt.Println("Profile Configuration:")
	fmt.Printf("  Name:            %s\n", prof.Name)
	fmt.Printf("  Address:         %s\n", prof.Address)
	fmt.Printf("  Type:            %s\n", prof.Type)
	if prof.BinaryPath != "" {
		fmt.Printf("  Binary Path:    %s\n", prof.BinaryPath)
	} else {
		fmt.Printf("  Binary:          %s\n", prof.GetBinaryPath())
	}
	if prof.Namespace != "" {
		fmt.Printf("  Namespace:       %s\n", prof.Namespace)
	}
	if prof.TLSSkipVerify {
		fmt.Printf("  TLS Skip Verify: true\n")
	}
	if prof.CACert != "" {
		fmt.Printf("  CA Cert:         %s\n", prof.CACert)
	}
	if prof.CAPath != "" {
		fmt.Printf("  CA Path:         %s\n", prof.CAPath)
	}
	if prof.ClientCert != "" {
		fmt.Printf("  Client Cert:     %s\n", prof.ClientCert)
	}
	if prof.ClientKey != "" {
		fmt.Printf("  Client Key:      %s\n", prof.ClientKey)
	}
	if prof.Name == cli.Config.Current {
		fmt.Printf("  Active:          yes\n")
	}
}

// printServerConnectivity prints server connectivity status.
func (cli *CLI) printServerConnectivity(server *vault.HealthStatus) {
	if server == nil {
		return
	}

	fmt.Println("Server Connectivity:")
	switch server.Status {
	case "healthy":
		fmt.Println("  [OK] Server is healthy (initialized, unsealed, active)")
	case "standby":
		fmt.Println("  [OK] Server is available (standby node)")
	case "uninitialized":
		fmt.Println("  [!!] Server is not initialized")
	case "sealed":
		fmt.Println("  [!!] Server is sealed")
	case "error":
		fmt.Printf("  [XX] Connection failed: %s\n", server.Message)
	default:
		fmt.Printf("  [!!] Unexpected status: %s\n", server.Message)
	}
	fmt.Println()
}

// printTokenInformation prints token information.
func (cli *CLI) printTokenInformation(tokenInfo *token.Status, storedToken string, lookupData *token.VaultTokenLookupData, showToken bool, conn *config.Connection) {
	fmt.Println("Token Information:")

	if tokenInfo == nil || !tokenInfo.Stored {
		fmt.Printf("  Status:          not logged in\n")
		fmt.Println()
		fmt.Println("Run 'patrol login' to authenticate.")
		return
	}

	// Print token (masked or full)
	if showToken {
		fmt.Printf("  Token:           %s\n", storedToken)
	} else {
		fmt.Printf("  Token:           %s\n", utils.Mask(storedToken))
	}

	// Print token details if available
	if tokenInfo.Valid {
		if tokenInfo.Accessor != "" {
			fmt.Printf("  Accessor:        %s\n", tokenInfo.Accessor)
		}
		if tokenInfo.DisplayName != "" {
			fmt.Printf("  Display Name:    %s\n", tokenInfo.DisplayName)
		}
		if tokenInfo.TTL > 0 {
			ttl := time.Duration(tokenInfo.TTL) * time.Second
			expiry := time.Now().Add(ttl)
			fmt.Printf("  TTL:             %s (expires %s)\n", utils.FormatDuration(ttl), expiry.Format(time.RFC3339))
		} else {
			fmt.Printf("  TTL:             ∞ (never expires)\n")
		}
		fmt.Printf("  Renewable:       %t\n", tokenInfo.Renewable)
		if len(tokenInfo.Policies) > 0 {
			fmt.Printf("  Policies:        %v\n", tokenInfo.Policies)
		}
		if tokenInfo.AuthPath != "" {
			fmt.Printf("  Auth Path:       %s\n", tokenInfo.AuthPath)
		}
		if tokenInfo.EntityID != "" {
			fmt.Printf("  Entity ID:       %s\n", tokenInfo.EntityID)
		}
		fmt.Printf("  Status:          valid\n")
		fmt.Println()

		// Renewal recommendation
		const TokenExpiryWarningSeconds = 300 // 5 minutes
		if tokenInfo.TTL > 0 && tokenInfo.TTL < TokenExpiryWarningSeconds && tokenInfo.Renewable {
			fmt.Println("Warning: Token will expire soon. Consider running 'patrol daemon' for auto-renewal.")
		}
	} else if tokenInfo.Error != "" {
		// Token stored but has an error
		if strings.Contains(tokenInfo.Error, "binary not found") {
			fmt.Printf("  Status:          stored (cannot verify - %s)\n", tokenInfo.Error)
		} else {
			fmt.Printf("  Status:          invalid or expired\n")
			fmt.Printf("  Error:           %s\n", tokenInfo.Error)
			fmt.Println()
			fmt.Println("Your stored token may have expired. Run 'patrol login' to re-authenticate.")
		}
	}
}
