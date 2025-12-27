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

// TestToken_InfoAfterLogin tests that patrol profile status works after login.
func TestToken_InfoAfterLogin(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Enable userpass for reliable authentication
	if err := env.EnableUserpass(ctx); err != nil {
		t.Fatalf("failed to enable userpass: %v", err)
	}

	testUser := "token-info-user"
	testPass := "token-info-pass-12345"

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

	// Create a profile config
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
		t.Fatalf("login failed: %v\nstderr: %s", err, stderr.String())
	}

	// Step 2: Run patrol profile status (includes token info)
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "status")
	cmd.Env = testEnv
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("profile status failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("profile status output: %s", output)

	// Verify output contains expected fields
	expectedFields := []string{"accessor", "policies", "ttl"}
	for _, field := range expectedFields {
		if !strings.Contains(strings.ToLower(output), field) {
			t.Errorf("expected %q in token info output", field)
		}
	}
}

// TestToken_RenewAfterLogin tests that patrol profile renew works after login.
func TestToken_RenewAfterLogin(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Enable userpass for reliable authentication
	if err := env.EnableUserpass(ctx); err != nil {
		t.Fatalf("failed to enable userpass: %v", err)
	}

	testUser := "token-renew-user"
	testPass := "token-renew-pass-12345"

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

	// Step 2: Run patrol profile renew
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "renew")
	cmd.Env = testEnv
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("profile renew failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("profile renew output: %s", output)

	// Verify renewal was successful
	if !strings.Contains(output, "renewed") && !strings.Contains(output, "Renewed") &&
		!strings.Contains(output, "success") && !strings.Contains(output, "Success") {
		t.Errorf("expected renewal success message, got: %s", output)
	}
}

// TestToken_InfoWithoutLogin tests that token info fails gracefully when not logged in.
func TestToken_InfoWithoutLogin(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment (no login)
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

	// Run profile status without login - should show no token
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "status")
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	t.Logf("profile status (no login) output: %s", output)

	// Should fail or show "not logged in" / "no token" message
	if err == nil {
		outputLower := strings.ToLower(output)
		if !strings.Contains(outputLower, "not logged in") &&
			!strings.Contains(outputLower, "no token") &&
			!strings.Contains(outputLower, "not stored") {
			t.Error("expected error or 'not logged in' / 'no token' message when not authenticated")
		}
	}
}

// TestVaultCommand_WithStoredToken tests that vault commands work with token from keyring.
func TestVaultCommand_WithStoredToken(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Enable userpass
	if err := env.EnableUserpass(ctx); err != nil {
		t.Fatalf("failed to enable userpass: %v", err)
	}

	testUser := "vault-cmd-user"
	testPass := "vault-cmd-pass-12345"

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

	// Step 2: Write to cubbyhole (default policy allows this)
	// Note: We don't pass VAULT_TOKEN - patrol should inject it from keyring
	testEnvNoToken := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"PATROL_TEST_KEYRING_DIR="+keyringDir,
		// Explicitly NOT setting VAULT_TOKEN
	)

	// Write a secret to cubbyhole (allowed by default policy)
	cmd = exec.CommandContext(ctx, binaryPath, "write", "cubbyhole/test-secret", "value=test-value-12345")
	cmd.Env = testEnvNoToken
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("cubbyhole write failed: %v\nstderr: %s", err, stderr.String())
	}

	// Step 3: Read it back to verify token injection worked
	cmd = exec.CommandContext(ctx, binaryPath, "read", "-field=value", "cubbyhole/test-secret")
	cmd.Env = testEnvNoToken
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("cubbyhole read failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("cubbyhole read output: %s", output)

	// Verify we got the value back
	if !strings.Contains(output, "test-value-12345") {
		t.Errorf("expected 'test-value-12345' in output, got: %s", output)
	}
}
