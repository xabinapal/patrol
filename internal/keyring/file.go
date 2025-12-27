package keyring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xabinapal/patrol/internal/utils"
)

// FileStore is a file-based keyring implementation for testing.
// It stores tokens in files within a directory.
// This should ONLY be used for testing, never in production.
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore creates a new file-based keyring store.
// The directory must exist and be writable.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	// Ensure directory exists with secure permissions
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keyring directory: %w", err)
	}

	return &FileStore{dir: dir}, nil
}

// Dir returns the directory path used by this store.
// This is exposed for testing purposes.
func (f *FileStore) Dir() string {
	return f.dir
}

// IsAvailable implements Store.
func (f *FileStore) IsAvailable() error {
	info, err := os.Stat(f.dir)
	if err != nil {
		return fmt.Errorf("%w: directory not accessible: %v", ErrKeyringUnavailable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: path is not a directory", ErrKeyringUnavailable)
	}
	return nil
}

// keyPath returns the file path for a key.
// It ensures the resulting path is within the store directory to prevent
// path traversal attacks.
func (f *FileStore) keyPath(key string) (string, error) {
	// Sanitize key to be safe for filesystem
	safeKey := utils.SanitizeKey(key)

	// Build the full path
	fullPath := filepath.Join(f.dir, safeKey)

	// Security: Verify the path is still within our directory
	// This prevents any path traversal that might have slipped through
	absDir, err := filepath.Abs(f.dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve directory: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure the path starts with our directory
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return "", fmt.Errorf("invalid key: path traversal detected")
	}

	return fullPath, nil
}

// Set implements Store.
func (f *FileStore) Set(key, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if key == "" {
		return ErrTokenNotFound
	}

	path, err := f.keyPath(key)
	if err != nil {
		return fmt.Errorf("failed to resolve key path: %w", err)
	}

	// Security: Remove any existing file first to prevent symlink attacks
	_ = os.Remove(path)

	// Security: Use O_EXCL to ensure we create a new file (prevents race conditions)
	// #nosec G304 - path is from keyPath() which constructs paths from config paths (controlled)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write([]byte(token)); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// Get implements Store.
func (f *FileStore) Get(key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if key == "" {
		return "", ErrTokenNotFound
	}

	path, err := f.keyPath(key)
	if err != nil {
		return "", fmt.Errorf("failed to resolve key path: %w", err)
	}

	// #nosec G304 - path is from keyPath() which constructs paths from config paths (controlled)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrTokenNotFound
		}
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	return string(data), nil
}

// Delete implements Store.
func (f *FileStore) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if key == "" {
		return nil
	}

	path, err := f.keyPath(key)
	if err != nil {
		return fmt.Errorf("failed to resolve key path: %w", err)
	}

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}
