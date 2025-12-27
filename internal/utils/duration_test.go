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

func TestFormatUptime(t *testing.T) {
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
			name:     "hours, minutes and seconds",
			duration: 2*time.Hour + 15*time.Minute + 30*time.Second,
			want:     "2h 15m 30s",
		},
		{
			name:     "days, hours, minutes and seconds",
			duration: 48*time.Hour + 6*time.Hour + 30*time.Minute + 15*time.Second,
			want:     "2d 6h 30m 15s",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "exactly one day",
			duration: 24 * time.Hour,
			want:     "1d 0h 0m 0s",
		},
		{
			name:     "exactly one hour",
			duration: 1 * time.Hour,
			want:     "1h 0m 0s",
		},
		{
			name:     "exactly one minute",
			duration: 1 * time.Minute,
			want:     "1m 0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatUptime(tt.duration); got != tt.want {
				t.Errorf("FormatUptime() = %q, want %q", got, tt.want)
			}
		})
	}
}
