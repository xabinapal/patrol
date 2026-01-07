package types

import (
	"testing"
	"time"
)

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
