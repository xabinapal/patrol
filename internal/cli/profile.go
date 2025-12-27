package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
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

// ProfileShowOutput represents profile show output for JSON.
type ProfileShowOutput struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Type          string `json:"type"`
	BinaryPath    string `json:"binary_path,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	CACert        string `json:"ca_cert,omitempty"`
	CAPath        string `json:"ca_path,omitempty"`
	ClientCert    string `json:"client_cert,omitempty"`
	ClientKey     string `json:"client_key,omitempty"`
	LoggedIn      bool   `json:"logged_in"`
	Active        bool   `json:"active"`
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

  # Show profile details
  patrol profile show prod`,
	}

	cmd.AddCommand(
		cli.newProfileListCmd(),
		cli.newProfileAddCmd(),
		cli.newProfileRemoveCmd(),
		cli.newProfileShowCmd(),
		cli.newProfileEditCmd(),
		cli.newProfileExportCmd(),
		cli.newProfileImportCmd(),
		cli.newProfileTestCmd(),
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

// newProfileShowCmd creates the profile show command.
func (cli *CLI) newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show details of a profile",
		Args:  cobra.MaximumNArgs(1),
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

			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				name = cli.Config.Current
			}

			if name == "" {
				return errors.New("no profile specified and no current profile set")
			}

			conn, err := cli.Config.GetConnection(name)
			if err != nil {
				return err
			}

			loggedIn := false
			if _, err := cli.Keyring.Get(conn.KeyringKey()); err == nil {
				loggedIn = true
			}

			profileOutput := ProfileShowOutput{
				Name:          conn.Name,
				Address:       conn.Address,
				Type:          string(conn.Type),
				BinaryPath:    conn.BinaryPath,
				Namespace:     conn.Namespace,
				TLSSkipVerify: conn.TLSSkipVerify,
				CACert:        conn.CACert,
				CAPath:        conn.CAPath,
				ClientCert:    conn.ClientCert,
				ClientKey:     conn.ClientKey,
				LoggedIn:      loggedIn,
				Active:        name == cli.Config.Current,
			}

			output := NewOutputWriter(format)
			return output.Write(profileOutput, func() {
				fmt.Printf("Name:           %s\n", conn.Name)
				fmt.Printf("Address:        %s\n", conn.Address)
				fmt.Printf("Type:           %s\n", conn.Type)
				if conn.BinaryPath != "" {
					fmt.Printf("Binary Path:    %s\n", conn.BinaryPath)
				}
				if conn.Namespace != "" {
					fmt.Printf("Namespace:      %s\n", conn.Namespace)
				}
				if conn.TLSSkipVerify {
					fmt.Printf("TLS Skip Verify: true\n")
				}
				if conn.CACert != "" {
					fmt.Printf("CA Cert:        %s\n", conn.CACert)
				}
				if conn.CAPath != "" {
					fmt.Printf("CA Path:        %s\n", conn.CAPath)
				}
				if conn.ClientCert != "" {
					fmt.Printf("Client Cert:    %s\n", conn.ClientCert)
				}
				if conn.ClientKey != "" {
					fmt.Printf("Client Key:     %s\n", conn.ClientKey)
				}

				if loggedIn {
					fmt.Printf("Logged In:      yes\n")
				} else {
					fmt.Printf("Logged In:      no\n")
				}

				if name == cli.Config.Current {
					fmt.Printf("Active:         yes\n")
				}
			})
		},
	}
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

// newUseCmd creates the use command for switching profiles.
func (cli *CLI) newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <profile>",
		Aliases: []string{"switch"},
		Short:   "Switch to a different profile",
		Long: `Switch the active Vault/OpenBao profile.

The active profile determines which Vault server commands are sent to
and which stored token is used for authentication.

Examples:
  # Switch to production profile
  patrol use prod

  # Switch to development profile
  patrol use dev`,
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

// ProfileExportData represents exported profile data.
type ProfileExportData struct {
	Name          string `yaml:"name" json:"name"`
	Address       string `yaml:"address" json:"address"`
	Type          string `yaml:"type,omitempty" json:"type,omitempty"`
	Namespace     string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	BinaryPath    string `yaml:"binary_path,omitempty" json:"binary_path,omitempty"`
	TLSSkipVerify bool   `yaml:"tls_skip_verify,omitempty" json:"tls_skip_verify,omitempty"`
	CACert        string `yaml:"ca_cert,omitempty" json:"ca_cert,omitempty"`
	CAPath        string `yaml:"ca_path,omitempty" json:"ca_path,omitempty"`
	ClientCert    string `yaml:"client_cert,omitempty" json:"client_cert,omitempty"`
	ClientKey     string `yaml:"client_key,omitempty" json:"client_key,omitempty"`
}

// newProfileExportCmd creates the profile export command.
func (cli *CLI) newProfileExportCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "export [profile-name]",
		Short: "Export a profile configuration",
		Long: `Export a profile configuration to YAML format.

If no profile name is given, exports the current profile.
Use -f/--file to write to a file instead of stdout.

Note: This does NOT export the token, only the connection configuration.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return cli.getProfileNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var conn *config.Connection
			var err error

			if len(args) > 0 {
				conn, err = cli.Config.GetConnection(args[0])
			} else {
				conn, err = cli.Config.GetCurrentConnection()
			}
			if err != nil {
				return err
			}

			// Create exportable config (without sensitive data)
			exportConfig := ProfileExportData{
				Name:          conn.Name,
				Address:       conn.Address,
				Type:          string(conn.Type),
				Namespace:     conn.Namespace,
				BinaryPath:    conn.BinaryPath,
				TLSSkipVerify: conn.TLSSkipVerify,
				CACert:        conn.CACert,
				CAPath:        conn.CAPath,
				ClientCert:    conn.ClientCert,
				ClientKey:     conn.ClientKey,
			}

			data, err := yaml.Marshal(exportConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal profile: %w", err)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, data, 0600); err != nil {
					return fmt.Errorf("failed to write file: %w", err)
				}
				fmt.Printf("Profile exported to %s\n", outputFile)
			} else {
				fmt.Print(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "file", "f", "", "Output file (default: stdout)")

	return cmd
}

// newProfileImportCmd creates the profile import command.
func (cli *CLI) newProfileImportCmd() *cobra.Command {
	var rename string

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a profile configuration",
		Long: `Import a profile configuration from a YAML file.

Use --name to rename the profile during import.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			var importedConn ProfileExportData
			if err := yaml.Unmarshal(data, &importedConn); err != nil {
				return fmt.Errorf("failed to parse file: %w", err)
			}

			// Apply rename if specified
			name := importedConn.Name
			if rename != "" {
				name = rename
			}
			if name == "" {
				return fmt.Errorf("profile name is required")
			}

			// Create connection
			conn := config.Connection{
				Name:          name,
				Address:       importedConn.Address,
				Type:          config.BinaryType(importedConn.Type),
				Namespace:     importedConn.Namespace,
				BinaryPath:    importedConn.BinaryPath,
				TLSSkipVerify: importedConn.TLSSkipVerify,
				CACert:        importedConn.CACert,
				CAPath:        importedConn.CAPath,
				ClientCert:    importedConn.ClientCert,
				ClientKey:     importedConn.ClientKey,
			}

			// Validate
			if err := conn.ValidateAddress(); err != nil {
				return fmt.Errorf("invalid profile: %w", err)
			}

			// Add to config
			if err := cli.Config.AddConnection(conn); err != nil {
				return err
			}

			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Profile %q imported successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&rename, "name", "", "Rename the profile during import")

	return cmd
}

// ProfileTestOutput represents profile test output for JSON.
type ProfileTestOutput struct {
	Profile string `json:"profile"`
	Address string `json:"address"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// newProfileTestCmd creates the profile test command.
func (cli *CLI) newProfileTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [profile-name]",
		Short: "Test connectivity to a profile's server",
		Long: `Test the connection to a profile's Vault/OpenBao server.

If no profile name is given, tests the current profile.`,
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

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			var conn *config.Connection

			if len(args) > 0 {
				conn, err = cli.Config.GetConnection(args[0])
			} else {
				conn, err = cli.Config.GetCurrentConnection()
			}
			if err != nil {
				return err
			}

			output := ProfileTestOutput{
				Profile: conn.Name,
				Address: conn.Address,
			}

			// Test HTTP connectivity
			client := &http.Client{Timeout: 5 * time.Second}
			healthURL := conn.Address + "/v1/sys/health"

			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				output.Status = "error"
				output.Message = fmt.Sprintf("invalid URL: %v", err)

				writer := NewOutputWriter(format)
				return writer.Write(output, func() {
					fmt.Printf("Testing connection to %s...\n", conn.Address)
					fmt.Printf("[XX] Connection failed: %v\n", err)
				})
			}

			resp, err := client.Do(req)
			if err != nil {
				output.Status = "error"
				output.Message = fmt.Sprintf("connection failed: %v", err)

				writer := NewOutputWriter(format)
				return writer.Write(output, func() {
					fmt.Printf("Testing connection to %s...\n", conn.Address)
					fmt.Printf("[XX] Connection failed: %v\n", err)
				})
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case 200:
				output.Status = "healthy"
				output.Message = "initialized, unsealed, active"
			case 429, 472, 473:
				output.Status = "standby"
				output.Message = "standby node"
			case 501:
				output.Status = "uninitialized"
				output.Message = "server is not initialized"
			case 503:
				output.Status = "sealed"
				output.Message = "server is sealed"
			default:
				output.Status = "unknown"
				output.Message = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
			}

			writer := NewOutputWriter(format)
			return writer.Write(output, func() {
				fmt.Printf("Testing connection to %s...\n", conn.Address)

				switch output.Status {
				case "healthy":
					fmt.Println("[OK] Server is healthy (initialized, unsealed, active)")
				case "standby":
					fmt.Println("[OK] Server is available (standby node)")
				case "uninitialized":
					fmt.Println("[!!] Server is not initialized")
				case "sealed":
					fmt.Println("[!!] Server is sealed")
				default:
					fmt.Printf("[!!] Unexpected status: %d\n", resp.StatusCode)
				}
			})
		},
	}
}
