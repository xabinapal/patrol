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
	"github.com/xabinapal/patrol/internal/tokenstore"
	"github.com/xabinapal/patrol/internal/types"
	"github.com/xabinapal/patrol/internal/utils"
	"github.com/xabinapal/patrol/internal/vault"
)

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

// ProfileListOutput represents profile list output for JSON.
type ProfileListOutput struct {
	Current  string                  `json:"current"`
	Profiles []ProfileListOutputItem `json:"profiles"`
}

// ProfileListOutputItem represents a single profile in the list output.
type ProfileListOutputItem struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool   `json:"current"`
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
	ctx := context.Background()
	pm := profile.NewProfileManager(ctx, cli.Config)
	profiles := pm.List()

	// Convert to output format
	profileItems := make([]ProfileListOutputItem, 0, len(profiles))
	for _, prof := range profiles {
		profileItems = append(profileItems, ProfileListOutputItem{
			Name:      prof.Name,
			Address:   prof.Address,
			Type:      prof.Type,
			Namespace: prof.Namespace,
			Current:   prof.Name == cli.Config.Current,
		})
	}

	profileList := ProfileListOutput{
		Current:  cli.Config.Current,
		Profiles: profileItems,
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
			if prof.Name == cli.Config.Current {
				current = "* "
			}

			// Check keyring to see if logged in (CLI layer responsibility)
			loggedInStr := "no"
			tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
			if tm.HasToken(prof) {
				loggedInStr = "yes"
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

			ctx := context.Background()
			pm := profile.NewProfileManager(ctx, cli.Config)
			prof, err := pm.Get(name)
			if err != nil {
				return err
			}

			tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
			hasToken := tm.HasToken(prof)

			if hasToken && !forceFlag {
				return fmt.Errorf("profile %q has a stored token. Use --force to remove anyway, or logout first", name)
			}

			// Remove token if exists
			if hasToken {
				if err := tm.Delete(prof); err != nil {
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

			conn, err := cli.Config.GetConnection(name)
			if err != nil {
				return err
			}

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

			ctx := context.Background()
			pm := profile.NewProfileManager(ctx, cli.Config)
			prof, err := pm.Get(name)
			if err != nil {
				// Profile not found, but continue with address from args
				fmt.Printf("Switched to profile %q\n", name)
				return nil
			}
			fmt.Printf("Switched to profile %q (%s)\n", name, prof.Address)

			// Check if logged in
			tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
			if !tm.HasToken(prof) {
				fmt.Println("Note: You are not logged in to this profile. Run 'patrol login' to authenticate.")
			}

			return nil
		},
	}
}

// getProfileNames returns a list of all profile names for completion.
func (cli *CLI) getProfileNames() []string {
	ctx := context.Background()
	pm := profile.NewProfileManager(ctx, cli.Config)
	profiles := pm.List()
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

			pm := profile.NewProfileManager(ctx, cli.Config)
			var prof *types.Profile
			var err error
			if len(args) > 0 {
				prof, err = pm.Get(args[0])
				if err != nil {
					return err
				}
			} else {
				prof, err = pm.GetCurrent()
				if err != nil {
					return err
				}
			}

			// Get current token for comparison
			tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
			storedToken, err := tm.Get(prof)
			if err != nil {
				return fmt.Errorf("no token stored for profile %q; run 'patrol login' first", prof.Name)
			}

			// Renew token
			tok, err := tm.Renew(prof, increment)
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

			pm := profile.NewProfileManager(ctx, cli.Config)
			var prof *types.Profile
			var err error
			if len(args) > 0 {
				prof, err = pm.Get(args[0])
				if err != nil {
					return err
				}
			} else {
				prof, err = pm.GetCurrent()
				if err != nil {
					return err
				}
			}

			// Check if token exists
			tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
			if !tm.HasToken(prof) {
				fmt.Printf("No token stored for profile %q\n", prof.Name)
				return nil
			}

			// Revoke token with Vault (unless skipped)
			if !skipRevoke {
				if err := tm.Revoke(prof); err != nil {
					fmt.Printf("Warning: failed to revoke token with Vault: %v\n", err)
					fmt.Println("The token will be removed from the keyring anyway.")
				} else {
					fmt.Println("Token revoked with Vault")
				}
			}

			// Remove from keyring
			if err := tm.Delete(prof); err != nil {
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
	Profile *ProfileStatusOutputProfileItem `json:"profile,omitempty"`
	Server  *ProfileStatusOutputServerItem  `json:"server,omitempty"`
	Token   *ProfileStatusOutputTokenItem   `json:"token,omitempty"`
}

// ProfileStatusOutputProfileItem represents a profile in status output.
type ProfileStatusOutputProfileItem struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Type          string `json:"type"`
	Namespace     string `json:"namespace,omitempty"`
	Binary        string `json:"binary"`
	BinaryPath    string `json:"binary_path,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	CACert        string `json:"ca_cert,omitempty"`
	CAPath        string `json:"ca_path,omitempty"`
	ClientCert    string `json:"client_cert,omitempty"`
	ClientKey     string `json:"client_key,omitempty"`
	Active        bool   `json:"active"`
}

// ProfileStatusOutputServerItem represents server health status output for JSON.
type ProfileStatusOutputServerItem struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ProfileStatusOutputTokenItem represents token status output for JSON.
type ProfileStatusOutputTokenItem struct {
	Token     string    `json:"token"`
	TTL       int       `json:"ttl"`
	Renewable bool      `json:"renewable"`
	Valid     bool      `json:"valid"`
	ExpiresAt time.Time `json:"expires_at"`
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

			ctx := cmd.Context()
			pm := profile.NewProfileManager(ctx, cli.Config)
			var prof *types.Profile
			if len(args) > 0 {
				var err error
				prof, err = pm.Get(args[0])
				if err != nil {
					return err
				}
			} else {
				var err error
				prof, err = pm.GetCurrent()
				if err != nil {
					return err
				}
			}

			return cli.runProfileStatus(ctx, prof, format, showToken)
		},
	}

	cmd.Flags().BoolVar(&showToken, "show-token", false, "Show the full token (not masked)")

	return cmd
}

// runProfileStatus displays comprehensive status for a profile.
func (cli *CLI) runProfileStatus(ctx context.Context, prof *types.Profile, format OutputFormat, showToken bool) error {
	output := NewOutputWriter(format)

	// Initialize status output for JSON
	status := &ProfileStatusOutput{}

	// Convert to output format
	status.Profile = &ProfileStatusOutputProfileItem{
		Name:          prof.Name,
		Address:       prof.Address,
		Type:          prof.Type,
		Namespace:     prof.Namespace,
		Binary:        prof.GetBinaryPath(),
		BinaryPath:    prof.BinaryPath,
		TLSSkipVerify: prof.TLSSkipVerify,
		CACert:        prof.CACert,
		CAPath:        prof.CAPath,
		ClientCert:    prof.ClientCert,
		ClientKey:     prof.ClientKey,
		Active:        prof.Name == cli.Config.Current,
	}

	// Test server connectivity
	healthExecutor := vault.NewHealthExecutor()
	serverStatus := healthExecutor.CheckHealth(ctx, prof)
	if serverStatus != nil {
		status.Server = &ProfileStatusOutputServerItem{
			Status:  serverStatus.Status,
			Message: serverStatus.Message,
		}
	}

	// Get token status
	tm := token.NewTokenManager(ctx, cli.Store, vault.NewTokenExecutor())
	tok, err := tm.Lookup(prof)

	// Get stored token string if we have a token (even if invalid)
	storedToken := ""
	var lookupErr error
	if err == nil || !errors.Is(err, tokenstore.ErrTokenNotFound) {
		// Token exists (may be invalid), try to get it for display
		var tokenErr error
		storedToken, tokenErr = tm.Get(prof)
		if tokenErr != nil {
			storedToken = ""
		}
		if err != nil {
			lookupErr = err
		}
	}

	// Create token output - use lookup result if available, otherwise use stored token if present
	var tokenOutput *ProfileStatusOutputTokenItem
	if tok != nil {
		tokenOutput = &ProfileStatusOutputTokenItem{
			Token:     tok.ClientToken,
			TTL:       tok.LeaseDuration,
			Renewable: tok.Renewable,
			Valid:     true,
			ExpiresAt: tok.ExpiresAt,
		}
	} else if storedToken != "" {
		// We have a stored token but lookup failed (invalid/expired token)
		tokenOutput = &ProfileStatusOutputTokenItem{
			Token:     storedToken,
			TTL:       0,
			Renewable: false,
			Valid:     false,
		}
	}
	status.Token = tokenOutput

	// Single unified output function
	return output.Write(status, func() {
		cli.printProfileStatusHeader(status.Profile)
		cli.printServerConnectivity(status.Server)
		cli.printTokenInformation(status.Token, err, lookupErr, storedToken, showToken)
	})
}

// printProfileStatusHeader prints the profile configuration header.
func (cli *CLI) printProfileStatusHeader(prof *ProfileStatusOutputProfileItem) {
	if prof == nil {
		return
	}
	fmt.Println("Profile Configuration:")
	fmt.Printf("  Name:            %s\n", prof.Name)
	fmt.Printf("  Address:         %s\n", prof.Address)
	fmt.Printf("  Type:            %s\n", prof.Type)
	if prof.BinaryPath != "" {
		fmt.Printf("  Binary Path:    %s\n", prof.BinaryPath)
	} else {
		fmt.Printf("  Binary:          %s\n", prof.Binary)
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
	fmt.Printf("  Active:          %t\n", prof.Active)
	fmt.Println()
}

// printServerConnectivity prints server connectivity status.
func (cli *CLI) printServerConnectivity(server *ProfileStatusOutputServerItem) {
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
func (cli *CLI) printTokenInformation(tok *ProfileStatusOutputTokenItem, err error, lookupErr error, storedToken string, showToken bool) {
	fmt.Println("Token Information:")

	// Check if token is not stored
	if err != nil && errors.Is(err, tokenstore.ErrTokenNotFound) {
		fmt.Printf("  Status:          not logged in\n")
		fmt.Println()
		fmt.Println("Run 'patrol login' to authenticate.")
		return
	}

	// Print token (masked or full)
	if showToken {
		fmt.Printf("  Token:           %s\n", storedToken)
	} else {
		fmt.Printf("  Token:           %s\n", utils.MaskToken(storedToken))
	}

	// Print token details if available
	if tok != nil && err == nil {
		if tok.TTL > 0 {
			ttl := time.Duration(tok.TTL) * time.Second
			if !tok.ExpiresAt.IsZero() {
				fmt.Printf("  TTL:             %s (expires %s)\n", utils.FormatDuration(ttl), tok.ExpiresAt.Format(time.RFC3339))
			} else {
				fmt.Printf("  TTL:             %s\n", utils.FormatDuration(ttl))
			}
		} else {
			fmt.Printf("  TTL:             ∞ (never expires)\n")
		}
		fmt.Printf("  Renewable:       %t\n", tok.Renewable)
		fmt.Printf("  Valid:           %t\n", tok.Valid)
		fmt.Println()

		// Renewal recommendation
		const TokenExpiryWarningSeconds = 300 // 5 minutes
		if tok.TTL > 0 && tok.TTL < TokenExpiryWarningSeconds && tok.Renewable {
			fmt.Println("Warning: Token will expire soon. Consider running 'patrol daemon' for auto-renewal.")
		}
	} else if lookupErr != nil {
		// Token stored but has an error
		errorMsg := lookupErr.Error()
		if strings.Contains(errorMsg, "binary not found") {
			fmt.Printf("  Status:          stored (cannot verify - %s)\n", errorMsg)
		} else {
			fmt.Printf("  Status:          invalid or expired\n")
			fmt.Printf("  Error:           %s\n", errorMsg)
			fmt.Println()
			fmt.Println("Your stored token may have expired. Run 'patrol login' to re-authenticate.")
		}
	}
}
