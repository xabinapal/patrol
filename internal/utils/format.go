package utils

import (
	"fmt"
	"time"
)

// FormatDuration formats a duration as a human-readable string with two units max.
// E.g., "5d 3h", "2h 30m", "45s"
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}

	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// FormatDurationSeconds is a convenience wrapper for FormatDuration that takes seconds.
func FormatDurationSeconds(seconds int) string {
	if seconds <= 0 {
		return "never expires"
	}
	return FormatDuration(time.Duration(seconds) * time.Second)
}

// Mask masks a sensitive string for display, showing only first and last few characters.
// E.g., "abc123xyz" -> "abc1****xyz"
func Mask(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
