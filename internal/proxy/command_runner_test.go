package proxy

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// mockCommandRunner is a mock implementation of CommandRunner for testing.
type mockCommandRunner struct {
	lookPathFunc func(file string) (string, error)
	commands     []*mockCommand
	mu           sync.Mutex
}

// mockCommand is a mock implementation of Command for testing.
type mockCommand struct {
	name       string
	args       []string
	env        []string
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	stdoutPipe *mockPipe
	stderrPipe *mockPipe
	exitCode   int
	err        error
	started    bool
	waited     bool
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
	// Default: return the file as-is if it's an absolute path or contains a path separator
	if strings.Contains(file, "/") || strings.Contains(file, "\\") {
		return file, nil
	}
	// For simple names, return them as-is (simulating found in PATH)
	return file, nil
}

// CommandContext implements CommandRunner.
func (m *mockCommandRunner) CommandContext(ctx context.Context, name string, args ...string) Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd := &mockCommand{
		name:       name,
		args:       args,
		stdoutPipe: &mockPipe{},
		stderrPipe: &mockPipe{},
		exitCode:   0,
	}
	m.commands = append(m.commands, cmd)
	return cmd
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
	c.stdin = stdin
}

// SetStdout implements Command.
func (c *mockCommand) SetStdout(stdout io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stdout = stdout
}

// SetStderr implements Command.
func (c *mockCommand) SetStderr(stderr io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stderr = stderr
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

	// Simulate command execution based on name/args
	// For "echo" behavior: write args to stdout
	if strings.Contains(c.name, "vault") || strings.Contains(c.name, "echo") {
		// Simulate echo: write args joined by space
		output := strings.Join(c.args, " ") + "\n"
		if c.stdout != nil {
			_, _ = c.stdout.Write([]byte(output))
		}
		if c.stdoutPipe != nil {
			c.stdoutPipe.mu.Lock()
			c.stdoutPipe.data = []byte(output)
			c.stdoutPipe.mu.Unlock()
		}
	}

	// For "env" behavior: write environment variables
	if strings.Contains(c.name, "env") || (len(c.args) == 0 && c.env != nil) {
		var output strings.Builder
		for _, e := range c.env {
			output.WriteString(e)
			output.WriteString("\n")
		}
		if c.stdout != nil {
			_, _ = c.stdout.Write([]byte(output.String()))
		}
		if c.stdoutPipe != nil {
			c.stdoutPipe.mu.Lock()
			c.stdoutPipe.data = []byte(output.String())
			c.stdoutPipe.mu.Unlock()
		}
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
		// We use a helper command to get a valid ProcessState
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
func (c *mockCommand) Process() Process {
	return &mockProcess{cmd: c}
}

// mockProcess is a mock implementation of Process.
type mockProcess struct {
	cmd *mockCommand
}

func (p *mockProcess) Signal(sig os.Signal) error {
	return nil
}

// setLookPathError sets an error to return from LookPath.
func (m *mockCommandRunner) setLookPathError(err error) {
	m.setLookPathFunc(func(file string) (string, error) {
		return "", err
	})
}
