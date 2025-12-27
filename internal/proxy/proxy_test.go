package proxy

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xabinapal/patrol/internal/config"
)

func TestSetEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		key      string
		value    string
		expected []string
	}{
		{
			name:     "add new var",
			env:      []string{"FOO=bar"},
			key:      "BAZ",
			value:    "qux",
			expected: []string{"FOO=bar", "BAZ=qux"},
		},
		{
			name:     "replace existing var",
			env:      []string{"FOO=bar", "BAZ=old"},
			key:      "BAZ",
			value:    "new",
			expected: []string{"FOO=bar", "BAZ=new"},
		},
		{
			name:     "empty env",
			env:      []string{},
			key:      "FOO",
			value:    "bar",
			expected: []string{"FOO=bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setEnv(tt.env, tt.key, tt.value)

			if len(result) != len(tt.expected) {
				t.Errorf("setEnv() returned %d items, want %d", len(result), len(tt.expected))
				return
			}

			// Check that expected values are present
			for _, exp := range tt.expected {
				found := false
				for _, r := range result {
					if r == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("setEnv() missing expected value %s", exp)
				}
			}
		})
	}
}

func TestBuildEnvironment(t *testing.T) {
	conn := &config.Connection{
		Address:       "https://vault.example.com:8200",
		Namespace:     "team1",
		TLSSkipVerify: true,
		CACert:        "/path/to/ca.crt",
	}

	exec := NewExecutor(conn, WithToken("test-token"))

	// Access buildEnvironment through a test that uses Execute
	env := exec.buildEnvironment()

	// Check that expected vars are set
	expectedVars := map[string]string{
		"VAULT_ADDR":        "https://vault.example.com:8200",
		"VAULT_TOKEN":       "test-token",
		"VAULT_NAMESPACE":   "team1",
		"VAULT_SKIP_VERIFY": "true",
		"VAULT_CACERT":      "/path/to/ca.crt",
	}

	for key, expectedVal := range expectedVars {
		found := false
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				val := strings.TrimPrefix(e, key+"=")
				if val != expectedVal {
					t.Errorf("env %s = %s, want %s", key, val, expectedVal)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("env %s not found", key)
		}
	}
}

func TestExecutorOptions(t *testing.T) {
	conn := &config.Connection{
		Address: "https://vault.example.com:8200",
	}

	stdin := bytes.NewBufferString("input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exec := NewExecutor(conn,
		WithToken("test-token"),
		WithStdin(stdin),
		WithStdout(stdout),
		WithStderr(stderr),
		WithEnviron([]string{"CUSTOM_VAR=value"}),
	)

	if exec.token != "test-token" {
		t.Errorf("token = %s, want test-token", exec.token)
	}

	if exec.stdin != stdin {
		t.Error("stdin not set correctly")
	}

	if exec.stdout != stdout {
		t.Error("stdout not set correctly")
	}

	if exec.stderr != stderr {
		t.Error("stderr not set correctly")
	}

	if len(exec.environ) != 1 || exec.environ[0] != "CUSTOM_VAR=value" {
		t.Errorf("environ = %v, want [CUSTOM_VAR=value]", exec.environ)
	}
}

func TestBinaryExists(t *testing.T) {
	// Note: BinaryExists uses exec.LookPath which we can't easily mock
	// without refactoring. This test verifies the function doesn't panic.
	conn := &config.Connection{
		Type:       config.BinaryTypeVault,
		Address:    "https://vault.example.com:8200",
		BinaryPath: "",
	}

	// This might return true or false depending on whether vault is installed
	// We just test it doesn't panic
	_ = BinaryExists(conn)
}

func TestExecuteWithEcho(t *testing.T) {
	conn := &config.Connection{
		Address:    "https://vault.example.com:8200",
		BinaryPath: "vault",
	}

	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathFunc(func(file string) (string, error) {
		return file, nil
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exec := NewExecutor(conn, WithStdout(stdout), WithStderr(stderr), WithCommandRunner(mockRunner))

	ctx := context.Background()
	exitCode, err := exec.Execute(ctx, []string{"hello", "world"})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "hello world" {
		t.Errorf("Execute() output = %q, want %q", output, "hello world")
	}
}

func TestExecuteCaptureWithEcho(t *testing.T) {
	conn := &config.Connection{
		Address:    "https://vault.example.com:8200",
		BinaryPath: "vault",
	}

	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathFunc(func(file string) (string, error) {
		return file, nil
	})

	exec := NewExecutor(conn, WithCommandRunner(mockRunner))

	ctx := context.Background()
	stdout, stderr, exitCode, err := exec.ExecuteCapture(ctx, []string{"captured", "output"})

	if err != nil {
		t.Fatalf("ExecuteCapture() failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("ExecuteCapture() exit code = %d, want 0", exitCode)
	}

	stdoutStr := strings.TrimSpace(string(stdout))
	if stdoutStr != "captured output" {
		t.Errorf("ExecuteCapture() stdout = %q, want %q", stdoutStr, "captured output")
	}

	if len(stderr) != 0 {
		t.Errorf("ExecuteCapture() stderr = %q, want empty", string(stderr))
	}
}

func TestExecuteNonExistentBinary(t *testing.T) {
	conn := &config.Connection{
		Address:    "https://vault.example.com:8200",
		BinaryPath: "/definitely/not/a/real/path/vault",
	}

	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathError(errors.New("executable file not found in $PATH"))

	exec := NewExecutor(conn, WithCommandRunner(mockRunner))

	ctx := context.Background()
	_, err := exec.Execute(ctx, []string{"arg"})

	if err == nil {
		t.Error("Execute() should fail for non-existent binary")
	}
}

func TestExecuteWithEnvInjection(t *testing.T) {
	conn := &config.Connection{
		Address:    "https://vault.test:8200",
		Namespace:  "testns",
		BinaryPath: "vault",
	}

	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathFunc(func(file string) (string, error) {
		return file, nil
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exec := NewExecutor(conn, WithToken("hvs.testtoken"), WithStdout(stdout), WithStderr(stderr), WithCommandRunner(mockRunner))

	ctx := context.Background()
	exitCode, err := exec.Execute(ctx, []string{})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0", exitCode)
	}

	output := stdout.String()

	// Check that our injected vars are present
	expectedVars := []string{
		"VAULT_ADDR=https://vault.test:8200",
		"VAULT_TOKEN=hvs.testtoken",
		"VAULT_NAMESPACE=testns",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(output, expected) {
			t.Errorf("Execute() output missing %q", expected)
		}
	}
}
