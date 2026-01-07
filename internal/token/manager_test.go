package token

import (
	"context"
	"errors"
	"testing"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/tokenstore"
	"github.com/xabinapal/patrol/internal/types"
	"github.com/xabinapal/patrol/internal/vault"
)

type mockStore struct {
	tokens map[string]string
}

func newMockStore() *mockStore {
	return &mockStore{
		tokens: make(map[string]string),
	}
}

func (m *mockStore) Get(prof *types.Profile) (string, error) {
	key := prof.Name
	if token, ok := m.tokens[key]; ok {
		return token, nil
	}
	return "", tokenstore.ErrTokenNotFound
}

func (m *mockStore) Set(prof *types.Profile, token string) error {
	key := prof.Name
	m.tokens[key] = token
	return nil
}

func (m *mockStore) Delete(prof *types.Profile) error {
	key := prof.Name
	if _, ok := m.tokens[key]; !ok {
		return tokenstore.ErrTokenNotFound
	}
	delete(m.tokens, key)
	return nil
}

func (m *mockStore) IsAvailable() error {
	return nil
}

// mockVaultExecutor is a mock vault executor for testing.
type mockVaultExecutor struct {
	renewTokenFunc  func(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*vault.TokenStatus, error)
	revokeTokenFunc func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error
	lookupTokenFunc func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*vault.TokenStatus, error)
}

// Ensure mockVaultExecutor implements vault.TokenExecutor
var _ vault.TokenExecutor = (*mockVaultExecutor)(nil)

func (m *mockVaultExecutor) RenewToken(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*vault.TokenStatus, error) {
	if m.renewTokenFunc != nil {
		return m.renewTokenFunc(ctx, prof, tokenStr, increment, opts...)
	}
	return &vault.TokenStatus{
		TTL:       3600,
		Renewable: true,
	}, nil
}

func (m *mockVaultExecutor) RevokeToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error {
	if m.revokeTokenFunc != nil {
		return m.revokeTokenFunc(ctx, prof, tokenStr, opts...)
	}
	return nil
}

func (m *mockVaultExecutor) LookupToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*vault.TokenStatus, error) {
	if m.lookupTokenFunc != nil {
		return m.lookupTokenFunc(ctx, prof, tokenStr, opts...)
	}
	return &vault.TokenStatus{
		TTL:       3600,
		Renewable: true,
	}, nil
}

func TestNewTokenManager(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	mockVault := &mockVaultExecutor{}

	tm := NewTokenManager(ctx, store, mockVault)
	if tm == nil {
		t.Fatal("NewTokenManager() returned nil")
	}
	if tm.ctx != ctx {
		t.Error("NewTokenManager() ctx mismatch")
	}
	if tm.store != store {
		t.Error("NewTokenManager() store mismatch")
	}
	if tm.vault != mockVault {
		t.Error("NewTokenManager() vault executor mismatch")
	}
}

func TestTokenManager_Get(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})

	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: no token stored
	_, err := tm.Get(prof)
	if err == nil {
		t.Error("Get() should return error when no token stored")
	}
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("Get() error = %v, want ErrTokenNotFound", err)
	}

	// Test: token stored
	testToken := "hvs.test-token-12345"
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	tokenStr, err := tm.Get(prof)
	if err != nil {
		t.Errorf("Get() error = %v, want nil", err)
	}
	if tokenStr != testToken {
		t.Errorf("Get() = %q, want %q", tokenStr, testToken)
	}
}

func TestTokenManager_Set(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})

	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	testToken := "hvs.test-token-12345"
	if err := tm.Set(prof, testToken); err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}

	// Verify token was stored
	stored, err := mockStore.Get(prof)
	if err != nil {
		t.Errorf("token not stored: %v", err)
	}
	if stored != testToken {
		t.Errorf("stored token = %q, want %q", stored, testToken)
	}
}

func TestTokenManager_Delete(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})

	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: delete non-existent token
	err := tm.Delete(prof)
	if err == nil {
		t.Error("Delete() should return error when token doesn't exist")
	}
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("Delete() error = %v, want ErrTokenNotFound", err)
	}

	// Test: delete existing token
	testToken := "hvs.test-token-12345"
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	if delErr := tm.Delete(prof); delErr != nil {
		t.Errorf("Delete() error = %v, want nil", delErr)
	}

	// Verify token was deleted
	_, err = mockStore.Get(prof)
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("token should be deleted, got error = %v", err)
	}
}

func TestTokenManager_HasToken(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})

	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: no token
	if tm.HasToken(prof) {
		t.Error("HasToken() = true, want false when no token stored")
	}

	// Test: token exists
	testToken := "hvs.test-token-12345"
	if err := mockStore.Set(prof, testToken); err != nil {
		t.Fatalf("failed to set token: %v", err)
	}

	if !tm.HasToken(prof) {
		t.Error("HasToken() = false, want true when token stored")
	}
}

func TestTokenManager_Renew(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: no token stored
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})
	_, err := tm.Renew(prof, "")
	if err == nil {
		t.Error("Renew() should return error when no token stored")
	}
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("Renew() error = %v, want ErrTokenNotFound", err)
	}

	// Test: successful renewal
	testToken := "hvs.test-token-12345"
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	mockVault := &mockVaultExecutor{
		renewTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*vault.TokenStatus, error) {
			if tokenStr != testToken {
				t.Errorf("RenewToken() called with token = %q, want %q", tokenStr, testToken)
			}
			return &vault.TokenStatus{
				TTL:       7200,
				Renewable: true,
			}, nil
		},
	}
	tm = NewTokenManager(ctx, mockStore, mockVault)
	tok, err := tm.Renew(prof, "")
	if err != nil {
		t.Errorf("Renew() error = %v, want nil", err)
	}
	if tok == nil {
		t.Fatal("Renew() returned nil token")
	}
	if tok.ClientToken != testToken {
		t.Errorf("Renew() ClientToken = %q, want %q", tok.ClientToken, testToken)
	}
	if tok.LeaseDuration != 7200 {
		t.Errorf("Renew() LeaseDuration = %d, want 7200", tok.LeaseDuration)
	}
	if !tok.Renewable {
		t.Error("Renew() Renewable = false, want true")
	}

	// Verify token was NOT updated in store (renewal doesn't change token string)
	storedToken, err := mockStore.Get(prof)
	if err != nil {
		t.Errorf("failed to get stored token: %v", err)
	}
	if storedToken != testToken {
		t.Errorf("stored token = %q, want %q (token should not change on renewal)", storedToken, testToken)
	}

	// Test: renewal with increment
	mockStore2 := newMockStore()
	if setErr := mockStore2.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}
	mockVault2 := &mockVaultExecutor{
		renewTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*vault.TokenStatus, error) {
			return &vault.TokenStatus{
				TTL:       3600,
				Renewable: true,
			}, nil
		},
	}
	tm2 := NewTokenManager(ctx, mockStore2, mockVault2)
	tok2, err := tm2.Renew(prof, "1h")
	if err != nil {
		t.Errorf("Renew() error = %v, want nil", err)
	}
	if tok2.ClientToken != testToken {
		t.Errorf("Renew() ClientToken = %q, want %q", tok2.ClientToken, testToken)
	}
	if tok2.LeaseDuration != 3600 {
		t.Errorf("Renew() LeaseDuration = %d, want 3600", tok2.LeaseDuration)
	}

	// Test: vault error
	mockVaultError := &mockVaultExecutor{
		renewTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*vault.TokenStatus, error) {
			return nil, errors.New("vault error")
		},
	}
	tm4 := NewTokenManager(ctx, mockStore, mockVaultError)
	_, err4 := tm4.Renew(prof, "")
	if err4 == nil {
		t.Error("Renew() should return error when vault fails")
	}
}

func TestTokenManager_Revoke(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: no token stored
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})
	err := tm.Revoke(prof)
	if err == nil {
		t.Error("Revoke() should return error when no token stored")
	}
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("Revoke() error = %v, want ErrTokenNotFound", err)
	}

	// Test: successful revocation
	testToken := "hvs.test-token-12345"
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	revokeCalled := false
	mockVault := &mockVaultExecutor{
		revokeTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error {
			if tokenStr != testToken {
				t.Errorf("RevokeToken() called with token = %q, want %q", tokenStr, testToken)
			}
			revokeCalled = true
			return nil
		},
	}
	tm = NewTokenManager(ctx, mockStore, mockVault)
	err = tm.Revoke(prof)
	if err != nil {
		t.Errorf("Revoke() error = %v, want nil", err)
	}
	if !revokeCalled {
		t.Error("Revoke() should call vault.RevokeToken")
	}

	// Test: vault error
	mockVaultError := &mockVaultExecutor{
		revokeTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error {
			return errors.New("vault error")
		},
	}
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}
	tm2 := NewTokenManager(ctx, mockStore, mockVaultError)
	err = tm2.Revoke(prof)
	if err == nil {
		t.Error("Revoke() should return error when vault fails")
	}
}

func TestTokenManager_Lookup(t *testing.T) {
	ctx := context.Background()
	mockStore := newMockStore()
	prof := types.FromConnection(&config.Connection{
		Name: "test",
	})

	// Test: no token stored
	tm := NewTokenManager(ctx, mockStore, &mockVaultExecutor{})
	_, err := tm.Lookup(prof)
	if err == nil {
		t.Error("Lookup() should return error when no token stored")
	}
	if !errors.Is(err, tokenstore.ErrTokenNotFound) {
		t.Errorf("Lookup() error = %v, want ErrTokenNotFound", err)
	}

	// Test: successful lookup
	testToken := "hvs.test-token-12345"
	if setErr := mockStore.Set(prof, testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	mockVault := &mockVaultExecutor{
		lookupTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*vault.TokenStatus, error) {
			if tokenStr != testToken {
				t.Errorf("LookupToken() called with token = %q, want %q", tokenStr, testToken)
			}
			return &vault.TokenStatus{
				TTL:       3600,
				Renewable: true,
			}, nil
		},
	}
	tm = NewTokenManager(ctx, mockStore, mockVault)
	data, err := tm.Lookup(prof)
	if err != nil {
		t.Errorf("Lookup() error = %v, want nil", err)
	}
	if data == nil {
		t.Fatal("Lookup() returned nil data")
	}
	if data.LeaseDuration != 3600 {
		t.Errorf("Lookup() LeaseDuration = %d, want 3600", data.LeaseDuration)
	}
	if !data.Renewable {
		t.Error("Lookup() Renewable = false, want true")
	}

	// Test: vault error
	mockVaultError := &mockVaultExecutor{
		lookupTokenFunc: func(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*vault.TokenStatus, error) {
			return nil, errors.New("vault error")
		},
	}
	tm2 := NewTokenManager(ctx, mockStore, mockVaultError)
	_, err = tm2.Lookup(prof)
	if err == nil {
		t.Error("Lookup() should return error when vault fails")
	}
}
