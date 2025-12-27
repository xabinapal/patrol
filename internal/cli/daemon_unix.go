//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// setUnixProcessAttributes sets Unix-specific process attributes for daemon detachment.
func setUnixProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// getTermSignal returns the termination signal for Unix platforms.
func getTermSignal() os.Signal {
	return syscall.SIGTERM
}
