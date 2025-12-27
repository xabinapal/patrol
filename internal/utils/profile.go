package utils

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
