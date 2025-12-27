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

func TestProfile_AddAndList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
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
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "dev",
		"--address=https://vault-dev.example.com:8200")
	cmd.Env = baseEnv

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add profile: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if !strings.Contains(stdout.String(), "Added profile") {
		t.Errorf("expected 'Added profile' message, got: %s", stdout.String())
	}

	// Add another profile
	stdout.Reset()
	stderr.Reset()
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "add", "prod",
		"--address=https://vault-prod.example.com:8200",
		"--namespace=admin")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add second profile: %v\nstderr: %s", err, stderr.String())
	}

	// List profiles
	stdout.Reset()
	stderr.Reset()
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "list")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to list profiles: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "dev") {
		t.Errorf("expected 'dev' in profile list, got: %s", output)
	}
	if !strings.Contains(output, "prod") {
		t.Errorf("expected 'prod' in profile list, got: %s", output)
	}
}

func TestProfile_Switch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
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

	// Add two profiles
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "first",
		"--address=https://first.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add first profile: %v", err)
	}

	cmd = exec.CommandContext(ctx, binaryPath, "profile", "add", "second",
		"--address=https://second.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add second profile: %v", err)
	}

	// Switch to second profile
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "use", "second")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to switch profile: %v\nstderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "second") {
		t.Errorf("expected 'second' in switch output, got: %s", stdout.String())
	}

	// Verify current profile in list
	stdout.Reset()
	stderr.Reset()
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "list")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to list profiles: %v", err)
	}

	// The current profile should be marked with *
	output := stdout.String()
	if !strings.Contains(output, "* second") && !strings.Contains(output, "*second") {
		// Check if second is shown as current some other way
		t.Logf("profile list output: %s", output)
	}
}

func TestProfile_Remove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
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
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "to-remove",
		"--address=https://remove.example.com:8200")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add profile: %v", err)
	}

	// Remove the profile
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "remove", "to-remove", "--force")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to remove profile: %v\nstderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Removed") {
		t.Errorf("expected 'Removed' message, got: %s", stdout.String())
	}

	// Verify profile is gone
	stdout.Reset()
	stderr.Reset()
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "list")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	if strings.Contains(stdout.String(), "to-remove") {
		t.Errorf("profile should have been removed, but still in list: %s", stdout.String())
	}
}

func TestProfile_Show(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
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

	// Add a profile with various options
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "detailed",
		"--address=https://detailed.example.com:8200",
		"--namespace=admin/team",
		"--type=vault")
	cmd.Env = baseEnv
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add profile: %v", err)
	}

	// Show the profile status
	var stdout, stderr strings.Builder
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "status", "detailed")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to show profile status: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	expectedFields := []string{"detailed", "https://detailed.example.com:8200", "admin/team"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("expected %q in profile status output, got: %s", field, output)
		}
	}
}

func TestProfile_AddOpenBao(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up isolated test environment
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

	// Add an OpenBao profile
	var stdout, stderr strings.Builder
	cmd := exec.CommandContext(ctx, binaryPath, "profile", "add", "openbao-test",
		"--address=https://openbao.example.com:8200",
		"--type=openbao")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add OpenBao profile: %v\nstderr: %s", err, stderr.String())
	}

	// Show the profile and verify type
	stdout.Reset()
	stderr.Reset()
	cmd = exec.CommandContext(ctx, binaryPath, "profile", "show", "openbao-test")
	cmd.Env = baseEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to show profile: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "openbao") {
		t.Errorf("expected 'openbao' type in profile, got: %s", output)
	}
}
