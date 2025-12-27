package cli

import (
	"testing"
	"time"
)

func TestCheckStatus_String(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{CheckOK, "OK"},
		{CheckWarning, "WARN"},
		{CheckError, "ERROR"},
		{CheckSkipped, "SKIP"},
		{CheckStatus(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("CheckStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckStatus_Icon(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{CheckOK, "[OK]"},
		{CheckWarning, "[!!]"},
		{CheckError, "[XX]"},
		{CheckSkipped, "[--]"},
		{CheckStatus(99), "[??]"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.Icon(); got != tt.want {
				t.Errorf("CheckStatus.Icon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDurationDoctor(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes",
			duration: 15 * time.Minute,
			want:     "15m",
		},
		{
			name:     "hours",
			duration: 3 * time.Hour,
			want:     "3h",
		},
		{
			name:     "days",
			duration: 48 * time.Hour,
			want:     "2d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDurationDoctor(tt.duration); got != tt.want {
				t.Errorf("formatDurationDoctor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTokenLookupDoctor(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		data := []byte(`{"data":{"ttl":3600}}`)
		info, err := parseTokenLookupDoctor(data)
		if err != nil {
			t.Errorf("parseTokenLookupDoctor() error = %v", err)
			return
		}
		if info.TTL != 3600 {
			t.Errorf("parseTokenLookupDoctor() TTL = %d, want 3600", info.TTL)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		data := []byte(`invalid json`)
		_, err := parseTokenLookupDoctor(data)
		if err == nil {
			t.Error("parseTokenLookupDoctor() expected error for invalid json")
		}
	})
}
