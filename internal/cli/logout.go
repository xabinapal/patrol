package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/profile"
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
	var prof *profile.Profile
	var err error

	if profileName == "" {
		prof, err = profile.GetCurrent(cli.Config)
		if err != nil {
			return err
		}
		profileName = prof.Name
	} else {
		prof, err = profile.Get(cli.Config, profileName)
		if err != nil {
			return err
		}
	}

	// Check if token exists
	if !prof.HasToken(cli.Keyring) {
		fmt.Printf("No token stored for profile %q\n", profileName)
		return nil
	}

	// Revoke the token if requested
	if revoke && cli.Config.RevokeOnLogout {
		if err := prof.RevokeToken(ctx, cli.Keyring); err != nil {
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
	if err := prof.DeleteToken(cli.Keyring); err != nil {
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
		prof := &profile.Profile{Connection: &conn}

		// Check if token exists
		if !prof.HasToken(cli.Keyring) {
			continue // No token for this profile
		}

		// Revoke if requested
		if revoke && cli.Config.RevokeOnLogout {
			if err := prof.RevokeToken(ctx, cli.Keyring); err != nil {
				if cli.verboseFlag {
					fmt.Fprintf(os.Stderr, "Warning: failed to revoke token for %s: %v\n", conn.Name, err)
				}
			}
		}

		// Delete from keyring
		if err := prof.DeleteToken(cli.Keyring); err != nil {
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
