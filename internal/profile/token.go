package profile

import (
	"context"

	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/token"
)

// GetTokenStatus retrieves the status of the token for this profile.
func (p *Profile) GetTokenStatus(ctx context.Context, kr keyring.Store) (*token.Status, string, error) {
	tokenStr, err := p.GetToken(kr)
	if err != nil {
		// No token stored
		return &token.Status{
			Stored: false,
			Valid:  false,
		}, "", nil
	}

	status, err := token.GetStatus(ctx, p.Connection, tokenStr)
	if err != nil {
		return nil, "", err
	}

	return status, tokenStr, nil
}

// GetToken retrieves the stored token for this profile.
func (p *Profile) GetToken(kr keyring.Store) (string, error) {
	tokenStr, err := kr.Get(p.KeyringKey())
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

// SetToken stores a token for this profile.
func (p *Profile) SetToken(kr keyring.Store, tokenStr string) error {
	return kr.Set(p.KeyringKey(), tokenStr)
}

// DeleteToken removes the token for this profile.
func (p *Profile) DeleteToken(kr keyring.Store) error {
	return kr.Delete(p.KeyringKey())
}

// RenewToken renews the token for this profile.
func (p *Profile) RenewToken(ctx context.Context, kr keyring.Store, increment string) (*token.Token, error) {
	tokenStr, err := p.GetToken(kr)
	if err != nil {
		return nil, err
	}

	tok, err := token.Renew(ctx, p.Connection, tokenStr, increment)
	if err != nil {
		return nil, err
	}

	// Update token in keyring if it changed
	if tok.ClientToken != "" && tok.ClientToken != tokenStr {
		if err := p.SetToken(kr, tok.ClientToken); err != nil {
			return nil, err
		}
	}

	return tok, nil
}

// RevokeToken revokes the token for this profile.
func (p *Profile) RevokeToken(ctx context.Context, kr keyring.Store) error {
	tokenStr, err := p.GetToken(kr)
	if err != nil {
		return err
	}
	return token.Revoke(ctx, p.Connection, tokenStr)
}

// LookupToken queries Vault for token information.
func (p *Profile) LookupToken(ctx context.Context, kr keyring.Store) (*token.VaultTokenLookupData, error) {
	tokenStr, err := p.GetToken(kr)
	if err != nil {
		return nil, err
	}
	return token.Lookup(ctx, p.Connection, tokenStr)
}

// HasToken checks if a token is stored for this profile.
func (p *Profile) HasToken(kr keyring.Store) bool {
	_, err := kr.Get(p.KeyringKey())
	return err == nil
}
