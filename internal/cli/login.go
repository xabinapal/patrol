package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/token"
)

// newLoginCmd creates the login command.
func (cli *CLI) newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [flags] [AUTH_K=AUTH_V...]",
		Short: "Authenticate to Vault/OpenBao and securely store the token",
		Long: `Authenticate to the Vault or OpenBao server using any authentication method.

This command wraps the underlying vault/bao login command and securely stores
the resulting token in your system's credential store (Keychain on macOS,
Credential Manager on Windows, Secret Service on Linux).

All authentication methods supported by the Vault CLI are supported, including:
  - Token (default)
  - Userpass
  - LDAP
  - GitHub
  - OIDC
  - AppRole
  - And more...

Examples:
  # Token authentication
  patrol login

  # Userpass authentication
  patrol login -method=userpass username=admin

  # GitHub authentication
  patrol login -method=github token=<github-token>

  # OIDC authentication
  patrol login -method=oidc`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.runLogin(cmd.Context(), args)
		},
		// Allow unknown flags to pass through to vault
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		DisableFlagParsing: true,
	}

	return cmd
}

// runLogin handles the login command execution.
func (cli *CLI) runLogin(ctx context.Context, args []string) error {
	// Check keyring availability first
	if err := cli.Keyring.IsAvailable(); err != nil {
		return fmt.Errorf("cannot store token: %w", err)
	}

	// Get the current connection
	conn, err := cli.GetCurrentConnection()
	if err != nil {
		return err
	}

	// Check if vault binary exists
	if !proxy.BinaryExists(conn) {
		return fmt.Errorf("vault/openbao binary %q not found in PATH", conn.GetBinaryPath())
	}

	// Build the login arguments
	// We need to inject -format=json to capture the token, unless already specified
	loginArgs := buildLoginArgs(args)

	// Check if user wants JSON output
	userWantsJSON := hasJSONFormat(args)

	// Create executor without token (we're logging in)
	exec := proxy.NewExecutor(conn)

	// Execute the login command and capture output
	stdout, stderr, exitCode, err := exec.ExecuteCapture(ctx, loginArgs)
	if err != nil {
		return err
	}

	// If login failed, show the error output and exit
	if exitCode != 0 {
		// #nosec G104 - Writing to stderr/stdout before exit; if write fails, we're exiting anyway
		_, _ = os.Stderr.Write(stderr)
		_, _ = os.Stdout.Write(stdout)
		os.Exit(exitCode)
	}

	// Parse the JSON response to extract the token
	tok, err := token.ParseLoginResponse(stdout)
	if err != nil {
		// Security: Do NOT wrap the error or include stdout which may contain the token
		// Only return a generic error message to avoid token leakage
		return errors.New("failed to parse login response: the token may not have been stored securely; please check your authentication method configuration")
	}

	// Store the token in the keyring
	if err := cli.Keyring.Set(conn.KeyringKey(), tok.ClientToken); err != nil {
		// Show original output
		if userWantsJSON {
			// #nosec G104 - Writing to stdout in error path; best effort to show output
			_, _ = os.Stdout.Write(stdout)
		}
		return fmt.Errorf("failed to store token securely: %w", err)
	}

	// Output based on user preference
	if userWantsJSON {
		// #nosec G104 - Writing to stdout; if write fails, user will see error from vault/openbao
		_, _ = os.Stdout.Write(stdout)
	} else {
		// Format human-readable output similar to Vault's default
		cli.printLoginSuccess(tok, conn.Name)
	}

	return nil
}

// buildLoginArgs builds the arguments for the vault login command.
func buildLoginArgs(args []string) []string {
	// Start with "login" command
	result := []string{"login"}

	// Check if -format is already specified
	hasFormat := false
	hasNoStore := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-format") || strings.HasPrefix(arg, "--format") {
			hasFormat = true
		}
		if arg == "-no-store" || arg == "--no-store" {
			hasNoStore = true
		}
	}

	// Add -format=json if not specified (we need JSON to parse the token)
	if !hasFormat {
		result = append(result, "-format=json")
	}

	// Add -no-store to prevent Vault from trying to use its token helper
	// Patrol handles token storage securely in the OS keyring
	if !hasNoStore {
		result = append(result, "-no-store")
	}

	// Add all the original arguments
	result = append(result, args...)

	return result
}

// hasJSONFormat checks if the user explicitly requested JSON output.
func hasJSONFormat(args []string) bool {
	for _, arg := range args {
		if arg == "-format=json" || arg == "--format=json" ||
			arg == "-format" || arg == "--format" {
			// Check next arg for json value
			return true
		}
	}
	return false
}

// printLoginSuccess prints a human-readable login success message.
func (cli *CLI) printLoginSuccess(tok *token.Token, profileName string) {
	fmt.Println("Success! You are now authenticated.")
	fmt.Println()

	// Token info
	fmt.Printf("Token:              %s (stored securely by Patrol)\n", tok.MaskedToken())
	fmt.Printf("Token Accessor:     %s\n", tok.Accessor)

	// TTL info
	if tok.LeaseDuration > 0 {
		fmt.Printf("Token Duration:     %s\n", tok.FormatTTL())
	} else {
		fmt.Printf("Token Duration:     ∞ (never expires)\n")
	}

	// Renewable status
	if tok.Renewable {
		fmt.Printf("Token Renewable:    true\n")
	} else {
		fmt.Printf("Token Renewable:    false\n")
	}

	// Policies
	if len(tok.Policies) > 0 {
		fmt.Printf("Token Policies:     %v\n", tok.Policies)
	}

	// Profile info
	if profileName != "" && profileName != "env" {
		fmt.Printf("Profile:            %s\n", profileName)
	}

	fmt.Println()
	fmt.Println("Your token has been securely stored in your system's credential store.")
	fmt.Println("It will be automatically used for subsequent vault commands via Patrol.")
}
