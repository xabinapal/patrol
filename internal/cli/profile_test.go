package cli

import (
	"testing"

	"github.com/xabinapal/patrol/internal/config"
)

func TestProfileListOutput(t *testing.T) {
	output := ProfileListOutput{
		Current: "prod",
		Profiles: []ProfileListOutputItem{
			{Name: "dev", Address: "http://localhost:8200", Type: "vault", Current: false},
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
