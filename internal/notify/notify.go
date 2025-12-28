// Package notify provides desktop notification support for Patrol.
package notify

import (
	"fmt"
	"time"

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

// Option configures a Notifier.
type Option func(*notifier)

// WithBackend sets a custom notification backend (for testing).
func WithBackend(backend Backend) Option {
	return func(n *notifier) {
		n.backend = backend
	}
}

// notifier sends desktop notifications using the system notification service.
type notifier struct {
	onRenewal bool
	onFailure bool
	backend   Backend
}

// NotifyRenewal sends a notification about successful token renewal.
func (n *notifier) NotifyRenewal(profile string, newTTL time.Duration) error {
	if !n.onRenewal {
		return nil
	}

	title := "Patrol: Token Renewed"
	message := fmt.Sprintf("Token for '%s' renewed successfully.\nNew TTL: %s", profile, utils.FormatDuration(newTTL))

	return n.backend.Notify(title, message, "")
}

// NotifyFailure sends a notification about renewal failure.
func (n *notifier) NotifyFailure(profile string, err error) error {
	if !n.onFailure {
		return nil
	}

	title := "Patrol: Renewal Failed"
	message := fmt.Sprintf("Failed to renew token for '%s'.\nError: %v", profile, err)

	return n.backend.Alert(title, message, "")
}

// New creates a new Notifier based on the configuration.
func New(cfg config.NotificationConfig, opts ...Option) Notifier {
	n := &notifier{
		onRenewal: cfg.Enabled && cfg.OnRenewal,
		onFailure: cfg.Enabled && cfg.OnFailure,
		backend:   newDesktopBackend(),
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}
