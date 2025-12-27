package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// LogLevel represents logging severity.
type LogLevel int

const (
	// LogLevelDebug includes detailed debugging information.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo includes standard operational information.
	LogLevelInfo
	// LogLevelWarn includes warnings about potential issues.
	LogLevelWarn
	// LogLevelError includes only error messages.
	LogLevelError
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a log level string.
func ParseLogLevel(s string) (LogLevel, error) {
	switch s {
	case "debug", "DEBUG":
		return LogLevelDebug, nil
	case "info", "INFO", "":
		return LogLevelInfo, nil
	case "warn", "WARN", "warning", "WARNING":
		return LogLevelWarn, nil
	case "error", "ERROR":
		return LogLevelError, nil
	default:
		return LogLevelInfo, fmt.Errorf("invalid log level: %s", s)
	}
}

// Logger provides structured logging for the daemon.
type Logger struct {
	mu       sync.Mutex
	writer   io.Writer
	level    LogLevel
	jsonMode bool

	// For log rotation
	filePath    string
	maxSize     int64 // bytes
	currentSize int64
}

// LoggerConfig configures the logger.
type LoggerConfig struct {
	Level    LogLevel
	FilePath string
	JSONMode bool
	MaxSize  int64 // Max file size before rotation (0 = no rotation)
}

// NewLogger creates a new Logger.
func NewLogger(cfg LoggerConfig) (*Logger, error) {
	l := &Logger{
		level:    cfg.Level,
		jsonMode: cfg.JSONMode,
		filePath: cfg.FilePath,
		maxSize:  cfg.MaxSize,
	}

	if cfg.FilePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		// Get current file size
		if info, err := f.Stat(); err == nil {
			l.currentSize = info.Size()
		}

		l.writer = f
		l.filePath = cfg.FilePath
	} else {
		l.writer = os.Stderr
	}

	return l, nil
}

// Close closes the logger.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if f, ok := l.writer.(*os.File); ok && f != os.Stderr && f != os.Stdout {
		return f.Close()
	}
	return nil
}

// logEntry represents a JSON log entry.
type logEntry struct {
	Time    string      `json:"time"`
	Level   string      `json:"level"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (l *Logger) log(level LogLevel, msg string, data interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)

	var line string
	if l.jsonMode {
		entry := logEntry{
			Time:    timestamp,
			Level:   level.String(),
			Message: msg,
			Data:    data,
		}
		b, err := json.Marshal(entry)
		if err != nil {
			// Fall back to simple format if JSON marshal fails
			line = fmt.Sprintf("%s [%s] %s\n", timestamp, level.String(), msg)
		} else {
			line = string(b) + "\n"
		}
	} else {
		if data != nil {
			line = fmt.Sprintf("%s [%s] %s %v\n", timestamp, level.String(), msg, data)
		} else {
			line = fmt.Sprintf("%s [%s] %s\n", timestamp, level.String(), msg)
		}
	}

	// Check if rotation needed
	if l.maxSize > 0 && l.filePath != "" {
		l.currentSize += int64(len(line))
		if l.currentSize > l.maxSize {
			l.rotate()
		}
	}

	if _, err := l.writer.Write([]byte(line)); err != nil {
		// Log write errors are non-fatal, but we can't do much about them
		_ = err
	}
}

func (l *Logger) rotate() {
	// Close current file
	if f, ok := l.writer.(*os.File); ok {
		_ = f.Close()
	}

	// Rename current file with timestamp
	rotatedPath := l.filePath + "." + time.Now().Format("20060102-150405")
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		// Log rotation failure is non-fatal
		_ = err
	}

	// Open new file
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		// Fall back to stderr
		l.writer = os.Stderr
		return
	}

	l.writer = f
	l.currentSize = 0

	// Clean up old rotated files (keep last 5)
	l.cleanupOldLogs()
}

func (l *Logger) cleanupOldLogs() {
	dir := filepath.Dir(l.filePath)
	base := filepath.Base(l.filePath)
	pattern := base + ".*"

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		// Glob failure is non-fatal
		return
	}
	if len(matches) <= 5 {
		return
	}

	// Sort to get oldest first
	sort.Strings(matches)

	// Remove oldest files, keeping last 5
	for i := 0; i < len(matches)-5; i++ {
		_ = os.Remove(matches[i])
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.log(LogLevelDebug, msg, d)
}

// Info logs an info message.
func (l *Logger) Info(msg string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.log(LogLevelInfo, msg, d)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.log(LogLevelWarn, msg, d)
}

// Error logs an error message.
func (l *Logger) Error(msg string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.log(LogLevelError, msg, d)
}

// Println logs an info message (for compatibility with standard log.Logger).
func (l *Logger) Println(v ...interface{}) {
	l.Info(fmt.Sprint(v...))
}

// Printf logs an info message with formatting (for compatibility with standard log.Logger).
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Info(fmt.Sprintf(format, v...))
}

// GetLevel returns the current log level.
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// SetLevel sets the log level.
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}
