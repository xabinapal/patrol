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

// FormatUptime formats a duration as a human-readable uptime string.
// Shows all units (days, hours, minutes, seconds) unlike FormatDuration which shows max 2 units.
func FormatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
