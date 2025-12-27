// Package keyring provides secure token storage using the OS keyring.
package keyring

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/xabinapal/patrol/internal/utils"
)

const (
	// ServicePrefix is the prefix used for keyring service names.
	// Each profile will have its own service entry: "Patrol - <profile_name>"
	ServicePrefix = "Patrol"

	// TestKeyringEnvVar is the environment variable that, when set to a directory path,
	// causes Patrol to use a file-based keyring instead of the OS keyring.
	// This is intended for testing purposes only and should NEVER be used in production.
	TestKeyringEnvVar = "PATROL_TEST_KEYRING_DIR"
)

// serviceName returns the keyring service name for a profile.
// This creates unique, identifiable entries in the OS keyring.
func serviceName(profile string) string {
	return ServicePrefix + " - " + profile
}

var (
	// ErrKeyringUnavailable is returned when no secure keyring is available.
	ErrKeyringUnavailable = errors.New("secure keyring is not available on this system")
	// ErrTokenNotFound is returned when a token is not found in the keyring.
	ErrTokenNotFound = errors.New("token not found in keyring")
	// ErrKeyringAccessDenied is returned when access to the keyring is denied.
	ErrKeyringAccessDenied = errors.New("access to keyring denied")
)

// Store represents a secure token storage backend.
type Store interface {
	// Set stores a token for the given key.
	Set(key, token string) error
	// Get retrieves a token for the given key.
	Get(key string) (string, error)
	// Delete removes a token for the given key.
	Delete(key string) error
	// IsAvailable checks if the keyring is available.
	IsAvailable() error
}

// DefaultStore returns the default keyring store for the current platform.
// If PATROL_TEST_KEYRING_DIR is set, a file-based store is used instead.
// This allows integration tests to run without accessing the OS keyring.
func DefaultStore() Store {
	// Check for test keyring directory
	if testDir := os.Getenv(TestKeyringEnvVar); testDir != "" {
		fileStore, err := NewFileStore(testDir)
		if err != nil {
			// If we can't create the file store, fall back to OS keyring
			// but this is unlikely in a properly configured test environment
			return &osKeyring{}
		}
		return fileStore
	}
	return &osKeyring{}
}

// osKeyring implements Store using the OS keyring.
type osKeyring struct{}

// IsAvailable checks if a secure keyring is available on this system.
func (k *osKeyring) IsAvailable() error {
	// Test keyring availability by attempting a get operation
	// This will fail with a specific error if keyring is not available
	_, err := gokeyring.Get(serviceName("__availability_check__"), "test")
	if err != nil {
		// ErrNotFound means keyring is working but key doesn't exist (expected)
		if errors.Is(err, gokeyring.ErrNotFound) {
			return nil
		}

		// Check for platform-specific unavailability errors
		errStr := err.Error()

		// Linux: D-Bus secret service not available
		if runtime.GOOS == "linux" {
			if utils.ContainsAny(errStr, "secret service", "dbus", "org.freedesktop.secrets") {
				return fmt.Errorf("%w: D-Bus secret service not available - please install and start gnome-keyring, kwallet, or another secret service provider", ErrKeyringUnavailable)
			}
		}

		// macOS: Keychain issues
		if runtime.GOOS == "darwin" {
			if utils.ContainsAny(errStr, "keychain", "security") {
				return fmt.Errorf("%w: macOS Keychain not accessible", ErrKeyringUnavailable)
			}
		}

		// Windows: Credential Manager issues
		if runtime.GOOS == "windows" {
			if utils.ContainsAny(errStr, "credential", "wincred") {
				return fmt.Errorf("%w: Windows Credential Manager not accessible", ErrKeyringUnavailable)
			}
		}

		// Other errors during availability check - treat as available
		// since the actual operations will provide better error messages
		return nil
	}

	return nil
}

// Set stores a token in the keyring.
// The key is the profile name, which becomes both the service suffix and account name.
func (k *osKeyring) Set(key, token string) error {
	if err := k.IsAvailable(); err != nil {
		return err
	}

	if key == "" {
		return errors.New("key cannot be empty")
	}
	if token == "" {
		return errors.New("token cannot be empty")
	}

	// Use profile-specific service name: "Patrol - <profile_name>"
	// The account is also the profile name for consistency
	err := gokeyring.Set(serviceName(key), key, token)
	if err != nil {
		return wrapKeyringError(err, "failed to store token")
	}

	return nil
}

// Get retrieves a token from the keyring.
// The key is the profile name.
func (k *osKeyring) Get(key string) (string, error) {
	if err := k.IsAvailable(); err != nil {
		return "", err
	}

	if key == "" {
		return "", errors.New("key cannot be empty")
	}

	// Use profile-specific service name
	token, err := gokeyring.Get(serviceName(key), key)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return "", ErrTokenNotFound
		}
		return "", wrapKeyringError(err, "failed to retrieve token")
	}

	return token, nil
}

// Delete removes a token from the keyring.
// The key is the profile name.
func (k *osKeyring) Delete(key string) error {
	if err := k.IsAvailable(); err != nil {
		return err
	}

	if key == "" {
		return errors.New("key cannot be empty")
	}

	// Use profile-specific service name
	err := gokeyring.Delete(serviceName(key), key)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			// Already deleted, not an error
			return nil
		}
		return wrapKeyringError(err, "failed to delete token")
	}

	return nil
}

// wrapKeyringError wraps a keyring error with context.
func wrapKeyringError(err error, context string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for access denied errors
	if utils.ContainsAny(errStr, "denied", "permission", "not allowed", "unauthorized") {
		return fmt.Errorf("%w: %s: %v", ErrKeyringAccessDenied, context, err)
	}

	// Check for unavailability errors
	if utils.ContainsAny(errStr, "not found", "no keyring", "unavailable", "secret service") {
		return fmt.Errorf("%w: %s: %v", ErrKeyringUnavailable, context, err)
	}

	return fmt.Errorf("%s: %w", context, err)
}
