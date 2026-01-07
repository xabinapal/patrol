package utils

import (
	"strings"
)

// ContainsAny checks if s contains any of the substrings (case-insensitive).
func ContainsAny(s string, substrings ...string) bool {
	sLower := strings.ToLower(s)
	for _, sub := range substrings {
		if strings.Contains(sLower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
