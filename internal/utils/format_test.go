package utils

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "expired",
		},
		{
			name:     "negative duration",
			duration: -1 * time.Second,
			want:     "expired",
		},
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
		{
			name:     "exactly one day",
			duration: 24 * time.Hour,
			want:     "1d 0h",
		},
		{
			name:     "large duration",
			duration: 10*24*time.Hour + 12*time.Hour,
			want:     "10d 12h",
		},
		{
			name:     "less than a minute with seconds",
			duration: 30 * time.Second,
			want:     "30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDuration(tt.duration); got != tt.want {
				t.Errorf("FormatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDurationSeconds(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{
			name:    "zero seconds",
			seconds: 0,
			want:    "never expires",
		},
		{
			name:    "negative seconds",
			seconds: -1,
			want:    "never expires",
		},
		{
			name:    "45 seconds",
			seconds: 45,
			want:    "45s",
		},
		{
			name:    "5 minutes 30 seconds",
			seconds: 330,
			want:    "5m 30s",
		},
		{
			name:    "3 hours 15 minutes",
			seconds: 11700,
			want:    "3h 15m",
		},
		{
			name:    "2 days 6 hours",
			seconds: 194400,
			want:    "2d 6h",
		},
		{
			name:    "one day",
			seconds: 86400,
			want:    "1d 0h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDurationSeconds(tt.seconds); got != tt.want {
				t.Errorf("FormatDurationSeconds() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal token",
			input: "hvs.CAESIJzMz_12345678901234567890",
			want:  "hvs.****7890",
		},
		{
			name:  "short token (8 chars)",
			input: "12345678",
			want:  "****",
		},
		{
			name:  "very short token (3 chars)",
			input: "abc",
			want:  "****",
		},
		{
			name:  "exactly 8 chars",
			input: "12345678",
			want:  "****",
		},
		{
			name:  "9 chars",
			input: "123456789",
			want:  "1234****6789",
		},
		{
			name:  "10 chars",
			input: "1234567890",
			want:  "1234****7890",
		},
		{
			name:  "empty string",
			input: "",
			want:  "****",
		},
		{
			name:  "single character",
			input: "a",
			want:  "****",
		},
		{
			name:  "long token",
			input: "hvs.CAESIJzMz_123456789012345678901234567890",
			want:  "hvs.****7890",
		},
		{
			name:  "exactly 4 chars",
			input: "1234",
			want:  "****",
		},
		{
			name:  "exactly 5 chars",
			input: "12345",
			want:  "****",
		},
		{
			name:  "exactly 7 chars",
			input: "1234567",
			want:  "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mask(tt.input); got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}
