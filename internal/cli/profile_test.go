package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/xabinapal/patrol/internal/config"
)

func TestProfileExportData(t *testing.T) {
	data := ProfileExportData{
		Name:          "test-profile",
		Address:       "https://vault.example.com:8200",
		Type:          "vault",
		Namespace:     "admin/team",
		TLSSkipVerify: true,
	}

	if data.Name != "test-profile" {
		t.Errorf("ProfileExportData.Name = %q, want %q", data.Name, "test-profile")
	}
	if data.Address != "https://vault.example.com:8200" {
		t.Errorf("ProfileExportData.Address = %q, want %q", data.Address, "https://vault.example.com:8200")
	}
	if data.Type != "vault" {
		t.Errorf("ProfileExportData.Type = %q, want %q", data.Type, "vault")
	}
	if data.Namespace != "admin/team" {
		t.Errorf("ProfileExportData.Namespace = %q, want %q", data.Namespace, "admin/team")
	}
	if !data.TLSSkipVerify {
		t.Error("ProfileExportData.TLSSkipVerify should be true")
	}
}

func TestProfileTestOutput(t *testing.T) {
	output := ProfileTestOutput{
		Profile: "prod",
		Status:  "healthy",
	}

	if output.Profile != "prod" {
		t.Errorf("ProfileTestOutput.Profile = %q, want %q", output.Profile, "prod")
	}
	if output.Status != "healthy" {
		t.Errorf("ProfileTestOutput.Status = %q, want %q", output.Status, "healthy")
	}
}

func TestProfileInfo(t *testing.T) {
	info := ProfileInfo{
		Name:     "test",
		LoggedIn: true,
		Current:  true,
	}

	if info.Name != "test" {
		t.Errorf("ProfileInfo.Name = %q, want %q", info.Name, "test")
	}
	if !info.LoggedIn {
		t.Error("ProfileInfo.LoggedIn should be true")
	}
	if !info.Current {
		t.Error("ProfileInfo.Current should be true")
	}
}

func TestProfileListOutput(t *testing.T) {
	output := ProfileListOutput{
		Current: "prod",
		Profiles: []ProfileInfo{
			{Name: "dev", Address: "http://localhost:8200", Type: "vault"},
			{Name: "prod", Address: "https://vault.example.com:8200", Type: "vault", Current: true},
		},
	}

	if output.Current != "prod" {
		t.Errorf("ProfileListOutput.Current = %q, want %q", output.Current, "prod")
	}
	if len(output.Profiles) != 2 {
		t.Errorf("ProfileListOutput.Profiles length = %d, want 2", len(output.Profiles))
	}
}

func TestProfileShowOutput(t *testing.T) {
	output := ProfileShowOutput{
		Name:      "prod",
		Namespace: "admin",
		Active:    true,
	}

	if output.Name != "prod" {
		t.Errorf("ProfileShowOutput.Name = %q, want %q", output.Name, "prod")
	}
	if output.Namespace != "admin" {
		t.Errorf("ProfileShowOutput.Namespace = %q, want %q", output.Namespace, "admin")
	}
	if output.TLSSkipVerify {
		t.Error("ProfileShowOutput.TLSSkipVerify should be false")
	}
	if !output.Active {
		t.Error("ProfileShowOutput.Active should be true")
	}
}

func TestGetProfileNames(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Connections: []config.Connection{
				{Name: "dev"},
				{Name: "staging"},
				{Name: "prod"},
			},
		},
	}

	names := cli.getProfileNames()

	if len(names) != 3 {
		t.Fatalf("getProfileNames() returned %d names, want 3", len(names))
	}

	expected := []string{"dev", "staging", "prod"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("getProfileNames()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestGetProfileNamesNilConfig(t *testing.T) {
	cli := &CLI{
		Config: nil,
	}

	names := cli.getProfileNames()

	if names != nil {
		t.Errorf("getProfileNames() = %v, want nil", names)
	}
}

func TestProfileExportImportRoundTrip(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	exportFile := filepath.Join(tmpDir, "profile.yaml")

	// Original profile data
	original := ProfileExportData{
		Name:          "roundtrip-test",
		Address:       "https://vault.example.com:8200",
		Type:          "vault",
		Namespace:     "admin/team",
		TLSSkipVerify: true,
		CACert:        "/path/to/ca.crt",
	}

	// Marshal to YAML
	data, err := marshalYAML(original)
	if err != nil {
		t.Fatalf("marshalYAML() error = %v", err)
	}

	// Write to file
	if writeErr := os.WriteFile(exportFile, data, 0600); writeErr != nil {
		t.Fatalf("WriteFile() error = %v", writeErr)
	}

	// Read back
	readData, err := os.ReadFile(exportFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Unmarshal
	var imported ProfileExportData
	if err := unmarshalYAML(readData, &imported); err != nil {
		t.Fatalf("unmarshalYAML() error = %v", err)
	}

	// Verify fields match
	if imported.Name != original.Name {
		t.Errorf("Name = %q, want %q", imported.Name, original.Name)
	}
	if imported.Address != original.Address {
		t.Errorf("Address = %q, want %q", imported.Address, original.Address)
	}
	if imported.Type != original.Type {
		t.Errorf("Type = %q, want %q", imported.Type, original.Type)
	}
	if imported.Namespace != original.Namespace {
		t.Errorf("Namespace = %q, want %q", imported.Namespace, original.Namespace)
	}
	if imported.TLSSkipVerify != original.TLSSkipVerify {
		t.Errorf("TLSSkipVerify = %v, want %v", imported.TLSSkipVerify, original.TLSSkipVerify)
	}
	if imported.CACert != original.CACert {
		t.Errorf("CACert = %q, want %q", imported.CACert, original.CACert)
	}
}

// Helper functions to wrap yaml operations for testing
func marshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func unmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
