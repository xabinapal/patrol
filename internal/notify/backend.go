package notify

import "github.com/gen2brain/beeep"

// Backend defines the interface for the notification backend.
type Backend interface {
	// Notify sends a standard notification.
	Notify(title, message, iconPath string) error
	// Alert sends an alert notification.
	Alert(title, message, iconPath string) error
}

// desktopBackend implements Backend by calling beeep functions directly.
type desktopBackend struct{}

// Notify implements Backend.
func (desktopBackend) Notify(title, message, iconPath string) error {
	return beeep.Notify(title, message, iconPath)
}

// Alert implements Backend.
func (desktopBackend) Alert(title, message, iconPath string) error {
	return beeep.Alert(title, message, iconPath)
}

// newDesktopBackend returns a Backend that uses beeep.
func newDesktopBackend() Backend {
	return desktopBackend{}
}
