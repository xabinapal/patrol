// Package token provides Vault token management functionality.
package token

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Token represents a Vault authentication token with metadata.
type Token struct {
	// ClientToken is the actual token string.
	ClientToken string `json:"client_token"`
	// Accessor is the token accessor.
	Accessor string `json:"accessor,omitempty"`
	// Policies are the policies attached to this token.
	Policies []string `json:"policies,omitempty"`
	// TokenPolicies are the token-specific policies.
	TokenPolicies []string `json:"token_policies,omitempty"`
	// IdentityPolicies are the identity-based policies.
	IdentityPolicies []string `json:"identity_policies,omitempty"`
	// Metadata contains token metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
	// LeaseDuration is the TTL in seconds.
	LeaseDuration int `json:"lease_duration,omitempty"`
	// Renewable indicates if the token can be renewed.
	Renewable bool `json:"renewable,omitempty"`
	// EntityID is the entity ID associated with this token.
	EntityID string `json:"entity_id,omitempty"`
	// TokenType is the type of token (service, batch, etc.).
	TokenType string `json:"token_type,omitempty"`
	// Orphan indicates if this is an orphan token.
	Orphan bool `json:"orphan,omitempty"`
	// NumUses is the remaining number of uses (-1 for unlimited).
	NumUses int `json:"num_uses,omitempty"`

	// CreatedAt is when this token was captured by Patrol.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// ExpiresAt is the calculated expiration time.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// VaultLoginResponse represents the JSON response from vault login.
type VaultLoginResponse struct {
	RequestID     string                 `json:"request_id"`
	LeaseID       string                 `json:"lease_id"`
	Renewable     bool                   `json:"renewable"`
	LeaseDuration int                    `json:"lease_duration"`
	Data          map[string]interface{} `json:"data"`
	WrapInfo      interface{}            `json:"wrap_info"`
	Warnings      []string               `json:"warnings"`
	Auth          *VaultAuthInfo         `json:"auth"`
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

// VaultTokenLookupResponse represents the response from token lookup.
type VaultTokenLookupResponse struct {
	RequestID     string                 `json:"request_id"`
	LeaseID       string                 `json:"lease_id"`
	Renewable     bool                   `json:"renewable"`
	LeaseDuration int                    `json:"lease_duration"`
	Data          *VaultTokenLookupData  `json:"data"`
	WrapInfo      interface{}            `json:"wrap_info"`
	Warnings      []string               `json:"warnings"`
	Auth          map[string]interface{} `json:"auth"`
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

// extractJSON finds and extracts the first JSON object from mixed output.
// This handles cases where warnings or other text appear before the JSON.
func extractJSON(data []byte) []byte {
	// Find the first '{' which indicates the start of a JSON object
	start := bytes.IndexByte(data, '{')
	if start == -1 {
		// No JSON object found, return original data
		return data
	}

	// Find the matching closing '}' by counting braces
	braceCount := 0
	for i := start; i < len(data); i++ {
		if data[i] == '{' {
			braceCount++
		} else if data[i] == '}' {
			braceCount--
			if braceCount == 0 {
				// Found the matching closing brace
				return data[start : i+1]
			}
		}
	}

	// If we didn't find a matching closing brace, return from start to end
	return data[start:]
}

// ParseLoginResponse parses a Vault login JSON response and extracts the token.
func ParseLoginResponse(data []byte) (*Token, error) {
	// Extract JSON from potentially mixed output (warnings may appear before JSON)
	jsonData := extractJSON(data)

	var resp VaultLoginResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}

	if resp.Auth == nil {
		return nil, errors.New("login response does not contain auth information")
	}

	if resp.Auth.ClientToken == "" {
		return nil, errors.New("login response does not contain a client token")
	}

	now := time.Now()
	token := &Token{
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

	// Calculate expiration time
	if token.LeaseDuration > 0 {
		token.ExpiresAt = now.Add(time.Duration(token.LeaseDuration) * time.Second)
	}

	return token, nil
}

// ParseLookupResponse parses a Vault token lookup response.
func ParseLookupResponse(data []byte) (*VaultTokenLookupData, error) {
	// Extract JSON from potentially mixed output (warnings may appear before JSON)
	jsonData := extractJSON(data)

	var resp VaultTokenLookupResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	if resp.Data == nil {
		return nil, errors.New("lookup response does not contain data")
	}

	return resp.Data, nil
}

// TTL returns the remaining time-to-live for the token.
func (t *Token) TTL() time.Duration {
	if t.ExpiresAt.IsZero() {
		// No expiration set, token doesn't expire
		return -1
	}
	ttl := time.Until(t.ExpiresAt)
	if ttl < 0 {
		return 0
	}
	return ttl
}

// IsExpired checks if the token has expired.
func (t *Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// NeedsRenewal checks if the token needs to be renewed based on threshold.
func (t *Token) NeedsRenewal(threshold float64, minTTL time.Duration) bool {
	if !t.Renewable {
		return false
	}

	if t.ExpiresAt.IsZero() {
		return false
	}

	ttl := t.TTL()
	if ttl <= 0 {
		return false // Already expired
	}

	// Check minimum TTL threshold
	if ttl < minTTL {
		return true
	}

	// Check percentage threshold
	totalDuration := time.Duration(t.LeaseDuration) * time.Second
	if totalDuration > 0 {
		elapsed := totalDuration - ttl
		elapsedRatio := float64(elapsed) / float64(totalDuration)
		if elapsedRatio >= threshold {
			return true
		}
	}

	return false
}

// FormatTTL returns a human-readable TTL string.
func (t *Token) FormatTTL() string {
	ttl := t.TTL()
	if ttl < 0 {
		return "never expires"
	}
	if ttl == 0 {
		return "expired"
	}

	hours := int(ttl.Hours())
	minutes := int(ttl.Minutes()) % 60
	seconds := int(ttl.Seconds()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// MaskedToken returns a masked version of the token for display.
func (t *Token) MaskedToken() string {
	if len(t.ClientToken) <= 8 {
		return "****"
	}
	return t.ClientToken[:4] + "****"
}
