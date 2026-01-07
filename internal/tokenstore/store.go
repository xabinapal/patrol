// Package tokenstore provides secure token storage backends.
package tokenstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/xabinapal/patrol/internal/types"
)

var (
	// ErrProfileNil is returned when a profile is nil.
	ErrProfileNil = errors.New("profile cannot be nil")
	// ErrProfileNameEmpty is returned when a profile name is empty.
	ErrProfileNameEmpty = errors.New("profile name cannot be empty")

	// ErrTokenEmpty is returned when a token is empty.
	ErrTokenEmpty = errors.New("token cannot be empty")
	// ErrTokenNotFound is returned when a token is not found in the store.
	ErrTokenNotFound = errors.New("token not found in store")

	// ErrTokenStore is returned when a token cannot be stored.
	ErrTokenStore = errors.New("failed to store token")
	// ErrTokenRetrieve is returned when a token cannot be retrieved.
	ErrTokenRetrieve = errors.New("failed to retrieve token")
	// ErrTokenDelete is returned when a token cannot be deleted.
	ErrTokenDelete = errors.New("failed to delete token")

	// ErrStoreUnavailable is returned when no secure store is available.
	ErrStoreUnavailable = errors.New("token store is not available")
	// ErrStoreAccessDenied is returned when access to the store is denied.
	ErrStoreAccessDenied = errors.New("access to token store denied")
)

const (
	// ServicePrefix is the prefix used for keyring service names.
	ServicePrefix = "patrol"

	// TestStoreEnvVar is the environment variable that, when set to a directory path,
	// causes Patrol to use a file-based store instead of the OS keyring.
	// This is intended for testing purposes only and should NEVER be used in production.
	TestStoreEnvVar = "PATROL_TEST_KEYRING_DIR"
)

// TokenStore represents a secure token storage backend.
type TokenStore interface {
	// IsAvailable checks if the store is available.
	IsAvailable() error
	// Get retrieves a token for the given profile.
	Get(prof *types.Profile) (string, error)
	// Set stores a token for the given profile.
	Set(prof *types.Profile, token string) error
	// Delete removes a token for the given profile.
	Delete(prof *types.Profile) error
}

// Store returns the default token store for the current platform.
// If PATROL_TEST_KEYRING_DIR is set, a file-based store is used instead.
func NewTokenStore() TokenStore {
	if testDir := os.Getenv(TestStoreEnvVar); testDir != "" {
		fileStore, err := NewFileStore(testDir)
		if err != nil {
			panic(fmt.Sprintf("failed to create file store for testing: %v", err))
		}
		return fileStore
	}
	return NewKeyringStore()
}

func KeyFromProfile(prof *types.Profile) string {
	if prof == nil {
		return ""
	}

	if prof.Name == "" {
		return ""
	}

	h := sha256.New()
	h.Write([]byte(prof.Name))
	hash := h.Sum(nil)

	return ServicePrefix + "_" + hex.EncodeToString(hash)
}
