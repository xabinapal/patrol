package keyring

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStore(t *testing.T) {
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

func TestFileStoreEmptyDir(t *testing.T) {
	_, err := NewFileStore("")
	if err == nil {
		t.Error("NewFileStore('') should fail")
	}
}

func TestFileStoreEmptyKey(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileStore(tmpDir)

	if err := store.Set("", "token"); err == nil || !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Set('', token) should return ErrTokenNotFound, got %v", err)
	}

	_, err := store.Get("")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Get('') should return ErrTokenNotFound, got %v", err)
	}
}

func TestFileStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and set token
	store1, _ := NewFileStore(tmpDir)
	if err := store1.Set("persist-key", "persist-token"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Create new store pointing to same dir
	store2, _ := NewFileStore(tmpDir)
	token, err := store2.Get("persist-key")
	if err != nil {
		t.Fatalf("Get() from second store failed: %v", err)
	}
	if token != "persist-token" {
		t.Errorf("Token not persisted: got %s, want persist-token", token)
	}
}

func TestFileStoreIsAvailableNotDir(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")
	os.WriteFile(filePath, []byte("not a dir"), 0600)

	store := &FileStore{dir: filePath}
	if err := store.IsAvailable(); err == nil {
		t.Error("IsAvailable() should fail for non-directory")
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		// Security: dots are now replaced with underscores
		{"with.dot", "with_dot"},
		{"with:colon", "with_colon"},
		// Security: slashes now trigger hashing
		{"with spaces", "with_spaces"},
		{"MixedCase123", "MixedCase123"},
	}

	for _, tt := range tests {
		result := sanitizeKey(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
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
		result := sanitizeKey(pattern)
		// Result should be a SHA256 hash (64 hex chars)
		if len(result) != 64 {
			t.Errorf("sanitizeKey(%q) should return hash, got %q", pattern, result)
		}
		// Verify it doesn't contain the original dangerous characters
		if strings.Contains(result, "/") || strings.Contains(result, "\\") || strings.Contains(result, "..") {
			t.Errorf("sanitizeKey(%q) = %q still contains dangerous characters", pattern, result)
		}
	}
}

func TestKeyPathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() failed: %v", err)
	}

	// Test that path traversal attempts are blocked
	traversalKeys := []string{
		"../etc/passwd",
		"../../..",
		"foo/../../../etc/passwd",
	}

	for _, key := range traversalKeys {
		// Set should work (key gets hashed)
		if err := store.Set(key, "test-value"); err != nil {
			t.Errorf("Set(%q) failed: %v", key, err)
		}

		// Verify the file was created inside tmpDir, not outside
		files, _ := filepath.Glob(filepath.Join(tmpDir, "*"))
		for _, f := range files {
			absFile, _ := filepath.Abs(f)
			absDir, _ := filepath.Abs(tmpDir)
			if !strings.HasPrefix(absFile, absDir) {
				t.Errorf("File %q escaped from directory %q", absFile, absDir)
			}
		}

		// Get should work too
		val, err := store.Get(key)
		if err != nil {
			t.Errorf("Get(%q) failed: %v", key, err)
		}
		if val != "test-value" {
			t.Errorf("Get(%q) = %q, want test-value", key, val)
		}
	}
}

func TestDefaultStoreWithTestEnv(t *testing.T) {
	tmpDir := t.TempDir()

	// Set test environment variable
	old := os.Getenv(TestKeyringEnvVar)
	os.Setenv(TestKeyringEnvVar, tmpDir)
	defer func() {
		if old != "" {
			os.Setenv(TestKeyringEnvVar, old)
		} else {
			os.Unsetenv(TestKeyringEnvVar)
		}
	}()

	store := DefaultStore()

	// Should be a FileStore, not osKeyring
	_, ok := store.(*FileStore)
	if !ok {
		t.Errorf("DefaultStore() should return FileStore when %s is set, got %T", TestKeyringEnvVar, store)
	}

	// Test that it works
	if err := store.Set("test", "value"); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	val, err := store.Get("test")
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if val != "value" {
		t.Errorf("Get() = %s, want value", val)
	}
}
