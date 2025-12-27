package profile

import (
	"errors"

	"github.com/xabinapal/patrol/internal/config"
)

// GetCurrent returns the currently active profile from config.
func GetCurrent(cfg *config.Config) (*Profile, error) {
	if cfg == nil || cfg.Current == "" {
		return nil, errors.New("no active profile configured - use 'patrol profile add' to add a profile, then 'patrol profile use <profile>' to activate it")
	}

	conn, err := cfg.GetConnection(cfg.Current)
	if err != nil {
		return nil, err
	}

	return &Profile{Connection: conn}, nil
}

// Get returns a profile by name from config.
func Get(cfg *config.Config, name string) (*Profile, error) {
	if cfg == nil {
		return nil, errors.New("no configuration loaded")
	}

	conn, err := cfg.GetConnection(name)
	if err != nil {
		return nil, err
	}

	return &Profile{Connection: conn}, nil
}

// List returns information about all profiles from config.
func List(cfg *config.Config) []Info {
	if cfg == nil {
		return nil
	}

	profiles := make([]Info, 0, len(cfg.Connections))
	for _, conn := range cfg.Connections {
		profiles = append(profiles, Info{
			Name:      conn.Name,
			Address:   conn.Address,
			Type:      string(conn.Type),
			Namespace: conn.Namespace,
			Current:   conn.Name == cfg.Current,
		})
	}

	return profiles
}

// GetStatus returns comprehensive status information for a profile.
func GetStatus(cfg *config.Config, name string) (*Status, error) {
	if cfg == nil {
		return nil, errors.New("no configuration loaded")
	}

	conn, err := cfg.GetConnection(name)
	if err != nil {
		return nil, err
	}

	return &Status{
		Name:          conn.Name,
		Address:       conn.Address,
		Type:          string(conn.Type),
		Namespace:     conn.Namespace,
		Binary:        conn.GetBinaryPath(),
		BinaryPath:    conn.BinaryPath,
		TLSSkipVerify: conn.TLSSkipVerify,
		CACert:        conn.CACert,
		CAPath:        conn.CAPath,
		ClientCert:    conn.ClientCert,
		ClientKey:     conn.ClientKey,
		Active:        name == cfg.Current,
	}, nil
}
