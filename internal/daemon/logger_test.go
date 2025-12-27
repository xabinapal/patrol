package daemon

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("LogLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    LogLevel
		wantErr bool
	}{
		{"debug", LogLevelDebug, false},
		{"DEBUG", LogLevelDebug, false},
		{"info", LogLevelInfo, false},
		{"INFO", LogLevelInfo, false},
		{"", LogLevelInfo, false}, // empty defaults to info
		{"warn", LogLevelWarn, false},
		{"WARN", LogLevelWarn, false},
		{"warning", LogLevelWarn, false},
		{"WARNING", LogLevelWarn, false},
		{"error", LogLevelError, false},
		{"ERROR", LogLevelError, false},
		{"invalid", LogLevelInfo, true},
		{"trace", LogLevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_TextOutput(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		writer:   &buf,
		level:    LogLevelDebug,
		jsonMode: false,
	}

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected output to contain [INFO], got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		writer:   &buf,
		level:    LogLevelDebug,
		jsonMode: true,
	}

	logger.Info("test message")

	output := buf.String()

	var entry logEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", entry.Message)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		writer:   &buf,
		level:    LogLevelWarn,
		jsonMode: false,
	}

	// Debug and Info should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Errorf("Expected no output for debug/info at WARN level, got: %s", buf.String())
	}

	// Warn and Error should pass through
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Errorf("Expected warn message to be logged")
	}

	buf.Reset()
	logger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Errorf("Expected error message to be logged")
	}
}

func TestLogger_WithData(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		writer:   &buf,
		level:    LogLevelDebug,
		jsonMode: true,
	}

	data := map[string]string{"key": "value"}
	logger.Info("message with data", data)

	output := buf.String()

	var entry logEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Data == nil {
		t.Error("Expected data field to be set")
	}
}

func TestLogger_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(LoggerConfig{
		Level:    LogLevelInfo,
		FilePath: logFile,
		JSONMode: false,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info("test file message")
	logger.Close()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test file message") {
		t.Errorf("Log file doesn't contain expected message")
	}
}

func TestLogger_Printf_Println(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		writer:   &buf,
		level:    LogLevelDebug,
		jsonMode: false,
	}

	logger.Printf("formatted %s", "message")
	if !strings.Contains(buf.String(), "formatted message") {
		t.Errorf("Printf didn't format correctly")
	}

	buf.Reset()
	logger.Println("println message")
	if !strings.Contains(buf.String(), "println message") {
		t.Errorf("Println didn't output correctly")
	}
}

func TestLogger_GetSetLevel(t *testing.T) {
	logger := &Logger{
		writer: os.Stderr,
		level:  LogLevelInfo,
	}

	if logger.GetLevel() != LogLevelInfo {
		t.Error("GetLevel() should return INFO")
	}

	logger.SetLevel(LogLevelDebug)
	if logger.GetLevel() != LogLevelDebug {
		t.Error("SetLevel(DEBUG) didn't change level")
	}
}

func TestNewLogger_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "subdir", "nested", "test.log")

	logger, err := NewLogger(LoggerConfig{
		Level:    LogLevelInfo,
		FilePath: logFile,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Directory should have been created
	dir := filepath.Dir(logFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}
