package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// SanitizeAddressForProfile converts an address to a safe profile name.
func SanitizeAddressForProfile(addr string) string {
	name := addr
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	// Remove trailing dashes
	name = strings.TrimRight(name, "-")
	return name
}

// SanitizeNamespace converts a namespace to a safe string.
func SanitizeNamespace(ns string) string {
	return strings.ReplaceAll(ns, "/", "-")
}

// SanitizeKey makes a key safe for use as a filename.
// For security, keys containing path traversal patterns are hashed.
func SanitizeKey(key string) string {
	// Security: If key contains any path traversal patterns, hash it instead
	if strings.Contains(key, "..") || strings.Contains(key, "/") ||
		strings.Contains(key, "\\") || strings.Contains(key, string(filepath.Separator)) {
		h := sha256.Sum256([]byte(key))
		return hex.EncodeToString(h[:])
	}

	// Replace any characters that might be problematic in filenames
	// Note: We explicitly exclude '.' to prevent hidden files and traversal
	result := make([]byte, len(key))
	for i, c := range []byte(key) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' {
			result[i] = c
		} else {
			result[i] = '_'
		}
	}
	return string(result)
}

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

// IndexOf returns the index of the first occurrence of sep in s, or -1 if not found.
// This is a convenience wrapper around strings.IndexByte for consistency.
func IndexOf(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return i
		}
	}
	return -1
}
