package utils

import (
	"bytes"
	"strings"
)

// Mask masks a sensitive string for display, showing only first and last few characters.
// E.g., "abc123xyz" -> "abc1****xyz"
func Mask(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// ExtractTokenFromOutput extracts the token from the captured output.
// With -token-only, the token is on the last line (prompts may be interleaved from stderr).
func ExtractTokenFromOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n\r"), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}

	return ""
}

// ExtractJSON finds and extracts the first JSON object from mixed output.
// This handles cases where warnings or other text appear before the JSON.
func ExtractJSON(data []byte) []byte {
	// Find the first '{' which indicates the start of a JSON object
	start := bytes.IndexByte(data, '{')
	if start == -1 {
		// No JSON object found, return original data
		return data
	}

	// Find the matching closing '}' by counting braces
	braceCount := 0
	for i := start; i < len(data); i++ {
		if data[i] == '{' {
			braceCount++
		} else if data[i] == '}' {
			braceCount--
			if braceCount == 0 {
				// Found the matching closing brace
				return data[start : i+1]
			}
		}
	}

	// If we didn't find a matching closing brace, return from start to end
	return data[start:]
}
