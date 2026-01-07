package utils

import (
	"strings"
	"testing"
)

func TestIsValidProfileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid simple name",
			input:    "profile1",
			expected: true,
		},
		{
			name:     "valid with dash",
			input:    "my-profile",
			expected: true,
		},
		{
			name:     "valid with underscore",
			input:    "my_profile",
			expected: true,
		},
		{
			name:     "valid with dot",
			input:    "my.profile",
			expected: true,
		},
		{
			name:     "valid mixed case",
			input:    "MyProfile123",
			expected: true,
		},
		{
			name:     "valid with numbers",
			input:    "profile123",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "too long",
			input:    strings.Repeat("a", 129), // 129 characters
			expected: false,
		},
		{
			name:     "exactly 128 characters",
			input:    strings.Repeat("a", 128), // 128 characters
			expected: true,
		},
		{
			name:     "contains space",
			input:    "my profile",
			expected: false,
		},
		{
			name:     "contains slash",
			input:    "my/profile",
			expected: false,
		},
		{
			name:     "contains backslash",
			input:    "my\\profile",
			expected: false,
		},
		{
			name:     "contains special characters",
			input:    "my@profile",
			expected: false,
		},
		{
			name:     "contains hash",
			input:    "my#profile",
			expected: false,
		},
		{
			name:     "contains exclamation",
			input:    "my!profile",
			expected: false,
		},
		{
			name:     "contains parentheses",
			input:    "my(profile)",
			expected: false,
		},
		{
			name:     "contains brackets",
			input:    "my[profile]",
			expected: false,
		},
		{
			name:     "contains braces",
			input:    "my{profile}",
			expected: false,
		},
		{
			name:     "contains pipe",
			input:    "my|profile",
			expected: false,
		},
		{
			name:     "contains ampersand",
			input:    "my&profile",
			expected: false,
		},
		{
			name:     "contains percent",
			input:    "my%profile",
			expected: false,
		},
		{
			name:     "contains asterisk",
			input:    "my*profile",
			expected: false,
		},
		{
			name:     "contains plus",
			input:    "my+profile",
			expected: false,
		},
		{
			name:     "contains equals",
			input:    "my=profile",
			expected: false,
		},
		{
			name:     "contains question mark",
			input:    "my?profile",
			expected: false,
		},
		{
			name:     "contains colon",
			input:    "my:profile",
			expected: false,
		},
		{
			name:     "contains semicolon",
			input:    "my;profile",
			expected: false,
		},
		{
			name:     "contains comma",
			input:    "my,profile",
			expected: false,
		},
		{
			name:     "contains quote",
			input:    "my\"profile",
			expected: false,
		},
		{
			name:     "contains single quote",
			input:    "my'profile",
			expected: false,
		},
		{
			name:     "contains backtick",
			input:    "my`profile",
			expected: false,
		},
		{
			name:     "contains tilde",
			input:    "my~profile",
			expected: false,
		},
		{
			name:     "contains dollar",
			input:    "my$profile",
			expected: false,
		},
		{
			name:     "valid single character",
			input:    "a",
			expected: true,
		},
		{
			name:     "valid single digit",
			input:    "1",
			expected: true,
		},
		{
			name:     "valid all allowed characters",
			input:    "aA0-_.zZ9",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidProfileName(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidProfileName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeAddressForProfile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "https URL",
			input:    "https://vault.example.com:8200",
			expected: "vault-example-com-8200",
		},
		{
			name:     "http URL",
			input:    "http://vault.example.com:8200",
			expected: "vault-example-com-8200",
		},
		{
			name:     "URL without scheme",
			input:    "vault.example.com:8200",
			expected: "vault-example-com-8200",
		},
		{
			name:     "URL with path",
			input:    "https://vault.example.com:8200/path/to/vault",
			expected: "vault-example-com-8200-path-to-vault",
		},
		{
			name:     "URL with trailing slash",
			input:    "https://vault.example.com:8200/",
			expected: "vault-example-com-8200",
		},
		{
			name:     "localhost",
			input:    "http://localhost:8200",
			expected: "localhost-8200",
		},
		{
			name:     "IP address",
			input:    "https://127.0.0.1:8200",
			expected: "127-0-0-1-8200",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAddressForProfile(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeAddressForProfile(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeNamespaceForProfile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple namespace",
			input:    "team1",
			expected: "team1",
		},
		{
			name:     "namespace with slash",
			input:    "team1/project1",
			expected: "team1-project1",
		},
		{
			name:     "nested namespace",
			input:    "team1/project1/env",
			expected: "team1-project1-env",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "leading slash",
			input:    "/team1",
			expected: "-team1",
		},
		{
			name:     "trailing slash",
			input:    "team1/",
			expected: "team1-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeNamespaceForProfile(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeNamespace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
