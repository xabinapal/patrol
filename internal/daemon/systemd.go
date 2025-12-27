//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const systemdServiceTemplate = `[Unit]
Description=Patrol Vault Token Manager
Documentation=https://github.com/xabinapal/patrol
After=network.target

[Service]
Type=simple
ExecStart={{.ExecutablePath}} daemon run
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=default.target
`

// SystemdManager manages systemd user services on Linux.
type SystemdManager struct {
	cfg         ServiceConfig
	servicePath string
}

// NewSystemdManager creates a new systemd manager.
func NewSystemdManager(cfg ServiceConfig) *SystemdManager {
	// Use XDG_CONFIG_HOME or default
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		//nolint:errcheck // Fall back to current directory if home dir unavailable
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}

	servicePath := filepath.Join(configHome, "systemd", "user", "patrol.service")

	return &SystemdManager{
		cfg:         cfg,
		servicePath: servicePath,
	}
}

// Install installs the systemd user service.
func (m *SystemdManager) Install() error {
	// Ensure directory exists
	dir := filepath.Dir(m.servicePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Generate service file
	tmpl, err := template.New("service").Parse(systemdServiceTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		ExecutablePath string
	}{
		ExecutablePath: m.cfg.ExecutablePath,
	}

	f, err := os.Create(m.servicePath)
	if err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable the service
	if err := exec.Command("systemctl", "--user", "enable", "patrol.service").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start the service
	if err := exec.Command("systemctl", "--user", "start", "patrol.service").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// Uninstall removes the systemd user service.
func (m *SystemdManager) Uninstall() error {
	// Stop the service (best effort)
	//nolint:errcheck // Ignore errors - service might not be running
	_ = exec.Command("systemctl", "--user", "stop", "patrol.service").Run()

	// Disable the service (best effort)
	//nolint:errcheck // Ignore errors - service might not be enabled
	_ = exec.Command("systemctl", "--user", "disable", "patrol.service").Run()

	// Remove service file
	if err := os.Remove(m.servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd (best effort)
	//nolint:errcheck // Ignore errors - reload might fail if service already removed
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

// IsInstalled checks if the systemd service is installed.
func (m *SystemdManager) IsInstalled() (bool, error) {
	_, err := os.Stat(m.servicePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Start starts the systemd service.
func (m *SystemdManager) Start() error {
	return exec.Command("systemctl", "--user", "start", "patrol.service").Run()
}

// Stop stops the systemd service.
func (m *SystemdManager) Stop() error {
	return exec.Command("systemctl", "--user", "stop", "patrol.service").Run()
}

// Status returns the current status of the systemd service.
func (m *SystemdManager) Status() (ServiceStatus, error) {
	status := ServiceStatus{}

	installed, err := m.IsInstalled()
	if err != nil {
		return status, err
	}
	status.Installed = installed

	if !installed {
		return status, nil
	}

	// Check status
	cmd := exec.Command("systemctl", "--user", "is-active", "patrol.service")
	//nolint:errcheck // Best effort - if command fails, service is not active
	output, _ := cmd.Output()
	status.Running = strings.TrimSpace(string(output)) == "active"

	// Get PID if running
	if status.Running {
		cmd := exec.Command("systemctl", "--user", "show", "-p", "MainPID", "patrol.service")
		if output, err := cmd.Output(); err == nil {
			var pid int
			//nolint:errcheck // Best effort - if parsing fails, PID remains 0
			_, _ = fmt.Sscanf(string(output), "MainPID=%d", &pid)
			if pid > 0 {
				status.PID = pid
			}
		}
	}

	return status, nil
}

// ServiceFilePath returns the path to the service file.
func (m *SystemdManager) ServiceFilePath() string {
	return m.servicePath
}
