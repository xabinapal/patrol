package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/token"
)

// ProfileListOutput represents profile list output for JSON.
type ProfileListOutput struct {
	Current  string        `json:"current"`
	Profiles []ProfileInfo `json:"profiles"`
}

// ProfileInfo represents a single profile in the list.
type ProfileInfo struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
	LoggedIn  bool   `json:"logged_in"`
	Current   bool   `json:"current"`
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

	// Build profile list for JSON output
	profileList := ProfileListOutput{
		Current:  cli.Config.Current,
		Profiles: make([]ProfileInfo, 0, len(cli.Config.Connections)),
	}

	for _, conn := range cli.Config.Connections {
		connType := string(conn.Type)
		if connType == "" {
			connType = "vault"
		}

		loggedIn := false
		if _, err := cli.Keyring.Get(conn.KeyringKey()); err == nil {
			loggedIn = true
		}

		profileList.Profiles = append(profileList.Profiles, ProfileInfo{
			Name:      conn.Name,
			Address:   conn.Address,
			Type:      connType,
			Namespace: conn.Namespace,
			LoggedIn:  loggedIn,
			Current:   conn.Name == cli.Config.Current,
		})
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

		for _, conn := range cli.Config.Connections {
			current := ""
			if conn.Name == cli.Config.Current {
				current = "* "
			}

			loggedIn := "no"
			if _, err := cli.Keyring.Get(conn.KeyringKey()); err == nil {
				loggedIn = "yes"
			}

			connType := string(conn.Type)
			if connType == "" {
				connType = "vault"
			}

			fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\n", current, conn.Name, conn.Address, connType, loggedIn)
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

			// Check if token exists
			conn, err := cli.Config.GetConnection(name)
			if err != nil {
				return err
			}

			hasToken := false
			if _, err := cli.Keyring.Get(conn.KeyringKey()); err == nil {
				hasToken = true
			}

			if hasToken && !forceFlag {
				return fmt.Errorf("profile %q has a stored token. Use --force to remove anyway, or logout first", name)
			}

			// Remove token if exists
			if hasToken {
				if err := cli.Keyring.Delete(conn.KeyringKey()); err != nil {
					if !errors.Is(err, keyring.ErrTokenNotFound) {
						return fmt.Errorf("failed to remove token: %w", err)
					}
				}
			}

			// Remove profile
			if err := cli.Config.RemoveConnection(name); err != nil {
				return err
			}

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

			conn, err := cli.Config.GetConnection(name)
			if err != nil {
				// Connection not found, but continue with address from args
				conn = &config.Connection{Address: name}
			}
			fmt.Printf("Switched to profile %q (%s)\n", name, conn.Address)

			// Check if logged in
			if _, err := cli.Keyring.Get(conn.KeyringKey()); err != nil {
				if errors.Is(err, keyring.ErrTokenNotFound) {
					fmt.Println("Note: You are not logged in to this profile. Run 'patrol login' to authenticate.")
				}
			}

			return nil
		},
	}
}

// getProfileNames returns a list of all profile names for completion.
func (cli *CLI) getProfileNames() []string {
	if cli.Config == nil {
		return nil
	}
	names := make([]string, 0, len(cli.Config.Connections))
	for _, conn := range cli.Config.Connections {
		names = append(names, conn.Name)
	}
	return names
}

// maskToken masks a token for display, showing only first and last few characters.
func maskToken(t string) string {
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "****" + t[len(t)-4:]
}

// formatTokenDuration formats a duration in a human-readable way.
func formatTokenDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
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

			var conn *config.Connection
			var err error
			if len(args) > 0 {
				conn, err = cli.Config.GetConnection(args[0])
			} else {
				conn, err = cli.GetCurrentConnection()
			}
			if err != nil {
				return err
			}

			// Get the stored token
			storedToken, err := cli.Keyring.Get(conn.KeyringKey())
			if err != nil {
				if errors.Is(err, keyring.ErrTokenNotFound) {
					return fmt.Errorf("no token stored for profile %q; run 'patrol login' first", conn.Name)
				}
				return fmt.Errorf("failed to get token: %w", err)
			}

			if !proxy.BinaryExists(conn) {
				return fmt.Errorf("vault/openbao binary %q not found", conn.GetBinaryPath())
			}

			// Build renew args
			renewArgs := []string{"token", "renew", "-format=json"}
			if increment != "" {
				renewArgs = append(renewArgs, "-increment="+increment)
			}

			// Silent - we only need to parse JSON, not show output
			exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
			var captureBuf bytes.Buffer
			exitCode, err := exec.Execute(ctx, renewArgs, &captureBuf)
			if err != nil {
				return fmt.Errorf("failed to renew token: %w", err)
			}

			if exitCode != 0 {
				return fmt.Errorf("token renewal failed: %s", captureBuf.String())
			}

			// Parse response
			tok, err := token.ParseLoginResponse(captureBuf.Bytes())
			if err != nil {
				fmt.Println("Token renewed successfully (could not parse response)")
				return nil
			}

			// Update token in keyring if it changed
			if tok.ClientToken != "" && tok.ClientToken != storedToken {
				if err := cli.Keyring.Set(conn.KeyringKey(), tok.ClientToken); err != nil {
					return fmt.Errorf("failed to update token in keyring: %w", err)
				}
				fmt.Println("Token renewed and updated in keyring")
			} else {
				fmt.Println("Token renewed successfully")
			}

			// Show new TTL
			if tok.LeaseDuration > 0 {
				ttl := time.Duration(tok.LeaseDuration) * time.Second
				fmt.Printf("New TTL: %s\n", formatTokenDuration(ttl))
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

			var conn *config.Connection
			var err error
			if len(args) > 0 {
				conn, err = cli.Config.GetConnection(args[0])
			} else {
				conn, err = cli.GetCurrentConnection()
			}
			if err != nil {
				return err
			}

			// Get the stored token
			storedToken, err := cli.Keyring.Get(conn.KeyringKey())
			if err != nil {
				if errors.Is(err, keyring.ErrTokenNotFound) {
					fmt.Printf("No token stored for profile %q\n", conn.Name)
					return nil
				}
				return fmt.Errorf("failed to get token: %w", err)
			}

			// Revoke token with Vault (unless skipped)
			if !skipRevoke {
				if proxy.BinaryExists(conn) {
					// Silent - we only need to check if revoke succeeded
					exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
					var captureBuf bytes.Buffer
					exitCode, err := exec.Execute(ctx, []string{"token", "revoke", "-self"}, &captureBuf)
					if err != nil || exitCode != 0 {
						fmt.Printf("Warning: failed to revoke token with Vault: %s\n", captureBuf.String())
						fmt.Println("The token will be removed from the keyring anyway.")
					} else {
						fmt.Println("Token revoked with Vault")
					}
				} else {
					fmt.Println("Warning: Vault binary not found, cannot revoke token remotely")
				}
			}

			// Remove from keyring
			if err := cli.Keyring.Delete(conn.KeyringKey()); err != nil {
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
	Profile *ProfileStatusInfo `json:"profile,omitempty"`
	Token   *TokenStatusInfo   `json:"token,omitempty"`
	Server  *ServerStatusInfo  `json:"server,omitempty"`
}

// ProfileStatusInfo represents profile information in status output.
type ProfileStatusInfo struct {
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

// ServerStatusInfo represents server connectivity test results.
type ServerStatusInfo struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TokenStatusInfo represents token information in status output.
type TokenStatusInfo struct {
	Stored       bool     `json:"stored"`
	Valid        bool     `json:"valid"`
	DisplayName  string   `json:"display_name,omitempty"`
	TTL          int      `json:"ttl,omitempty"`
	TTLFormatted string   `json:"ttl_formatted,omitempty"`
	Renewable    bool     `json:"renewable,omitempty"`
	Policies     []string `json:"policies,omitempty"`
	AuthPath     string   `json:"auth_path,omitempty"`
	EntityID     string   `json:"entity_id,omitempty"`
	Error        string   `json:"error,omitempty"`
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

			var conn *config.Connection
			var name string
			if len(args) > 0 {
				var err error
				conn, err = cli.Config.GetConnection(args[0])
				if err != nil {
					return err
				}
				name = args[0]
			} else {
				var err error
				conn, err = cli.GetCurrentConnection()
				if err != nil {
					return err
				}
				name = cli.Config.Current
			}

			return cli.runProfileStatus(cmd.Context(), conn, name, format, showToken)
		},
	}

	cmd.Flags().BoolVar(&showToken, "show-token", false, "Show the full token (not masked)")

	return cmd
}

// runProfileStatus displays comprehensive status for a profile.
func (cli *CLI) runProfileStatus(ctx context.Context, conn *config.Connection, name string, format OutputFormat, showToken bool) error {
	output := NewOutputWriter(format)

	// Initialize status output for JSON
	status := &ProfileStatusOutput{}

	// Set comprehensive profile info
	status.Profile = &ProfileStatusInfo{
		Name:          conn.Name,
		Address:       conn.Address,
		Type:          string(conn.Type),
		Namespace:     conn.Namespace,
		Binary:        conn.GetBinaryPath(),
		BinaryPath:    conn.BinaryPath,
		TLSSkipVerify: conn.TLSSkipVerify,
		CACert:        conn.CACert,
		CAPath:        conn.CAPath,
		ClientCert:    conn.ClientCert,
		ClientKey:     conn.ClientKey,
		Active:        name == cli.Config.Current,
	}

	// Test server connectivity
	serverStatus := cli.testServerConnectivity(ctx, conn)
	status.Server = serverStatus

	// Get stored token and determine token state
	var storedToken string
	var lookupData *token.VaultTokenLookupData

	var tokenErr error
	storedToken, tokenErr = cli.Keyring.Get(conn.KeyringKey())
	if errors.Is(tokenErr, keyring.ErrTokenNotFound) {
		status.Token = &TokenStatusInfo{
			Stored: false,
			Valid:  false,
		}
	} else if tokenErr != nil {
		return fmt.Errorf("failed to retrieve token: %w", tokenErr)
	} else {
		// Token is stored
		status.Token = &TokenStatusInfo{
			Stored: true,
		}

		// Try to get token details from Vault
		if proxy.BinaryExists(conn) {
			exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
			var captureBuf bytes.Buffer
			exitCode, err := exec.Execute(ctx, []string{"token", "lookup", "-format=json"}, &captureBuf)
			if err != nil {
				return fmt.Errorf("failed to lookup token: %w", err)
			}

			if exitCode == 0 {
				lookupData, err = token.ParseLookupResponse(captureBuf.Bytes())
				if err == nil {
					status.Token.Valid = true
					status.Token.DisplayName = lookupData.DisplayName
					status.Token.TTL = lookupData.TTL
					if lookupData.TTL > 0 {
						status.Token.TTLFormatted = FormatDurationSeconds(lookupData.TTL)
					}
					status.Token.Renewable = lookupData.Renewable
					status.Token.Policies = lookupData.Policies
					status.Token.AuthPath = lookupData.Path
					status.Token.EntityID = lookupData.EntityID
				} else {
					status.Token.Valid = true
					status.Token.Error = "details unavailable"
				}
			} else {
				status.Token.Valid = false
				status.Token.Error = captureBuf.String()
			}
		} else {
			status.Token.Error = fmt.Sprintf("%s binary not found", conn.GetBinaryPath())
		}
	}

	// Single unified output function
	return output.Write(status, func() {
		cli.printProfileStatusHeader(conn, name)
		fmt.Println()
		cli.printServerConnectivity(status.Server)
		cli.printTokenInformation(status.Token, storedToken, lookupData, showToken, conn)
	})
}

// testServerConnectivity tests HTTP connectivity to the server.
func (cli *CLI) testServerConnectivity(ctx context.Context, conn *config.Connection) *ServerStatusInfo {
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status := &ServerStatusInfo{}

	// Test HTTP connectivity
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := conn.Address + "/v1/sys/health"

	req, err := http.NewRequestWithContext(testCtx, "GET", healthURL, nil)
	if err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("invalid URL: %v", err)
		return status
	}

	resp, err := client.Do(req)
	if err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("connection failed: %v", err)
		return status
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		status.Status = "healthy"
		status.Message = "initialized, unsealed, active"
	case 429, 472, 473:
		status.Status = "standby"
		status.Message = "standby node"
	case 501:
		status.Status = "uninitialized"
		status.Message = "server is not initialized"
	case 503:
		status.Status = "sealed"
		status.Message = "server is sealed"
	default:
		status.Status = "unknown"
		status.Message = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}

	return status
}

// printProfileStatusHeader prints the profile configuration header.
func (cli *CLI) printProfileStatusHeader(conn *config.Connection, name string) {
	fmt.Println("Profile Configuration:")
	fmt.Printf("  Name:            %s\n", conn.Name)
	fmt.Printf("  Address:         %s\n", conn.Address)
	fmt.Printf("  Type:            %s\n", conn.Type)
	if conn.BinaryPath != "" {
		fmt.Printf("  Binary Path:    %s\n", conn.BinaryPath)
	} else {
		fmt.Printf("  Binary:          %s\n", conn.GetBinaryPath())
	}
	if conn.Namespace != "" {
		fmt.Printf("  Namespace:       %s\n", conn.Namespace)
	}
	if conn.TLSSkipVerify {
		fmt.Printf("  TLS Skip Verify: true\n")
	}
	if conn.CACert != "" {
		fmt.Printf("  CA Cert:         %s\n", conn.CACert)
	}
	if conn.CAPath != "" {
		fmt.Printf("  CA Path:         %s\n", conn.CAPath)
	}
	if conn.ClientCert != "" {
		fmt.Printf("  Client Cert:     %s\n", conn.ClientCert)
	}
	if conn.ClientKey != "" {
		fmt.Printf("  Client Key:      %s\n", conn.ClientKey)
	}
	if name == cli.Config.Current {
		fmt.Printf("  Active:          yes\n")
	}
}

// printServerConnectivity prints server connectivity status.
func (cli *CLI) printServerConnectivity(server *ServerStatusInfo) {
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
func (cli *CLI) printTokenInformation(tokenInfo *TokenStatusInfo, storedToken string, lookupData *token.VaultTokenLookupData, showToken bool, conn *config.Connection) {
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
		fmt.Printf("  Token:           %s\n", maskToken(storedToken))
	}

	// Print token details if available
	if lookupData != nil && tokenInfo.Valid {
		if lookupData.Accessor != "" {
			fmt.Printf("  Accessor:        %s\n", lookupData.Accessor)
		}
		if lookupData.DisplayName != "" {
			fmt.Printf("  Display Name:    %s\n", lookupData.DisplayName)
		}
		if lookupData.TTL > 0 {
			ttl := time.Duration(lookupData.TTL) * time.Second
			expiry := time.Now().Add(ttl)
			fmt.Printf("  TTL:             %s (expires %s)\n", formatTokenDuration(ttl), expiry.Format(time.RFC3339))
		} else {
			fmt.Printf("  TTL:             ∞ (never expires)\n")
		}
		fmt.Printf("  Renewable:       %t\n", lookupData.Renewable)
		if len(lookupData.Policies) > 0 {
			fmt.Printf("  Policies:        %v\n", lookupData.Policies)
		}
		if lookupData.Path != "" {
			fmt.Printf("  Auth Path:       %s\n", lookupData.Path)
		}
		if lookupData.EntityID != "" {
			fmt.Printf("  Entity ID:       %s\n", lookupData.EntityID)
		}
		fmt.Printf("  Status:          valid\n")
		fmt.Println()

		// Renewal recommendation
		const TokenExpiryWarningSeconds = 300 // 5 minutes
		if lookupData.TTL > 0 && lookupData.TTL < TokenExpiryWarningSeconds && lookupData.Renewable {
			fmt.Println("Warning: Token will expire soon. Consider running 'patrol daemon' for auto-renewal.")
		}
	} else if tokenInfo.Error != "" {
		// Token stored but has an error
		if strings.Contains(tokenInfo.Error, "binary not found") {
			fmt.Printf("  Status:          stored (cannot verify - %s)\n", tokenInfo.Error)
		} else if tokenInfo.Valid {
			fmt.Printf("  Status:          valid (details unavailable)\n")
		} else {
			fmt.Printf("  Status:          invalid or expired\n")
			fmt.Printf("  Error:           %s\n", tokenInfo.Error)
			fmt.Println()
			fmt.Println("Your stored token may have expired. Run 'patrol login' to re-authenticate.")
		}
	}
}
