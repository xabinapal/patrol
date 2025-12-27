//go:build !windows

package daemon

// WindowsManager is a stub for non-Windows platforms.
type WindowsManager struct{}

// NewWindowsManager creates a stub Windows manager.
func NewWindowsManager(cfg ServiceConfig) *WindowsManager {
	return &WindowsManager{}
}

// Install is not supported on this platform.
func (m *WindowsManager) Install() error { return ErrServiceNotSupported }

// Uninstall is not supported on this platform.
func (m *WindowsManager) Uninstall() error { return ErrServiceNotSupported }

// IsInstalled is not supported on this platform.
func (m *WindowsManager) IsInstalled() (bool, error) { return false, ErrServiceNotSupported }

// Start is not supported on this platform.
func (m *WindowsManager) Start() error { return ErrServiceNotSupported }

// Stop is not supported on this platform.
func (m *WindowsManager) Stop() error { return ErrServiceNotSupported }

// Status is not supported on this platform.
func (m *WindowsManager) Status() (ServiceStatus, error) {
	return ServiceStatus{}, ErrServiceNotSupported
}

// ServiceFilePath is not supported on this platform.
func (m *WindowsManager) ServiceFilePath() string { return "" }
