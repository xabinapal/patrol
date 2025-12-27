package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// OutputFormat represents the output format type.
type OutputFormat string

const (
	// OutputFormatText is the default human-readable format.
	OutputFormatText OutputFormat = "text"
	// OutputFormatJSON outputs data as JSON.
	OutputFormatJSON OutputFormat = "json"
)

const (
	// TokenExpiryWarningSeconds is the TTL threshold in seconds below which a
	// token expiry warning should be shown (5 minutes).
	TokenExpiryWarningSeconds = 300
)

// OutputWriter handles formatted output.
type OutputWriter struct {
	format OutputFormat
	writer io.Writer
}

// NewOutputWriter creates a new OutputWriter.
func NewOutputWriter(format OutputFormat) *OutputWriter {
	return &OutputWriter{
		format: format,
		writer: os.Stdout,
	}
}

// WriteJSON writes data as JSON to stdout.
func (o *OutputWriter) WriteJSON(data interface{}) error {
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Write writes data according to the configured format.
// textFunc is called for text output, data is used for JSON output.
func (o *OutputWriter) Write(data interface{}, textFunc func()) error {
	if o.format == OutputFormatJSON {
		return o.WriteJSON(data)
	}
	textFunc()
	return nil
}

// IsJSON returns true if output format is JSON.
func (o *OutputWriter) IsJSON() bool {
	return o.format == OutputFormatJSON
}

// ParseOutputFormat parses a string into an OutputFormat.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case "text", "":
		return OutputFormatText, nil
	case "json":
		return OutputFormatJSON, nil
	default:
		return "", fmt.Errorf("invalid output format %q: must be 'text' or 'json'", s)
	}
}

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
	return FormatDuration(time.Duration(seconds) * time.Second)
}

// FormatDurationBrief formats a duration showing only the most significant unit.
// E.g., "5d", "3h", "45m", "30s"
func FormatDurationBrief(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatDurationFull formats a duration showing all non-zero units.
// E.g., "2d 5h 30m 15s", "1h 5m 30s"
func FormatDurationFull(d time.Duration) string {
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
