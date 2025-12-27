package cli

import (
	"testing"
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
