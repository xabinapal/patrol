package utils

import (
	"testing"
)

func TestSetEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		key      string
		value    string
		expected []string
	}{
		{
			name:     "add new var",
			env:      []string{"FOO=bar"},
			key:      "BAZ",
			value:    "qux",
			expected: []string{"FOO=bar", "BAZ=qux"},
		},
		{
			name:     "replace existing var",
			env:      []string{"FOO=bar", "BAZ=old"},
			key:      "BAZ",
			value:    "new",
			expected: []string{"FOO=bar", "BAZ=new"},
		},
		{
			name:     "empty env",
			env:      []string{},
			key:      "FOO",
			value:    "bar",
			expected: []string{"FOO=bar"},
		},
		{
			name:     "replace first var",
			env:      []string{"FOO=old", "BAZ=qux"},
			key:      "FOO",
			value:    "new",
			expected: []string{"FOO=new", "BAZ=qux"},
		},
		{
			name:     "replace middle var",
			env:      []string{"FOO=bar", "BAZ=old", "QUX=quux"},
			key:      "BAZ",
			value:    "new",
			expected: []string{"FOO=bar", "BAZ=new", "QUX=quux"},
		},
		{
			name:     "replace last var",
			env:      []string{"FOO=bar", "BAZ=old"},
			key:      "BAZ",
			value:    "new",
			expected: []string{"FOO=bar", "BAZ=new"},
		},
		{
			name:     "add to multiple vars",
			env:      []string{"FOO=bar", "BAZ=qux"},
			key:      "QUX",
			value:    "quux",
			expected: []string{"FOO=bar", "BAZ=qux", "QUX=quux"},
		},
		{
			name:     "value with equals sign",
			env:      []string{"FOO=bar"},
			key:      "BAZ",
			value:    "qux=quux",
			expected: []string{"FOO=bar", "BAZ=qux=quux"},
		},
		{
			name:     "value with special characters",
			env:      []string{"FOO=bar"},
			key:      "BAZ",
			value:    "qux/quux:test",
			expected: []string{"FOO=bar", "BAZ=qux/quux:test"},
		},
		{
			name:     "empty value",
			env:      []string{"FOO=bar"},
			key:      "BAZ",
			value:    "",
			expected: []string{"FOO=bar", "BAZ="},
		},
		{
			name:     "replace with empty value",
			env:      []string{"FOO=bar", "BAZ=old"},
			key:      "BAZ",
			value:    "",
			expected: []string{"FOO=bar", "BAZ="},
		},
		{
			name:     "key with special characters",
			env:      []string{"FOO=bar"},
			key:      "BAZ_QUX",
			value:    "test",
			expected: []string{"FOO=bar", "BAZ_QUX=test"},
		},
		{
			name:     "case sensitive key",
			env:      []string{"FOO=bar", "foo=old"},
			key:      "FOO",
			value:    "new",
			expected: []string{"FOO=new", "foo=old"},
		},
		{
			name:     "key prefix match but not exact",
			env:      []string{"FOO_BAR=old"},
			key:      "FOO",
			value:    "new",
			expected: []string{"FOO_BAR=old", "FOO=new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SetEnv(tt.env, tt.key, tt.value)

			if len(result) != len(tt.expected) {
				t.Errorf("SetEnv() returned %d items, want %d", len(result), len(tt.expected))
				return
			}

			// Check that expected values are present
			for _, exp := range tt.expected {
				found := false
				for _, r := range result {
					if r == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("SetEnv() missing expected value %s", exp)
				}
			}

			// Verify no unexpected values
			for _, r := range result {
				found := false
				for _, exp := range tt.expected {
					if r == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("SetEnv() returned unexpected value %s", r)
				}
			}
		})
	}
}

func TestSetEnvImmutability(t *testing.T) {
	// Verify that SetEnv doesn't modify the original slice
	original := []string{"FOO=bar", "BAZ=qux"}
	result := SetEnv(original, "QUX", "quux")

	// Original should be unchanged
	if len(original) != 2 {
		t.Errorf("SetEnv modified original slice length: got %d, want 2", len(original))
	}
	if original[0] != "FOO=bar" || original[1] != "BAZ=qux" {
		t.Errorf("SetEnv modified original slice contents")
	}

	// Result should have new value
	if len(result) != 3 {
		t.Errorf("SetEnv result should have 3 items, got %d", len(result))
	}
}
