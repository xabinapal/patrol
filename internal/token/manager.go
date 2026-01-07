// Package token provides Vault token management functionality.
package token

import (
	"context"
	"time"

	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/tokenstore"
	"github.com/xabinapal/patrol/internal/types"
	"github.com/xabinapal/patrol/internal/vault"
)

// TokenManager manages tokens and provides high-level token operations.
type TokenManager struct {
	ctx   context.Context
	store tokenstore.TokenStore
	vault vault.TokenExecutor
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager(ctx context.Context, store tokenstore.TokenStore, executor vault.TokenExecutor) *TokenManager {
	return &TokenManager{
		ctx:   ctx,
		store: store,
		vault: executor,
	}
}

func (tm *TokenManager) Get(prof *types.Profile) (string, error) {
	return tm.store.Get(prof)
}

func (tm *TokenManager) Set(prof *types.Profile, tokenStr string) error {
	return tm.store.Set(prof, tokenStr)
}

func (tm *TokenManager) Delete(prof *types.Profile) error {
	return tm.store.Delete(prof)
}

func (tm *TokenManager) HasToken(prof *types.Profile) bool {
	_, err := tm.store.Get(prof)
	return err == nil
}

func (tm *TokenManager) Renew(prof *types.Profile, increment string, opts ...proxy.Option) (*types.Token, error) {
	tokenStr, err := tm.store.Get(prof)
	if err != nil {
		return nil, err
	}

	status, err := tm.vault.RenewToken(tm.ctx, prof, tokenStr, increment, opts...)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tok := &types.Token{
		ClientToken:   tokenStr,
		LeaseDuration: status.TTL,
		Renewable:     status.Renewable,
		ExpiresAt:     now.Add(time.Duration(status.TTL) * time.Second),
	}

	return tok, nil
}

func (tm *TokenManager) Revoke(prof *types.Profile, opts ...proxy.Option) error {
	tokenStr, err := tm.store.Get(prof)
	if err != nil {
		return err
	}
	return tm.vault.RevokeToken(tm.ctx, prof, tokenStr, opts...)
}

func (tm *TokenManager) Lookup(prof *types.Profile, opts ...proxy.Option) (*types.Token, error) {
	tokenStr, err := tm.store.Get(prof)
	if err != nil {
		return nil, err
	}

	status, err := tm.vault.LookupToken(tm.ctx, prof, tokenStr, opts...)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tok := &types.Token{
		ClientToken:   tokenStr,
		LeaseDuration: status.TTL,
		Renewable:     status.Renewable,
		ExpiresAt:     now.Add(time.Duration(status.TTL) * time.Second),
	}

	return tok, nil
}
