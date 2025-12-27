package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Security-related errors.
var (
	// ErrInvalidBinaryPath indicates the binary path is not valid.
	ErrInvalidBinaryPath = errors.New("invalid binary path")
	// ErrInvalidAddress indicates the address is not a valid URL.
	ErrInvalidAddress = errors.New("invalid address")
)

// BinaryType represents the type of Vault-compatible binary.
type BinaryType string

const (
	// BinaryTypeVault represents HashiCorp Vault.
	BinaryTypeVault BinaryType = "vault"
	// BinaryTypeOpenBao represents OpenBao.
	BinaryTypeOpenBao BinaryType = "openbao"
)

// Connection represents a Vault/OpenBao server connection profile.
type Connection struct {
	// Name is the unique identifier for this connection.
	Name string `yaml:"name"`
	// Address is the Vault/OpenBao server URL.
	Address string `yaml:"address"`
	// Type is the binary type (vault or openbao).
	Type BinaryType `yaml:"type,omitempty"`
	// BinaryPath is an optional custom path to the vault/bao binary.
	BinaryPath string `yaml:"binary_path,omitempty"`
	// Namespace is the Vault namespace (Enterprise feature).
	Namespace string `yaml:"namespace,omitempty"`
	// TLSSkipVerify disables TLS certificate verification.
	TLSSkipVerify bool `yaml:"tls_skip_verify,omitempty"`
	// CACert is the path to a CA certificate file.
	CACert string `yaml:"ca_cert,omitempty"`
	// CAPath is the path to a directory of CA certificates.
	CAPath string `yaml:"ca_path,omitempty"`
	// ClientCert is the path to a client certificate file.
	ClientCert string `yaml:"client_cert,omitempty"`
	// ClientKey is the path to a client key file.
	ClientKey string `yaml:"client_key,omitempty"`
}

// DaemonConfig holds settings for the background renewal daemon.
type DaemonConfig struct {
	// AutoStart indicates whether to auto-start the daemon on login.
	AutoStart bool `yaml:"auto_start,omitempty"`
	// CheckInterval is how often to check tokens for renewal.
	CheckInterval time.Duration `yaml:"check_interval,omitempty"`
	// RenewThreshold is the fraction of TTL elapsed before renewal (0.0-1.0).
	RenewThreshold float64 `yaml:"renew_threshold,omitempty"`
	// MinRenewTTL is the minimum remaining TTL before forcing renewal.
	MinRenewTTL time.Duration `yaml:"min_renew_ttl,omitempty"`
	// InitialRetryBackoff is the initial backoff duration for renewal retries.
	InitialRetryBackoff time.Duration `yaml:"initial_retry_backoff,omitempty"`
	// MaxRetryBackoff is the maximum backoff duration for renewal retries.
	MaxRetryBackoff time.Duration `yaml:"max_retry_backoff,omitempty"`
	// LogFile is the path to the daemon log file.
	LogFile string `yaml:"log_file,omitempty"`
	// PIDFile is the path to the daemon PID file.
	PIDFile string `yaml:"pid_file,omitempty"`
	// LogLevel is the logging level (debug, info, warn, error).
	LogLevel string `yaml:"log_level,omitempty"`
	// LogJSON enables JSON-formatted logging.
	LogJSON bool `yaml:"log_json,omitempty"`
	// LogMaxSize is the maximum log file size in MB before rotation.
	LogMaxSize int `yaml:"log_max_size,omitempty"`
	// HealthEndpoint is the address for the health HTTP endpoint (e.g., localhost:9090).
	HealthEndpoint string `yaml:"health_endpoint,omitempty"`
	// Notifications holds notification settings.
	Notifications NotificationConfig `yaml:"notifications,omitempty"`
}

// NotificationConfig holds settings for desktop notifications.
type NotificationConfig struct {
	// Enabled enables desktop notifications.
	Enabled bool `yaml:"enabled,omitempty"`
	// OnRenewal sends notification on successful token renewal.
	OnRenewal bool `yaml:"on_renewal,omitempty"`
	// OnFailure sends notification on renewal failure.
	OnFailure bool `yaml:"on_failure,omitempty"`
}

// Config represents the Patrol configuration.
type Config struct {
	// Current is the name of the currently active connection.
	Current string `yaml:"current,omitempty"`
	// Connections is a list of configured Vault/OpenBao connections.
	Connections []Connection `yaml:"connections,omitempty"`
	// Daemon holds daemon-specific configuration.
	Daemon DaemonConfig `yaml:"daemon,omitempty"`
	// RevokeOnLogout indicates whether to revoke tokens on logout.
	RevokeOnLogout bool `yaml:"revoke_on_logout,omitempty"`

	// filePath is the path where this config was loaded from.
	filePath string `yaml:"-"`
}

// Default returns a new Config with default values.
func Default() *Config {
	paths := GetPaths()
	return &Config{
		Connections: []Connection{},
		Daemon: DaemonConfig{
			AutoStart:           false,
			CheckInterval:       time.Minute,
			RenewThreshold:      0.75,
			MinRenewTTL:         5 * time.Minute,
			InitialRetryBackoff: 30 * time.Second,
			MaxRetryBackoff:     15 * time.Minute,
			LogFile:             "",
			PIDFile:             "",
			Notifications: NotificationConfig{
				Enabled:   false,
				OnRenewal: true,
				OnFailure: true,
			},
		},
		RevokeOnLogout: true,
		filePath:       paths.ConfigFile,
	}
}

// Load loads the configuration from the default path.
func Load() (*Config, error) {
	paths := GetPaths()
	return LoadFrom(paths.ConfigFile)
}

// LoadFrom loads the configuration from a specific path.
func LoadFrom(path string) (*Config, error) {
	cfg := Default()
	cfg.filePath = path

	// #nosec G304 - path is the config file path (controlled, from user config directory)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, return defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing daemon values
	if cfg.Daemon.CheckInterval == 0 {
		cfg.Daemon.CheckInterval = time.Minute
	}
	if cfg.Daemon.RenewThreshold == 0 {
		cfg.Daemon.RenewThreshold = 0.75
	}
	if cfg.Daemon.MinRenewTTL == 0 {
		cfg.Daemon.MinRenewTTL = 5 * time.Minute
	}

	return cfg, nil
}

// Save writes the configuration to its file path.
func (c *Config) Save() error {
	if c.filePath == "" {
		return errors.New("config file path not set")
	}

	// Ensure the directory exists
	paths := GetPaths()
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConnection returns a connection by name.
func (c *Config) GetConnection(name string) (*Connection, error) {
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			return &c.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection %q not found", name)
}

// GetCurrentConnection returns the currently active connection.
func (c *Config) GetCurrentConnection() (*Connection, error) {
	if c.Current == "" {
		return nil, errors.New("no active connection configured")
	}
	return c.GetConnection(c.Current)
}

// AddConnection adds a new connection to the config.
func (c *Config) AddConnection(conn Connection) error {
	// Validate required fields
	if conn.Name == "" {
		return errors.New("connection name is required")
	}
	if conn.Address == "" {
		return errors.New("connection address is required")
	}

	// Check for duplicate names
	for _, existing := range c.Connections {
		if existing.Name == conn.Name {
			return fmt.Errorf("connection %q already exists", conn.Name)
		}
	}

	// Set default type if not specified
	if conn.Type == "" {
		conn.Type = BinaryTypeVault
	}

	c.Connections = append(c.Connections, conn)

	// If this is the first connection, make it active
	if len(c.Connections) == 1 {
		c.Current = conn.Name
	}

	return nil
}

// RemoveConnection removes a connection by name.
func (c *Config) RemoveConnection(name string) error {
	for i, conn := range c.Connections {
		if conn.Name == name {
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			// If we removed the current connection, clear it
			if c.Current == name {
				if len(c.Connections) > 0 {
					c.Current = c.Connections[0].Name
				} else {
					c.Current = ""
				}
			}
			return nil
		}
	}
	return fmt.Errorf("connection %q not found", name)
}

// SetCurrent sets the active connection.
func (c *Config) SetCurrent(name string) error {
	// Verify the connection exists
	if _, err := c.GetConnection(name); err != nil {
		return err
	}
	c.Current = name
	return nil
}

// GetBinaryPath returns the binary path for a connection.
func (conn *Connection) GetBinaryPath() string {
	if conn.BinaryPath != "" {
		return conn.BinaryPath
	}
	switch conn.Type {
	case BinaryTypeOpenBao:
		return "bao"
	default:
		return "vault"
	}
}

// KeyringKey returns the profile name for keyring storage.
// The profile name is used as the unique identifier for tokens.
func (conn *Connection) KeyringKey() string {
	return conn.Name
}

// ValidateBinaryPath validates that the binary path is safe to execute.
// This prevents command injection attacks via malicious config files.
// Returns nil if the binary path is safe, or an error describing the issue.
func (conn *Connection) ValidateBinaryPath() error {
	binaryPath := conn.BinaryPath

	// If no custom path, the default is always safe (will be looked up in PATH)
	if binaryPath == "" {
		return nil
	}

	// Get the base name
	baseName := filepath.Base(binaryPath)

	// Check if it's just a binary name (no path separators)
	// If so, it will be looked up in PATH - this is safe
	if binaryPath == baseName {
		return nil
	}

	// Custom path provided - apply strict validation:
	// 1. Must be an absolute path
	if !filepath.IsAbs(binaryPath) {
		return fmt.Errorf("%w: custom binary path must be absolute, got %q", ErrInvalidBinaryPath, binaryPath)
	}

	// 2. Path must not contain suspicious patterns (path traversal, etc.)
	// Check for ".." explicitly to prevent path traversal attacks
	if strings.Contains(binaryPath, "..") {
		return fmt.Errorf("%w: binary path contains suspicious components (path traversal)", ErrInvalidBinaryPath)
	}
	// Also check that cleaned path matches original (catches other suspicious patterns)
	cleanPath := filepath.Clean(binaryPath)
	if cleanPath != binaryPath {
		return fmt.Errorf("%w: binary path contains suspicious components", ErrInvalidBinaryPath)
	}

	// 4. Check for symlinks first (security: use Lstat to not follow symlinks)
	info, err := os.Lstat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: binary not found at %q", ErrInvalidBinaryPath, binaryPath)
		}
		return fmt.Errorf("%w: cannot access binary at %q: %v", ErrInvalidBinaryPath, binaryPath, err)
	}

	// 5. Reject symlinks (security: prevent symlink attacks)
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q is a symlink; symlinks are not allowed for security reasons", ErrInvalidBinaryPath, binaryPath)
	}

	// 6. Must be a regular file (not a directory, device, etc.)
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q is not a regular file", ErrInvalidBinaryPath, binaryPath)
	}

	// 7. Must be executable (check at least one execute bit)
	// Skip on Windows where execute bits don't apply - Windows uses file extensions and associations
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("%w: %q is not executable", ErrInvalidBinaryPath, binaryPath)
		}
	}

	return nil
}

// ValidateAddress validates that the connection address is a valid HTTP/HTTPS URL.
func (conn *Connection) ValidateAddress() error {
	if conn.Address == "" {
		return fmt.Errorf("%w: address is required", ErrInvalidAddress)
	}

	parsed, err := url.Parse(conn.Address)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidAddress, err)
	}

	// Must be HTTP or HTTPS
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: address must use http or https scheme, got %q", ErrInvalidAddress, parsed.Scheme)
	}

	// Must have a host
	if parsed.Host == "" {
		return fmt.Errorf("%w: address must have a host", ErrInvalidAddress)
	}

	return nil
}

// Validate performs security validation on the connection.
// This should be called before using a connection for any operations.
func (conn *Connection) Validate() error {
	if err := conn.ValidateAddress(); err != nil {
		return err
	}
	if err := conn.ValidateBinaryPath(); err != nil {
		return err
	}
	return nil
}

// FilePath returns the path where this config was loaded from.
func (c *Config) FilePath() string {
	return c.filePath
}
