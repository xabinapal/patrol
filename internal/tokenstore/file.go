package tokenstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/xabinapal/patrol/internal/types"
)

// FileStore is a file-based token store implementation for testing.
// This should ONLY be used for testing, never in production.
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore creates a new file-based token store.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	// Ensure directory exists with secure permissions
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	return &FileStore{dir: dir}, nil
}

func (f *FileStore) IsAvailable() error {
	info, err := os.Stat(f.dir)
	if err != nil {
		return fmt.Errorf("%w: directory not accessible: %v", ErrStoreUnavailable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: path is not a directory", ErrStoreUnavailable)
	}
	return nil
}

func (f *FileStore) Get(prof *types.Profile) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.IsAvailable(); err != nil {
		return "", err
	}

	if prof == nil {
		return "", ErrProfileNil
	}
	key := KeyFromProfile(prof)
	if key == "" {
		return "", ErrProfileNameEmpty
	}

	path := filepath.Join(f.dir, key)

	// #nosec G304 - path is from keyPath() which constructs paths from config paths
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrTokenNotFound
		}
		return "", ErrTokenRetrieve
	}

	return string(data), nil
}

func (f *FileStore) Set(prof *types.Profile, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.IsAvailable(); err != nil {
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

	path := filepath.Join(f.dir, key)
	_ = os.Remove(path)

	// #nosec G304 - path is from keyPath() which constructs paths from config paths
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrTokenStore
	}
	defer file.Close()

	if _, err := file.Write([]byte(token)); err != nil {
		return ErrTokenStore
	}

	return nil
}

func (f *FileStore) Delete(prof *types.Profile) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.IsAvailable(); err != nil {
		return err
	}

	if prof == nil {
		return ErrProfileNil
	}
	key := KeyFromProfile(prof)
	if key == "" {
		return ErrProfileNameEmpty
	}

	path := filepath.Join(f.dir, key)

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return ErrTokenDelete
	}

	return nil
}
