//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"strings"
)

const taskName = "PatrolTokenManager"

// WindowsManager manages scheduled tasks on Windows.
type WindowsManager struct {
	cfg ServiceConfig
}

// NewWindowsManager creates a new Windows task manager.
func NewWindowsManager(cfg ServiceConfig) *WindowsManager {
	return &WindowsManager{cfg: cfg}
}

// Install creates a scheduled task that runs at logon.
func (m *WindowsManager) Install() error {
	// Create a scheduled task that runs at logon
	args := []string{
		"/create",
		"/tn", taskName,
		"/tr", fmt.Sprintf(`"%s" daemon run`, m.cfg.ExecutablePath),
		"/sc", "onlogon",
		"/rl", "limited",
		"/f", // Force overwrite if exists
	}

	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %s: %w", string(output), err)
	}

	// Enable and start the task
	if err := m.enable(); err != nil {
		return fmt.Errorf("failed to enable task: %w", err)
	}

	return m.Start()
}

// Uninstall removes the scheduled task.
func (m *WindowsManager) Uninstall() error {
	// Disable and stop the task first
	if err := m.disable(); err != nil {
		// Ignore error - task might not be enabled
		_ = err
	}

	// Stop the task (ignore error - might not be running)
	//nolint:errcheck // Best effort to stop before uninstall
	_ = m.Stop()

	// Delete the task
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/delete", "/tn", taskName, "/f")
	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "does not exist") {
			return fmt.Errorf("failed to delete scheduled task: %s: %w", string(output), err)
		}
	}

	return nil
}

// IsInstalled checks if the scheduled task exists.
func (m *WindowsManager) IsInstalled() (bool, error) {
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/query", "/tn", taskName)
	err := cmd.Run()
	return err == nil, nil
}

// Start runs the scheduled task.
func (m *WindowsManager) Start() error {
	// Enable the task first
	if err := m.enable(); err != nil {
		return fmt.Errorf("failed to enable task: %w", err)
	}

	// Run the task
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/run", "/tn", taskName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start task: %s: %w", string(output), err)
	}
	return nil
}

// Stop ends the scheduled task.
func (m *WindowsManager) Stop() error {
	// End the task first (ignore error - might not be running)
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/end", "/tn", taskName)
	//nolint:errcheck // Ignore error - task might not be running
	_ = cmd.Run()

	// Disable the task
	if err := m.disable(); err != nil {
		return fmt.Errorf("failed to disable task: %w", err)
	}

	return nil
}

// Restart restarts the scheduled task.
func (m *WindowsManager) Restart() error {
	// Stop first
	if err := m.Stop(); err != nil {
		return fmt.Errorf("failed to stop task: %w", err)
	}

	// Start again
	if err := m.Start(); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	return nil
}

// Status returns the current status of the scheduled task.
func (m *WindowsManager) Status() (ServiceStatus, error) {
	status := ServiceStatus{}

	installed, err := m.IsInstalled()
	if err != nil {
		return status, err
	}
	status.Installed = installed

	if !installed {
		return status, nil
	}

	// Check if running - query task status
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/query", "/tn", taskName, "/fo", "csv", "/v")
	output, err := cmd.Output()
	if err == nil {
		status.Running = strings.Contains(string(output), "Running")
	}

	return status, nil
}

// ServiceFilePath returns the task name.
func (m *WindowsManager) ServiceFilePath() string {
	return fmt.Sprintf("Task Scheduler: %s", taskName)
}

// enable enables the scheduled task.
func (m *WindowsManager) enable() error {
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/change", "/tn", taskName, "/enable")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable task: %s: %w", string(output), err)
	}
	return nil
}

// disable disables the scheduled task.
func (m *WindowsManager) disable() error {
	// #nosec G204 - schtasks.exe is a Windows system utility, args are controlled
	cmd := exec.Command("schtasks.exe", "/change", "/tn", taskName, "/disable")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to disable task: %s: %w", string(output), err)
	}
	return nil
}
