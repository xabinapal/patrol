package utils

import (
	"testing"
)

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "********",
		},
		{
			name:  "single character",
			input: "a",
			want:  "********",
		},
		{
			name:  "short token",
			input: "12345678",
			want:  "********",
		},
		{
			name:  "normal token",
			input: "hvs.CAESIJzMz_12345678901234567890",
			want:  "hvs.********7890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskToken(tt.input); got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}
