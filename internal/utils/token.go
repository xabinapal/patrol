package utils

// MaskToken masks a token for display, showing only first and last few characters.
func MaskToken(s string) string {
	if len(s) <= 16 {
		return "********"
	}
	return s[:4] + "********" + s[len(s)-4:]
}
