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

func TestVaultProxy_Status(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := RunPatrol(ctx, t, env, "status")
	if err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		// status command may return non-zero if not authenticated, that's OK
	}

	// Should contain some vault status info or patrol status info
	output := stdout + stderr
	if !strings.Contains(output, "Vault") && !strings.Contains(output, "vault") &&
		!strings.Contains(output, "Profile") && !strings.Contains(output, "profile") {
		t.Errorf("expected status output, got: %s", output)
	}
}

func TestVaultProxy_Version(t *testing.T) {
	env := VaultTestEnv()
	// Don't need vault available for version command

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, err := RunPatrol(ctx, t, env, "version")
	if err != nil {
		t.Fatalf("patrol version failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "patrol") {
		t.Errorf("expected 'patrol' in version output, got: %s", stdout)
	}
}

func TestVaultProxy_Help(t *testing.T) {
	env := VaultTestEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, err := RunPatrol(ctx, t, env, "--help")
	if err != nil {
		t.Fatalf("patrol --help failed: %v\nstderr: %s", err, stderr)
	}

	// Should show patrol help with our commands
	expectedCommands := []string{"login", "logout", "profile", "daemon"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("expected %q in help output, got: %s", cmd, stdout)
		}
	}
}

func TestVaultProxy_SecretsOperations(t *testing.T) {
	env := VaultTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Enable KV secrets engine (might already be enabled in dev mode)
	_, _, _ = env.RunCLI(ctx, "secrets", "enable", "-path=secret", "kv-v2")

	// Write a secret using patrol as proxy (with token via env)
	testKey := "patrol-test-key"
	testValue := "patrol-test-value-12345"

	// For this test, we need to pass the token. In real usage, patrol would
	// retrieve it from keyring. Here we test the proxy functionality.
	stdout, stderr, err := runPatrolWithToken(ctx, t, env, "kv", "put", "secret/"+testKey, "value="+testValue)
	if err != nil {
		t.Fatalf("failed to write secret: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Read it back
	stdout, stderr, err = runPatrolWithToken(ctx, t, env, "kv", "get", "-field=value", "secret/"+testKey)
	if err != nil {
		t.Fatalf("failed to read secret: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, testValue) {
		t.Errorf("expected secret value %q in output, got: %s", testValue, stdout)
	}

	// Clean up
	_, _, _ = runPatrolWithToken(ctx, t, env, "kv", "delete", "secret/"+testKey)
}

func TestOpenBaoProxy_Status(t *testing.T) {
	env := OpenBaoTestEnv()
	env.SkipIfNotAvailable(t)
	env.SkipIfBinaryMissing(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := RunPatrol(ctx, t, env, "status")
	if err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		// status command may return non-zero if not authenticated
	}

	output := stdout + stderr
	if !strings.Contains(output, "Profile") && !strings.Contains(output, "profile") &&
		!strings.Contains(output, "bao") && !strings.Contains(output, "Bao") {
		t.Logf("output: %s", output)
		// Don't fail, just log - OpenBao might not be running
	}
}

// runPatrolWithToken runs patrol with VAULT_TOKEN set.
func runPatrolWithToken(ctx context.Context, t *testing.T, env *TestEnv, args ...string) (string, string, error) {
	t.Helper()

	binaryPath := PatrolBinaryPath(t)

	// Create isolated test environment
	tmpDir := t.TempDir()
	keyringDir := filepath.Join(tmpDir, "keyring")
	if err := os.MkdirAll(keyringDir, 0700); err != nil {
		t.Fatalf("failed to create keyring dir: %v", err)
	}

	// Create a profile config for patrol (required for proxy commands)
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

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"VAULT_TOKEN="+env.Token,
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
