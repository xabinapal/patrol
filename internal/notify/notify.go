// Package notify provides desktop notification support for Patrol.
package notify

import (
	"fmt"
	"time"

	"github.com/gen2brain/beeep"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/utils"
)

// Notifier defines the interface for sending desktop notifications.
type Notifier interface {
	// NotifyRenewal sends a notification about successful token renewal.
	NotifyRenewal(profile string, newTTL time.Duration) error
	// NotifyFailure sends a notification about renewal failure.
	NotifyFailure(profile string, err error) error
}

// New creates a new Notifier based on the configuration.
func New(cfg config.NotificationConfig) Notifier {
	if !cfg.Enabled {
		return &noopNotifier{}
	}
	return &desktopNotifier{
		onRenewal: cfg.OnRenewal,
		onFailure: cfg.OnFailure,
	}
}

// noopNotifier is a no-op implementation that does nothing.
type noopNotifier struct{}

func (n *noopNotifier) NotifyRenewal(profile string, newTTL time.Duration) error {
	return nil
}

func (n *noopNotifier) NotifyFailure(profile string, err error) error {
	return nil
}

// desktopNotifier sends desktop notifications using the system notification service.
type desktopNotifier struct {
	onRenewal bool
	onFailure bool
}

// NotifyRenewal sends a notification about successful token renewal.
func (n *desktopNotifier) NotifyRenewal(profile string, newTTL time.Duration) error {
	if !n.onRenewal {
		return nil
	}

	title := "Patrol: Token Renewed"
	message := fmt.Sprintf("Token for '%s' renewed successfully.\nNew TTL: %s", profile, utils.FormatDuration(newTTL))

	return beeep.Notify(title, message, "")
}

// NotifyFailure sends a notification about renewal failure.
func (n *desktopNotifier) NotifyFailure(profile string, err error) error {
	if !n.onFailure {
		return nil
	}

	title := "Patrol: Renewal Failed"
	message := fmt.Sprintf("Failed to renew token for '%s'.\nError: %v", profile, err)

	return beeep.Alert(title, message, "")
}
