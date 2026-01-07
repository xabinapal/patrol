package tokenstore

import (
	"os"
	"testing"

	"github.com/xabinapal/patrol/internal/types"
)

func TestNewTokenStore(t *testing.T) {
	t.Run("With no environment variable set", func(t *testing.T) {
		os.Unsetenv(TestStoreEnvVar)

		store := NewTokenStore()
		_, ok := store.(*KeyringStore)
		if !ok {
			t.Errorf("NewTokenStore() should return KeyringStore when %s is not set, got %T", TestStoreEnvVar, store)
		}
	})

	t.Run("With environment variable set to valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		defer os.RemoveAll(tmpDir)

		os.Setenv(TestStoreEnvVar, tmpDir)

		store := NewTokenStore()
		_, ok := store.(*FileStore)
		if !ok {
			t.Errorf("NewTokenStore() should return FileStore when %s is set, got %T", TestStoreEnvVar, store)
		}
	})

	t.Run("With environment variable set to invalid directory", func(t *testing.T) {
		os.Setenv(TestStoreEnvVar, "/nonexistent/path/that/does/not/exist")

		defer func() {
			if r := recover(); r == nil {
				t.Error("NewTokenStore() should panic when file store cannot be created")
			}
		}()
		_ = NewTokenStore()
		t.Error("NewTokenStore() should have panicked")
	})
}

func TestKeyFromProfile(t *testing.T) {
	tests := []struct {
		name     string
		profile  *types.Profile
		expected string
	}{
		{
			name:     "normal profile",
			profile:  &types.Profile{Name: "test-profile"},
			expected: "patrol_910b1739d68db5624812a1a1de9e5da44d8418ca920ffe7416b77f9af1603d31",
		},
		{
			name:     "empty profile",
			profile:  &types.Profile{Name: ""},
			expected: "",
		},
		{
			name:     "nil profile",
			profile:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KeyFromProfile(tt.profile)
			if result != tt.expected {
				t.Errorf("keyFromProfile() = %q, want %q", result, tt.expected)
			}
		})
	}
}
