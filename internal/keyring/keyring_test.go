package keyring

import (
	"errors"
	"testing"

	"github.com/xabinapal/patrol/internal/utils"
)

func TestFileStoreBasic(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() failed: %v", err)
	}

	// Test IsAvailable
	if availErr := store.IsAvailable(); availErr != nil {
		t.Errorf("IsAvailable() should not error: %v", availErr)
	}

	// Test Set and Get
	if setErr := store.Set("test-key", "test-token"); setErr != nil {
		t.Errorf("Set() failed: %v", setErr)
	}

	token, err := store.Get("test-key")
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if token != "test-token" {
		t.Errorf("Get() = %s, want test-token", token)
	}

	// Test Get non-existent
	_, err = store.Get("non-existent")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Get(non-existent) should return ErrTokenNotFound, got %v", err)
	}

	// Test Delete
	if delErr := store.Delete("test-key"); delErr != nil {
		t.Errorf("Delete() failed: %v", delErr)
	}

	_, err = store.Get("test-key")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Get() after Delete() should return ErrTokenNotFound, got %v", err)
	}

	// Test Delete non-existent (should not error)
	if err := store.Delete("non-existent"); err != nil {
		t.Errorf("Delete(non-existent) should not error: %v", err)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s        string
		subs     []string
		expected bool
	}{
		{"hello world", []string{"hello"}, true},
		{"Hello World", []string{"hello"}, true}, // case insensitive
		{"test string", []string{"foo", "bar"}, false},
		{"secret service error", []string{"secret service"}, true},
		{"", []string{"anything"}, false},
	}

	for _, tt := range tests {
		result := utils.ContainsAny(tt.s, tt.subs...)
		if result != tt.expected {
			t.Errorf("ContainsAny(%q, %v) = %v, want %v", tt.s, tt.subs, result, tt.expected)
		}
	}
}

func TestWrapKeyringError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		context  string
		wantType error
	}{
		{
			name:    "nil error",
			err:     nil,
			context: "test",
		},
		{
			name:     "denied error",
			err:      errors.New("permission denied"),
			context:  "test",
			wantType: ErrKeyringAccessDenied,
		},
		{
			name:     "unavailable error",
			err:      errors.New("secret service not found"),
			context:  "test",
			wantType: ErrKeyringUnavailable,
		},
		{
			name:    "generic error",
			err:     errors.New("some other error"),
			context: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapKeyringError(tt.err, tt.context)

			if tt.err == nil {
				if result != nil {
					t.Errorf("wrapKeyringError(nil) should return nil")
				}
				return
			}

			if tt.wantType != nil && !errors.Is(result, tt.wantType) {
				t.Errorf("wrapKeyringError() should wrap with %v, got %v", tt.wantType, result)
			}
		})
	}
}
