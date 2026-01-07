// Package profile provides profile-related types and operations.
package profile

import (
	"context"
	"errors"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/types"
)

// ProfileManager manages profiles and provides high-level operations.
type ProfileManager struct {
	ctx context.Context
	cfg *config.Config
}

// NewProfileManager creates a new ProfileManager initialized with context and config.
func NewProfileManager(ctx context.Context, cfg *config.Config) *ProfileManager {
	return &ProfileManager{
		ctx: ctx,
		cfg: cfg,
	}
}

// GetCurrent returns the currently active profile.
func (pm *ProfileManager) GetCurrent() (*types.Profile, error) {
	if pm.cfg == nil || pm.cfg.Current == "" {
		return nil, errors.New("no active profile configured")
	}

	conn, err := pm.cfg.GetConnection(pm.cfg.Current)
	if err != nil {
		return nil, err
	}

	return types.FromConnection(conn), nil
}

// Get returns a profile by name.
func (pm *ProfileManager) Get(name string) (*types.Profile, error) {
	if pm.cfg == nil {
		return nil, errors.New("no configuration loaded")
	}

	conn, err := pm.cfg.GetConnection(name)
	if err != nil {
		return nil, err
	}

	return types.FromConnection(conn), nil
}

// List returns all profiles.
func (pm *ProfileManager) List() []*types.Profile {
	if pm.cfg == nil {
		return nil
	}

	profiles := make([]*types.Profile, 0, len(pm.cfg.Connections))
	for _, conn := range pm.cfg.Connections {
		profiles = append(profiles, types.FromConnection(&conn))
	}

	return profiles
}
