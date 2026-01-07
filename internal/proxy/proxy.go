// Package proxy handles execution of Vault/OpenBao CLI commands.
package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/utils"
)

// Executor handles proxying commands to the Vault/OpenBao CLI.
type Executor struct {
	conn          *config.Connection
	token         string
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	environ       []string
	commandRunner CommandRunner
}

// Option configures an Executor.
type Option func(*Executor)

// WithToken sets the Vault token to use.
func WithToken(token string) Option {
	return func(e *Executor) {
		e.token = token
	}
}

// WithStdin sets the stdin reader.
func WithStdin(r io.Reader) Option {
	return func(e *Executor) {
		e.stdin = r
	}
}

// WithStdout sets the stdout writer.
func WithStdout(w io.Writer) Option {
	return func(e *Executor) {
		e.stdout = w
	}
}

// WithStderr sets the stderr writer.
func WithStderr(w io.Writer) Option {
	return func(e *Executor) {
		e.stderr = w
	}
}

// WithEnviron sets additional environment variables.
func WithEnviron(env []string) Option {
	return func(e *Executor) {
		e.environ = env
	}
}

// WithCommandRunner sets a custom command runner (for testing).
func WithCommandRunner(runner CommandRunner) Option {
	return func(e *Executor) {
		e.commandRunner = runner
	}
}

// NewExecutor creates a new Executor for the given connection.
// By default, output is discarded (silent). Use WithStdout/WithStderr
// to stream output when needed (e.g., for proxy mode or interactive commands).
func NewExecutor(conn *config.Connection, opts ...Option) *Executor {
	e := &Executor{
		conn:          conn,
		stdin:         os.Stdin,
		stdout:        io.Discard,
		stderr:        io.Discard,
		commandRunner: NewCommandRunner(),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// orderedWriter writes to both a primary writer and a capture buffer.
// It uses a mutex to ensure ordered writes to the capture buffer when
// multiple goroutines are writing (stdout and stderr).
type orderedWriter struct {
	writer  io.Writer
	capture *bytes.Buffer
	mu      *sync.Mutex
}

func (w *orderedWriter) Write(p []byte) (n int, err error) {
	// Always write to the primary writer first (non-blocking)
	n, err = w.writer.Write(p)
	if err != nil {
		return n, err
	}

	// Then write to capture buffer with mutex to maintain order
	if w.capture != nil {
		w.mu.Lock()
		_, _ = w.capture.Write(p) // Ignore errors, best effort
		w.mu.Unlock()
	}

	return n, err
}

// Execute runs a Vault/OpenBao command with the given arguments.
// Output is always streamed to the configured stdout/stderr in real-time.
// If capture is provided, all output (stdout and stderr) is also captured
// to that buffer in order for parsing (e.g., JSON responses).
func (e *Executor) Execute(ctx context.Context, args []string, capture *bytes.Buffer) (int, error) {
	// Security: Validate the connection before executing
	if err := e.conn.Validate(); err != nil {
		return 1, fmt.Errorf("connection validation failed: %w", err)
	}

	// Security: Warn about TLS skip verify
	if e.conn.TLSSkipVerify {
		fmt.Fprintln(e.stderr, "WARNING: TLS certificate verification is disabled. This is insecure and should only be used for development.")
	}

	binary := e.conn.GetBinaryPath()

	// Check if the binary exists
	binaryPath, err := e.commandRunner.LookPath(binary)
	if err != nil {
		return 1, fmt.Errorf("vault/openbao binary %q not found: %w\nPlease install Vault/OpenBao or configure the binary path in your Patrol config", binary, err)
	}

	// Build the command
	// #nosec G204 - binaryPath is validated via LookPath, args are user-controlled but passed to vault/openbao CLI
	cmd := e.commandRunner.CommandContext(ctx, binaryPath, args...)

	// Set up environment
	cmd.SetEnv(e.buildEnvironment())

	// Connect stdin if provided
	if e.stdin != nil {
		cmd.SetStdin(e.stdin)
	}

	// Set up signal forwarding
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	if capture == nil {
		// Simple case: no capture needed, stream directly
		cmd.SetStdout(e.stdout)
		cmd.SetStderr(e.stderr)

		// Start the command
		if startErr := cmd.Start(); startErr != nil {
			return 1, fmt.Errorf("failed to start %s: %w", binary, startErr)
		}

		// Forward signals to the child process
		go func() {
			for sig := range sigChan {
				proc := cmd.Process()
				if proc != nil {
					// Ignore signal errors - process may have already exited
					_ = proc.Signal(sig) //nolint:errcheck // Signal errors are non-fatal
				}
			}
		}()

		// Wait for completion
		waitErr := cmd.Wait()

		// Get exit code
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return 1, fmt.Errorf("failed to execute %s: %w", binary, waitErr)
		}

		return 0, nil
	}

	// Capture case: use pipes to stream and optionally capture
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if startErr := cmd.Start(); startErr != nil {
		return 1, fmt.Errorf("failed to start %s: %w", binary, startErr)
	}

	// Forward signals to the child process
	go func() {
		for sig := range sigChan {
			proc := cmd.Process()
			if proc != nil {
				// Ignore signal errors - process may have already exited
				_ = proc.Signal(sig) //nolint:errcheck // Signal errors are non-fatal
			}
		}
	}()

	// Build writers: always stream to configured stdout/stderr, optionally capture
	// Use a mutex to ensure ordered writes to the capture buffer
	var captureMu sync.Mutex
	captureWriter := func(w io.Writer) io.Writer {
		if capture == nil {
			return w
		}
		return &orderedWriter{
			writer:  w,
			capture: capture,
			mu:      &captureMu,
		}
	}
	stdoutWriter := captureWriter(e.stdout)
	stderrWriter := captureWriter(e.stderr)

	// Read stdout in a goroutine (stream and optionally capture)
	stdoutDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stdoutWriter, stdoutPipe)
		stdoutDone <- copyErr
	}()

	// Read stderr and stream it in real-time (and optionally capture)
	_, copyErr := io.Copy(stderrWriter, stderrPipe)
	if copyErr != nil && copyErr != io.EOF {
		// Wait for stdout to finish before returning
		<-stdoutDone
		return 1, fmt.Errorf("failed to read stderr: %w", copyErr)
	}

	// Wait for stdout to finish
	if readErr := <-stdoutDone; readErr != nil {
		return 1, fmt.Errorf("failed to read stdout: %w", readErr)
	}

	// Wait for completion
	waitErr := cmd.Wait()

	// Get exit code
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("failed to execute %s: %w", binary, waitErr)
	}

	return 0, nil
}

// buildEnvironment constructs the environment for the Vault CLI.
func (e *Executor) buildEnvironment() []string {
	// Start with the current environment
	env := os.Environ()

	// Add any custom environment variables (using SetEnv to properly override)
	for _, kv := range e.environ {
		if idx := strings.IndexByte(kv, '='); idx > 0 {
			key := kv[:idx]
			value := kv[idx+1:]
			env = utils.SetEnv(env, key, value)
		}
	}

	// Set VAULT_ADDR from connection
	if e.conn.Address != "" {
		env = utils.SetEnv(env, "VAULT_ADDR", e.conn.Address)
	}

	// Set VAULT_TOKEN if we have one
	if e.token != "" {
		env = utils.SetEnv(env, "VAULT_TOKEN", e.token)
	}

	// Set VAULT_NAMESPACE if specified
	if e.conn.Namespace != "" {
		env = utils.SetEnv(env, "VAULT_NAMESPACE", e.conn.Namespace)
	}

	// Set TLS options
	if e.conn.TLSSkipVerify {
		env = utils.SetEnv(env, "VAULT_SKIP_VERIFY", "true")
	}
	if e.conn.CACert != "" {
		env = utils.SetEnv(env, "VAULT_CACERT", e.conn.CACert)
	}
	if e.conn.CAPath != "" {
		env = utils.SetEnv(env, "VAULT_CAPATH", e.conn.CAPath)
	}
	if e.conn.ClientCert != "" {
		env = utils.SetEnv(env, "VAULT_CLIENT_CERT", e.conn.ClientCert)
	}
	if e.conn.ClientKey != "" {
		env = utils.SetEnv(env, "VAULT_CLIENT_KEY", e.conn.ClientKey)
	}

	return env
}

// BinaryExists checks if the Vault/OpenBao binary exists.
// Options can be provided for testing (e.g., WithCommandRunner).
func BinaryExists(conn *config.Connection, opts ...Option) bool {
	binary := conn.GetBinaryPath()
	exec := NewExecutor(conn, opts...)
	_, err := exec.commandRunner.LookPath(binary)
	return err == nil
}
