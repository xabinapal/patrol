package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
func (o *OutputWriter) WriteJSON(data any) error {
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Write writes data according to the configured format.
// textFunc is called for text output, data is used for JSON output.
func (o *OutputWriter) Write(data any, textFunc func()) error {
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
