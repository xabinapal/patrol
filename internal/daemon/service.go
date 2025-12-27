// Package daemon provides the background token renewal service and platform-specific service management.
package daemon

import (
	"fmt"
	"runtime"
)

// ServiceManager provides service installation and management.
type ServiceManager interface {
	// Install installs the service.
	Install() error
	// Uninstall removes the service.
	Uninstall() error
	// IsInstalled checks if the service is installed.
	IsInstalled() (bool, error)
	// Start starts the service.
	Start() error
	// Stop stops the service.
	Stop() error
	// Status returns the service status.
	Status() (ServiceStatus, error)
	// ServiceFilePath returns the path to the service definition file.
	ServiceFilePath() string
}

// ServiceStatus represents the current status of the service.
type ServiceStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ServiceConfig holds configuration for service installation.
type ServiceConfig struct {
	// ExecutablePath is the path to the patrol binary.
	ExecutablePath string
	// LogPath is the path for service logs.
	LogPath string
	// ConfigPath is the path to the patrol config file (optional).
	ConfigPath string
}

// NewServiceManager creates a platform-appropriate service manager.
func NewServiceManager(cfg ServiceConfig) (ServiceManager, error) {
	switch runtime.GOOS {
	case "darwin":
		return NewLaunchdManager(cfg), nil
	case "linux":
		return NewSystemdManager(cfg), nil
	case "windows":
		return NewWindowsManager(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// ServicePlatformName returns a human-readable name for the service system.
func ServicePlatformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "linux":
		return "systemd"
	case "windows":
		return "Task Scheduler"
	default:
		return "unknown"
	}
}

// ErrServiceNotSupported is returned when an operation is not supported on the current platform.
var ErrServiceNotSupported = fmt.Errorf("not supported on this platform")
