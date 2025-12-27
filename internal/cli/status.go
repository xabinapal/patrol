package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/daemon"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/token"
)

// StatusOutput represents status information for JSON output.
type StatusOutput struct {
	Profile *ProfileStatusInfo `json:"profile,omitempty"`
	Keyring *KeyringStatus     `json:"keyring"`
	Token   *TokenStatusInfo   `json:"token,omitempty"`
	Daemon  *DaemonStatusInfo  `json:"daemon"`
}

// ProfileStatusInfo represents profile information in status output.
type ProfileStatusInfo struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
	Binary    string `json:"binary"`
}

// KeyringStatus represents keyring availability status.
type KeyringStatus struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
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

// DaemonStatusInfo represents daemon status in status output.
type DaemonStatusInfo struct {
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// newStatusCmd creates the status command.
func (cli *CLI) newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status and token information",
		Long: `Display the current authentication status, including:
- Current profile and Vault address
- Token validity and TTL
- Token policies and metadata

This command checks the stored token and optionally queries the Vault server
for current token information.

Examples:
  # Show status for current profile
  patrol status

  # Show status in JSON format
  patrol status -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}
			return cli.runStatus(cmd.Context(), format)
		},
	}

	return cmd
}

// runStatus displays the current authentication status.
func (cli *CLI) runStatus(ctx context.Context, format OutputFormat) error {
	output := NewOutputWriter(format)

	// Initialize status output for JSON
	status := &StatusOutput{
		Keyring: &KeyringStatus{},
		Daemon:  &DaemonStatusInfo{},
	}

	// Get current connection
	conn, err := cli.GetCurrentConnection()
	if err != nil {
		return err
	}

	// Set profile info
	status.Profile = &ProfileStatusInfo{
		Name:      conn.Name,
		Address:   conn.Address,
		Type:      string(conn.Type),
		Namespace: conn.Namespace,
		Binary:    conn.GetBinaryPath(),
	}

	// Check daemon status
	if daemon.IsRunningFromPID(cli.Config) {
		pid, pidErr := daemon.GetPID(cli.Config)
		if pidErr == nil {
			status.Daemon.Running = true
			status.Daemon.PID = pid
		}
	}

	// Check keyring availability
	keyringErr := cli.Keyring.IsAvailable()
	if keyringErr != nil {
		status.Keyring.Available = false
		status.Keyring.Error = keyringErr.Error()

		return output.Write(status, func() {
			fmt.Printf("Profile:     %s\n", conn.Name)
			fmt.Printf("Address:     %s\n", conn.Address)
			if conn.Namespace != "" {
				fmt.Printf("Namespace:   %s\n", conn.Namespace)
			}
			fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
			fmt.Println()
			fmt.Printf("Keyring:     unavailable (%v)\n", keyringErr)
		})
	}
	status.Keyring.Available = true

	// Get stored token
	storedToken, err := cli.Keyring.Get(conn.KeyringKey())
	if err != nil {
		if errors.Is(err, keyring.ErrTokenNotFound) {
			status.Token = &TokenStatusInfo{
				Stored: false,
				Valid:  false,
			}

			return output.Write(status, func() {
				fmt.Printf("Profile:     %s\n", conn.Name)
				fmt.Printf("Address:     %s\n", conn.Address)
				if conn.Namespace != "" {
					fmt.Printf("Namespace:   %s\n", conn.Namespace)
				}
				fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
				fmt.Println()
				fmt.Printf("Keyring:     available\n")
				fmt.Printf("Token:       not logged in\n")
				fmt.Println()
				fmt.Println("Run 'patrol login' to authenticate.")
			})
		}
		return fmt.Errorf("failed to retrieve token: %w", err)
	}

	// Token is stored
	status.Token = &TokenStatusInfo{
		Stored: true,
	}

	// Query Vault for token info if binary exists
	if !proxy.BinaryExists(conn) {
		status.Token.Error = fmt.Sprintf("%s binary not found", conn.GetBinaryPath())

		return output.Write(status, func() {
			fmt.Printf("Profile:     %s\n", conn.Name)
			fmt.Printf("Address:     %s\n", conn.Address)
			if conn.Namespace != "" {
				fmt.Printf("Namespace:   %s\n", conn.Namespace)
			}
			fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
			fmt.Println()
			fmt.Printf("Keyring:     available\n")
			fmt.Printf("Token:       stored (****%s)\n", storedToken[len(storedToken)-4:])
			fmt.Println()
			fmt.Printf("Note: Cannot verify token - %s binary not found\n", conn.GetBinaryPath())
		})
	}

	// Look up token details
	exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
	stdout, stderr, exitCode, err := exec.ExecuteCapture(ctx, []string{"token", "lookup", "-format=json"})
	if err != nil {
		return fmt.Errorf("failed to lookup token: %w", err)
	}

	if exitCode != 0 {
		status.Token.Valid = false
		status.Token.Error = string(stderr)

		return output.Write(status, func() {
			fmt.Printf("Profile:     %s\n", conn.Name)
			fmt.Printf("Address:     %s\n", conn.Address)
			if conn.Namespace != "" {
				fmt.Printf("Namespace:   %s\n", conn.Namespace)
			}
			fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
			fmt.Println()
			fmt.Printf("Keyring:     available\n")
			fmt.Printf("Token:       stored (****%s)\n", storedToken[len(storedToken)-4:])
			fmt.Println()
			fmt.Printf("Token Status: invalid or expired\n")
			fmt.Printf("Error:        %s\n", string(stderr))
			fmt.Println()
			fmt.Println("Your stored token may have expired. Run 'patrol login' to re-authenticate.")
		})
	}

	// Parse lookup response
	lookupData, err := token.ParseLookupResponse(stdout)
	if err != nil {
		if cli.verboseFlag {
			fmt.Fprintf(os.Stderr, "Warning: could not parse token lookup response: %v\n", err)
		}
		status.Token.Valid = true
		status.Token.Error = "details unavailable"

		return output.Write(status, func() {
			fmt.Printf("Profile:     %s\n", conn.Name)
			fmt.Printf("Address:     %s\n", conn.Address)
			if conn.Namespace != "" {
				fmt.Printf("Namespace:   %s\n", conn.Namespace)
			}
			fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
			fmt.Println()
			fmt.Printf("Keyring:     available\n")
			fmt.Printf("Token:       stored (****%s)\n", storedToken[len(storedToken)-4:])
			fmt.Println()
			fmt.Printf("Token Status: valid (details unavailable)\n")
		})
	}

	// Token is valid with full details
	status.Token.Valid = true
	status.Token.DisplayName = lookupData.DisplayName
	status.Token.TTL = lookupData.TTL
	if lookupData.TTL > 0 {
		status.Token.TTLFormatted = formatDuration(lookupData.TTL)
	}
	status.Token.Renewable = lookupData.Renewable
	status.Token.Policies = lookupData.Policies
	status.Token.AuthPath = lookupData.Path
	status.Token.EntityID = lookupData.EntityID

	return output.Write(status, func() {
		fmt.Printf("Profile:     %s\n", conn.Name)
		fmt.Printf("Address:     %s\n", conn.Address)
		if conn.Namespace != "" {
			fmt.Printf("Namespace:   %s\n", conn.Namespace)
		}
		fmt.Printf("Binary:      %s\n", conn.GetBinaryPath())
		fmt.Println()
		fmt.Printf("Keyring:     available\n")
		fmt.Printf("Token:       stored (****%s)\n", storedToken[len(storedToken)-4:])
		fmt.Println()
		fmt.Printf("Token Status: valid\n")
		if lookupData.DisplayName != "" {
			fmt.Printf("Display Name: %s\n", lookupData.DisplayName)
		}
		if lookupData.TTL > 0 {
			fmt.Printf("TTL:          %s\n", formatDuration(lookupData.TTL))
		} else {
			fmt.Printf("TTL:          ∞ (never expires)\n")
		}
		fmt.Printf("Renewable:    %t\n", lookupData.Renewable)
		if len(lookupData.Policies) > 0 {
			fmt.Printf("Policies:     %v\n", lookupData.Policies)
		}
		if lookupData.Path != "" {
			fmt.Printf("Auth Path:    %s\n", lookupData.Path)
		}
		if lookupData.EntityID != "" {
			fmt.Printf("Entity ID:    %s\n", lookupData.EntityID)
		}

		// Renewal recommendation
		if lookupData.TTL > 0 && lookupData.TTL < TokenExpiryWarningSeconds && lookupData.Renewable {
			fmt.Println()
			fmt.Println("Warning: Token will expire soon. Consider running 'patrol daemon' for auto-renewal.")
		}

		// Daemon status
		if status.Daemon.Running {
			fmt.Printf("\nDaemon:      running (PID: %d)\n", status.Daemon.PID)
		}
	})
}

// formatDuration is an alias for FormatDurationSeconds for backward compatibility.
func formatDuration(seconds int) string {
	return FormatDurationSeconds(seconds)
}
