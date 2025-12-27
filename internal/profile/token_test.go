package profile

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/keyring"
)

// newTestKeyring creates a FileStore for testing.
func newTestKeyring(t *testing.T) keyring.Store {
	tmpDir := t.TempDir()
	store, err := keyring.NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() failed: %v", err)
	}
	return store
}

func TestProfile_GetToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name: "test",
		},
	}

	// Test: no token stored
	token, err := prof.GetToken(mockKeyring)
	if err == nil {
		t.Error("GetToken() should return error when no token stored")
	}
	if !errors.Is(err, keyring.ErrTokenNotFound) {
		t.Errorf("GetToken() error = %v, want ErrTokenNotFound", err)
	}
	if token != "" {
		t.Errorf("GetToken() = %q, want empty string", token)
	}

	// Test: token stored
	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	token, err = prof.GetToken(mockKeyring)
	if err != nil {
		t.Errorf("GetToken() error = %v", err)
	}
	if token != testToken {
		t.Errorf("GetToken() = %q, want %q", token, testToken)
	}

	// Test: keyring error (simulate by removing directory)
	fileStore := mockKeyring.(*keyring.FileStore)
	tmpDir := fileStore.Dir()
	if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
		t.Fatalf("failed to remove directory: %v", removeErr)
	}
	_, err = prof.GetToken(mockKeyring)
	if err == nil {
		t.Error("GetToken() should return error when keyring is unavailable")
	}
}

func TestProfile_SetToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name: "test",
		},
	}

	testToken := "hvs.test-token-12345"
	if err := prof.SetToken(mockKeyring, testToken); err != nil {
		t.Fatalf("SetToken() error = %v", err)
	}

	// Verify token was stored
	stored, err := mockKeyring.Get(prof.KeyringKey())
	if err != nil {
		t.Fatalf("failed to get stored token: %v", err)
	}
	if stored != testToken {
		t.Errorf("SetToken() stored %q, want %q", stored, testToken)
	}

	// Test: update existing token
	newToken := "hvs.new-token-67890"
	if setErr := prof.SetToken(mockKeyring, newToken); setErr != nil {
		t.Fatalf("SetToken() error updating token: %v", setErr)
	}
	stored, err = mockKeyring.Get(prof.KeyringKey())
	if err != nil {
		t.Fatalf("failed to get updated token: %v", err)
	}
	if stored != newToken {
		t.Errorf("SetToken() updated token = %q, want %q", stored, newToken)
	}

	// Test: keyring error (simulate by removing directory)
	fileStore := mockKeyring.(*keyring.FileStore)
	if err := os.RemoveAll(fileStore.Dir()); err != nil {
		t.Fatalf("failed to remove directory: %v", err)
	}
	if err := prof.SetToken(mockKeyring, "test"); err == nil {
		t.Error("SetToken() should return error when keyring is unavailable")
	}
}

func TestProfile_DeleteToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name: "test",
		},
	}

	// Test: delete non-existent token (should not error)
	if err := prof.DeleteToken(mockKeyring); err != nil {
		t.Errorf("DeleteToken() on non-existent token should not error, got %v", err)
	}

	// Test: delete existing token
	testToken := "hvs.test-token-12345"
	if err := mockKeyring.Set(prof.KeyringKey(), testToken); err != nil {
		t.Fatalf("failed to set token: %v", err)
	}

	if err := prof.DeleteToken(mockKeyring); err != nil {
		t.Errorf("DeleteToken() error = %v", err)
	}

	// Verify token was deleted
	_, err := mockKeyring.Get(prof.KeyringKey())
	if !errors.Is(err, keyring.ErrTokenNotFound) {
		t.Errorf("DeleteToken() should remove token, got error = %v", err)
	}

	// Test: keyring error (simulate by removing directory)
	// Note: Delete() may fail when directory is removed, but it's idempotent
	fileStore := mockKeyring.(*keyring.FileStore)
	if err := os.RemoveAll(fileStore.Dir()); err != nil {
		t.Fatalf("failed to remove directory: %v", err)
	}
	// Delete may or may not error when directory is removed
	// Both behaviors are acceptable for an idempotent delete operation
	_ = prof.DeleteToken(mockKeyring)
}

func TestProfile_HasToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name: "test",
		},
	}

	// Test: no token
	if prof.HasToken(mockKeyring) {
		t.Error("HasToken() = true, want false when no token stored")
	}

	// Test: token exists
	testToken := "hvs.test-token-12345"
	if err := mockKeyring.Set(prof.KeyringKey(), testToken); err != nil {
		t.Fatalf("failed to set token: %v", err)
	}

	if !prof.HasToken(mockKeyring) {
		t.Error("HasToken() = false, want true when token stored")
	}

	// Test: after deletion
	if err := prof.DeleteToken(mockKeyring); err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}
	if prof.HasToken(mockKeyring) {
		t.Error("HasToken() = true, want false after deletion")
	}
}

func TestProfile_GetTokenStatus(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name:    "test",
			Address: "https://vault.example.com:8200",
			Type:    config.BinaryTypeVault,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test: no token stored - should return status with Stored=false
	status, token, err := prof.GetTokenStatus(ctx, mockKeyring)
	if err != nil {
		t.Errorf("GetTokenStatus() error = %v", err)
	}
	if status == nil {
		t.Fatal("GetTokenStatus() returned nil status")
	}
	if status.Stored {
		t.Error("GetTokenStatus() status.Stored = true, want false")
	}
	if status.Valid {
		t.Error("GetTokenStatus() status.Valid = true, want false")
	}
	if token != "" {
		t.Errorf("GetTokenStatus() token = %q, want empty", token)
	}

	// Test: token stored but binary doesn't exist - should return status with error
	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	status, token, err = prof.GetTokenStatus(ctx, mockKeyring)
	if err != nil {
		t.Errorf("GetTokenStatus() error = %v", err)
	}
	if status == nil {
		t.Fatal("GetTokenStatus() returned nil status")
	}
	if !status.Stored {
		t.Error("GetTokenStatus() status.Stored = false, want true")
	}
	if token != testToken {
		t.Errorf("GetTokenStatus() token = %q, want %q", token, testToken)
	}
	// Should have error about binary not found
	if status.Error == "" {
		t.Error("GetTokenStatus() should have error when binary doesn't exist")
	}
}

func TestProfile_RenewToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name:    "test",
			Address: "https://vault.example.com:8200",
			Type:    config.BinaryTypeVault,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test: no token stored
	_, err := prof.RenewToken(ctx, mockKeyring, "")
	if err == nil {
		t.Error("RenewToken() should return error when no token stored")
	}
	if !errors.Is(err, keyring.ErrTokenNotFound) {
		t.Errorf("RenewToken() error = %v, want ErrTokenNotFound", err)
	}

	// Test: token stored but binary doesn't exist
	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	_, err = prof.RenewToken(ctx, mockKeyring, "")
	if err == nil {
		t.Error("RenewToken() should return error when binary doesn't exist")
	}
}

func TestProfile_RenewToken_WithIncrement(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name:    "test",
			Address: "https://vault.example.com:8200",
			Type:    config.BinaryTypeVault,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	// Test with increment parameter
	_, err := prof.RenewToken(ctx, mockKeyring, "1h")
	if err == nil {
		t.Error("RenewToken() should return error when binary doesn't exist")
	}
}

func TestProfile_RevokeToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name:    "test",
			Address: "https://vault.example.com:8200",
			Type:    config.BinaryTypeVault,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test: no token stored
	err := prof.RevokeToken(ctx, mockKeyring)
	if err == nil {
		t.Error("RevokeToken() should return error when no token stored")
	}
	if !errors.Is(err, keyring.ErrTokenNotFound) {
		t.Errorf("RevokeToken() error = %v, want ErrTokenNotFound", err)
	}

	// Test: token stored but binary doesn't exist
	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	err = prof.RevokeToken(ctx, mockKeyring)
	if err == nil {
		t.Error("RevokeToken() should return error when binary doesn't exist")
	}
}

func TestProfile_LookupToken(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof := &Profile{
		Connection: &config.Connection{
			Name:    "test",
			Address: "https://vault.example.com:8200",
			Type:    config.BinaryTypeVault,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test: no token stored
	_, err := prof.LookupToken(ctx, mockKeyring)
	if err == nil {
		t.Error("LookupToken() should return error when no token stored")
	}
	if !errors.Is(err, keyring.ErrTokenNotFound) {
		t.Errorf("LookupToken() error = %v, want ErrTokenNotFound", err)
	}

	// Test: token stored but binary doesn't exist
	testToken := "hvs.test-token-12345"
	if setErr := mockKeyring.Set(prof.KeyringKey(), testToken); setErr != nil {
		t.Fatalf("failed to set token: %v", setErr)
	}

	_, err = prof.LookupToken(ctx, mockKeyring)
	if err == nil {
		t.Error("LookupToken() should return error when binary doesn't exist")
	}
}

func TestProfile_TokenMethods_UseCorrectKeyringKey(t *testing.T) {
	mockKeyring := newTestKeyring(t)
	prof1 := &Profile{
		Connection: &config.Connection{
			Name: "profile1",
		},
	}
	prof2 := &Profile{
		Connection: &config.Connection{
			Name: "profile2",
		},
	}

	// Store tokens for both profiles
	token1 := "hvs.token-1"
	token2 := "hvs.token-2"

	if err := prof1.SetToken(mockKeyring, token1); err != nil {
		t.Fatalf("failed to set token for profile1: %v", err)
	}
	if err := prof2.SetToken(mockKeyring, token2); err != nil {
		t.Fatalf("failed to set token for profile2: %v", err)
	}

	// Verify each profile gets its own token
	got1, err := prof1.GetToken(mockKeyring)
	if err != nil {
		t.Fatalf("failed to get token for profile1: %v", err)
	}
	if got1 != token1 {
		t.Errorf("profile1.GetToken() = %q, want %q", got1, token1)
	}

	got2, err := prof2.GetToken(mockKeyring)
	if err != nil {
		t.Fatalf("failed to get token for profile2: %v", err)
	}
	if got2 != token2 {
		t.Errorf("profile2.GetToken() = %q, want %q", got2, token2)
	}

	// Verify HasToken works correctly
	if !prof1.HasToken(mockKeyring) {
		t.Error("profile1.HasToken() = false, want true")
	}
	if !prof2.HasToken(mockKeyring) {
		t.Error("profile2.HasToken() = false, want true")
	}

	// Delete one token, verify the other still exists
	if err := prof1.DeleteToken(mockKeyring); err != nil {
		t.Fatalf("failed to delete token for profile1: %v", err)
	}

	if prof1.HasToken(mockKeyring) {
		t.Error("profile1.HasToken() = true, want false after deletion")
	}
	if !prof2.HasToken(mockKeyring) {
		t.Error("profile2.HasToken() = false, want true (should not be affected)")
	}
}
