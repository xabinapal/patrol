package token

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/proxy"
)

// GetStatus retrieves the status of a token by looking it up with Vault.
// The caller is responsible for retrieving the token from storage.
// Options can be provided for testing (e.g., WithCommandRunner).
func GetStatus(ctx context.Context, conn *config.Connection, tokenStr string, opts ...proxy.Option) (*Status, error) {
	status := &Status{
		Stored: true,
	}

	// Try to get token details from Vault
	if !proxy.BinaryExists(conn, opts...) {
		status.Error = fmt.Sprintf("%s binary not found", conn.GetBinaryPath())
		return status, nil
	}

	lookupData, err := Lookup(ctx, conn, tokenStr, opts...)
	if err != nil {
		status.Valid = false
		status.Error = err.Error()
		return status, nil
	}

	// Token is valid with full details
	status.Valid = true
	status.DisplayName = lookupData.DisplayName
	status.TTL = lookupData.TTL
	status.Renewable = lookupData.Renewable
	status.Policies = lookupData.Policies
	status.AuthPath = lookupData.Path
	status.EntityID = lookupData.EntityID
	status.Accessor = lookupData.Accessor

	return status, nil
}

// Lookup queries Vault for token information.
// Options can be provided for testing (e.g., WithCommandRunner).
func Lookup(ctx context.Context, conn *config.Connection, tokenStr string, opts ...proxy.Option) (*VaultTokenLookupData, error) {
	allOpts := append([]proxy.Option{proxy.WithToken(tokenStr)}, opts...)
	exec := proxy.NewExecutor(conn, allOpts...)
	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"token", "lookup", "-format=json"}, &captureBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("token lookup failed: %s", captureBuf.String())
	}

	return ParseLookupResponse(captureBuf.Bytes())
}

// Renew renews a token with optional increment.
// The caller is responsible for updating storage if the token changes.
// Options can be provided for testing (e.g., WithCommandRunner).
func Renew(ctx context.Context, conn *config.Connection, tokenStr string, increment string, opts ...proxy.Option) (*Token, error) {
	if !proxy.BinaryExists(conn, opts...) {
		return nil, fmt.Errorf("vault/openbao binary %q not found", conn.GetBinaryPath())
	}

	// Build renew args
	renewArgs := []string{"token", "renew", "-format=json"}
	if increment != "" {
		renewArgs = append(renewArgs, "-increment="+increment)
	}

	allOpts := append([]proxy.Option{proxy.WithToken(tokenStr)}, opts...)
	exec := proxy.NewExecutor(conn, allOpts...)
	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, renewArgs, &captureBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to renew token: %w", err)
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("token renewal failed: %s", captureBuf.String())
	}

	// Parse response
	tok, err := ParseLoginResponse(captureBuf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse renewal response: %w", err)
	}

	return tok, nil
}

// Revoke revokes a token with Vault.
// Options can be provided for testing (e.g., WithCommandRunner).
func Revoke(ctx context.Context, conn *config.Connection, tokenStr string, opts ...proxy.Option) error {
	if !proxy.BinaryExists(conn, opts...) {
		return fmt.Errorf("vault/openbao binary %q not found", conn.GetBinaryPath())
	}

	allOpts := append([]proxy.Option{proxy.WithToken(tokenStr)}, opts...)
	exec := proxy.NewExecutor(conn, allOpts...)
	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"token", "revoke", "-self"}, &captureBuf)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("token revocation failed: %s", captureBuf.String())
	}

	return nil
}
