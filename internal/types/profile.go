// Package types provides shared types used across the application.
package types

import (
	"github.com/xabinapal/patrol/internal/config"
)

// Profile represents a connection profile.
type Profile struct {
	Name      string
	Type      string
	Address   string
	Namespace string

	BinaryPath string

	TLSSkipVerify bool
	CACert        string
	CAPath        string
	ClientCert    string
	ClientKey     string
}

func (p *Profile) GetBinaryPath() string {
	if p.BinaryPath != "" {
		return p.BinaryPath
	}
	switch p.Type {
	case "openbao":
		return "bao"
	default:
		return "vault"
	}
}

func (p *Profile) ToConnection() *config.Connection {
	return &config.Connection{
		Name:          p.Name,
		Address:       p.Address,
		Type:          config.BinaryType(p.Type),
		BinaryPath:    p.BinaryPath,
		Namespace:     p.Namespace,
		TLSSkipVerify: p.TLSSkipVerify,
		CACert:        p.CACert,
		CAPath:        p.CAPath,
		ClientCert:    p.ClientCert,
		ClientKey:     p.ClientKey,
	}
}

// FromConnection creates a Profile from a config.Connection.
func FromConnection(conn *config.Connection) *Profile {
	if conn == nil {
		return nil
	}
	return &Profile{
		Name:          conn.Name,
		Type:          string(conn.Type),
		Address:       conn.Address,
		Namespace:     conn.Namespace,
		BinaryPath:    conn.BinaryPath,
		TLSSkipVerify: conn.TLSSkipVerify,
		CACert:        conn.CACert,
		CAPath:        conn.CAPath,
		ClientCert:    conn.ClientCert,
		ClientKey:     conn.ClientKey,
	}
}
