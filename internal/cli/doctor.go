package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/daemon"
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/profile"
	"github.com/xabinapal/patrol/internal/utils"
)

// CheckResult represents the result of a diagnostic check.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Fix     string      `json:"fix,omitempty"`
}

// CheckStatus represents the status of a diagnostic check.
type CheckStatus int

const (
	// CheckOK indicates the check passed.
	CheckOK CheckStatus = iota
	// CheckWarning indicates a non-critical issue.
	CheckWarning
	// CheckError indicates a critical failure.
	CheckError
	// CheckSkipped indicates the check was skipped.
	CheckSkipped
)

// String returns the status icon.
func (s CheckStatus) String() string {
	switch s {
	case CheckOK:
		return "OK"
	case CheckWarning:
		return "WARN"
	case CheckError:
		return "ERROR"
	case CheckSkipped:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// Icon returns the status icon for display.
func (s CheckStatus) Icon() string {
	switch s {
	case CheckOK:
		return "[OK]"
	case CheckWarning:
		return "[!!]"
	case CheckError:
		return "[XX]"
	case CheckSkipped:
		return "[--]"
	default:
		return "[??]"
	}
}

// MarshalJSON implements json.Marshaler.
func (s CheckStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// DoctorOutput represents the doctor command output for JSON.
type DoctorOutput struct {
	Checks      []CheckResult `json:"checks"`
	HasErrors   bool          `json:"has_errors"`
	HasWarnings bool          `json:"has_warnings"`
}

// newDoctorCmd creates the doctor command.
func (cli *CLI) newDoctorCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose common issues",
		Long: `Run diagnostic checks to identify and troubleshoot common issues.

The doctor command checks:
  - Configuration file validity
  - Keyring availability
  - Vault/OpenBao CLI binary
  - Server connectivity
  - Token status
  - Daemon status

Use --verbose for more detailed output.

Examples:
  # Run diagnostics
  patrol doctor

  # Run with verbose output and suggested fixes
  patrol doctor --verbose

  # Output as JSON
  patrol doctor -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			results := cli.runDiagnostics(ctx)

			hasErrors := false
			hasWarnings := false
			for _, r := range results {
				if r.Status == CheckError {
					hasErrors = true
				}
				if r.Status == CheckWarning {
					hasWarnings = true
				}
			}

			output := DoctorOutput{
				Checks:      results,
				HasErrors:   hasErrors,
				HasWarnings: hasWarnings,
			}

			writer := NewOutputWriter(format)
			writeErr := writer.Write(output, func() {
				fmt.Println("Patrol Diagnostics")
				fmt.Println("==================")
				fmt.Println()

				for _, r := range results {
					fmt.Printf("%s %s", r.Status.Icon(), r.Name)
					if r.Message != "" {
						fmt.Printf(": %s", r.Message)
					}
					fmt.Println()

					if (r.Status == CheckError || r.Status == CheckWarning) && r.Fix != "" && verbose {
						fmt.Printf("      -> %s\n", r.Fix)
					}
				}

				fmt.Println()
				if hasErrors {
					fmt.Println("Some checks failed. Run with --verbose for suggested fixes.")
				} else if hasWarnings {
					fmt.Println("All critical checks passed with some warnings.")
				} else {
					fmt.Println("All checks passed!")
				}
			})

			if writeErr != nil {
				return writeErr
			}

			if hasErrors {
				return fmt.Errorf("diagnostics failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Show detailed output and suggested fixes")

	return cmd
}

func (cli *CLI) runDiagnostics(ctx context.Context) []CheckResult {
	var results []CheckResult

	// Check 1: Configuration file
	results = append(results, cli.checkConfigFile())

	// Check 2: Keyring
	results = append(results, cli.checkKeyring())

	// Check 3: Profile configured
	results = append(results, cli.checkProfileConfigured())

	// Check 4: Binary exists (for current profile)
	results = append(results, cli.checkBinary()...)

	// Check 5: Server connectivity (for current profile)
	results = append(results, cli.checkServerConnectivity(ctx)...)

	// Check 6: Token status (for current profile)
	results = append(results, cli.checkTokenStatus(ctx)...)

	// Check 7: Daemon status
	results = append(results, cli.checkDaemonStatus())

	// Check 8: Vault token helper configuration
	results = append(results, cli.checkVaultTokenHelper())

	return results
}

func (cli *CLI) checkConfigFile() CheckResult {
	paths := config.GetPaths()

	if _, err := os.Stat(paths.ConfigFile); os.IsNotExist(err) {
		return CheckResult{
			Name:    "Configuration file",
			Status:  CheckWarning,
			Message: "not found",
			Fix:     "Run 'patrol profile add' to create a profile and configuration",
		}
	}

	// Try to load and validate
	cfg, err := config.Load()
	if err != nil {
		return CheckResult{
			Name:    "Configuration file",
			Status:  CheckError,
			Message: fmt.Sprintf("invalid: %v", err),
			Fix:     "Run 'patrol config validate' to see detailed errors",
		}
	}

	return CheckResult{
		Name:    "Configuration file",
		Status:  CheckOK,
		Message: fmt.Sprintf("found (%d profiles)", len(cfg.Connections)),
	}
}

func (cli *CLI) checkKeyring() CheckResult {
	if err := cli.Keyring.IsAvailable(); err != nil {
		return CheckResult{
			Name:    "Keyring",
			Status:  CheckError,
			Message: fmt.Sprintf("unavailable: %v", err),
			Fix:     "Install and configure a keyring service (gnome-keyring, kwallet, or macOS Keychain)",
		}
	}

	// Determine keyring type
	var keyringType string
	switch cli.Keyring.(type) {
	case *keyring.FileStore:
		keyringType = "file-based (test mode)"
	default:
		keyringType = "OS keyring"
	}

	return CheckResult{
		Name:    "Keyring",
		Status:  CheckOK,
		Message: keyringType,
	}
}

func (cli *CLI) checkProfileConfigured() CheckResult {
	if len(cli.Config.Connections) == 0 {
		return CheckResult{
			Name:    "Profile",
			Status:  CheckError,
			Message: "no profiles configured",
			Fix:     "Run 'patrol profile add' to create a profile",
		}
	}

	if cli.Config.Current == "" {
		return CheckResult{
			Name:    "Profile",
			Status:  CheckWarning,
			Message: "no active profile selected",
			Fix:     "Run 'patrol profile use <name>' to select a profile",
		}
	}

	return CheckResult{
		Name:    "Profile",
		Status:  CheckOK,
		Message: fmt.Sprintf("'%s' selected", cli.Config.Current),
	}
}

func (cli *CLI) checkBinary() []CheckResult {
	var results []CheckResult

	prof, err := profile.GetCurrent(cli.Config)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Vault/OpenBao binary",
			Status:  CheckSkipped,
			Message: "no profile selected",
		})
		return results
	}
	conn := prof.Connection

	binaryName := conn.GetBinaryPath()

	// Check if binary exists
	path, err := exec.LookPath(binaryName)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Vault/OpenBao binary",
			Status:  CheckError,
			Message: fmt.Sprintf("'%s' not found in PATH", binaryName),
			Fix:     fmt.Sprintf("Install %s or configure binary_path in your profile", binaryName),
		})
		return results
	}

	// Get version
	// #nosec G204 - path is validated vault/openbao binary path, "version" is a static argument
	cmd := exec.Command(path, "version")
	output, err := cmd.Output()
	version := "unknown"
	if err == nil {
		version = string(output)
		// Trim to first line
		for i, c := range version {
			if c == '\n' {
				version = version[:i]
				break
			}
		}
	}

	results = append(results, CheckResult{
		Name:    "Vault/OpenBao binary",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	})

	return results
}

func (cli *CLI) checkServerConnectivity(ctx context.Context) []CheckResult {
	var results []CheckResult

	prof, err := profile.GetCurrent(cli.Config)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckSkipped,
			Message: "no profile selected",
		})
		return results
	}
	conn := prof.Connection

	// HTTP health check
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := conn.Address + "/v1/sys/health"

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckError,
			Message: fmt.Sprintf("invalid URL: %v", err),
			Fix:     "Check the address in your profile configuration",
		})
		return results
	}

	resp, err := client.Do(req)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckError,
			Message: fmt.Sprintf("connection failed: %v", err),
			Fix:     "Check that the Vault/OpenBao server is running and accessible",
		})
		return results
	}
	defer resp.Body.Close()

	// Interpret status codes
	// 200 = initialized, unsealed, active
	// 429 = unsealed, standby
	// 472 = disaster recovery secondary
	// 473 = performance standby
	// 501 = not initialized
	// 503 = sealed
	switch resp.StatusCode {
	case 200:
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckOK,
			Message: fmt.Sprintf("%s is healthy", conn.Address),
		})
	case 429, 472, 473:
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckOK,
			Message: fmt.Sprintf("%s is available (standby node)", conn.Address),
		})
	case 501:
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckWarning,
			Message: "server not initialized",
			Fix:     "Initialize the Vault/OpenBao server with 'vault operator init'",
		})
	case 503:
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckWarning,
			Message: "server is sealed",
			Fix:     "Unseal the Vault/OpenBao server with 'vault operator unseal'",
		})
	default:
		results = append(results, CheckResult{
			Name:    "Server connectivity",
			Status:  CheckWarning,
			Message: fmt.Sprintf("unexpected status %d", resp.StatusCode),
		})
	}

	return results
}

func (cli *CLI) checkTokenStatus(ctx context.Context) []CheckResult {
	var results []CheckResult

	prof, err := profile.GetCurrent(cli.Config)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Token",
			Status:  CheckSkipped,
			Message: "no profile selected",
		})
		return results
	}

	// Get token status
	tokenStatus, _, err := prof.GetTokenStatus(ctx, cli.Keyring)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Token",
			Status:  CheckError,
			Message: fmt.Sprintf("error checking token: %v", err),
		})
		return results
	}

	if !tokenStatus.Stored {
		results = append(results, CheckResult{
			Name:    "Token",
			Status:  CheckWarning,
			Message: "not logged in",
			Fix:     "Run 'patrol login' to authenticate",
		})
		return results
	}

	if !tokenStatus.Valid {
		results = append(results, CheckResult{
			Name:    "Token",
			Status:  CheckError,
			Message: "stored but invalid or expired",
			Fix:     "Run 'patrol login' to re-authenticate",
		})
		return results
	}

	if tokenStatus.Error != "" {
		results = append(results, CheckResult{
			Name:    "Token",
			Status:  CheckOK,
			Message: fmt.Sprintf("stored (%s)", tokenStatus.Error),
		})
		return results
	}

	ttlMsg := "never expires"
	if tokenStatus.TTL > 0 {
		ttlMsg = fmt.Sprintf("expires in %s", utils.FormatDurationSeconds(tokenStatus.TTL))
	}

	status := CheckOK
	if tokenStatus.TTL > 0 && tokenStatus.TTL < TokenExpiryWarningSeconds {
		status = CheckWarning
	}

	results = append(results, CheckResult{
		Name:    "Token",
		Status:  status,
		Message: fmt.Sprintf("valid (%s)", ttlMsg),
	})

	return results
}

func (cli *CLI) checkDaemonStatus() CheckResult {
	if daemon.IsRunningFromPID(cli.Config) {
		pid, err := daemon.GetPID(cli.Config)
		if err != nil {
			// PID file might be stale, but daemon is running
			return CheckResult{
				Name:    "Daemon",
				Status:  CheckOK,
				Message: "running (PID unavailable)",
			}
		}
		return CheckResult{
			Name:    "Daemon",
			Status:  CheckOK,
			Message: fmt.Sprintf("running (PID: %d)", pid),
		}
	}

	return CheckResult{
		Name:    "Daemon",
		Status:  CheckWarning,
		Message: "not running",
		Fix:     "Run 'patrol daemon start' to enable automatic token renewal",
	}
}

// checkVaultTokenHelper checks if Vault's token helper is configured correctly.
func (cli *CLI) checkVaultTokenHelper() CheckResult {
	// Find the .vault config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckSkipped,
			Message: "could not determine home directory",
		}
	}

	vaultConfigPath := filepath.Join(homeDir, ".vault")
	// #nosec G304 - vaultConfigPath is from user's home directory (controlled)
	file, err := os.Open(vaultConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No .vault config file - that's fine, no token helper configured
			return CheckResult{
				Name:    "Vault token helper",
				Status:  CheckOK,
				Message: "not configured (using default)",
			}
		}
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckWarning,
			Message: fmt.Sprintf("cannot read %s: %v", vaultConfigPath, err),
		}
	}
	defer file.Close()

	// Parse the config file to find token_helper
	var tokenHelperPath string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Look for token_helper = "path"
		if strings.HasPrefix(line, "token_helper") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				tokenHelperPath = strings.TrimSpace(parts[1])
				// Remove quotes
				tokenHelperPath = strings.Trim(tokenHelperPath, "\"'")
			}
		}
	}

	if tokenHelperPath == "" {
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckOK,
			Message: "not configured in ~/.vault",
		}
	}

	// Check if the token helper path exists
	info, err := os.Stat(tokenHelperPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Find the correct patrol binary path
			patrolPath, execErr := os.Executable()
			if execErr != nil {
				return CheckResult{
					Name:    "Vault token helper",
					Status:  CheckError,
					Message: fmt.Sprintf("configured path does not exist: %s", tokenHelperPath),
					Fix:     "Update ~/.vault to use a valid patrol binary path",
				}
			}
			return CheckResult{
				Name:    "Vault token helper",
				Status:  CheckError,
				Message: fmt.Sprintf("configured path does not exist: %s", tokenHelperPath),
				Fix:     fmt.Sprintf("Update ~/.vault to use: token_helper = \"%s\"", patrolPath),
			}
		}
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckError,
			Message: fmt.Sprintf("cannot access %s: %v", tokenHelperPath, err),
		}
	}

	// Check if it's executable
	if info.Mode().Perm()&0111 == 0 {
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckError,
			Message: fmt.Sprintf("token helper is not executable: %s", tokenHelperPath),
			Fix:     fmt.Sprintf("Run: chmod +x %s", tokenHelperPath),
		}
	}

	// Check if it contains "patrol" in the path (likely our helper)
	baseName := filepath.Base(tokenHelperPath)
	if strings.Contains(strings.ToLower(baseName), "patrol") {
		return CheckResult{
			Name:    "Vault token helper",
			Status:  CheckOK,
			Message: fmt.Sprintf("configured to use Patrol: %s", tokenHelperPath),
		}
	}

	// Some other token helper is configured
	return CheckResult{
		Name:    "Vault token helper",
		Status:  CheckOK,
		Message: fmt.Sprintf("configured: %s", tokenHelperPath),
	}
}
