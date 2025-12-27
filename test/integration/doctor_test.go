//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDoctor_Basic tests that patrol doctor runs and produces output.
func TestDoctor_Basic(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	keyringDir := filepath.Join(homeDir, "keyring")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	configContent := `current: test
connections:
  - name: test
    address: ` + env.Address + `
    type: vault
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	testEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"PATROL_TEST_KEYRING_DIR="+keyringDir,
	)

	binaryPath := PatrolBinaryPath(t)

	cmd := exec.CommandContext(ctx, binaryPath, "doctor")
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Doctor may return non-zero if issues are found, that's OK
	_ = cmd.Run()

	output := stdout.String()
	t.Logf("doctor output:\n%s", output)

	// Doctor should check various components
	expectedChecks := []string{"Profile", "Vault", "Keyring"}
	for _, check := range expectedChecks {
		if !strings.Contains(output, check) {
			t.Errorf("expected doctor to check %q", check)
		}
	}

	// Should show status indicators
	if !strings.Contains(output, "OK") && !strings.Contains(output, "WARN") &&
		!strings.Contains(output, "ERROR") && !strings.Contains(output, "SKIP") {
		t.Error("expected doctor to show status indicators")
	}
}

// TestDoctor_WithLogin tests doctor output when logged in.
func TestDoctor_WithLogin(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Enable userpass
	if err := env.EnableUserpass(ctx); err != nil {
		t.Fatalf("failed to enable userpass: %v", err)
	}

	testUser := "doctor-test-user"
	testPass := "doctor-test-pass-12345"

	if err := env.CreateUserpassUser(ctx, testUser, testPass); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Set up isolated test environment
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	keyringDir := filepath.Join(homeDir, "keyring")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	configContent := `current: test
connections:
  - name: test
    address: ` + env.Address + `
    type: vault
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	testEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"PATROL_TEST_KEYRING_DIR="+keyringDir,
	)

	binaryPath := PatrolBinaryPath(t)

	// Step 1: Login
	cmd := exec.CommandContext(ctx, binaryPath, "login",
		"-method=userpass",
		"username="+testUser,
		"password="+testPass,
	)
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "keyring") {
			t.Skip("keyring not available")
		}
		t.Fatalf("login failed: %v", err)
	}

	// Step 2: Run doctor
	cmd = exec.CommandContext(ctx, binaryPath, "doctor")
	cmd.Env = testEnv
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	output := stdout.String()
	t.Logf("doctor (logged in) output:\n%s", output)

	// When logged in, Token check should be present and show valid
	if !strings.Contains(output, "Token") {
		t.Error("expected doctor to check Token when logged in")
	}
}

// TestDoctor_JSONOutput tests doctor with JSON output format.
func TestDoctor_JSONOutput(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	keyringDir := filepath.Join(homeDir, "keyring")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	configContent := `current: test
connections:
  - name: test
    address: ` + env.Address + `
    type: vault
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	testEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"PATROL_TEST_KEYRING_DIR="+keyringDir,
	)

	binaryPath := PatrolBinaryPath(t)

	cmd := exec.CommandContext(ctx, binaryPath, "doctor", "-o", "json")
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	output := stdout.String()
	t.Logf("doctor JSON output:\n%s", output)

	// JSON output should be valid JSON structure
	if !strings.HasPrefix(strings.TrimSpace(output), "{") && !strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Errorf("expected JSON output, got: %s", output)
	}

	// Should contain expected fields
	if !strings.Contains(output, "status") {
		t.Error("expected 'status' field in JSON output")
	}
}
