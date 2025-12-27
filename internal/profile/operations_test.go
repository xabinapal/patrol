package profile

import (
	"testing"

	"github.com/xabinapal/patrol/internal/config"
)

func TestGetCurrent(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := GetCurrent(nil)
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("empty current", func(t *testing.T) {
		cfg := &config.Config{
			Current: "",
		}
		_, err := GetCurrent(cfg)
		if err == nil {
			t.Error("expected error for empty current profile")
		}
	})

	t.Run("current profile not found", func(t *testing.T) {
		cfg := &config.Config{
			Current:     "nonexistent",
			Connections: []config.Connection{},
		}
		_, err := GetCurrent(cfg)
		if err == nil {
			t.Error("expected error for nonexistent current profile")
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			Current: "test",
			Connections: []config.Connection{
				{Name: "test", Address: "https://vault.example.com:8200"},
			},
		}
		prof, err := GetCurrent(cfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if prof == nil {
			t.Fatal("expected profile, got nil")
		}
		if prof.Connection.Name != "test" {
			t.Errorf("expected name 'test', got %q", prof.Connection.Name)
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := Get(nil, "test")
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("profile not found", func(t *testing.T) {
		cfg := &config.Config{
			Connections: []config.Connection{
				{Name: "other", Address: "https://vault.example.com:8200"},
			},
		}
		_, err := Get(cfg, "test")
		if err == nil {
			t.Error("expected error for nonexistent profile")
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			Connections: []config.Connection{
				{Name: "test", Address: "https://vault.example.com:8200"},
			},
		}
		prof, err := Get(cfg, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if prof == nil {
			t.Fatal("expected profile, got nil")
		}
		if prof.Connection.Name != "test" {
			t.Errorf("expected name 'test', got %q", prof.Connection.Name)
		}
	})
}

func TestList(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		result := List(nil)
		if result != nil {
			t.Error("expected nil for nil config")
		}
	})

	t.Run("empty connections", func(t *testing.T) {
		cfg := &config.Config{
			Connections: []config.Connection{},
		}
		result := List(cfg)
		if len(result) != 0 {
			t.Errorf("expected empty list, got %d items", len(result))
		}
	})

	t.Run("multiple profiles", func(t *testing.T) {
		cfg := &config.Config{
			Current: "prod",
			Connections: []config.Connection{
				{Name: "dev", Address: "https://vault-dev.example.com:8200", Type: config.BinaryTypeVault},
				{Name: "prod", Address: "https://vault-prod.example.com:8200", Type: config.BinaryTypeVault, Namespace: "admin"},
			},
		}
		result := List(cfg)
		if len(result) != 2 {
			t.Fatalf("expected 2 profiles, got %d", len(result))
		}

		// Check first profile
		if result[0].Name != "dev" {
			t.Errorf("expected first profile name 'dev', got %q", result[0].Name)
		}
		if result[0].Current {
			t.Error("expected first profile not to be current")
		}

		// Check second profile
		if result[1].Name != "prod" {
			t.Errorf("expected second profile name 'prod', got %q", result[1].Name)
		}
		if !result[1].Current {
			t.Error("expected second profile to be current")
		}
		if result[1].Namespace != "admin" {
			t.Errorf("expected namespace 'admin', got %q", result[1].Namespace)
		}
	})
}

func TestGetStatus(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := GetStatus(nil, "test")
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("profile not found", func(t *testing.T) {
		cfg := &config.Config{
			Connections: []config.Connection{},
		}
		_, err := GetStatus(cfg, "test")
		if err == nil {
			t.Error("expected error for nonexistent profile")
		}
	})

	t.Run("success with all fields", func(t *testing.T) {
		cfg := &config.Config{
			Current: "test",
			Connections: []config.Connection{
				{
					Name:          "test",
					Address:       "https://vault.example.com:8200",
					Type:          config.BinaryTypeVault,
					Namespace:     "admin",
					BinaryPath:    "/usr/local/bin/vault",
					TLSSkipVerify: true,
					CACert:        "/path/to/ca.crt",
					CAPath:        "/path/to/certs",
					ClientCert:    "/path/to/client.crt",
					ClientKey:     "/path/to/client.key",
				},
			},
		}
		status, err := GetStatus(cfg, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected status, got nil")
		}

		// Check all fields
		if status.Name != "test" {
			t.Errorf("expected name 'test', got %q", status.Name)
		}
		if status.Address != "https://vault.example.com:8200" {
			t.Errorf("expected address 'https://vault.example.com:8200', got %q", status.Address)
		}
		if status.Type != "vault" {
			t.Errorf("expected type 'vault', got %q", status.Type)
		}
		if status.Namespace != "admin" {
			t.Errorf("expected namespace 'admin', got %q", status.Namespace)
		}
		if status.BinaryPath != "/usr/local/bin/vault" {
			t.Errorf("expected binary path '/usr/local/bin/vault', got %q", status.BinaryPath)
		}
		if !status.TLSSkipVerify {
			t.Error("expected TLSSkipVerify to be true")
		}
		if status.CACert != "/path/to/ca.crt" {
			t.Errorf("expected CACert '/path/to/ca.crt', got %q", status.CACert)
		}
		if status.CAPath != "/path/to/certs" {
			t.Errorf("expected CAPath '/path/to/certs', got %q", status.CAPath)
		}
		if status.ClientCert != "/path/to/client.crt" {
			t.Errorf("expected ClientCert '/path/to/client.crt', got %q", status.ClientCert)
		}
		if status.ClientKey != "/path/to/client.key" {
			t.Errorf("expected ClientKey '/path/to/client.key', got %q", status.ClientKey)
		}
		if !status.Active {
			t.Error("expected status to be active")
		}
	})

	t.Run("inactive profile", func(t *testing.T) {
		cfg := &config.Config{
			Current: "other",
			Connections: []config.Connection{
				{Name: "test", Address: "https://vault.example.com:8200"},
				{Name: "other", Address: "https://vault-other.example.com:8200"},
			},
		}
		status, err := GetStatus(cfg, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status.Active {
			t.Error("expected status to be inactive")
		}
	})
}
