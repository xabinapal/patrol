package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	if cfg.Daemon.CheckInterval != time.Minute {
		t.Errorf("expected CheckInterval %v, got %v", time.Minute, cfg.Daemon.CheckInterval)
	}

	if cfg.Daemon.RenewThreshold != 0.75 {
		t.Errorf("expected RenewThreshold 0.75, got %v", cfg.Daemon.RenewThreshold)
	}

	if cfg.Daemon.MinRenewTTL != 5*time.Minute {
		t.Errorf("expected MinRenewTTL %v, got %v", 5*time.Minute, cfg.Daemon.MinRenewTTL)
	}

	if !cfg.RevokeOnLogout {
		t.Error("expected RevokeOnLogout to be true by default")
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Load from non-existent file should return defaults
	tmpDir := t.TempDir()
	oldEnv := os.Getenv("PATROL_CONFIG_DIR")
	os.Setenv("PATROL_CONFIG_DIR", tmpDir)
	defer os.Setenv("PATROL_CONFIG_DIR", oldEnv)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Should have defaults
	if cfg.Daemon.CheckInterval != time.Minute {
		t.Errorf("expected default CheckInterval, got %v", cfg.Daemon.CheckInterval)
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	cfg := Default()
	cfg.filePath = configFile
	cfg.Current = "test-profile"
	cfg.Connections = []Connection{
		{
			Name:    "test-profile",
			Address: "https://vault.example.com:8200",
			Type:    BinaryTypeVault,
		},
	}

	// Save
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load
	loaded, err := LoadFrom(configFile)
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if loaded.Current != "test-profile" {
		t.Errorf("expected Current 'test-profile', got '%s'", loaded.Current)
	}

	if len(loaded.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(loaded.Connections))
	}

	if loaded.Connections[0].Address != "https://vault.example.com:8200" {
		t.Errorf("expected address 'https://vault.example.com:8200', got '%s'",
			loaded.Connections[0].Address)
	}
}

func TestAddConnection(t *testing.T) {
	cfg := Default()

	conn := Connection{
		Name:    "test",
		Address: "https://vault.example.com:8200",
		Type:    BinaryTypeVault,
	}

	if err := cfg.AddConnection(conn); err != nil {
		t.Fatalf("AddConnection() failed: %v", err)
	}

	// First connection should become current
	if cfg.Current != "test" {
		t.Errorf("expected Current 'test', got '%s'", cfg.Current)
	}

	// Adding duplicate should fail
	if err := cfg.AddConnection(conn); err == nil {
		t.Error("AddConnection() should fail for duplicate name")
	}
}

func TestAddConnectionValidation(t *testing.T) {
	tests := []struct {
		name    string
		conn    Connection
		wantErr bool
	}{
		{
			name:    "missing name",
			conn:    Connection{Address: "https://vault.example.com"},
			wantErr: true,
		},
		{
			name:    "missing address",
			conn:    Connection{Name: "test"},
			wantErr: true,
		},
		{
			name: "valid connection",
			conn: Connection{
				Name:    "test",
				Address: "https://vault.example.com:8200",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := Default()
			err := testCfg.AddConnection(tt.conn)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddConnection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveConnection(t *testing.T) {
	cfg := Default()
	cfg.Connections = []Connection{
		{Name: "first", Address: "https://first.example.com"},
		{Name: "second", Address: "https://second.example.com"},
	}
	cfg.Current = "first"

	// Remove current connection
	if err := cfg.RemoveConnection("first"); err != nil {
		t.Fatalf("RemoveConnection() failed: %v", err)
	}

	// Current should switch to remaining connection
	if cfg.Current != "second" {
		t.Errorf("expected Current 'second', got '%s'", cfg.Current)
	}

	// Remove non-existent should fail
	if err := cfg.RemoveConnection("nonexistent"); err == nil {
		t.Error("RemoveConnection() should fail for non-existent connection")
	}
}

func TestSetCurrent(t *testing.T) {
	cfg := Default()
	cfg.Connections = []Connection{
		{Name: "test", Address: "https://vault.example.com"},
	}

	if err := cfg.SetCurrent("test"); err != nil {
		t.Fatalf("SetCurrent() failed: %v", err)
	}

	if cfg.Current != "test" {
		t.Errorf("expected Current 'test', got '%s'", cfg.Current)
	}

	// Non-existent should fail
	if err := cfg.SetCurrent("nonexistent"); err == nil {
		t.Error("SetCurrent() should fail for non-existent connection")
	}
}

func TestConnectionGetBinaryPath(t *testing.T) {
	tests := []struct {
		name     string
		conn     Connection
		expected string
	}{
		{
			name:     "default vault",
			conn:     Connection{Type: BinaryTypeVault},
			expected: "vault",
		},
		{
			name:     "openbao",
			conn:     Connection{Type: BinaryTypeOpenBao},
			expected: "bao",
		},
		{
			name:     "custom path",
			conn:     Connection{Type: BinaryTypeVault, BinaryPath: "/opt/vault/bin/vault"},
			expected: "/opt/vault/bin/vault",
		},
		{
			name:     "empty type defaults to vault",
			conn:     Connection{},
			expected: "vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.conn.GetBinaryPath()
			if result != tt.expected {
				t.Errorf("GetBinaryPath() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestConnectionKeyringKey(t *testing.T) {
	tests := []struct {
		name     string
		conn     Connection
		expected string
	}{
		{
			name: "profile name used as key",
			conn: Connection{
				Name:    "production",
				Address: "https://vault.example.com:8200",
			},
			expected: "production",
		},
		{
			name: "profile with namespace still uses name",
			conn: Connection{
				Name:      "staging",
				Address:   "https://vault.example.com:8200",
				Namespace: "team/ns1",
			},
			expected: "staging",
		},
		{
			name: "different profile same address",
			conn: Connection{
				Name:    "dev-local",
				Address: "http://localhost:8200",
			},
			expected: "dev-local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.conn.KeyringKey()
			if result != tt.expected {
				t.Errorf("KeyringKey() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestGetCurrentConnection(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		connections []Connection
		wantErr     bool
		errContains string
	}{
		{
			name:        "no active connection",
			current:     "",
			connections: []Connection{},
			wantErr:     true,
			errContains: "no active connection configured",
		},
		{
			name:    "active connection exists",
			current: "test",
			connections: []Connection{
				{Name: "test", Address: "https://vault.example.com"},
			},
			wantErr: false,
		},
		{
			name:    "active connection not found",
			current: "missing",
			connections: []Connection{
				{Name: "test", Address: "https://vault.example.com"},
			},
			wantErr:     true,
			errContains: "connection \"missing\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Current = tt.current
			cfg.Connections = tt.connections

			conn, err := cfg.GetCurrentConnection()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetCurrentConnection() error = %v, should contain %q", err, tt.errContains)
				}
			} else {
				if conn == nil {
					t.Error("GetCurrentConnection() returned nil connection")
				} else if conn.Name != tt.current {
					t.Errorf("GetCurrentConnection() returned connection %q, expected %q", conn.Name, tt.current)
				}
			}
		})
	}
}

func TestLoadFromInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	invalidYAML := "this is not: valid: yaml: content"
	if err := os.WriteFile(configFile, []byte(invalidYAML), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadFrom(configFile)
	if err == nil {
		t.Error("LoadFrom() should fail with invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("LoadFrom() error = %v, should contain 'failed to parse config file'", err)
	}
}

func TestLoadFromReadError(t *testing.T) {
	// Try to read a directory instead of a file
	tmpDir := t.TempDir()

	_, err := LoadFrom(tmpDir)
	if err == nil {
		t.Error("LoadFrom() should fail when reading a directory")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("LoadFrom() error = %v, should contain 'failed to read config file'", err)
	}
}

func TestSaveWithoutFilePath(t *testing.T) {
	cfg := Default()
	cfg.filePath = ""

	err := cfg.Save()
	if err == nil {
		t.Error("Save() should fail when filePath is empty")
	}
	if !strings.Contains(err.Error(), "config file path not set") {
		t.Errorf("Save() error = %v, should contain 'config file path not set'", err)
	}
}

func TestRemoveLastConnection(t *testing.T) {
	cfg := Default()
	cfg.Connections = []Connection{
		{Name: "only", Address: "https://vault.example.com"},
	}
	cfg.Current = "only"

	if err := cfg.RemoveConnection("only"); err != nil {
		t.Fatalf("RemoveConnection() failed: %v", err)
	}

	// Current should be empty when last connection is removed
	if cfg.Current != "" {
		t.Errorf("expected Current to be empty, got %q", cfg.Current)
	}

	if len(cfg.Connections) != 0 {
		t.Errorf("expected 0 connections, got %d", len(cfg.Connections))
	}
}

func TestRemoveNonCurrentConnection(t *testing.T) {
	cfg := Default()
	cfg.Connections = []Connection{
		{Name: "first", Address: "https://first.example.com"},
		{Name: "second", Address: "https://second.example.com"},
		{Name: "third", Address: "https://third.example.com"},
	}
	cfg.Current = "first"

	// Remove a connection that's not current
	if err := cfg.RemoveConnection("second"); err != nil {
		t.Fatalf("RemoveConnection() failed: %v", err)
	}

	// Current should remain unchanged
	if cfg.Current != "first" {
		t.Errorf("expected Current 'first', got %q", cfg.Current)
	}

	if len(cfg.Connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(cfg.Connections))
	}

	// Verify the right connection was removed
	for _, conn := range cfg.Connections {
		if conn.Name == "second" {
			t.Error("'second' connection should have been removed")
		}
	}
}

func TestGetConnection(t *testing.T) {
	cfg := Default()
	cfg.Connections = []Connection{
		{Name: "test", Address: "https://vault.example.com"},
		{Name: "prod", Address: "https://prod.example.com"},
	}

	// Test successful get
	conn, err := cfg.GetConnection("test")
	if err != nil {
		t.Fatalf("GetConnection() failed: %v", err)
	}
	if conn.Name != "test" {
		t.Errorf("expected connection name 'test', got %q", conn.Name)
	}

	// Test not found
	_, err = cfg.GetConnection("nonexistent")
	if err == nil {
		t.Error("GetConnection() should fail for non-existent connection")
	}
	if !strings.Contains(err.Error(), "connection \"nonexistent\" not found") {
		t.Errorf("GetConnection() error = %v, should contain 'connection \"nonexistent\" not found'", err)
	}
}

func TestAddConnectionDefaultType(t *testing.T) {
	cfg := Default()

	conn := Connection{
		Name:    "test",
		Address: "https://vault.example.com",
		// No Type specified
	}

	if err := cfg.AddConnection(conn); err != nil {
		t.Fatalf("AddConnection() failed: %v", err)
	}

	// Should default to Vault
	if cfg.Connections[0].Type != BinaryTypeVault {
		t.Errorf("expected Type to default to %q, got %q", BinaryTypeVault, cfg.Connections[0].Type)
	}
}

func TestLoadFromAppliesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a config file with missing daemon values
	yamlContent := `current: test
connections:
  - name: test
    address: https://vault.example.com
daemon:
  auto_start: true
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg, err := LoadFrom(configFile)
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	// Check that defaults were applied
	if cfg.Daemon.CheckInterval != time.Minute {
		t.Errorf("expected default CheckInterval %v, got %v", time.Minute, cfg.Daemon.CheckInterval)
	}
	if cfg.Daemon.RenewThreshold != 0.75 {
		t.Errorf("expected default RenewThreshold 0.75, got %v", cfg.Daemon.RenewThreshold)
	}
	if cfg.Daemon.MinRenewTTL != 5*time.Minute {
		t.Errorf("expected default MinRenewTTL %v, got %v", 5*time.Minute, cfg.Daemon.MinRenewTTL)
	}
	// Check that the auto_start value was preserved
	if !cfg.Daemon.AutoStart {
		t.Error("expected AutoStart to be true as specified in config")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Override PATROL_CONFIG_DIR to use our temp directory
	oldEnv := os.Getenv("PATROL_CONFIG_DIR")
	os.Setenv("PATROL_CONFIG_DIR", tmpDir)
	defer os.Setenv("PATROL_CONFIG_DIR", oldEnv)

	// Use a nested path that doesn't exist
	configFile := filepath.Join(tmpDir, "config.yaml")

	cfg := Default()
	cfg.filePath = configFile
	cfg.Current = "test"
	cfg.Connections = []Connection{
		{Name: "test", Address: "https://vault.example.com"},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() should create directory: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file should have been created")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a config with all fields set
	cfg := Default()
	cfg.filePath = configFile
	cfg.Current = "prod"
	cfg.Connections = []Connection{
		{
			Name:          "dev",
			Address:       "https://vault-dev.example.com:8200",
			Type:          BinaryTypeVault,
			BinaryPath:    "/usr/local/bin/vault",
			Namespace:     "team/dev",
			TLSSkipVerify: true,
			CACert:        "/etc/ssl/ca.pem",
			CAPath:        "/etc/ssl/certs",
			ClientCert:    "/etc/ssl/client.pem",
			ClientKey:     "/etc/ssl/client-key.pem",
		},
		{
			Name:    "prod",
			Address: "https://vault-prod.example.com:8200",
			Type:    BinaryTypeOpenBao,
		},
	}
	cfg.Daemon.AutoStart = true
	cfg.Daemon.CheckInterval = 2 * time.Minute
	cfg.Daemon.RenewThreshold = 0.8
	cfg.Daemon.MinRenewTTL = 10 * time.Minute
	cfg.Daemon.LogFile = "/var/log/patrol.log"
	cfg.Daemon.PIDFile = "/var/run/patrol.pid"
	cfg.RevokeOnLogout = true // Keep default value for proper round-trip

	// Save
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load
	loaded, err := LoadFrom(configFile)
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	// Verify all fields
	if loaded.Current != cfg.Current {
		t.Errorf("Current mismatch: got %q, want %q", loaded.Current, cfg.Current)
	}
	if len(loaded.Connections) != len(cfg.Connections) {
		t.Fatalf("Connections count mismatch: got %d, want %d", len(loaded.Connections), len(cfg.Connections))
	}

	// Check first connection in detail
	conn := loaded.Connections[0]
	if conn.Name != "dev" || conn.Address != "https://vault-dev.example.com:8200" {
		t.Errorf("Connection details mismatch: got %+v", conn)
	}
	if conn.BinaryPath != "/usr/local/bin/vault" {
		t.Errorf("BinaryPath mismatch: got %q", conn.BinaryPath)
	}
	if conn.Namespace != "team/dev" {
		t.Errorf("Namespace mismatch: got %q", conn.Namespace)
	}
	if !conn.TLSSkipVerify {
		t.Error("TLSSkipVerify should be true")
	}
	if conn.CACert != "/etc/ssl/ca.pem" {
		t.Errorf("CACert mismatch: got %q", conn.CACert)
	}

	// Check daemon config
	if loaded.Daemon.AutoStart != true {
		t.Error("Daemon.AutoStart should be true")
	}
	if loaded.Daemon.CheckInterval != 2*time.Minute {
		t.Errorf("CheckInterval mismatch: got %v, want %v", loaded.Daemon.CheckInterval, 2*time.Minute)
	}
	if loaded.Daemon.RenewThreshold != 0.8 {
		t.Errorf("RenewThreshold mismatch: got %v, want 0.8", loaded.Daemon.RenewThreshold)
	}
	if loaded.RevokeOnLogout != cfg.RevokeOnLogout {
		t.Errorf("RevokeOnLogout mismatch: got %v, want %v", loaded.RevokeOnLogout, cfg.RevokeOnLogout)
	}
}

// Security validation tests

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid https",
			address: "https://vault.example.com:8200",
			wantErr: false,
		},
		{
			name:    "valid http",
			address: "http://localhost:8200",
			wantErr: false,
		},
		{
			name:    "valid https without port",
			address: "https://vault.example.com",
			wantErr: false,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			address: "ftp://vault.example.com",
			wantErr: true,
		},
		{
			name:    "no scheme",
			address: "vault.example.com:8200",
			wantErr: true,
		},
		{
			name:    "no host",
			address: "https://",
			wantErr: true,
		},
		{
			name:    "file scheme",
			address: "file:///etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{Address: tt.address}
			err := conn.ValidateAddress()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidAddress) {
				t.Errorf("ValidateAddress() error should wrap ErrInvalidAddress, got %v", err)
			}
		})
	}
}

func TestValidateBinaryPath(t *testing.T) {
	// Create a temporary directory with test binaries
	tmpDir := t.TempDir()

	// Create a fake "vault" binary
	vaultPath := filepath.Join(tmpDir, "vault")
	if runtime.GOOS == "windows" {
		vaultPath += ".exe"
	}
	if err := os.WriteFile(vaultPath, []byte("#!/bin/sh\necho fake vault"), 0755); err != nil {
		t.Fatalf("failed to create fake vault binary: %v", err)
	}

	// Create a fake "bao" binary
	baoPath := filepath.Join(tmpDir, "bao")
	if runtime.GOOS == "windows" {
		baoPath += ".exe"
	}
	if err := os.WriteFile(baoPath, []byte("#!/bin/sh\necho fake bao"), 0755); err != nil {
		t.Fatalf("failed to create fake bao binary: %v", err)
	}

	// Create a malicious binary with a different name
	maliciousPath := filepath.Join(tmpDir, "malware")
	if err := os.WriteFile(maliciousPath, []byte("#!/bin/sh\necho malware"), 0755); err != nil {
		t.Fatalf("failed to create malicious binary: %v", err)
	}

	// Create a non-executable file
	nonExecPath := filepath.Join(tmpDir, "vault-noexec")
	if err := os.WriteFile(nonExecPath, []byte("not executable"), 0644); err != nil {
		t.Fatalf("failed to create non-executable file: %v", err)
	}

	// Create a directory (not a file)
	dirPath := filepath.Join(tmpDir, "vault-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	tests := []struct {
		name       string
		binaryPath string
		wantErr    bool
	}{
		{
			name:       "empty path (uses default)",
			binaryPath: "",
			wantErr:    false,
		},
		{
			name:       "allowed name: vault",
			binaryPath: "vault",
			wantErr:    false,
		},
		{
			name:       "allowed name: bao",
			binaryPath: "bao",
			wantErr:    false,
		},
		{
			name:       "any binary name (looked up in PATH)",
			binaryPath: "malware",
			wantErr:    false, // Binary names are allowed - will be looked up in PATH
		},
		{
			name:       "relative path (not allowed)",
			binaryPath: "bin/vault",
			wantErr:    true,
		},
		{
			name:       "absolute path to valid vault",
			binaryPath: vaultPath,
			wantErr:    false,
		},
		{
			name:       "absolute path to valid bao",
			binaryPath: baoPath,
			wantErr:    false,
		},
		{
			name:       "absolute path to any executable binary",
			binaryPath: maliciousPath,
			wantErr:    false, // Binary name doesn't matter - if path is absolute and executable, it's allowed
		},
		{
			name:       "path traversal (contains ..)",
			binaryPath: tmpDir + "/../" + filepath.Base(tmpDir) + "/vault",
			wantErr:    true, // Path traversal is detected by checking for ".." in the path
		},
		{
			name:       "non-existent path",
			binaryPath: filepath.Join(tmpDir, "nonexistent", "vault"),
			wantErr:    true,
		},
		{
			name:       "directory instead of file",
			binaryPath: dirPath,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{BinaryPath: tt.binaryPath}
			err := conn.ValidateBinaryPath()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBinaryPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidBinaryPath) {
				t.Errorf("ValidateBinaryPath() error should wrap ErrInvalidBinaryPath, got %v", err)
			}
		})
	}

	// Test non-executable file separately (skip on Windows where execute bits work differently)
	if runtime.GOOS != "windows" {
		t.Run("non-executable file", func(t *testing.T) {
			// Create a file named "vault" but non-executable
			nonExecVault := filepath.Join(tmpDir, "vault-nonexec")
			// First remove any existing file
			os.Remove(nonExecVault)
			// Create parent dir with vault name
			subDir := filepath.Join(tmpDir, "noexec-test")
			os.MkdirAll(subDir, 0755)
			nonExecVaultPath := filepath.Join(subDir, "vault")
			if err := os.WriteFile(nonExecVaultPath, []byte("not executable"), 0644); err != nil {
				t.Fatalf("failed to create non-executable vault: %v", err)
			}

			conn := &Connection{BinaryPath: nonExecVaultPath}
			err := conn.ValidateBinaryPath()
			if err == nil {
				t.Error("ValidateBinaryPath() should fail for non-executable file")
			}
			if !errors.Is(err, ErrInvalidBinaryPath) {
				t.Errorf("ValidateBinaryPath() error should wrap ErrInvalidBinaryPath, got %v", err)
			}
		})
	}
}

func TestValidateConnection(t *testing.T) {
	// Create a valid temporary binary for tests
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "vault")
	if runtime.GOOS == "windows" {
		vaultPath += ".exe"
	}
	if err := os.WriteFile(vaultPath, []byte("#!/bin/sh\necho fake vault"), 0755); err != nil {
		t.Fatalf("failed to create fake vault binary: %v", err)
	}

	tests := []struct {
		name    string
		conn    Connection
		wantErr bool
	}{
		{
			name: "valid connection with defaults",
			conn: Connection{
				Name:    "test",
				Address: "https://vault.example.com:8200",
				Type:    BinaryTypeVault,
			},
			wantErr: false,
		},
		{
			name: "valid connection with custom binary path",
			conn: Connection{
				Name:       "test",
				Address:    "https://vault.example.com:8200",
				Type:       BinaryTypeVault,
				BinaryPath: vaultPath,
			},
			wantErr: false,
		},
		{
			name: "invalid address",
			conn: Connection{
				Name:    "test",
				Address: "not-a-url",
				Type:    BinaryTypeVault,
			},
			wantErr: true,
		},
		{
			name: "valid connection with custom binary name",
			conn: Connection{
				Name:       "test",
				Address:    "https://vault.example.com:8200",
				Type:       BinaryTypeVault,
				BinaryPath: "vault1.21.1", // Any binary name is allowed
			},
			wantErr: false,
		},
		{
			name: "invalid address (binary name is valid)",
			conn: Connection{
				Name:       "test",
				Address:    "ftp://invalid",
				BinaryPath: "vault1.21.1", // Binary name is valid, but address is invalid
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conn.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
