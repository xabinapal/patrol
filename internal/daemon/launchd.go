//go:build darwin

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

const launchdLabel = "com.patrol.agent"

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecutablePath}}</string>
        <string>daemon</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
    <key>ProcessType</key>
    <string>Background</string>
    <key>LowPriorityIO</key>
    <true/>
</dict>
</plist>
`

// LaunchdManager manages launchd user agents on macOS.
type LaunchdManager struct {
	cfg       ServiceConfig
	plistPath string
}

// NewLaunchdManager creates a new launchd manager.
func NewLaunchdManager(cfg ServiceConfig) *LaunchdManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fall back to current directory if home dir unavailable
		homeDir = "."
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", launchdLabel+".plist")

	return &LaunchdManager{
		cfg:       cfg,
		plistPath: plistPath,
	}
}

// Install installs the launchd user agent.
func (m *LaunchdManager) Install() error {
	// Ensure LaunchAgents directory exists
	dir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(m.cfg.LogPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate plist content
	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		Label          string
		ExecutablePath string
		LogPath        string
	}{
		Label:          launchdLabel,
		ExecutablePath: m.cfg.ExecutablePath,
		LogPath:        m.cfg.LogPath,
	}

	f, err := os.Create(m.plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Bootstrap the agent (RunAtLoad will start it automatically)
	uid := os.Getuid()
	// #nosec G204 - plistPath is constructed from user home directory, not user input
	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), m.plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore error if already bootstrapped
		if !strings.Contains(string(output), "already bootstrapped") && !strings.Contains(string(output), "already loaded") {
			return fmt.Errorf("failed to install agent: %s: %w", string(output), err)
		}
	}

	return nil
}

// Uninstall removes the launchd user agent.
func (m *LaunchdManager) Uninstall() error {
	installed, installErr := m.IsInstalled()
	if installErr == nil && installed {
		uid := os.Getuid()

		// Bootout the agent (this stops and unloads it)
		// #nosec G204 - plistPath is constructed from user home directory, not user input
		cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), m.plistPath)
		_ = cmd.Run() //nolint:errcheck // Ignore error - might not be loaded
	}

	// Remove the plist file
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

// IsInstalled checks if the launchd agent is installed.
func (m *LaunchdManager) IsInstalled() (bool, error) {
	_, err := os.Stat(m.plistPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Start starts the launchd agent (used internally by Restart).
func (m *LaunchdManager) Start() error {
	uid := os.Getuid()
	// #nosec G204 - plistPath is constructed from user home directory, not user input
	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), m.plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore error if already bootstrapped
		if !strings.Contains(string(output), "already bootstrapped") && !strings.Contains(string(output), "already loaded") {
			return fmt.Errorf("failed to start agent: %s: %w", string(output), err)
		}
	}
	return nil
}

// Stop stops the launchd agent (used internally by Restart and Uninstall).
// On macOS with KeepAlive, we can't truly "stop" without unloading, so this just bootouts.
func (m *LaunchdManager) Stop() error {
	uid := os.Getuid()
	// #nosec G204 - plistPath is constructed from user home directory, not user input
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), m.plistPath)
	_ = cmd.Run() //nolint:errcheck // Ignore error - might not be loaded
	return nil
}

// Restart restarts the launchd agent.
func (m *LaunchdManager) Restart() error {
	uid := os.Getuid()

	// Bootout (stop and unload)
	// #nosec G204 - plistPath is constructed from user home directory, not user input
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), m.plistPath)
	_ = cmd.Run() //nolint:errcheck // Ignore error - might not be loaded

	// Bootstrap again (will start due to RunAtLoad)
	// #nosec G204 - plistPath is constructed from user home directory, not user input
	cmd = exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), m.plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart agent: %s: %w", string(output), err)
	}

	return nil
}

// Status returns the current status of the launchd agent.
func (m *LaunchdManager) Status() (ServiceStatus, error) {
	status := ServiceStatus{}

	installed, err := m.IsInstalled()
	if err != nil {
		return status, err
	}
	status.Installed = installed

	if !installed {
		return status, nil
	}

	// Check if running
	// Modern launchctl outputs JSON, older versions output tab-separated
	cmd := exec.Command("launchctl", "list", launchdLabel)
	output, err := cmd.Output()
	if err != nil {
		// Not running
		return status, nil
	}

	outputStr := string(output)

	// Try to parse as JSON first (modern macOS)
	// JSON format: {"PID" = 12345; ...} or {"PID": 12345, ...}
	if strings.Contains(outputStr, `"PID"`) {
		// Extract PID from JSON-like output
		// Look for "PID" = <number> or "PID": <number>
		pidPattern := `"PID"\s*[=:]\s*(\d+)`
		re := regexp.MustCompile(pidPattern)
		matches := re.FindStringSubmatch(outputStr)
		if len(matches) > 1 {
			if pid, err := strconv.Atoi(matches[1]); err == nil && pid > 0 {
				status.Running = true
				status.PID = pid
				return status, nil
			}
		}
	}

	// Fallback: Try to parse as tab-separated (older macOS)
	// Format: PID	Status	Label
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] != "-" {
			if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
				status.Running = true
				status.PID = pid
				return status, nil
			}
		}
	}

	return status, nil
}

// ServiceFilePath returns the path to the plist file.
func (m *LaunchdManager) ServiceFilePath() string {
	return m.plistPath
}
