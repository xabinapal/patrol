package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestSanitizeNamespace(t *testing.T) {
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
			result := SanitizeNamespace(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeNamespace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple key",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "key with dash",
			input:    "with-dash",
			expected: "with-dash",
		},
		{
			name:     "key with underscore",
			input:    "with_underscore",
			expected: "with_underscore",
		},
		{
			name:     "key with dot",
			input:    "with.dot",
			expected: "with_dot",
		},
		{
			name:     "key with colon",
			input:    "with:colon",
			expected: "with_colon",
		},
		{
			name:     "key with spaces",
			input:    "with spaces",
			expected: "with_spaces",
		},
		{
			name:     "mixed case and numbers",
			input:    "MixedCase123",
			expected: "MixedCase123",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "special characters",
			input:    "key@#$%",
			expected: "key____",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeKey(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeKeyPathTraversal(t *testing.T) {
	// Security: Keys with path traversal patterns should be hashed
	traversalPatterns := []string{
		"../etc/passwd",
		"..\\windows\\system32",
		"foo/bar",
		"foo\\bar",
		"../../..",
	}

	for _, pattern := range traversalPatterns {
		t.Run(pattern, func(t *testing.T) {
			result := SanitizeKey(pattern)
			// Result should be a SHA256 hash (64 hex chars)
			if len(result) != 64 {
				t.Errorf("SanitizeKey(%q) should return hash, got %q (length %d)", pattern, result, len(result))
			}
			// Verify it doesn't contain the original dangerous characters
			if strings.Contains(result, "/") || strings.Contains(result, "\\") || strings.Contains(result, "..") {
				t.Errorf("SanitizeKey(%q) = %q still contains dangerous characters", pattern, result)
			}
			// Verify it's a valid hex string (SHA256 hash)
			if _, err := hex.DecodeString(result); err != nil {
				t.Errorf("SanitizeKey(%q) = %q is not a valid hex string", pattern, result)
			}
			// Verify the hash is deterministic
			result2 := SanitizeKey(pattern)
			if result != result2 {
				t.Errorf("SanitizeKey(%q) is not deterministic: got %q and %q", pattern, result, result2)
			}
			// Verify it's actually a SHA256 hash of the input
			h := sha256.Sum256([]byte(pattern))
			expectedHash := hex.EncodeToString(h[:])
			if result != expectedHash {
				t.Errorf("SanitizeKey(%q) = %q, want %q", pattern, result, expectedHash)
			}
		})
	}
}

func TestSanitizeKeyWithFilepathSeparator(t *testing.T) {
	// Test with actual filepath separator
	pattern := "key" + string(filepath.Separator) + "value"
	result := SanitizeKey(pattern)

	// Should be hashed
	if len(result) != 64 {
		t.Errorf("SanitizeKey with filepath separator should return hash, got %q", result)
	}
	// Should not contain separator
	if strings.Contains(result, string(filepath.Separator)) {
		t.Errorf("SanitizeKey result contains filepath separator")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		subs     []string
		expected bool
	}{
		{
			name:     "contains first substring",
			s:        "hello world",
			subs:     []string{"hello", "foo"},
			expected: true,
		},
		{
			name:     "contains second substring",
			s:        "hello world",
			subs:     []string{"foo", "world"},
			expected: true,
		},
		{
			name:     "case insensitive match",
			s:        "Hello World",
			subs:     []string{"hello"},
			expected: true,
		},
		{
			name:     "case insensitive substring",
			s:        "hello world",
			subs:     []string{"HELLO"},
			expected: true,
		},
		{
			name:     "no match",
			s:        "test string",
			subs:     []string{"foo", "bar"},
			expected: false,
		},
		{
			name:     "contains multi-word substring",
			s:        "secret service error",
			subs:     []string{"secret service"},
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			subs:     []string{"anything"},
			expected: false,
		},
		{
			name:     "empty substrings",
			s:        "test",
			subs:     []string{},
			expected: false,
		},
		{
			name:     "empty string and empty subs",
			s:        "",
			subs:     []string{},
			expected: false,
		},
		{
			name:     "partial word match",
			s:        "database",
			subs:     []string{"data"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsAny(tt.s, tt.subs...)
			if result != tt.expected {
				t.Errorf("ContainsAny(%q, %v) = %v, want %v", tt.s, tt.subs, result, tt.expected)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		sep      byte
		expected int
	}{
		{
			name:     "find equals sign",
			s:        "KEY=value",
			sep:      '=',
			expected: 3,
		},
		{
			name:     "find colon",
			s:        "host:port",
			sep:      ':',
			expected: 4,
		},
		{
			name:     "not found",
			s:        "no-separator",
			sep:      '=',
			expected: -1,
		},
		{
			name:     "first character",
			s:        "=value",
			sep:      '=',
			expected: 0,
		},
		{
			name:     "last character",
			s:        "value=",
			sep:      '=',
			expected: 5,
		},
		{
			name:     "multiple occurrences",
			s:        "a=b=c",
			sep:      '=',
			expected: 1, // Should return first occurrence
		},
		{
			name:     "empty string",
			s:        "",
			sep:      '=',
			expected: -1,
		},
		{
			name:     "single character match",
			s:        "a",
			sep:      'a',
			expected: 0,
		},
		{
			name:     "single character no match",
			s:        "a",
			sep:      'b',
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndexOf(tt.s, tt.sep)
			if result != tt.expected {
				t.Errorf("IndexOf(%q, %c) = %d, want %d", tt.s, tt.sep, result, tt.expected)
			}
		})
	}
}
