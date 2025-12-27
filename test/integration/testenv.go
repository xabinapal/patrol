//go:build integration

// Package integration provides integration tests for Patrol.
package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestEnv represents a test environment with Vault or OpenBao.
type TestEnv struct {
	Type       string // "vault" or "openbao"
	Address    string
	Token      string
	BinaryPath string
}

// VaultTestEnv returns a test environment for HashiCorp Vault.
func VaultTestEnv() *TestEnv {
	addr := os.Getenv("VAULT_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8200"
	}
	token := os.Getenv("VAULT_TEST_TOKEN")
	if token == "" {
		token = "root-token"
	}
	return &TestEnv{
		Type:       "vault",
		Address:    addr,
		Token:      token,
		BinaryPath: "vault",
	}
}

// OpenBaoTestEnv returns a test environment for OpenBao.
func OpenBaoTestEnv() *TestEnv {
	addr := os.Getenv("OPENBAO_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8210"
	}
	token := os.Getenv("OPENBAO_TEST_TOKEN")
	if token == "" {
		token = "root-token"
	}
	return &TestEnv{
		Type:       "openbao",
		Address:    addr,
		Token:      token,
		BinaryPath: "bao",
	}
}

// IsAvailable checks if the test environment is available.
func (e *TestEnv) IsAvailable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(e.Address + "/v1/sys/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// Vault returns 200 for initialized+unsealed, 429 for standby, 472 for DR secondary
	// All are acceptable for our tests
	return resp.StatusCode == 200 || resp.StatusCode == 429 || resp.StatusCode == 472
}

// WaitForReady waits for the test environment to be ready.
func (e *TestEnv) WaitForReady(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s not ready: %w", e.Type, ctx.Err())
		case <-ticker.C:
			if e.IsAvailable() {
				return nil
			}
		}
	}
}

// SkipIfNotAvailable skips the test if the environment is not available.
func (e *TestEnv) SkipIfNotAvailable(t *testing.T) {
	t.Helper()
	if !e.IsAvailable() {
		t.Skipf("%s test environment not available at %s", e.Type, e.Address)
	}
}

// SkipIfBinaryMissing skips the test if the CLI binary is not available.
func (e *TestEnv) SkipIfBinaryMissing(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(e.BinaryPath); err != nil {
		t.Skipf("%s binary not found in PATH", e.BinaryPath)
	}
}

// RunCLI runs a CLI command against the test environment.
func (e *TestEnv) RunCLI(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, e.BinaryPath, args...)
	cmd.Env = append(os.Environ(),
		"VAULT_ADDR="+e.Address,
		"VAULT_TOKEN="+e.Token,
		"BAO_ADDR="+e.Address,
		"BAO_TOKEN="+e.Token,
	)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// CreateTestToken creates a test token with specific TTL.
func (e *TestEnv) CreateTestToken(ctx context.Context, ttl string, renewable bool) (string, error) {
	// Include "default" policy so the token can look itself up (required for vault login)
	args := []string{"token", "create", "-format=json", "-ttl=" + ttl, "-policy=default"}
	if !renewable {
		args = append(args, "-renewable=false")
	}

	stdout, stderr, err := e.RunCLI(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %s: %w", stderr, err)
	}

	// Parse token from JSON output
	// Simple extraction without full JSON parsing
	tokenStart := strings.Index(stdout, `"client_token"`)
	if tokenStart == -1 {
		return "", fmt.Errorf("client_token not found in output: %s", stdout)
	}

	// Find the token value
	valueStart := strings.Index(stdout[tokenStart:], `":"`) + tokenStart + 3
	valueEnd := strings.Index(stdout[valueStart:], `"`) + valueStart

	if valueStart >= valueEnd {
		return "", fmt.Errorf("failed to parse token from output: %s", stdout)
	}

	return stdout[valueStart:valueEnd], nil
}

// EnableUserpass enables the userpass auth method for testing.
func (e *TestEnv) EnableUserpass(ctx context.Context) error {
	_, stderr, err := e.RunCLI(ctx, "auth", "enable", "userpass")
	if err != nil {
		// Ignore "already enabled" errors
		if strings.Contains(stderr, "already in use") || strings.Contains(stderr, "path is already in use") {
			return nil
		}
		return fmt.Errorf("failed to enable userpass: %s: %w", stderr, err)
	}
	return nil
}

// CreateUserpassUser creates a userpass user for testing.
func (e *TestEnv) CreateUserpassUser(ctx context.Context, username, password string) error {
	_, stderr, err := e.RunCLI(ctx, "write",
		fmt.Sprintf("auth/userpass/users/%s", username),
		fmt.Sprintf("password=%s", password),
		"policies=default",
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %s: %w", stderr, err)
	}
	return nil
}

// PatrolBinaryPath returns the path to the patrol binary.
func PatrolBinaryPath(t *testing.T) string {
	t.Helper()

	// Check if PATROL_BINARY is set
	if path := os.Getenv("PATROL_BINARY"); path != "" {
		return path
	}

	// Try to find it relative to the test directory
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get caller information")
	}

	// Go up from test/integration to project root
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	binaryPath := filepath.Join(projectRoot, "bin", "patrol")

	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("patrol binary not found at %s - run 'make build' first", binaryPath)
	}

	return binaryPath
}

// RunPatrol runs the patrol CLI with the given arguments.
func RunPatrol(ctx context.Context, t *testing.T, env *TestEnv, args ...string) (string, string, error) {
	t.Helper()

	binaryPath := PatrolBinaryPath(t)
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	// Create isolated test environment
	tmpDir := t.TempDir()
	keyringDir := filepath.Join(tmpDir, "keyring")
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	// Create a profile config for patrol
	configDir := filepath.Join(tmpDir, ".config", "patrol")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configContent := `current: test
connections:
  - name: test
    address: ` + env.Address + `
    type: vault
`
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cmd.Env = append(os.Environ(),
		"HOME="+tmpDir,
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, ".config"),
		"PATROL_TEST_KEYRING_DIR="+keyringDir, // Use file-based keyring for tests
	)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TempConfigDir creates a temporary config directory for testing.
func TempConfigDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "patrol-config")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("failed to create temp config dir: %v", err)
	}
	return dir
}
