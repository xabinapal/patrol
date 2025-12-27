package token

import (
	"testing"
	"time"
)

func TestParseLoginResponse(t *testing.T) {
	jsonData := []byte(`{
		"request_id": "test-request-id",
		"lease_id": "",
		"renewable": false,
		"lease_duration": 0,
		"auth": {
			"client_token": "hvs.test-token-12345",
			"accessor": "test-accessor",
			"policies": ["default", "admin"],
			"token_policies": ["default", "admin"],
			"metadata": {
				"username": "testuser"
			},
			"lease_duration": 3600,
			"renewable": true,
			"entity_id": "entity-123",
			"token_type": "service",
			"orphan": false,
			"num_uses": 0
		}
	}`)

	tok, err := ParseLoginResponse(jsonData)
	if err != nil {
		t.Fatalf("ParseLoginResponse() failed: %v", err)
	}

	if tok.ClientToken != "hvs.test-token-12345" {
		t.Errorf("expected ClientToken 'hvs.test-token-12345', got '%s'", tok.ClientToken)
	}

	if tok.Accessor != "test-accessor" {
		t.Errorf("expected Accessor 'test-accessor', got '%s'", tok.Accessor)
	}

	if tok.LeaseDuration != 3600 {
		t.Errorf("expected LeaseDuration 3600, got %d", tok.LeaseDuration)
	}

	if !tok.Renewable {
		t.Error("expected Renewable to be true")
	}

	if len(tok.Policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(tok.Policies))
	}

	if tok.TokenType != "service" {
		t.Errorf("expected TokenType 'service', got '%s'", tok.TokenType)
	}
}

func TestParseLoginResponse_NoAuth(t *testing.T) {
	jsonData := []byte(`{
		"request_id": "test",
		"data": {}
	}`)

	_, err := ParseLoginResponse(jsonData)
	if err == nil {
		t.Error("ParseLoginResponse() should fail when auth is missing")
	}
}

func TestParseLoginResponse_NoToken(t *testing.T) {
	jsonData := []byte(`{
		"auth": {
			"accessor": "test"
		}
	}`)

	_, err := ParseLoginResponse(jsonData)
	if err == nil {
		t.Error("ParseLoginResponse() should fail when client_token is missing")
	}
}

func TestParseLoginResponse_InvalidJSON(t *testing.T) {
	jsonData := []byte(`invalid json`)

	_, err := ParseLoginResponse(jsonData)
	if err == nil {
		t.Error("ParseLoginResponse() should fail for invalid JSON")
	}
}

func TestParseLoginResponse_WithWarnings(t *testing.T) {
	// Simulate Vault output with warning message before JSON
	mixedOutput := []byte(`The token was not stored in token helper. Set the VAULT_TOKEN environment
variable or pass the token below with each request to Vault.

{
  "request_id": "",
  "lease_id": "",
  "lease_duration": 0,
  "renewable": false,
  "data": null,
  "warnings": null,
  "auth": {
    "client_token": "hvs.test-token-12345",
    "accessor": "test-accessor",
    "policies": ["default", "admin"],
    "token_policies": ["default", "admin"],
    "lease_duration": 3600,
    "renewable": true,
    "entity_id": "entity-123",
    "token_type": "service",
    "orphan": false,
    "num_uses": 0
  }
}`)

	tok, err := ParseLoginResponse(mixedOutput)
	if err != nil {
		t.Fatalf("ParseLoginResponse() failed with mixed output: %v", err)
	}

	if tok.ClientToken != "hvs.test-token-12345" {
		t.Errorf("expected ClientToken 'hvs.test-token-12345', got '%s'", tok.ClientToken)
	}

	if tok.Accessor != "test-accessor" {
		t.Errorf("expected Accessor 'test-accessor', got '%s'", tok.Accessor)
	}
}

func TestParseLookupResponse(t *testing.T) {
	jsonData := []byte(`{
		"request_id": "test",
		"data": {
			"accessor": "test-accessor",
			"creation_time": 1609459200,
			"creation_ttl": 3600,
			"display_name": "userpass-admin",
			"entity_id": "entity-123",
			"expire_time": "2021-01-01T02:00:00Z",
			"explicit_max_ttl": 0,
			"id": "hvs.test-token",
			"issue_time": "2021-01-01T01:00:00Z",
			"meta": {"username": "admin"},
			"num_uses": 0,
			"orphan": false,
			"path": "auth/userpass/login/admin",
			"policies": ["default", "admin"],
			"renewable": true,
			"ttl": 1800,
			"type": "service"
		}
	}`)

	data, err := ParseLookupResponse(jsonData)
	if err != nil {
		t.Fatalf("ParseLookupResponse() failed: %v", err)
	}

	if data.Accessor != "test-accessor" {
		t.Errorf("expected Accessor 'test-accessor', got '%s'", data.Accessor)
	}

	if data.TTL != 1800 {
		t.Errorf("expected TTL 1800, got %d", data.TTL)
	}

	if !data.Renewable {
		t.Error("expected Renewable to be true")
	}

	if data.Path != "auth/userpass/login/admin" {
		t.Errorf("expected Path 'auth/userpass/login/admin', got '%s'", data.Path)
	}
}

func TestParseLookupResponse_NoData(t *testing.T) {
	jsonData := []byte(`{"request_id": "test"}`)

	_, err := ParseLookupResponse(jsonData)
	if err == nil {
		t.Error("ParseLookupResponse() should fail when data is missing")
	}
}

func TestTokenTTL(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		token     Token
		wantNeg   bool // TTL should be negative (never expires)
		wantZero  bool // TTL should be zero or near zero
		wantRange time.Duration
	}{
		{
			name: "no expiration",
			token: Token{
				ExpiresAt: time.Time{},
			},
			wantNeg: true,
		},
		{
			name: "expired",
			token: Token{
				ExpiresAt: now.Add(-time.Hour),
			},
			wantZero: true,
		},
		{
			name: "expires in 1 hour",
			token: Token{
				ExpiresAt: now.Add(time.Hour),
			},
			wantRange: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := tt.token.TTL()

			if tt.wantNeg && ttl >= 0 {
				t.Errorf("expected negative TTL, got %v", ttl)
			}
			if tt.wantZero && ttl > time.Second {
				t.Errorf("expected zero TTL, got %v", ttl)
			}
			if tt.wantRange > 0 {
				// Allow some tolerance
				diff := tt.wantRange - ttl
				if diff < 0 {
					diff = -diff
				}
				if diff > time.Second {
					t.Errorf("expected TTL around %v, got %v", tt.wantRange, ttl)
				}
			}
		})
	}
}

func TestTokenIsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		token   Token
		expired bool
	}{
		{
			name:    "no expiration",
			token:   Token{ExpiresAt: time.Time{}},
			expired: false,
		},
		{
			name:    "expired",
			token:   Token{ExpiresAt: now.Add(-time.Hour)},
			expired: true,
		},
		{
			name:    "not expired",
			token:   Token{ExpiresAt: now.Add(time.Hour)},
			expired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestTokenNeedsRenewal(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		token     Token
		threshold float64
		minTTL    time.Duration
		wants     bool
	}{
		{
			name: "not renewable",
			token: Token{
				Renewable:     false,
				LeaseDuration: 3600,
				ExpiresAt:     now.Add(time.Hour),
			},
			threshold: 0.75,
			minTTL:    5 * time.Minute,
			wants:     false,
		},
		{
			name: "no expiration",
			token: Token{
				Renewable: true,
				ExpiresAt: time.Time{},
			},
			threshold: 0.75,
			minTTL:    5 * time.Minute,
			wants:     false,
		},
		{
			name: "below min TTL",
			token: Token{
				Renewable:     true,
				LeaseDuration: 3600,
				ExpiresAt:     now.Add(2 * time.Minute),
			},
			threshold: 0.75,
			minTTL:    5 * time.Minute,
			wants:     true,
		},
		{
			name: "above threshold",
			token: Token{
				Renewable:     true,
				LeaseDuration: 3600,
				ExpiresAt:     now.Add(5 * time.Minute), // 8.3% remaining
			},
			threshold: 0.75,
			minTTL:    1 * time.Minute,
			wants:     true,
		},
		{
			name: "no renewal needed",
			token: Token{
				Renewable:     true,
				LeaseDuration: 3600,
				ExpiresAt:     now.Add(50 * time.Minute), // 83% remaining
			},
			threshold: 0.75,
			minTTL:    5 * time.Minute,
			wants:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.NeedsRenewal(tt.threshold, tt.minTTL); got != tt.wants {
				t.Errorf("NeedsRenewal() = %v, want %v", got, tt.wants)
			}
		})
	}
}

func TestTokenFormatTTL(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		token    Token
		contains string
	}{
		{
			name:     "never expires",
			token:    Token{ExpiresAt: time.Time{}},
			contains: "never",
		},
		{
			name:     "expired",
			token:    Token{ExpiresAt: now.Add(-time.Hour)},
			contains: "expired",
		},
		{
			name:     "days",
			token:    Token{ExpiresAt: now.Add(48 * time.Hour)},
			contains: "d",
		},
		{
			name:     "hours",
			token:    Token{ExpiresAt: now.Add(2 * time.Hour)},
			contains: "h",
		},
		{
			name:     "minutes",
			token:    Token{ExpiresAt: now.Add(30 * time.Minute)},
			contains: "m",
		},
		{
			name:     "seconds",
			token:    Token{ExpiresAt: now.Add(30 * time.Second)},
			contains: "s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.FormatTTL()
			if !containsString(result, tt.contains) {
				t.Errorf("FormatTTL() = %s, want to contain %s", result, tt.contains)
			}
		})
	}
}

func TestTokenMaskedToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "normal token",
			token:    "hvs.test-token-12345",
			expected: "hvs.****",
		},
		{
			name:     "short token",
			token:    "short",
			expected: "****",
		},
		{
			name:     "very short token",
			token:    "ab",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := Token{ClientToken: tt.token}
			if got := tok.MaskedToken(); got != tt.expected {
				t.Errorf("MaskedToken() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
