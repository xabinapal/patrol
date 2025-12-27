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

// TestLogin_InvalidCredentials tests that login fails with invalid credentials.
func TestLogin_InvalidCredentials(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Enable userpass
	if err := env.EnableUserpass(ctx); err != nil {
		t.Fatalf("failed to enable userpass: %v", err)
	}

	// Create a valid user (so userpass is enabled)
	if err := env.CreateUserpassUser(ctx, "valid-user", "valid-pass"); err != nil {
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

	// Try to login with invalid credentials
	cmd := exec.CommandContext(ctx, binaryPath, "login",
		"-method=userpass",
		"username=nonexistent-user",
		"password=wrong-password",
	)
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should fail
	if err == nil {
		t.Error("login should fail with invalid credentials")
	}

	output := stdout.String() + stderr.String()
	t.Logf("invalid login output: %s", output)

	// Should show authentication error
	if !strings.Contains(strings.ToLower(output), "error") &&
		!strings.Contains(strings.ToLower(output), "denied") &&
		!strings.Contains(strings.ToLower(output), "invalid") {
		t.Errorf("expected error message for invalid credentials, got: %s", output)
	}
}

// TestLogin_UnreachableServer tests login to an unreachable server.
func TestLogin_UnreachableServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Set up isolated test environment with unreachable address
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	keyringDir := filepath.Join(homeDir, "keyring")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	// Use an address that won't exist
	configContent := `current: test
connections:
  - name: test
    address: http://127.0.0.1:59999
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

	cmd := exec.CommandContext(ctx, binaryPath, "login",
		"-method=userpass",
		"username=test",
		"password=test",
	)
	cmd.Env = testEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should fail
	if err == nil {
		t.Error("login should fail with unreachable server")
	}

	output := stdout.String() + stderr.String()
	t.Logf("unreachable server output: %s", output)

	// Should show connection error
	if !strings.Contains(strings.ToLower(output), "connection") &&
		!strings.Contains(strings.ToLower(output), "refused") &&
		!strings.Contains(strings.ToLower(output), "error") {
		t.Errorf("expected connection error, got: %s", output)
	}
}

// TestProfile_AddInvalidAddress tests adding a profile with invalid address.
func TestProfile_AddInvalidAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	binaryPath := PatrolBinaryPath(t)
	baseEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
	)

	// Try to add profile with invalid address (no scheme)
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "invalid",
		"--address=not-a-valid-url")
	cmd.Env = baseEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	t.Logf("invalid address output: %s", output)

	// Should fail or warn about invalid address
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "invalid") &&
			!strings.Contains(strings.ToLower(output), "error") {
			t.Error("expected error for invalid address")
		}
	}
}

// TestProfile_AddDuplicate tests adding a duplicate profile name.
func TestProfile_AddDuplicate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	binaryPath := PatrolBinaryPath(t)
	baseEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
	)

	// Add first profile
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "duplicate-test",
		"--address=https://first.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add first profile: %v", err)
	}

	// Try to add profile with same name
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "add", "duplicate-test",
		"--address=https://second.example.com:8200")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	t.Logf("duplicate profile output: %s", output)

	// Should fail
	if err == nil {
		t.Error("adding duplicate profile should fail")
	}

	// Should mention duplicate or already exists
	if !strings.Contains(strings.ToLower(output), "exist") &&
		!strings.Contains(strings.ToLower(output), "duplicate") &&
		!strings.Contains(strings.ToLower(output), "already") {
		t.Errorf("expected duplicate/exists error, got: %s", output)
	}
}

// TestProfile_UseNonexistent tests switching to a nonexistent profile.
func TestProfile_UseNonexistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	binaryPath := PatrolBinaryPath(t)
	baseEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
	)

	// Add a profile first
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "exists",
		"--address=https://exists.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add profile: %v", err)
	}

	// Try to switch to nonexistent profile
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "use", "nonexistent")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	t.Logf("use nonexistent output: %s", output)

	// Should fail
	if err == nil {
		t.Error("switching to nonexistent profile should fail")
	}

	// Should mention not found
	if !strings.Contains(strings.ToLower(output), "not found") &&
		!strings.Contains(strings.ToLower(output), "does not exist") &&
		!strings.Contains(strings.ToLower(output), "no profile") {
		t.Errorf("expected 'not found' error, got: %s", output)
	}
}

// TestLogout_WhenNotLoggedIn tests logout when not logged in.
func TestLogout_WhenNotLoggedIn(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment (NOT logged in)
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

	// Logout when not logged in
	var stdout, stderr strings.Builder
	cmd := exec.CommandContext(ctx, binaryPath, "logout")
	cmd.Env = testEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	t.Logf("logout (not logged in) output: %s", output)

	// Should succeed or show "no token" message (not an error crash)
	if err != nil {
		// If it fails, should be a graceful failure
		if !strings.Contains(strings.ToLower(output), "no token") &&
			!strings.Contains(strings.ToLower(output), "not logged in") {
			t.Errorf("logout without login should handle gracefully, got error: %v, output: %s", err, output)
		}
	} else {
		// If it succeeds, should mention no token
		if !strings.Contains(strings.ToLower(output), "no token") &&
			!strings.Contains(strings.ToLower(output), "not logged") {
			t.Logf("logout without login succeeded without message: %s", output)
		}
	}
}

// TestStatus_JSONOutput tests status command with JSON output.
func TestStatus_JSONOutput(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	var stdout, stderr strings.Builder
	cmd := exec.CommandContext(ctx, binaryPath, "status", "-o", "json")
	cmd.Env = testEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // May return non-zero if not logged in

	output := stdout.String()
	t.Logf("status JSON output: %s", output)

	// Should be valid JSON
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("expected JSON output starting with '{', got: %s", output)
	}

	// Should contain profile info
	if !strings.Contains(output, "profile") {
		t.Error("expected 'profile' in JSON output")
	}
}

// TestProfile_ListJSONOutput tests profile list with JSON output.
func TestProfile_ListJSONOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "patrol")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	binaryPath := PatrolBinaryPath(t)
	baseEnv := append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
	)

	// Add a profile
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "json-test",
		"--address=https://json.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add profile: %v", err)
	}

	// List profiles with JSON output
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "list", "-o", "json")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("profile list failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("profile list JSON output: %s", output)

	// Should be valid JSON array or object
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
		t.Errorf("expected JSON output, got: %s", output)
	}

	// Should contain our profile
	if !strings.Contains(output, "json-test") {
		t.Error("expected 'json-test' in JSON output")
	}
}
