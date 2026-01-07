package types

import (
	"testing"

	"github.com/xabinapal/patrol/internal/config"
)

func TestProfile_GetBinaryPath(t *testing.T) {
	tests := []struct {
		name     string
		profile  *Profile
		expected string
	}{
		{
			name: "BinaryPath set",
			profile: &Profile{
				Type:       "vault",
				BinaryPath: "/custom/path/vault",
			},
			expected: "/custom/path/vault",
		},
		{
			name: "Type openbao, no BinaryPath or Binary",
			profile: &Profile{
				Type: "openbao",
			},
			expected: "bao",
		},
		{
			name: "Type vault, no BinaryPath or Binary",
			profile: &Profile{
				Type: "vault",
			},
			expected: "vault",
		},
		{
			name: "Type empty, defaults to vault",
			profile: &Profile{
				Type: "",
			},
			expected: "vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.profile.GetBinaryPath()
			if result != tt.expected {
				t.Errorf("GetBinaryPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProfile_ToConnection(t *testing.T) {
	prof := &Profile{
		Name:          "test-profile",
		Type:          "vault",
		Address:       "https://vault.example.com",
		Namespace:     "ns1",
		BinaryPath:    "/custom/path/vault",
		TLSSkipVerify: true,
		CACert:        "/path/to/ca.crt",
		CAPath:        "/path/to/ca",
		ClientCert:    "/path/to/client.crt",
		ClientKey:     "/path/to/client.key",
	}

	conn := prof.ToConnection()

	if conn == nil {
		t.Fatal("ToConnection() returned nil")
	}

	if conn.Name != prof.Name {
		t.Errorf("ToConnection() Name = %q, want %q", conn.Name, prof.Name)
	}
	if conn.Address != prof.Address {
		t.Errorf("ToConnection() Address = %q, want %q", conn.Address, prof.Address)
	}
	if string(conn.Type) != prof.Type {
		t.Errorf("ToConnection() Type = %q, want %q", conn.Type, prof.Type)
	}
	if conn.Namespace != prof.Namespace {
		t.Errorf("ToConnection() Namespace = %q, want %q", conn.Namespace, prof.Namespace)
	}
	if conn.BinaryPath != prof.BinaryPath {
		t.Errorf("ToConnection() BinaryPath = %q, want %q", conn.BinaryPath, prof.BinaryPath)
	}
	if conn.TLSSkipVerify != prof.TLSSkipVerify {
		t.Errorf("ToConnection() TLSSkipVerify = %v, want %v", conn.TLSSkipVerify, prof.TLSSkipVerify)
	}
	if conn.CACert != prof.CACert {
		t.Errorf("ToConnection() CACert = %q, want %q", conn.CACert, prof.CACert)
	}
	if conn.CAPath != prof.CAPath {
		t.Errorf("ToConnection() CAPath = %q, want %q", conn.CAPath, prof.CAPath)
	}
	if conn.ClientCert != prof.ClientCert {
		t.Errorf("ToConnection() ClientCert = %q, want %q", conn.ClientCert, prof.ClientCert)
	}
	if conn.ClientKey != prof.ClientKey {
		t.Errorf("ToConnection() ClientKey = %q, want %q", conn.ClientKey, prof.ClientKey)
	}
}

func TestFromConnection(t *testing.T) {
	tests := []struct {
		name     string
		conn     *config.Connection
		expected *Profile
	}{
		{
			name: "full connection",
			conn: &config.Connection{
				Name:          "test-profile",
				Type:          config.BinaryTypeVault,
				Address:       "https://vault.example.com",
				Namespace:     "ns1",
				BinaryPath:    "/custom/path/vault",
				TLSSkipVerify: true,
				CACert:        "/path/to/ca.crt",
				CAPath:        "/path/to/ca",
				ClientCert:    "/path/to/client.crt",
				ClientKey:     "/path/to/client.key",
			},
			expected: &Profile{
				Name:          "test-profile",
				Type:          "vault",
				Address:       "https://vault.example.com",
				Namespace:     "ns1",
				BinaryPath:    "/custom/path/vault",
				TLSSkipVerify: true,
				CACert:        "/path/to/ca.crt",
				CAPath:        "/path/to/ca",
				ClientCert:    "/path/to/client.crt",
				ClientKey:     "/path/to/client.key",
			},
		},
		{
			name: "minimal connection",
			conn: &config.Connection{
				Name:    "minimal",
				Type:    config.BinaryTypeVault,
				Address: "https://vault.example.com",
			},
			expected: &Profile{
				Name:    "minimal",
				Type:    "vault",
				Address: "https://vault.example.com",
			},
		},
		{
			name:     "nil connection",
			conn:     nil,
			expected: nil,
		},
		{
			name: "openbao connection",
			conn: &config.Connection{
				Name:    "openbao-profile",
				Type:    config.BinaryTypeOpenBao,
				Address: "https://bao.example.com",
			},
			expected: &Profile{
				Name:    "openbao-profile",
				Type:    "openbao",
				Address: "https://bao.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromConnection(tt.conn)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("FromConnection() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("FromConnection() returned nil")
			}

			if result.Name != tt.expected.Name {
				t.Errorf("FromConnection() Name = %q, want %q", result.Name, tt.expected.Name)
			}
			if result.Type != tt.expected.Type {
				t.Errorf("FromConnection() Type = %q, want %q", result.Type, tt.expected.Type)
			}
			if result.Address != tt.expected.Address {
				t.Errorf("FromConnection() Address = %q, want %q", result.Address, tt.expected.Address)
			}
			if result.Namespace != tt.expected.Namespace {
				t.Errorf("FromConnection() Namespace = %q, want %q", result.Namespace, tt.expected.Namespace)
			}
			if result.BinaryPath != tt.expected.BinaryPath {
				t.Errorf("FromConnection() BinaryPath = %q, want %q", result.BinaryPath, tt.expected.BinaryPath)
			}
			if result.TLSSkipVerify != tt.expected.TLSSkipVerify {
				t.Errorf("FromConnection() TLSSkipVerify = %v, want %v", result.TLSSkipVerify, tt.expected.TLSSkipVerify)
			}
			if result.CACert != tt.expected.CACert {
				t.Errorf("FromConnection() CACert = %q, want %q", result.CACert, tt.expected.CACert)
			}
			if result.CAPath != tt.expected.CAPath {
				t.Errorf("FromConnection() CAPath = %q, want %q", result.CAPath, tt.expected.CAPath)
			}
			if result.ClientCert != tt.expected.ClientCert {
				t.Errorf("FromConnection() ClientCert = %q, want %q", result.ClientCert, tt.expected.ClientCert)
			}
			if result.ClientKey != tt.expected.ClientKey {
				t.Errorf("FromConnection() ClientKey = %q, want %q", result.ClientKey, tt.expected.ClientKey)
			}
		})
	}
}
