//go:build windows

package cli

import (
	"os"
	"os/exec"
)

// setUnixProcessAttributes is a no-op on Windows (Setpgid is not available).
func setUnixProcessAttributes(cmd *exec.Cmd) {
	// No-op on Windows - Setpgid is Unix-specific
}

// getTermSignal returns the termination signal for Windows.
// On Windows, process.Signal works but we use os.Kill as the equivalent.
func getTermSignal() os.Signal {
	return os.Kill
}
