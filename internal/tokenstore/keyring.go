package tokenstore

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/xabinapal/patrol/internal/types"
	"github.com/xabinapal/patrol/internal/utils"
)

// KeyringStore implements Store using the OS keyring.
type KeyringStore struct {
	mu sync.Mutex
}

// NewKeyringStore creates a new KeyringStore instance.
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{}
}

func (k *KeyringStore) IsAvailable() error {
	_, err := gokeyring.Get(ServicePrefix+"_availability_check", "test")
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return nil
		}

		errStr := err.Error()

		if runtime.GOOS == "linux" {
			if utils.ContainsAny(errStr, "secret service", "dbus", "org.freedesktop.secrets") {
				return fmt.Errorf("%w: D-Bus secret service not available", ErrStoreUnavailable)
			}
		}

		if runtime.GOOS == "darwin" {
			if utils.ContainsAny(errStr, "keychain", "security") {
				return fmt.Errorf("%w: macOS Keychain not accessible", ErrStoreUnavailable)
			}
		}

		if runtime.GOOS == "windows" {
			if utils.ContainsAny(errStr, "credential", "wincred") {
				return fmt.Errorf("%w: Windows Credential Manager not accessible", ErrStoreUnavailable)
			}
		}

		return nil
	}

	return nil
}

func (k *KeyringStore) Get(prof *types.Profile) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := k.IsAvailable(); err != nil {
		return "", err
	}

	if prof == nil {
		return "", ErrProfileNil
	}
	key := KeyFromProfile(prof)
	if key == "" {
		return "", ErrProfileNameEmpty
	}

	token, err := gokeyring.Get(key, "")
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return "", ErrTokenNotFound
		}
		return "", wrapKeyringStoreError(err, ErrTokenRetrieve)
	}

	return token, nil
}

func (k *KeyringStore) Set(prof *types.Profile, token string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := k.IsAvailable(); err != nil {
		return err
	}

	if prof == nil {
		return ErrProfileNil
	}
	key := KeyFromProfile(prof)
	if key == "" {
		return ErrProfileNameEmpty
	}

	if token == "" {
		return ErrTokenEmpty
	}

	err := gokeyring.Set(key, "", token)
	if err != nil {
		return wrapKeyringStoreError(err, ErrTokenStore)
	}

	return nil
}

func (k *KeyringStore) Delete(prof *types.Profile) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := k.IsAvailable(); err != nil {
		return err
	}

	if prof == nil {
		return ErrProfileNil
	}
	key := KeyFromProfile(prof)
	if key == "" {
		return ErrProfileNameEmpty
	}

	err := gokeyring.Delete(key, "")
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return nil
		}
		return wrapKeyringStoreError(err, ErrTokenDelete)
	}

	return nil
}

func wrapKeyringStoreError(err error, errType error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	if utils.ContainsAny(errStr, "denied", "permission", "not allowed", "unauthorized") {
		return fmt.Errorf("%w: %s: %v", ErrStoreAccessDenied, errType, err)
	}

	if utils.ContainsAny(errStr, "not found", "no keyring", "unavailable", "secret service") {
		return fmt.Errorf("%w: %s: %v", ErrStoreUnavailable, errType, err)
	}

	return fmt.Errorf("%s: %w", errType, err)
}
