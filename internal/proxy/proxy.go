// Package proxy handles execution of Vault/OpenBao CLI commands.
package proxy

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/xabinapal/patrol/internal/config"
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
func NewExecutor(conn *config.Connection, opts ...Option) *Executor {
	e := &Executor{
		conn:          conn,
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		commandRunner: NewCommandRunner(),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Execute runs a Vault/OpenBao command with the given arguments.
func (e *Executor) Execute(ctx context.Context, args []string) (int, error) {
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

	// Connect I/O
	cmd.SetStdin(e.stdin)
	cmd.SetStdout(e.stdout)
	cmd.SetStderr(e.stderr)

	// Set up signal forwarding
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Start the command
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start %s: %w", binary, err)
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
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("failed to execute %s: %w", binary, waitErr)
		}
	}

	return exitCode, nil
}

// ExecuteCapture runs a command and captures its output.
func (e *Executor) ExecuteCapture(ctx context.Context, args []string) (stdout, stderr []byte, exitCode int, err error) {
	// Security: Validate the connection before executing
	validateErr := e.conn.Validate()
	if validateErr != nil {
		return nil, nil, 1, fmt.Errorf("connection validation failed: %w", validateErr)
	}

	binary := e.conn.GetBinaryPath()

	// Check if the binary exists
	binaryPath, lookErr := e.commandRunner.LookPath(binary)
	if lookErr != nil {
		return nil, nil, 1, fmt.Errorf("vault/openbao binary %q not found: %w", binary, lookErr)
	}

	// Build the command
	// #nosec G204 - binaryPath is validated via LookPath, args are user-controlled but passed to vault/openbao CLI
	cmd := e.commandRunner.CommandContext(ctx, binaryPath, args...)

	// Set up environment
	cmd.SetEnv(e.buildEnvironment())

	// Capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, 1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, 1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Connect stdin if provided
	if e.stdin != nil {
		cmd.SetStdin(e.stdin)
	}

	// Start the command
	if startErr := cmd.Start(); startErr != nil {
		return nil, nil, 1, fmt.Errorf("failed to start %s: %w", binary, startErr)
	}

	// Read outputs
	stdout, err = io.ReadAll(stdoutPipe)
	if err != nil {
		return nil, nil, 1, fmt.Errorf("failed to read stdout: %w", err)
	}
	stderr, err = io.ReadAll(stderrPipe)
	if err != nil {
		return nil, nil, 1, fmt.Errorf("failed to read stderr: %w", err)
	}

	// Wait for completion
	waitErr := cmd.Wait()

	// Get exit code
	exitCode = 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return stdout, stderr, 1, fmt.Errorf("failed to execute %s: %w", binary, waitErr)
		}
	}

	return stdout, stderr, exitCode, nil
}

// buildEnvironment constructs the environment for the Vault CLI.
func (e *Executor) buildEnvironment() []string {
	// Start with the current environment
	env := os.Environ()

	// Add any custom environment variables (using setEnv to properly override)
	for _, kv := range e.environ {
		if idx := indexOf(kv, '='); idx > 0 {
			key := kv[:idx]
			value := kv[idx+1:]
			env = setEnv(env, key, value)
		}
	}

	// Set VAULT_ADDR from connection
	if e.conn.Address != "" {
		env = setEnv(env, "VAULT_ADDR", e.conn.Address)
	}

	// Set VAULT_TOKEN if we have one
	if e.token != "" {
		env = setEnv(env, "VAULT_TOKEN", e.token)
	}

	// Set VAULT_NAMESPACE if specified
	if e.conn.Namespace != "" {
		env = setEnv(env, "VAULT_NAMESPACE", e.conn.Namespace)
	}

	// Set TLS options
	if e.conn.TLSSkipVerify {
		env = setEnv(env, "VAULT_SKIP_VERIFY", "true")
	}
	if e.conn.CACert != "" {
		env = setEnv(env, "VAULT_CACERT", e.conn.CACert)
	}
	if e.conn.CAPath != "" {
		env = setEnv(env, "VAULT_CAPATH", e.conn.CAPath)
	}
	if e.conn.ClientCert != "" {
		env = setEnv(env, "VAULT_CLIENT_CERT", e.conn.ClientCert)
	}
	if e.conn.ClientKey != "" {
		env = setEnv(env, "VAULT_CLIENT_KEY", e.conn.ClientKey)
	}

	return env
}

// setEnv sets or replaces an environment variable in the env slice.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// indexOf returns the index of the first occurrence of sep in s, or -1 if not found.
func indexOf(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return i
		}
	}
	return -1
}

// BinaryExists checks if the Vault/OpenBao binary exists.
func BinaryExists(conn *config.Connection) bool {
	binary := conn.GetBinaryPath()
	runner := NewCommandRunner()
	_, err := runner.LookPath(binary)
	return err == nil
}
