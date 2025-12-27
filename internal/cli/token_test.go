package cli

import (
	"testing"
	"time"
)

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "normal token",
			token: "hvs.CAESIJzMz_12345678901234567890",
			want:  "hvs.****7890",
		},
		{
			name:  "short token",
			token: "12345678",
			want:  "****",
		},
		{
			name:  "very short token",
			token: "abc",
			want:  "****",
		},
		{
			name:  "exactly 8 chars",
			token: "12345678",
			want:  "****",
		},
		{
			name:  "9 chars",
			token: "123456789",
			want:  "1234****6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskToken(tt.token); got != tt.want {
				t.Errorf("maskToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTokenDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 5*time.Minute + 30*time.Second,
			want:     "5m 30s",
		},
		{
			name:     "hours and minutes",
			duration: 3*time.Hour + 15*time.Minute,
			want:     "3h 15m",
		},
		{
			name:     "days and hours",
			duration: 48*time.Hour + 6*time.Hour,
			want:     "2d 6h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTokenDuration(tt.duration); got != tt.want {
				t.Errorf("formatTokenDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}
