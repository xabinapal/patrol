package keyring

import "sync"

// MockStore is an in-memory keyring implementation for testing.
type MockStore struct {
	mu      sync.RWMutex
	data    map[string]string
	failing bool
}

// NewMockStore creates a new mock keyring store.
func NewMockStore() *MockStore {
	return &MockStore{
		data: make(map[string]string),
	}
}

// SetFailing makes all operations fail.
func (m *MockStore) SetFailing(failing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failing = failing
}

// IsAvailable implements Store.
func (m *MockStore) IsAvailable() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.failing {
		return ErrKeyringUnavailable
	}
	return nil
}

// Set implements Store.
func (m *MockStore) Set(key, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failing {
		return ErrKeyringUnavailable
	}

	if key == "" {
		return ErrTokenNotFound
	}

	m.data[key] = token
	return nil
}

// Get implements Store.
func (m *MockStore) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.failing {
		return "", ErrKeyringUnavailable
	}

	token, ok := m.data[key]
	if !ok {
		return "", ErrTokenNotFound
	}

	return token, nil
}

// Delete implements Store.
func (m *MockStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failing {
		return ErrKeyringUnavailable
	}

	delete(m.data, key)
	return nil
}

// Clear removes all stored tokens.
func (m *MockStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]string)
}

// Count returns the number of stored tokens.
func (m *MockStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
