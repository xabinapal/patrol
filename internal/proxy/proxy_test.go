package proxy

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
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
	conn := &config.Connection{
		Type:       config.BinaryTypeVault,
		Address:    "https://vault.example.com:8200",
		BinaryPath: "",
	}

	// Test with real command runner (default behavior)
	// This might return true or false depending on whether vault is installed
	_ = BinaryExists(conn)

	// Test with mocked command runner
	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathFunc(func(file string) (string, error) {
		return file, nil
	})
	result := BinaryExists(conn, WithCommandRunner(mockRunner))
	if !result {
		t.Error("BinaryExists() with mock runner = false, want true")
	}

	// Test with mocked command runner that returns error
	mockRunner2 := newMockCommandRunner()
	mockRunner2.setLookPathFunc(func(file string) (string, error) {
		return "", exec.ErrNotFound
	})
	result2 := BinaryExists(conn, WithCommandRunner(mockRunner2))
	if result2 {
		t.Error("BinaryExists() with mock runner returning error = true, want false")
	}
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
	exitCode, err := exec.Execute(ctx, []string{"hello", "world"}, nil)

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
	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"captured", "output"}, &captureBuf)

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0", exitCode)
	}

	output := strings.TrimSpace(captureBuf.String())
	if output != "captured output" {
		t.Errorf("Execute() captured = %q, want %q", output, "captured output")
	}
}

func TestExecuteStreamsInRealTime(t *testing.T) {
	conn := &config.Connection{
		Address:    "https://vault.example.com:8200",
		BinaryPath: "vault",
	}

	mockRunner := newMockCommandRunner()
	mockRunner.setLookPathFunc(func(file string) (string, error) {
		return file, nil
	})

	// Use channels to verify streaming happens in real-time
	stdoutLines := make(chan string, 10)
	stderrLines := make(chan string, 10)

	stdout := &streamingWriter{lines: stdoutLines}
	stderr := &streamingWriter{lines: stderrLines}

	exec := NewExecutor(conn, WithStdout(stdout), WithStderr(stderr), WithCommandRunner(mockRunner))

	ctx := context.Background()
	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"line1", "line2"}, &captureBuf)

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0", exitCode)
	}

	// Verify both stdout and stderr were streamed
	close(stdoutLines)
	close(stderrLines)

	stdoutCount := 0
	for range stdoutLines {
		stdoutCount++
	}

	stderrCount := 0
	for range stderrLines {
		stderrCount++
	}

	if stdoutCount == 0 && stderrCount == 0 {
		t.Error("Execute() should have streamed output to stdout or stderr")
	}

	// Verify capture buffer contains the output
	captured := strings.TrimSpace(captureBuf.String())
	if captured == "" {
		t.Error("Execute() should have captured output")
	}
}

// streamingWriter writes to a channel for each line to simulate real-time streaming
type streamingWriter struct {
	lines chan<- string
	buf   []byte
}

func (w *streamingWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx == -1 {
			break
		}
		line := string(w.buf[:idx])
		w.lines <- line
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
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
	_, err := exec.Execute(ctx, []string{"arg"}, nil)

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
	exitCode, err := exec.Execute(ctx, []string{}, nil)

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
