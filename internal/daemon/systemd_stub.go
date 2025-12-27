//go:build !linux

package daemon

// SystemdManager is a stub for non-Linux platforms.
type SystemdManager struct{}

// NewSystemdManager creates a stub systemd manager.
func NewSystemdManager(cfg ServiceConfig) *SystemdManager {
	return &SystemdManager{}
}

// Install is not supported on this platform.
func (m *SystemdManager) Install() error { return ErrServiceNotSupported }

// Uninstall is not supported on this platform.
func (m *SystemdManager) Uninstall() error { return ErrServiceNotSupported }

// IsInstalled is not supported on this platform.
func (m *SystemdManager) IsInstalled() (bool, error) { return false, ErrServiceNotSupported }

// Start is not supported on this platform.
func (m *SystemdManager) Start() error { return ErrServiceNotSupported }

// Stop is not supported on this platform.
func (m *SystemdManager) Stop() error { return ErrServiceNotSupported }

// Status is not supported on this platform.
func (m *SystemdManager) Status() (ServiceStatus, error) {
	return ServiceStatus{}, ErrServiceNotSupported
}

// ServiceFilePath is not supported on this platform.
func (m *SystemdManager) ServiceFilePath() string { return "" }
