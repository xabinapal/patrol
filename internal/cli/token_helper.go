package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/profile"
)

// newTokenHelperCmd creates a hidden command group for token helper operations.
// These commands are typically invoked by Vault itself, not by users directly.
func (cli *CLI) newTokenHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "token-helper",
		Short:  "Token helper commands (used by Vault)",
		Hidden: true,
	}

	cmd.AddCommand(
		cli.newGetCmd(),
		cli.newStoreCmd(),
		cli.newEraseCmd(),
	)

	return cmd
}

// newGetCmd creates the get command for token helper mode.
func (cli *CLI) newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "get",
		Short:  "Get the stored token (token helper)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.handleTokenHelperGet()
		},
	}
}

// newStoreCmd creates the store command for token helper mode.
func (cli *CLI) newStoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "store",
		Short:  "Store a token (token helper)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.handleTokenHelperStore()
		},
	}
}

// newEraseCmd creates the erase command for token helper mode.
func (cli *CLI) newEraseCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "erase",
		Short:  "Erase the stored token (token helper)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.handleTokenHelperErase()
		},
	}
}

// IsTokenHelperCommand checks if the given command name is a token helper command.
func IsTokenHelperCommand(name string) bool {
	return name == "get" || name == "store" || name == "erase"
}

// ShouldHandleAsTokenHelper checks if the arguments indicate a token helper invocation.
func (cli *CLI) ShouldHandleAsTokenHelper(args []string) bool {
	if len(args) < 1 {
		return false
	}
	return IsTokenHelperCommand(args[0])
}

// handleTokenHelper handles direct token helper invocations.
// This is called when patrol is invoked as: patrol get, patrol store, or patrol erase
func (cli *CLI) handleTokenHelper(ctx context.Context, command string) error {
	switch command {
	case "get":
		return cli.handleTokenHelperGet()
	case "store":
		return cli.handleTokenHelperStore()
	case "erase":
		return cli.handleTokenHelperErase()
	default:
		return fmt.Errorf("unknown token helper command: %s", command)
	}
}

// handleTokenHelperGet retrieves and outputs the stored token.
// Per Vault token helper spec:
// - Read VAULT_ADDR from environment to determine which token to return
// - Output the token to stdout (no newline)
// - Exit 0 on success, non-zero on error
func (cli *CLI) handleTokenHelperGet() error {
	conn, err := cli.getTokenHelperConnection()
	if err != nil {
		// Per spec, if we can't determine the address or find a token, just exit 0
		// without outputting anything. Vault will handle the "no token" case.
		return nil
	}

	// Check keyring availability
	keyringErr := cli.Keyring.IsAvailable()
	if keyringErr != nil {
		// Cannot access keyring, treat as no token
		fmt.Fprintf(os.Stderr, "patrol: keyring unavailable: %v\n", keyringErr)
		return nil
	}

	// Create profile from connection
	prof := &profile.Profile{Connection: conn}

	// Get the token
	token, err := prof.GetToken(cli.Keyring)
	if err != nil {
		if errors.Is(err, keyring.ErrTokenNotFound) {
			// No token stored, this is normal
			return nil
		}
		// Other error
		fmt.Fprintf(os.Stderr, "patrol: failed to get token: %v\n", err)
		os.Exit(1)
	}

	// Output the token (no newline, per spec)
	fmt.Print(token)
	return nil
}

// handleTokenHelperStore stores a token from stdin.
// Per Vault token helper spec:
// - Read the token from stdin
// - Read VAULT_ADDR from environment to determine storage key
// - Store the token securely
// - Output nothing to stdout
// - Exit 0 on success, non-zero on error
func (cli *CLI) handleTokenHelperStore() error {
	conn, err := cli.getTokenHelperConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "patrol: %v\n", err)
		os.Exit(1)
	}

	// Check keyring availability
	keyringErr := cli.Keyring.IsAvailable()
	if keyringErr != nil {
		fmt.Fprintf(os.Stderr, "patrol: secure keyring not available: %v\n", keyringErr)
		os.Exit(1)
	}

	// Read token from stdin
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil && err.Error() != "EOF" {
		// Try reading without newline delimiter (some cases may not have newline)
		// Re-read from beginning isn't possible, so we work with what we got
		if token == "" {
			fmt.Fprintf(os.Stderr, "patrol: failed to read token from stdin: %v\n", err)
			os.Exit(1)
		}
	}

	// Trim any whitespace
	token = strings.TrimSpace(token)

	if token == "" {
		fmt.Fprintf(os.Stderr, "patrol: empty token received\n")
		os.Exit(1)
	}

	// Store the token
	prof := &profile.Profile{Connection: conn}
	if err := prof.SetToken(cli.Keyring, token); err != nil {
		fmt.Fprintf(os.Stderr, "patrol: failed to store token: %v\n", err)
		os.Exit(1)
	}

	return nil
}

// handleTokenHelperErase removes the stored token.
// Per Vault token helper spec:
// - Read VAULT_ADDR from environment to determine which token to erase
// - Remove the token from storage
// - Output nothing to stdout
// - Exit 0 on success (including if token doesn't exist)
func (cli *CLI) handleTokenHelperErase() error {
	conn, err := cli.getTokenHelperConnection()
	if err != nil {
		// Can't determine address, nothing to erase
		return nil
	}

	// Delete the token (ignore "not found" errors)
	prof := &profile.Profile{Connection: conn}
	if err := prof.DeleteToken(cli.Keyring); err != nil {
		if !errors.Is(err, keyring.ErrTokenNotFound) {
			fmt.Fprintf(os.Stderr, "patrol: failed to erase token: %v\n", err)
			os.Exit(1)
		}
	}

	return nil
}

// getTokenHelperConnection creates a connection from environment variables.
// Used when Patrol is invoked as a token helper.
func (cli *CLI) getTokenHelperConnection() (*config.Connection, error) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		return nil, errors.New("VAULT_ADDR not set")
	}

	namespace := os.Getenv("VAULT_NAMESPACE")

	// Create a unique profile name from the address for token helper mode
	// This ensures different servers have different keyring entries
	profileName := sanitizeAddressForProfile(addr)
	if namespace != "" {
		profileName += "-" + sanitizeNamespace(namespace)
	}

	conn := &config.Connection{
		Name:      profileName,
		Address:   addr,
		Namespace: namespace,
	}

	// Security: Validate the address format to prevent malformed URLs
	if err := conn.ValidateAddress(); err != nil {
		return nil, fmt.Errorf("invalid VAULT_ADDR: %w", err)
	}

	return conn, nil
}

// sanitizeAddressForProfile converts an address to a safe profile name.
func sanitizeAddressForProfile(addr string) string {
	name := addr
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	// Remove trailing dashes
	name = strings.TrimRight(name, "-")
	return name
}

// sanitizeNamespace converts a namespace to a safe string.
func sanitizeNamespace(ns string) string {
	return strings.ReplaceAll(ns, "/", "-")
}
