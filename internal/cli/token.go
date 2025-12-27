package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/token"
)

// TokenInfoOutput represents token info for JSON output.
type TokenInfoOutput struct {
	Profile     string   `json:"profile"`
	Address     string   `json:"address"`
	Token       string   `json:"token,omitempty"`
	TokenMasked string   `json:"token_masked"`
	Accessor    string   `json:"accessor,omitempty"`
	TTL         int      `json:"ttl,omitempty"`
	Renewable   bool     `json:"renewable,omitempty"`
	Policies    []string `json:"policies,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Valid       bool     `json:"valid"`
}

// newTokenCmd creates the token command group.
func (cli *CLI) newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage stored Vault tokens",
		Long: `Manage Vault tokens stored in the keyring.

These commands allow you to inspect, renew, or revoke tokens stored
by Patrol without needing to interact with the Vault CLI directly.`,
	}

	cmd.AddCommand(
		cli.newTokenInfoCmd(),
		cli.newTokenRenewCmd(),
		cli.newTokenRevokeCmd(),
	)

	return cmd
}

// newTokenInfoCmd creates the token info command.
func (cli *CLI) newTokenInfoCmd() *cobra.Command {
	var showToken bool

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show information about the stored token",
		Long: `Display information about the token stored for the current profile.

By default, the full token is masked. Use --show-token to display it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			conn, err := cli.GetCurrentConnection()
			if err != nil {
				return err
			}

			// Get the stored token
			storedToken, err := cli.Keyring.Get(conn.KeyringKey())
			if err != nil {
				if errors.Is(err, keyring.ErrTokenNotFound) {
					output := TokenInfoOutput{
						Profile:     conn.Name,
						Address:     conn.Address,
						Valid:       false,
						TokenMasked: "(not logged in)",
					}

					writer := NewOutputWriter(format)
					return writer.Write(output, func() {
						fmt.Printf("Profile:     %s\n", conn.Name)
						fmt.Printf("Address:     %s\n", conn.Address)
						fmt.Println()
						fmt.Printf("No token stored for profile %q\n", conn.Name)
						fmt.Println("Run 'patrol login' to authenticate.")
					})
				}
				return fmt.Errorf("failed to get token: %w", err)
			}

			output := TokenInfoOutput{
				Profile:     conn.Name,
				Address:     conn.Address,
				TokenMasked: maskToken(storedToken),
				Valid:       true,
			}

			if showToken {
				output.Token = storedToken
			}

			// Try to get more info from Vault
			if proxy.BinaryExists(conn) {
				exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
				stdout, _, exitCode, err := exec.ExecuteCapture(ctx, []string{"token", "lookup", "-format=json"})
				if err == nil && exitCode == 0 {
					lookupData, err := token.ParseLookupResponse(stdout)
					if err == nil {
						output.Accessor = lookupData.Accessor
						output.TTL = lookupData.TTL
						output.Renewable = lookupData.Renewable
						output.Policies = lookupData.Policies
						output.DisplayName = lookupData.DisplayName
					}
				} else {
					output.Valid = false
				}
			}

			writer := NewOutputWriter(format)
			return writer.Write(output, func() {
				fmt.Printf("Profile:     %s\n", conn.Name)
				fmt.Printf("Address:     %s\n", conn.Address)
				fmt.Println()

				if showToken {
					fmt.Printf("Token:       %s\n", storedToken)
				} else {
					fmt.Printf("Token:       %s\n", output.TokenMasked)
				}

				if output.Accessor != "" {
					fmt.Printf("Accessor:    %s\n", output.Accessor)
				}

				if output.TTL > 0 {
					ttl := time.Duration(output.TTL) * time.Second
					expiry := time.Now().Add(ttl)
					fmt.Printf("TTL:         %s (expires %s)\n", formatTokenDuration(ttl), expiry.Format(time.RFC3339))
				} else if output.Accessor != "" {
					fmt.Printf("TTL:         never expires\n")
				}

				if output.Accessor != "" {
					fmt.Printf("Renewable:   %t\n", output.Renewable)
				}

				if len(output.Policies) > 0 {
					fmt.Printf("Policies:    %v\n", output.Policies)
				}

				if output.DisplayName != "" {
					fmt.Printf("Display:     %s\n", output.DisplayName)
				}

				if !output.Valid && output.Accessor == "" {
					fmt.Println("\n(Cannot retrieve token details - binary not found or token invalid)")
				}
			})
		},
	}

	cmd.Flags().BoolVar(&showToken, "show-token", false, "Show the full token (not masked)")

	return cmd
}

// newTokenRenewCmd creates the token renew command.
func (cli *CLI) newTokenRenewCmd() *cobra.Command {
	var increment string

	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Manually renew the stored token",
		Long: `Manually trigger renewal of the token stored for the current profile.

This is useful when you want to extend your token's TTL immediately
without waiting for the daemon to do it automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			conn, err := cli.GetCurrentConnection()
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

			exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
			stdout, stderr, exitCode, err := exec.ExecuteCapture(ctx, renewArgs)
			if err != nil {
				return fmt.Errorf("failed to renew token: %w", err)
			}

			if exitCode != 0 {
				return fmt.Errorf("token renewal failed: %s", string(stderr))
			}

			// Parse response
			tok, err := token.ParseLoginResponse(stdout)
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

// newTokenRevokeCmd creates the token revoke command.
func (cli *CLI) newTokenRevokeCmd() *cobra.Command {
	var skipRevoke bool

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke and clear the stored token",
		Long: `Revoke the token with Vault and remove it from the keyring.

By default, this command will:
1. Revoke the token with the Vault server
2. Remove the token from the keyring

Use --skip-revoke to only remove from keyring without revoking.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			conn, err := cli.GetCurrentConnection()
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
					exec := proxy.NewExecutor(conn, proxy.WithToken(storedToken))
					_, stderr, exitCode, err := exec.ExecuteCapture(ctx, []string{"token", "revoke", "-self"})
					if err != nil || exitCode != 0 {
						fmt.Printf("Warning: failed to revoke token with Vault: %s\n", string(stderr))
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
