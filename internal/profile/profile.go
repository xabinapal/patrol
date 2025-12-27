// Package profile provides profile-related types and operations.
// Profiles are configuration-only - token operations are handled separately.
package profile

import (
	"github.com/xabinapal/patrol/internal/config"
)

// Profile represents a connection profile.
// It embeds *config.Connection, so all Connection methods (KeyringKey, GetBinaryPath, etc.)
// are directly accessible on Profile through Go's method promotion.
type Profile struct {
	*config.Connection
}

// Validate validates the profile configuration.
func (p *Profile) Validate() error {
	return p.Connection.Validate()
}
