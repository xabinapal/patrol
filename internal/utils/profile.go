package utils

import "strings"

// IsValidProfileName checks if a profile name contains only safe characters.
// This prevents log injection and other security issues from malicious env vars.
func IsValidProfileName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		// Allow alphanumeric, hyphen, underscore, and dot
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

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

// SanitizeNamespaceForProfile converts a namespace to a safe profile name.
func SanitizeNamespaceForProfile(ns string) string {
	return strings.ReplaceAll(ns, "/", "-")
}
