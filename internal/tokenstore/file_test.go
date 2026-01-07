package tokenstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xabinapal/patrol/internal/types"
)

func TestFileStore(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() failed: %v", err)
	}

	// Test IsAvailable
	if availErr := store.IsAvailable(); availErr != nil {
		t.Errorf("IsAvailable() should not error: %v", availErr)
	}

	// Test Get non-existent
	nonExistentProf := &types.Profile{Name: "non-existent"}
	_, err = store.Get(nonExistentProf)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Get(non-existent) should return ErrTokenNotFound, got %v", err)
	}

	// Test Set and Get
	prof := &types.Profile{Name: "test-key"}
	if setErr := store.Set(prof, "test-token"); setErr != nil {
		t.Errorf("Set() failed: %v", setErr)
	}

	token, err := store.Get(prof)
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if token != "test-token" {
		t.Errorf("Get() = %s, want test-token", token)
	}

	// Test Delete
	if delErr := store.Delete(prof); delErr != nil {
		t.Errorf("Delete() failed: %v", delErr)
	}

	_, err = store.Get(prof)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("Get() after Delete() should return ErrTokenNotFound, got %v", err)
	}

	// Test Delete non-existent (should not error)
	if err := store.Delete(nonExistentProf); err != nil {
		t.Errorf("Delete(non-existent) should not error: %v", err)
	}
}

func TestFileStoreEmptyDir(t *testing.T) {
	_, err := NewFileStore("")
	if err == nil {
		t.Error("NewFileStore(\"\") should return error, got nil")
	}
}

func TestFileStoreEmptyKey(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	store, _ := NewFileStore(tmpDir)

	prof := &types.Profile{Name: ""}
	if err := store.Set(prof, "token"); err == nil {
		t.Errorf("Set(\"\", token) should return error, got nil")
	}

	_, err := store.Get(prof)
	if !errors.Is(err, ErrProfileNameEmpty) {
		t.Errorf("Get(\"\") should return ErrProfileNameEmpty, got %v", err)
	}
}

func TestFileStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	prof := &types.Profile{Name: "persist-key"}

	// Create store and set token
	store1, _ := NewFileStore(tmpDir)
	if err := store1.Set(prof, "persist-token"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Create new store pointing to same dir
	store2, _ := NewFileStore(tmpDir)
	token, err := store2.Get(prof)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if token != "persist-token" {
		t.Errorf("Token not persisted: got %s, want persist-token", token)
	}
}

func TestFileStoreIsAvailableNotDir(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(filePath, []byte("not a dir"), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	store := &FileStore{dir: filePath}
	if err := store.IsAvailable(); err == nil {
		t.Error("IsAvailable() should fail for non-directory")
	}
}
