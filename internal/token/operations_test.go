package token

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/proxy"
)

// mockCommandRunner is a mock implementation of CommandRunner for testing.
type mockCommandRunner struct {
	lookPathFunc    func(file string) (string, error)
	commandConfigFn func(cmd *mockCommand) // Called when a command is created
	commands        []*mockCommand
	mu              sync.Mutex
}

// mockCommand is a mock implementation of Command for testing.
type mockCommand struct {
	name       string
	args       []string
	env        []string
	stdoutPipe *mockPipe
	stderrPipe *mockPipe
	exitCode   int
	err        error
	started    bool
	waited     bool
	outputFunc func() []byte // Function to generate output based on args
	mu         sync.Mutex
}

// mockPipe is a mock pipe for reading output.
type mockPipe struct {
	data []byte
	mu   sync.Mutex
}

func (p *mockPipe) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.data) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.data)
	p.data = p.data[n:]
	return n, nil
}

func (p *mockPipe) Close() error {
	return nil
}

// newMockCommandRunner creates a new mock command runner.
func newMockCommandRunner() *mockCommandRunner {
	return &mockCommandRunner{
		commands: make([]*mockCommand, 0),
	}
}

// setLookPathFunc sets a custom LookPath function.
func (m *mockCommandRunner) setLookPathFunc(fn func(file string) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lookPathFunc = fn
}

// LookPath implements CommandRunner.
func (m *mockCommandRunner) LookPath(file string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lookPathFunc != nil {
		return m.lookPathFunc(file)
	}
	// Default: return the file as-is
	return file, nil
}

// CommandContext implements CommandRunner.
func (m *mockCommandRunner) CommandContext(ctx context.Context, name string, args ...string) proxy.Command {
	m.mu.Lock()
	cmd := &mockCommand{
		name:       name,
		args:       args,
		stdoutPipe: &mockPipe{},
		stderrPipe: &mockPipe{},
		exitCode:   0,
	}
	m.commands = append(m.commands, cmd)
	// Apply configuration function if set (unlock before calling to avoid deadlock)
	configFn := m.commandConfigFn
	m.mu.Unlock()
	if configFn != nil {
		configFn(cmd)
	}
	return cmd
}

// setCommandConfig sets a function that will configure each new command.
func (m *mockCommandRunner) setCommandConfig(fn func(cmd *mockCommand)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandConfigFn = fn
}

// getLastCommand returns the last created command.
func (m *mockCommandRunner) getLastCommand() *mockCommand {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.commands) == 0 {
		return nil
	}
	return m.commands[len(m.commands)-1]
}

// SetEnv implements Command.
func (c *mockCommand) SetEnv(env []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.env = env
}

// SetStdin implements Command.
func (c *mockCommand) SetStdin(stdin io.Reader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Not used in our tests
}

// SetStdout implements Command.
func (c *mockCommand) SetStdout(stdout io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Not used in our tests
}

// SetStderr implements Command.
func (c *mockCommand) SetStderr(stderr io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Not used in our tests
}

// StdoutPipe implements Command.
func (c *mockCommand) StdoutPipe() (io.ReadCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdoutPipe, nil
}

// StderrPipe implements Command.
func (c *mockCommand) StderrPipe() (io.ReadCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stderrPipe, nil
}

// Start implements Command.
func (c *mockCommand) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.started = true

	// Generate output based on outputFunc or default behavior
	var output []byte
	if c.outputFunc != nil {
		output = c.outputFunc()
	} else {
		// Default: generate JSON response based on command
		if strings.Contains(strings.Join(c.args, " "), "lookup") {
			output = []byte(`{
				"data": {
					"display_name": "root",
					"ttl": 3600,
					"renewable": true,
					"policies": ["default", "admin"],
					"path": "auth/userpass/login/testuser",
					"entity_id": "entity-123",
					"accessor": "accessor-456",
					"creation_ttl": 7200
				}
			}`)
		} else if strings.Contains(strings.Join(c.args, " "), "renew") {
			output = []byte(`{
				"auth": {
					"client_token": "hvs.renewed-token-12345",
					"accessor": "accessor-789",
					"policies": ["default", "admin"],
					"lease_duration": 3600,
					"renewable": true
				}
			}`)
		}
	}

	if c.stdoutPipe != nil && len(output) > 0 {
		c.stdoutPipe.mu.Lock()
		c.stdoutPipe.data = output
		c.stdoutPipe.mu.Unlock()
	}

	return nil
}

// Wait implements Command.
func (c *mockCommand) Wait() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.waited = true
	if c.exitCode != 0 {
		// Create a proper ExitError with the exit code
		var cmd *exec.Cmd
		if c.exitCode == 1 {
			cmd = exec.Command("sh", "-c", "exit 1")
		} else {
			cmd = exec.Command("sh", "-c", "exit 2")
		}
		_ = cmd.Run() // Run to populate ProcessState
		return &exec.ExitError{ProcessState: cmd.ProcessState}
	}
	return nil
}

// Process implements Command.
func (c *mockCommand) Process() proxy.Process {
	return &mockProcess{cmd: c}
}

// mockProcess is a mock implementation of Process.
type mockProcess struct {
	cmd *mockCommand
}

func (p *mockProcess) Signal(sig os.Signal) error {
	return nil
}

// setOutputFunc sets a function to generate output for the command.
func (c *mockCommand) setOutputFunc(fn func() []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.outputFunc = fn
}

// setExitCode sets the exit code for the command.
func (c *mockCommand) setExitCode(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exitCode = code
}

func TestGetStatus(t *testing.T) {
	ctx := context.Background()
	conn := &config.Connection{
		Name:    "test",
		Address: "https://vault.example.com:8200",
		Type:    config.BinaryTypeVault,
	}

	t.Run("binary not found", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setLookPathFunc(func(file string) (string, error) {
			return "", exec.ErrNotFound
		})

		status, err := GetStatus(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("GetStatus() error = %v, want nil", err)
		}
		if status == nil {
			t.Fatal("GetStatus() returned nil status")
		}
		if status.Stored != true {
			t.Error("GetStatus() status.Stored = false, want true")
		}
		if status.Valid {
			t.Error("GetStatus() status.Valid = true, want false")
		}
		if status.Error == "" {
			t.Error("GetStatus() status.Error is empty, want error message")
		}
	})

	t.Run("lookup failure", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "lookup") {
				cmd.setExitCode(1)
				cmd.setOutputFunc(func() []byte {
					return []byte("error: permission denied")
				})
			}
		})

		status, err := GetStatus(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("GetStatus() error = %v, want nil", err)
		}
		if status == nil {
			t.Fatal("GetStatus() returned nil status")
		}
		if status.Valid {
			t.Error("GetStatus() status.Valid = true, want false")
		}
		if status.Error == "" {
			t.Error("GetStatus() status.Error is empty, want error message")
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRunner := newMockCommandRunner()

		status, err := GetStatus(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("GetStatus() error = %v", err)
		}
		if status == nil {
			t.Fatal("GetStatus() returned nil status")
		}
		if !status.Valid {
			t.Error("GetStatus() status.Valid = false, want true")
		}
		if status.DisplayName != "root" {
			t.Errorf("GetStatus() status.DisplayName = %q, want 'root'", status.DisplayName)
		}
		if status.TTL != 3600 {
			t.Errorf("GetStatus() status.TTL = %d, want 3600", status.TTL)
		}
		if !status.Renewable {
			t.Error("GetStatus() status.Renewable = false, want true")
		}
		if len(status.Policies) != 2 {
			t.Errorf("GetStatus() status.Policies length = %d, want 2", len(status.Policies))
		}
	})
}

func TestLookup(t *testing.T) {
	ctx := context.Background()
	conn := &config.Connection{
		Name:    "test",
		Address: "https://vault.example.com:8200",
		Type:    config.BinaryTypeVault,
	}

	t.Run("command execution error", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "lookup") {
				cmd.err = exec.ErrNotFound
			}
		})

		_, err := Lookup(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Lookup() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to lookup token") {
			t.Errorf("Lookup() error = %v, want 'failed to lookup token'", err)
		}
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "lookup") {
				cmd.setExitCode(1)
				cmd.setOutputFunc(func() []byte {
					return []byte("error: invalid token")
				})
			}
		})

		_, err := Lookup(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Lookup() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "token lookup failed") {
			t.Errorf("Lookup() error = %v, want 'token lookup failed'", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "lookup") {
				cmd.setOutputFunc(func() []byte {
					return []byte("invalid json")
				})
			}
		})

		_, err := Lookup(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Lookup() expected error for invalid JSON, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRunner := newMockCommandRunner()

		data, err := Lookup(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("Lookup() error = %v", err)
		}
		if data == nil {
			t.Fatal("Lookup() returned nil data")
		}
		if data.DisplayName != "root" {
			t.Errorf("Lookup() data.DisplayName = %q, want 'root'", data.DisplayName)
		}
		if data.TTL != 3600 {
			t.Errorf("Lookup() data.TTL = %d, want 3600", data.TTL)
		}
		if !data.Renewable {
			t.Error("Lookup() data.Renewable = false, want true")
		}
	})
}

func TestRenew(t *testing.T) {
	ctx := context.Background()
	conn := &config.Connection{
		Name:    "test",
		Address: "https://vault.example.com:8200",
		Type:    config.BinaryTypeVault,
	}

	t.Run("binary not found", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setLookPathFunc(func(file string) (string, error) {
			return "", exec.ErrNotFound
		})

		_, err := Renew(ctx, conn, "test-token", "", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Renew() expected error, got nil")
		}
		// The error comes from the executor, which checks BinaryExists first
		// But BinaryExists uses its own runner, so we get a different error
		// The executor will also check LookPath and fail there
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Renew() error = %v, want to contain 'not found'", err)
		}
	})

	t.Run("command execution error", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "renew") {
				cmd.err = exec.ErrNotFound
			}
		})

		_, err := Renew(ctx, conn, "test-token", "", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Renew() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to renew token") {
			t.Errorf("Renew() error = %v, want 'failed to renew token'", err)
		}
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "renew") {
				cmd.setExitCode(1)
				cmd.setOutputFunc(func() []byte {
					return []byte("error: token expired")
				})
			}
		})

		_, err := Renew(ctx, conn, "test-token", "", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Renew() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "token renewal failed") {
			t.Errorf("Renew() error = %v, want 'token renewal failed'", err)
		}
	})

	t.Run("with increment", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "renew") {
				cmd.setOutputFunc(func() []byte {
					return []byte(`{
						"auth": {
							"client_token": "hvs.renewed-token-12345",
							"accessor": "accessor-789",
							"policies": ["default", "admin"],
							"lease_duration": 7200,
							"renewable": true
						}
					}`)
				})
			}
		})

		tok, err := Renew(ctx, conn, "test-token", "1h", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("Renew() error = %v", err)
		}
		if tok == nil {
			t.Fatal("Renew() returned nil token")
		}
		if tok.ClientToken != "hvs.renewed-token-12345" {
			t.Errorf("Renew() token.ClientToken = %q, want 'hvs.renewed-token-12345'", tok.ClientToken)
		}
		if tok.LeaseDuration != 7200 {
			t.Errorf("Renew() token.LeaseDuration = %d, want 7200", tok.LeaseDuration)
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRunner := newMockCommandRunner()

		tok, err := Renew(ctx, conn, "test-token", "", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("Renew() error = %v", err)
		}
		if tok == nil {
			t.Fatal("Renew() returned nil token")
		}
		if tok.ClientToken != "hvs.renewed-token-12345" {
			t.Errorf("Renew() token.ClientToken = %q, want 'hvs.renewed-token-12345'", tok.ClientToken)
		}
		if tok.LeaseDuration != 3600 {
			t.Errorf("Renew() token.LeaseDuration = %d, want 3600", tok.LeaseDuration)
		}
		if !tok.Renewable {
			t.Error("Renew() token.Renewable = false, want true")
		}
	})
}

func TestRevoke(t *testing.T) {
	ctx := context.Background()
	conn := &config.Connection{
		Name:    "test",
		Address: "https://vault.example.com:8200",
		Type:    config.BinaryTypeVault,
	}

	t.Run("binary not found", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setLookPathFunc(func(file string) (string, error) {
			return "", exec.ErrNotFound
		})

		err := Revoke(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Revoke() expected error, got nil")
		}
		// The error comes from the executor, which checks BinaryExists first
		// But BinaryExists uses its own runner, so we get a different error
		// The executor will also check LookPath and fail there
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Revoke() error = %v, want to contain 'not found'", err)
		}
	})

	t.Run("command execution error", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "revoke") {
				cmd.err = exec.ErrNotFound
			}
		})

		err := Revoke(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Revoke() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to revoke token") {
			t.Errorf("Revoke() error = %v, want 'failed to revoke token'", err)
		}
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		mockRunner := newMockCommandRunner()
		mockRunner.setCommandConfig(func(cmd *mockCommand) {
			if strings.Contains(strings.Join(cmd.args, " "), "revoke") {
				cmd.setExitCode(1)
				cmd.setOutputFunc(func() []byte {
					return []byte("error: permission denied")
				})
			}
		})

		err := Revoke(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err == nil {
			t.Error("Revoke() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "token revocation failed") {
			t.Errorf("Revoke() error = %v, want 'token revocation failed'", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRunner := newMockCommandRunner()

		err := Revoke(ctx, conn, "test-token", proxy.WithCommandRunner(mockRunner))
		if err != nil {
			t.Errorf("Revoke() error = %v", err)
		}
	})
}
