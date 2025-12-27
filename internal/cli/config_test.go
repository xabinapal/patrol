package cli

import (
	"testing"
)

func TestSuggestProfileName(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "localhost returns local",
			address: "http://localhost:8200",
			want:    "local",
		},
		{
			name:    "127.0.0.1 returns local",
			address: "https://127.0.0.1:8200",
			want:    "local",
		},
		{
			name:    "simple hostname",
			address: "https://vault:8200",
			want:    "vault",
		},
		{
			name:    "hostname with domain",
			address: "https://vault.company.com:8200",
			want:    "vault-company-com",
		},
		{
			name:    "example.com suffix removed",
			address: "https://prod.example.com:8200",
			want:    "prod",
		},
		{
			name:    "no protocol",
			address: "vault.example.com:8200",
			want:    "vault",
		},
		{
			name:    "empty address returns default",
			address: "",
			want:    "default",
		},
		{
			name:    "just protocol returns default",
			address: "https://",
			want:    "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestProfileName(tt.address)
			if got != tt.want {
				t.Errorf("suggestProfileName(%q) = %q, want %q", tt.address, got, tt.want)
			}
		})
	}
}
