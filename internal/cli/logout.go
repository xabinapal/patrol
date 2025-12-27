package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/proxy"
)

// newLogoutCmd creates the logout command.
func (cli *CLI) newLogoutCmd() *cobra.Command {
	var (
		revokeFlag bool
		allFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Remove stored token and optionally revoke it",
		Long: `Remove the stored authentication token from the system credential store.

By default, this command also attempts to revoke the token on the Vault server
to invalidate it immediately. Use --no-revoke to only remove the local token
without revoking it on the server.

Examples:
  # Logout from current profile
  patrol logout

  # Logout from a specific profile
  patrol logout prod

  # Logout without revoking the token
  patrol logout --no-revoke

  # Logout from all profiles
  patrol logout --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if allFlag {
				return cli.runLogoutAll(cmd.Context(), revokeFlag)
			}

			var profileName string
			if len(args) > 0 {
				profileName = args[0]
			}
			return cli.runLogout(cmd.Context(), profileName, revokeFlag)
		},
	}

	cmd.Flags().BoolVar(&revokeFlag, "revoke", true, "Revoke the token on the Vault server")
	cmd.Flags().BoolVar(&revokeFlag, "no-revoke", false, "Do not revoke the token on the Vault server")
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Logout from all profiles")

	return cmd
}

// runLogout handles logging out from a single profile.
func (cli *CLI) runLogout(ctx context.Context, profileName string, revoke bool) error {
	var conn *config.Connection
	var err error

	if profileName == "" {
		conn, err = cli.GetCurrentConnection()
		if err != nil {
			return err
		}
		profileName = conn.Name
	} else {
		conn, err = cli.Config.GetConnection(profileName)
		if err != nil {
			return err
		}
	}

	// Get the current token before deleting
	token, err := cli.Keyring.Get(conn.KeyringKey())
	if err != nil {
		if errors.Is(err, keyring.ErrTokenNotFound) {
			fmt.Printf("No token stored for profile %q\n", profileName)
			return nil
		}
		return fmt.Errorf("failed to retrieve token: %w", err)
	}

	// Revoke the token if requested
	if revoke && cli.Config.RevokeOnLogout {
		if err := cli.revokeToken(ctx, conn, token); err != nil {
			// Log the error but continue with local removal
			if cli.verboseFlag {
				fmt.Fprintf(os.Stderr, "Warning: failed to revoke token: %v\n", err)
			}
		} else {
			if cli.verboseFlag {
				fmt.Println("Token revoked on server")
			}
		}
	}

	// Delete the token from keyring
	if err := cli.Keyring.Delete(conn.KeyringKey()); err != nil {
		return fmt.Errorf("failed to remove token: %w", err)
	}

	fmt.Printf("Successfully logged out from %q\n", profileName)
	if !revoke || !cli.Config.RevokeOnLogout {
		fmt.Println("Note: The token was not revoked on the server and may still be valid until it expires.")
	}

	return nil
}

// runLogoutAll handles logging out from all profiles.
func (cli *CLI) runLogoutAll(ctx context.Context, revoke bool) error {
	if len(cli.Config.Connections) == 0 {
		fmt.Println("No profiles configured")
		return nil
	}

	var loggedOut int
	var errs []error

	for _, conn := range cli.Config.Connections {
		// Get the token
		token, err := cli.Keyring.Get(conn.KeyringKey())
		if err != nil {
			if errors.Is(err, keyring.ErrTokenNotFound) {
				continue // No token for this profile
			}
			errs = append(errs, fmt.Errorf("%s: %w", conn.Name, err))
			continue
		}

		// Revoke if requested
		if revoke && cli.Config.RevokeOnLogout {
			if err := cli.revokeToken(ctx, &conn, token); err != nil {
				if cli.verboseFlag {
					fmt.Fprintf(os.Stderr, "Warning: failed to revoke token for %s: %v\n", conn.Name, err)
				}
			}
		}

		// Delete from keyring
		if err := cli.Keyring.Delete(conn.KeyringKey()); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", conn.Name, err))
			continue
		}

		loggedOut++
		if cli.verboseFlag {
			fmt.Printf("Logged out from %s\n", conn.Name)
		}
	}

	fmt.Printf("Logged out from %d profile(s)\n", loggedOut)

	if len(errs) > 0 {
		return fmt.Errorf("some profiles failed to logout: %v", errs)
	}

	return nil
}

// revokeToken revokes a token on the Vault server.
func (cli *CLI) revokeToken(ctx context.Context, conn *config.Connection, token string) error {
	// Check if binary exists
	if !proxy.BinaryExists(conn) {
		return fmt.Errorf("vault/openbao binary not found")
	}

	// Create executor with the token to revoke
	exec := proxy.NewExecutor(conn, proxy.WithToken(token))

	// Execute token revoke -self
	_, stderr, exitCode, err := exec.ExecuteCapture(ctx, []string{"token", "revoke", "-self"})
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return fmt.Errorf("revoke failed: %s", string(stderr))
	}

	return nil
}
