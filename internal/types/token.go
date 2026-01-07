// Package types provides shared types used across the application.
package types

import (
	"time"
)

// Token represents a Vault authentication token with metadata.
type Token struct {
	// ClientToken is the actual token string.
	ClientToken string
	// LeaseDuration is the TTL in seconds.
	LeaseDuration int
	// Renewable indicates if the token can be renewed.
	Renewable bool
	// ExpiresAt is the calculated expiration time.
	ExpiresAt time.Time
}

func (t *Token) NeedsRenewal(threshold float64, minTTL time.Duration) bool {
	if !t.Renewable {
		return false
	}

	if t.ExpiresAt.IsZero() {
		return false
	}

	ttl := time.Until(t.ExpiresAt)
	if ttl <= 0 {
		return false
	}

	if ttl < minTTL {
		return true
	}

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
