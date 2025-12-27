package utils

import (
	"testing"
)

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

func TestExtractTokenFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "token only",
			output:   "root-token",
			expected: "root-token",
		},
		{
			name:     "token with newline",
			output:   "root-token\n",
			expected: "root-token",
		},
		{
			name:     "token with prompt interleaved",
			output:   "Token (will be hidden): \nroot-token",
			expected: "root-token",
		},
		{
			name:     "token on last line with multiple lines",
			output:   "Token (will be hidden): \nroot-token\n",
			expected: "root-token",
		},
		{
			name:     "token with stderr prompt",
			output:   "Token (will be hidden): \nsome-warning\nroot-token",
			expected: "root-token",
		},
		{
			name:     "token with carriage return",
			output:   "Token (will be hidden): \r\nroot-token\r\n",
			expected: "root-token",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "only whitespace",
			output:   "   \n  \n  ",
			expected: "",
		},
		{
			name:     "token with leading/trailing spaces",
			output:   "  root-token  \n",
			expected: "root-token",
		},
		{
			name:     "multiple empty lines before token",
			output:   "\n\n\nroot-token",
			expected: "root-token",
		},
		{
			name:     "token with mixed line endings",
			output:   "prompt\r\nroot-token\n",
			expected: "root-token",
		},
		{
			name:     "token on first line with trailing empty lines",
			output:   "root-token\n\n\n",
			expected: "root-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTokenFromOutput(tt.output)
			if result != tt.expected {
				t.Errorf("ExtractTokenFromOutput(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple JSON object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with text before",
			input:    `Warning: some warning\n{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with text after",
			input:    `{"key": "value"}more text`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested JSON object",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "JSON with array",
			input:    `{"items": [1, 2, 3]}`,
			expected: `{"items": [1, 2, 3]}`,
		},
		{
			name:     "multiple JSON objects",
			input:    `{"first": "value"}{"second": "value"}`,
			expected: `{"first": "value"}`,
		},
		{
			name:     "no JSON object",
			input:    `just some text`,
			expected: `just some text`,
		},
		{
			name:     "empty string",
			input:    ``,
			expected: ``,
		},
		{
			name:     "JSON with escaped quotes",
			input:    `{"key": "value with \"quotes\""}`,
			expected: `{"key": "value with \"quotes\""}`,
		},
		{
			name:     "incomplete JSON",
			input:    `{"key": "value"`,
			expected: `{"key": "value"`,
		},
		{
			name:     "JSON with whitespace before",
			input:    `   {"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "complex nested JSON",
			input:    `Warning text\n{"auth": {"client_token": "token", "policies": ["policy1", "policy2"]}}`,
			expected: `{"auth": {"client_token": "token", "policies": ["policy1", "policy2"]}}`,
		},
		{
			name:     "JSON with braces in string",
			input:    `{"message": "text with {braces}"}`,
			expected: `{"message": "text with {braces}"}`,
		},
		{
			name:     "multiple opening braces",
			input:    `{{"key": "value"}}`,
			expected: `{{"key": "value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSON([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("ExtractJSON(%q) = %q, want %q", tt.input, string(result), tt.expected)
			}
		})
	}
}

func TestExtractJSONWithBinaryData(t *testing.T) {
	// Test with actual binary data (not just strings)
	input := []byte{0x00, 0x01, 0x02, '{', '"', 'k', 'e', 'y', '"', ':', '"', 'v', 'a', 'l', 'u', 'e', '"', '}', 0xFF, 0xFE}
	result := ExtractJSON(input)

	expected := `{"key":"value"}`
	if string(result) != expected {
		t.Errorf("ExtractJSON with binary data = %q, want %q", string(result), expected)
	}
}
