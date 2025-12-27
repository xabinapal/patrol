//go:build !darwin

package daemon

// LaunchdManager is a stub for non-macOS platforms.
type LaunchdManager struct{}

// NewLaunchdManager creates a stub launchd manager.
func NewLaunchdManager(cfg ServiceConfig) *LaunchdManager {
	return &LaunchdManager{}
}

// Install is not supported on this platform.
func (m *LaunchdManager) Install() error { return ErrServiceNotSupported }

// Uninstall is not supported on this platform.
func (m *LaunchdManager) Uninstall() error { return ErrServiceNotSupported }

// IsInstalled is not supported on this platform.
func (m *LaunchdManager) IsInstalled() (bool, error) { return false, ErrServiceNotSupported }

// Start is not supported on this platform.
func (m *LaunchdManager) Start() error { return ErrServiceNotSupported }

// Stop is not supported on this platform.
func (m *LaunchdManager) Stop() error { return ErrServiceNotSupported }

// Status is not supported on this platform.
func (m *LaunchdManager) Status() (ServiceStatus, error) {
	return ServiceStatus{}, ErrServiceNotSupported
}

// ServiceFilePath is not supported on this platform.
func (m *LaunchdManager) ServiceFilePath() string { return "" }
