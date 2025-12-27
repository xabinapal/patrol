package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/profile"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/utils"
)

// newLoginCmd creates the login command.
func (cli *CLI) newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [-method=METHOD] [-path=PATH] [AUTH_K=AUTH_V...]",
		Short: "Authenticate to Vault/OpenBao and securely store the token",
		Long: `Authenticate to the Vault or OpenBao server using any authentication method.

This command wraps the underlying vault/bao login command and securely stores
the resulting token in your system's credential store (Keychain on macOS,
Credential Manager on Windows, Secret Service on Linux).

Supported authentication methods:
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
  patrol login -method=userpass username=admin password=secret

  # LDAP authentication with custom path
  patrol login -method=ldap -path=ldap-corp username=user password=pass

  # GitHub authentication
  patrol login -method=github token=<github-token>

  # OIDC authentication
  patrol login -method=oidc`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" || arg == "help" {
					return cmd.Help()
				}
			}

			method, path, remainingArgs, err := parseLoginFlags(args, "", "")
			if err != nil {
				return err
			}

			finalArgs, err := buildLoginArgs(method, path, remainingArgs)
			if err != nil {
				return err
			}
			return cli.runLogin(cmd.Context(), finalArgs)
		},
	}

	return cmd
}

// parseLoginFlags extracts -method and -path flags from args and returns them
// along with the remaining arguments.
func parseLoginFlags(args []string, currentMethod, currentPath string) (method, path string, remaining []string, err error) {
	method = currentMethod
	path = currentPath
	for i, arg := range args {
		if strings.HasPrefix(arg, "-method=") {
			method = strings.TrimPrefix(arg, "-method=")
			continue
		}
		if strings.HasPrefix(arg, "--method=") {
			method = strings.TrimPrefix(arg, "--method=")
			continue
		}

		if strings.HasPrefix(arg, "-path=") {
			path = strings.TrimPrefix(arg, "-path=")
			continue
		}
		if strings.HasPrefix(arg, "--path=") {
			path = strings.TrimPrefix(arg, "--path=")
			continue
		}

		if arg == "-method" || arg == "--method" {
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("flag %s requires a value", arg)
			}
			newArgs := append(args[:i], args[i+2:]...)
			return parseLoginFlags(newArgs, args[i+1], path)
		}

		if arg == "-path" || arg == "--path" {
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("flag %s requires a value", arg)
			}
			newArgs := append(args[:i], args[i+2:]...)
			return parseLoginFlags(newArgs, method, args[i+1])
		}

		if strings.HasPrefix(arg, "-") {
			return "", "", nil, fmt.Errorf("invalid flag: %q (only -method and -path are allowed)", arg)
		}

		remaining = append(remaining, arg)
	}

	return method, path, remaining, nil
}

// runLogin handles the login command execution.
func (cli *CLI) runLogin(ctx context.Context, args []string) error {
	if err := cli.Keyring.IsAvailable(); err != nil {
		return fmt.Errorf("cannot store token: %w", err)
	}

	prof, err := profile.GetCurrent(cli.Config)
	if err != nil {
		return err
	}

	if !proxy.BinaryExists(prof.Connection) {
		return fmt.Errorf("vault/openbao binary %q not found in PATH", prof.GetBinaryPath())
	}

	loginArgs := buildVaultLoginArgs(args)

	exec := proxy.NewExecutor(prof.Connection,
		proxy.WithStdout(os.Stdout),
		proxy.WithStderr(os.Stderr),
	)

	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, loginArgs, &captureBuf)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	tokenStr := utils.ExtractTokenFromOutput(captureBuf.String())
	if tokenStr == "" {
		return errors.New("login succeeded but no token was returned")
	}

	if err := prof.SetToken(cli.Keyring, tokenStr); err != nil {
		return fmt.Errorf("failed to store token securely: %w", err)
	}

	fmt.Println()
	fmt.Println("Success! You are now authenticated.")
	fmt.Printf("Token stored securely in your system's credential store.\n")
	if prof.Name != "" && prof.Name != "env" {
		fmt.Printf("Profile: %s\n", prof.Name)
	}
	fmt.Println()
	fmt.Println("Your token will be automatically used for subsequent vault commands via Patrol.")

	return nil
}

// buildLoginArgs validates and builds login arguments from user input.
func buildLoginArgs(method, path string, args []string) ([]string, error) {
	result := make([]string, 0)

	if method != "" {
		result = append(result, "-method="+method)
	}

	if path != "" {
		result = append(result, "-path="+path)
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("invalid argument: %q (only -method and -path flags are allowed)", arg)
		}

		if !strings.Contains(arg, "=") {
			return nil, fmt.Errorf("invalid argument: %q (authentication parameters must be in K=V format)", arg)
		}

		result = append(result, arg)
	}

	return result, nil
}

// buildVaultLoginArgs builds the final arguments for the vault login command.
func buildVaultLoginArgs(args []string) []string {
	result := []string{"login", "-token-only", "-no-store"}
	result = append(result, args...)
	return result
}
