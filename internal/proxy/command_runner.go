package proxy

import (
	"context"
	"io"
	"os"
	"os/exec"
)

// CommandRunner is an interface for executing commands.
// This allows mocking in tests without actually executing binaries.
type CommandRunner interface {
	// LookPath finds the executable in PATH
	LookPath(file string) (string, error)
	// CommandContext creates a command that can be executed
	CommandContext(ctx context.Context, name string, args ...string) Command
}

// Command represents an executable command.
type Command interface {
	// SetEnv sets the environment variables
	SetEnv(env []string)
	// SetStdin sets the stdin reader
	SetStdin(stdin io.Reader)
	// SetStdout sets the stdout writer
	SetStdout(stdout io.Writer)
	// SetStderr sets the stderr writer
	SetStderr(stderr io.Writer)
	// StdoutPipe returns a pipe for reading stdout
	StdoutPipe() (io.ReadCloser, error)
	// StderrPipe returns a pipe for reading stderr
	StderrPipe() (io.ReadCloser, error)
	// Start starts the command
	Start() error
	// Wait waits for the command to complete
	Wait() error
	// Process returns the underlying process
	Process() Process
}

// Process represents a running process.
type Process interface {
	// Signal sends a signal to the process
	Signal(sig os.Signal) error
}

// realCommandRunner is the real implementation using os/exec.
type realCommandRunner struct{}

// NewCommandRunner creates a new real command runner.
func NewCommandRunner() CommandRunner {
	return &realCommandRunner{}
}

func (r *realCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (r *realCommandRunner) CommandContext(ctx context.Context, name string, args ...string) Command {
	return &realCommand{cmd: exec.CommandContext(ctx, name, args...)}
}

// realCommand wraps exec.Cmd to implement the Command interface.
type realCommand struct {
	cmd *exec.Cmd
}

func (c *realCommand) SetEnv(env []string) {
	c.cmd.Env = env
}

func (c *realCommand) SetStdin(stdin io.Reader) {
	c.cmd.Stdin = stdin
}

func (c *realCommand) SetStdout(stdout io.Writer) {
	c.cmd.Stdout = stdout
}

func (c *realCommand) SetStderr(stderr io.Writer) {
	c.cmd.Stderr = stderr
}

func (c *realCommand) StdoutPipe() (io.ReadCloser, error) {
	return c.cmd.StdoutPipe()
}

func (c *realCommand) StderrPipe() (io.ReadCloser, error) {
	return c.cmd.StderrPipe()
}

func (c *realCommand) Start() error {
	return c.cmd.Start()
}

func (c *realCommand) Wait() error {
	return c.cmd.Wait()
}

func (c *realCommand) Process() Process {
	if c.cmd.Process == nil {
		return nil
	}
	return &realProcess{proc: c.cmd.Process}
}

// realProcess wraps os.Process to implement the Process interface.
type realProcess struct {
	proc *os.Process
}

func (p *realProcess) Signal(sig os.Signal) error {
	return p.proc.Signal(sig)
}
