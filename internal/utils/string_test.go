package utils

import (
	"testing"
)

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		subs     []string
		expected bool
	}{
		{
			name:     "contains first substring",
			s:        "hello world",
			subs:     []string{"hello", "foo"},
			expected: true,
		},
		{
			name:     "contains second substring",
			s:        "hello world",
			subs:     []string{"foo", "world"},
			expected: true,
		},
		{
			name:     "case insensitive match",
			s:        "Hello World",
			subs:     []string{"hello"},
			expected: true,
		},
		{
			name:     "case insensitive substring",
			s:        "hello world",
			subs:     []string{"HELLO"},
			expected: true,
		},
		{
			name:     "no match",
			s:        "test string",
			subs:     []string{"foo", "bar"},
			expected: false,
		},
		{
			name:     "contains multi-word substring",
			s:        "secret service error",
			subs:     []string{"secret service"},
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			subs:     []string{"anything"},
			expected: false,
		},
		{
			name:     "empty substrings",
			s:        "test",
			subs:     []string{},
			expected: false,
		},
		{
			name:     "empty string and empty subs",
			s:        "",
			subs:     []string{},
			expected: false,
		},
		{
			name:     "partial word match",
			s:        "database",
			subs:     []string{"data"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsAny(tt.s, tt.subs...)
			if result != tt.expected {
				t.Errorf("ContainsAny(%q, %v) = %v, want %v", tt.s, tt.subs, result, tt.expected)
			}
		})
	}
}
