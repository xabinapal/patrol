package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
)

// configPathOutput represents config path output for JSON.
type configPathOutput struct {
	ConfigFile   string `json:"config_file"`
	ConfigDir    string `json:"config_dir"`
	DataDir      string `json:"data_dir"`
	CacheDir     string `json:"cache_dir"`
	ConfigExists bool   `json:"config_exists"`
}

// validationResult represents validation output for JSON.
type validationResult struct {
	Valid    bool                `json:"valid"`
	Profiles []profileValidation `json:"profiles"`
	Daemon   daemonValidation    `json:"daemon"`
	Errors   []string            `json:"errors,omitempty"`
}

// profileValidation represents profile validation for JSON.
type profileValidation struct {
	Name         string `json:"name"`
	AddressValid bool   `json:"address_valid"`
	BinaryValid  bool   `json:"binary_valid"`
	Address      string `json:"address"`
	Binary       string `json:"binary"`
	Error        string `json:"error,omitempty"`
}

// daemonValidation represents daemon validation for JSON.
type daemonValidation struct {
	CheckIntervalValid  bool    `json:"check_interval_valid"`
	RenewThresholdValid bool    `json:"renew_threshold_valid"`
	CheckInterval       string  `json:"check_interval"`
	RenewThreshold      float64 `json:"renew_threshold"`
}

// newConfigCmd creates the config command group.
func (cli *CLI) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Patrol configuration",
		Long: `Manage Patrol configuration files and settings.

Use 'patrol config init' for interactive setup.
Use 'patrol config path' to see configuration file locations.
Use 'patrol config edit' to open the configuration in your editor.`,
	}

	cmd.AddCommand(
		cli.newConfigInitCmd(),
		cli.newConfigPathCmd(),
		cli.newConfigEditCmd(),
		cli.newConfigValidateCmd(),
	)

	return cmd
}

// newConfigInitCmd creates the config init command.
func (cli *CLI) newConfigInitCmd() *cobra.Command {
	var nonInteractive bool
	var address, name, serverType string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Patrol configuration interactively",
		Long: `Initialize Patrol configuration with an interactive wizard.

This command guides you through setting up your first Vault/OpenBao
connection profile. You can also use flags for non-interactive setup.

Examples:
  # Interactive setup
  patrol config init

  # Non-interactive setup
  patrol config init --address https://vault.example.com:8200 --name prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)
			var err error

			// Check if config already exists with profiles
			if len(cli.Config.Connections) > 0 && !nonInteractive {
				fmt.Printf("Configuration already exists with %d profile(s).\n", len(cli.Config.Connections))
				fmt.Print("Add another profile? [y/N]: ")
				var response string
				response, err = reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Get address
			if address == "" && !nonInteractive {
				fmt.Print("Vault/OpenBao address (e.g., https://vault.example.com:8200): ")
				address, err = reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read address: %w", err)
				}
				address = strings.TrimSpace(address)
			}

			if address == "" {
				return fmt.Errorf("address is required")
			}

			// Validate address
			testConn := &config.Connection{Address: address}
			if validateErr := testConn.ValidateAddress(); validateErr != nil {
				return fmt.Errorf("invalid address: %w", validateErr)
			}

			// Get server type
			if serverType == "" && !nonInteractive {
				fmt.Print("Server type [vault/openbao] (default: vault): ")
				serverType, err = reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read server type: %w", err)
				}
				serverType = strings.TrimSpace(strings.ToLower(serverType))
				if serverType == "" {
					serverType = "vault"
				}
			}
			if serverType == "" {
				serverType = "vault"
			}

			var binaryType config.BinaryType
			switch serverType {
			case "vault":
				binaryType = config.BinaryTypeVault
			case "openbao", "bao":
				binaryType = config.BinaryTypeOpenBao
			default:
				return fmt.Errorf("invalid server type %q: must be 'vault' or 'openbao'", serverType)
			}

			// Get profile name
			if name == "" && !nonInteractive {
				defaultName := suggestProfileName(address)
				fmt.Printf("Profile name (default: %s): ", defaultName)
				name, err = reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read profile name: %w", err)
				}
				name = strings.TrimSpace(name)
				if name == "" {
					name = defaultName
				}
			}

			if name == "" {
				name = suggestProfileName(address)
			}

			// Check for duplicate name
			for _, conn := range cli.Config.Connections {
				if conn.Name == name {
					return fmt.Errorf("profile %q already exists", name)
				}
			}

			// Create the connection
			conn := config.Connection{
				Name:    name,
				Address: address,
				Type:    binaryType,
			}

			if err := cli.Config.AddConnection(conn); err != nil {
				return fmt.Errorf("failed to add profile: %w", err)
			}

			if err := cli.Config.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("\nProfile %q created successfully!\n", name)
			fmt.Printf("  Address: %s\n", address)
			fmt.Printf("  Type:    %s\n", serverType)
			fmt.Printf("\nConfiguration saved to: %s\n", cli.Config.FilePath())
			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  1. Run 'patrol login' to authenticate\n")
			fmt.Printf("  2. Run 'patrol daemon start' to enable auto-renewal\n")

			return nil
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Run without prompts")
	cmd.Flags().StringVar(&address, "address", "", "Vault/OpenBao server address")
	cmd.Flags().StringVar(&name, "name", "", "Profile name")
	cmd.Flags().StringVar(&serverType, "type", "", "Server type (vault or openbao)")

	return cmd
}

// newConfigPathCmd creates the config path command.
func (cli *CLI) newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			paths := config.GetPaths()

			_, configErr := os.Stat(paths.ConfigFile)
			output := configPathOutput{
				ConfigFile:   paths.ConfigFile,
				ConfigDir:    paths.ConfigDir,
				DataDir:      paths.DataDir,
				CacheDir:     paths.CacheDir,
				ConfigExists: configErr == nil,
			}

			writer := NewOutputWriter(format)
			return writer.Write(output, func() {
				fmt.Println("Configuration paths:")
				fmt.Printf("  Config file:  %s\n", paths.ConfigFile)
				fmt.Printf("  Config dir:   %s\n", paths.ConfigDir)
				fmt.Printf("  Data dir:     %s\n", paths.DataDir)
				fmt.Printf("  Cache dir:    %s\n", paths.CacheDir)

				fmt.Println("\nStatus:")
				if output.ConfigExists {
					fmt.Printf("  Config file exists\n")
				} else {
					fmt.Printf("  Config file does not exist\n")
				}
			})
		},
	}
}

// newConfigEditCmd creates the config edit command.
func (cli *CLI) newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open configuration file in editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				// Try common editors
				for _, e := range []string{"vim", "vi", "nano", "notepad"} {
					if _, err := exec.LookPath(e); err == nil {
						editor = e
						break
					}
				}
			}
			if editor == "" {
				return fmt.Errorf("no editor found: set $EDITOR environment variable")
			}

			configPath := cli.Config.FilePath()

			// Ensure config file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// Create default config
				if err := cli.Config.Save(); err != nil {
					return fmt.Errorf("failed to create config file: %w", err)
				}
			}

			// #nosec G204 - editor is from $EDITOR env var (user-controlled but expected), configPath is from config file path (controlled)
			editorCmd := exec.Command(editor, configPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			return editorCmd.Run()
		},
	}
}

// newConfigValidateCmd creates the config validate command.
func (cli *CLI) newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			// Try to load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("configuration error: %w", err)
			}

			result := validationResult{
				Valid:    true,
				Profiles: make([]profileValidation, 0, len(cfg.Connections)),
				Daemon: daemonValidation{
					CheckIntervalValid:  cfg.Daemon.CheckInterval > 0,
					RenewThresholdValid: cfg.Daemon.RenewThreshold > 0 && cfg.Daemon.RenewThreshold <= 1,
					CheckInterval:       cfg.Daemon.CheckInterval.String(),
					RenewThreshold:      cfg.Daemon.RenewThreshold,
				},
			}

			// Check each connection
			for _, conn := range cfg.Connections {
				pv := profileValidation{
					Name:         conn.Name,
					AddressValid: true,
					BinaryValid:  true,
					Address:      conn.Address,
					Binary:       conn.GetBinaryPath(),
				}

				if err := conn.ValidateAddress(); err != nil {
					pv.AddressValid = false
					pv.Error = err.Error()
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("profile %s: %v", conn.Name, err))
				}

				if err := conn.ValidateBinaryPath(); err != nil {
					pv.BinaryValid = false
					if pv.Error == "" {
						pv.Error = err.Error()
					}
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("profile %s: %v", conn.Name, err))
				}

				result.Profiles = append(result.Profiles, pv)
			}

			// Check daemon config
			if !result.Daemon.CheckIntervalValid {
				result.Valid = false
				result.Errors = append(result.Errors, "daemon: invalid check interval")
			}
			if !result.Daemon.RenewThresholdValid {
				result.Valid = false
				result.Errors = append(result.Errors, "daemon: renew threshold must be between 0 and 1")
			}

			writer := NewOutputWriter(format)
			writeErr := writer.Write(result, func() {
				fmt.Println("Configuration validation:")

				for _, pv := range result.Profiles {
					fmt.Printf("\nProfile: %s\n", pv.Name)
					if pv.AddressValid {
						fmt.Printf("  Address: %s\n", pv.Address)
					} else {
						fmt.Printf("  Address: %s (invalid)\n", pv.Address)
					}
					if pv.BinaryValid {
						fmt.Printf("  Binary path: %s\n", pv.Binary)
					} else {
						fmt.Printf("  Binary path: %s (invalid)\n", pv.Binary)
					}
				}

				fmt.Printf("\nDaemon configuration:\n")
				if result.Daemon.CheckIntervalValid {
					fmt.Printf("  Check interval: %s\n", result.Daemon.CheckInterval)
				} else {
					fmt.Printf("  Check interval: invalid value\n")
				}
				if result.Daemon.RenewThresholdValid {
					fmt.Printf("  Renew threshold: %.0f%%\n", result.Daemon.RenewThreshold*100)
				} else {
					fmt.Printf("  Renew threshold: must be between 0 and 1\n")
				}

				fmt.Println()
				if result.Valid {
					fmt.Println("Configuration is valid")
				} else {
					fmt.Println("Configuration has errors")
				}
			})

			if writeErr != nil {
				return writeErr
			}

			if !result.Valid {
				return fmt.Errorf("configuration has errors")
			}
			return nil
		},
	}
}

// suggestProfileName suggests a profile name based on the address.
func suggestProfileName(address string) string {
	// Extract hostname from URL
	addr := strings.TrimPrefix(address, "https://")
	addr = strings.TrimPrefix(addr, "http://")

	// Remove port
	if idx := strings.Index(addr, ":"); idx != -1 {
		addr = addr[:idx]
	}

	// Remove domain suffix for common patterns
	addr = strings.TrimSuffix(addr, ".example.com")

	// Use "local" for localhost
	if addr == "localhost" || addr == "127.0.0.1" {
		return "local"
	}

	// Clean up the name
	addr = strings.ReplaceAll(addr, ".", "-")

	if addr == "" {
		return "default"
	}

	return addr
}
