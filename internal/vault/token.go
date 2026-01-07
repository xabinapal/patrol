// Package vault provides Vault/OpenBao server interaction utilities.
package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/types"
)

// TokenExecutor provides an interface for executing Vault token operations.
type TokenExecutor interface {
	// RenewToken renews a token with optional increment.
	RenewToken(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*TokenStatus, error)
	// RevokeToken revokes a token.
	RevokeToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error
	// LookupToken queries Vault for token information.
	LookupToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*TokenStatus, error)
}

type tokenExecutor struct{}

// NewTokenExecutor creates a new TokenExecutor.
func NewTokenExecutor() TokenExecutor {
	return &tokenExecutor{}
}

// TokenStatus represents the status of a token from Vault.
type TokenStatus struct {
	TTL       int  `json:"ttl"`
	Renewable bool `json:"renewable"`
}

// VaultLoginResponse represents the JSON response from vault login.
type VaultLoginResponse struct {
	RequestID     string         `json:"request_id"`
	LeaseID       string         `json:"lease_id"`
	Renewable     bool           `json:"renewable"`
	LeaseDuration int            `json:"lease_duration"`
	Data          map[string]any `json:"data"`
	WrapInfo      any            `json:"wrap_info"`
	Warnings      []string       `json:"warnings"`
	Auth          *VaultAuthInfo `json:"auth"`
}

// VaultAuthInfo represents the auth section of a Vault response.
type VaultAuthInfo struct {
	ClientToken      string            `json:"client_token"`
	Accessor         string            `json:"accessor"`
	Policies         []string          `json:"policies"`
	TokenPolicies    []string          `json:"token_policies"`
	IdentityPolicies []string          `json:"identity_policies"`
	Metadata         map[string]string `json:"metadata"`
	LeaseDuration    int               `json:"lease_duration"`
	Renewable        bool              `json:"renewable"`
	EntityID         string            `json:"entity_id"`
	TokenType        string            `json:"token_type"`
	Orphan           bool              `json:"orphan"`
	NumUses          int               `json:"num_uses"`
}

// VaultTokenResponse represents a token response from Vault operations.
type VaultTokenResponse struct {
	ClientToken      string
	Accessor         string
	Policies         []string
	TokenPolicies    []string
	IdentityPolicies []string
	Metadata         map[string]string
	LeaseDuration    int
	Renewable        bool
	EntityID         string
	TokenType        string
	Orphan           bool
	NumUses          int
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

// ParseLoginResponse parses a Vault login JSON response and extracts the token data.
func ParseLoginResponse(data []byte) (*VaultTokenResponse, error) {
	var resp VaultLoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}

	if resp.Auth == nil {
		return nil, errors.New("login response does not contain auth information")
	}

	if resp.Auth.ClientToken == "" {
		return nil, errors.New("login response does not contain a client token")
	}

	now := time.Now()
	tok := &VaultTokenResponse{
		ClientToken:      resp.Auth.ClientToken,
		Accessor:         resp.Auth.Accessor,
		Policies:         resp.Auth.Policies,
		TokenPolicies:    resp.Auth.TokenPolicies,
		IdentityPolicies: resp.Auth.IdentityPolicies,
		Metadata:         resp.Auth.Metadata,
		LeaseDuration:    resp.Auth.LeaseDuration,
		Renewable:        resp.Auth.Renewable,
		EntityID:         resp.Auth.EntityID,
		TokenType:        resp.Auth.TokenType,
		Orphan:           resp.Auth.Orphan,
		NumUses:          resp.Auth.NumUses,
		CreatedAt:        now,
	}

	if tok.LeaseDuration > 0 {
		tok.ExpiresAt = now.Add(time.Duration(tok.LeaseDuration) * time.Second)
	}

	return tok, nil
}

func (e *tokenExecutor) RenewToken(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*TokenStatus, error) {
	client, err := buildHTTPClient(prof)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := prof.Address + "/v1/auth/token/renew-self"

	var requestBody map[string]any
	if increment != "" {
		requestBody = map[string]any{
			"increment": increment,
		}
	}

	var bodyReader io.Reader
	if requestBody != nil {
		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", tokenStr)
	req.Header.Set("Content-Type", "application/json")
	if prof.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", prof.Namespace)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to renew token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token renewal failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	vaultResp, err := ParseLoginResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse renewal response: %w", err)
	}

	return &TokenStatus{
		TTL:       vaultResp.LeaseDuration,
		Renewable: vaultResp.Renewable,
	}, nil
}

// VaultTokenLookupResponse represents the response from token lookup.
type VaultTokenLookupResponse struct {
	RequestID     string                `json:"request_id"`
	LeaseID       string                `json:"lease_id"`
	Renewable     bool                  `json:"renewable"`
	LeaseDuration int                   `json:"lease_duration"`
	Data          *VaultTokenLookupData `json:"data"`
	WrapInfo      any                   `json:"wrap_info"`
	Warnings      []string              `json:"warnings"`
	Auth          map[string]any        `json:"auth"`
}

// VaultTokenLookupData represents the data from token lookup.
type VaultTokenLookupData struct {
	Accessor         string            `json:"accessor"`
	CreationTime     int64             `json:"creation_time"`
	CreationTTL      int               `json:"creation_ttl"`
	DisplayName      string            `json:"display_name"`
	EntityID         string            `json:"entity_id"`
	ExpireTime       *string           `json:"expire_time"`
	ExplicitMaxTTL   int               `json:"explicit_max_ttl"`
	ID               string            `json:"id"`
	IdentityPolicies []string          `json:"identity_policies"`
	IssueTime        string            `json:"issue_time"`
	Meta             map[string]string `json:"meta"`
	NumUses          int               `json:"num_uses"`
	Orphan           bool              `json:"orphan"`
	Path             string            `json:"path"`
	Policies         []string          `json:"policies"`
	Renewable        bool              `json:"renewable"`
	TTL              int               `json:"ttl"`
	Type             string            `json:"type"`
}

// ParseLookupResponse parses a Vault token lookup response.
func ParseLookupResponse(data []byte) (*VaultTokenLookupData, error) {
	var resp VaultTokenLookupResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	if resp.Data == nil {
		return nil, errors.New("lookup response does not contain data")
	}

	return resp.Data, nil
}

func (e *tokenExecutor) LookupToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) (*TokenStatus, error) {
	client, err := buildHTTPClient(prof)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := prof.Address + "/v1/auth/token/lookup-self"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", tokenStr)
	if prof.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", prof.Namespace)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token lookup failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	lookupData, err := ParseLookupResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	return &TokenStatus{
		TTL:       lookupData.TTL,
		Renewable: lookupData.Renewable,
	}, nil
}

func (e *tokenExecutor) RevokeToken(ctx context.Context, prof *types.Profile, tokenStr string, opts ...proxy.Option) error {
	client, err := buildHTTPClient(prof)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := prof.Address + "/v1/auth/token/revoke-self"

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", tokenStr)
	if prof.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", prof.Namespace)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token revocation failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
