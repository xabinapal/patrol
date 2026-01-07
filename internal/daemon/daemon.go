// Package daemon provides the background token renewal service.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/notify"
	"github.com/xabinapal/patrol/internal/token"
	"github.com/xabinapal/patrol/internal/tokenstore"
	"github.com/xabinapal/patrol/internal/types"
	"github.com/xabinapal/patrol/internal/vault"
)

// connectionBackoff tracks the backoff state for a connection.
type connectionBackoff struct {
	failureCount int
	nextRetry    time.Time
}

// Daemon manages automatic token renewal.
type Daemon struct {
	config       *config.Config
	configPath   string // Path to the config file for reloading
	store        tokenstore.TokenStore
	logger       *Logger
	healthServer *HealthServer
	notifier     notify.Notifier

	mu           sync.Mutex
	running      bool
	stopChan     chan struct{}
	backoffState map[string]*connectionBackoff // keyed by connection name
}

// New creates a new Daemon instance.
func New(cfg *config.Config, ts tokenstore.TokenStore) *Daemon {
	// Create default logger
	logger, err := NewLogger(LoggerConfig{
		Level:    LogLevelInfo,
		JSONMode: false,
	})
	if err != nil {
		// Fall back to stderr logger if file logger fails
		logger = &Logger{writer: os.Stderr}
	}

	// Create notifier based on config
	notifier := notify.New(cfg.Daemon.Notifications)

	// Get config file path (use default if not set in config)
	configPath := config.GetPaths().ConfigFile

	return &Daemon{
		config:       cfg,
		configPath:   configPath,
		store:        ts,
		logger:       logger,
		notifier:     notifier,
		backoffState: make(map[string]*connectionBackoff),
	}
}

// SetLogger sets a custom logger for the daemon.
func (d *Daemon) SetLogger(logger *Logger) {
	d.logger = logger
}

// SetHealthServer sets a health server for the daemon.
func (d *Daemon) SetHealthServer(server *HealthServer) {
	d.healthServer = server
}

// Run starts the daemon and blocks until it's stopped.
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return errors.New("daemon is already running")
	}
	d.running = true
	d.stopChan = make(chan struct{})
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	// Check if another instance is already running
	if IsRunningFromPID(d.config) {
		return fmt.Errorf("daemon is already running (another instance detected)")
	}

	// Check keyring availability
	if err := d.store.IsAvailable(); err != nil {
		return fmt.Errorf("keyring not available: %w", err)
	}

	d.logger.Info("Starting token renewal daemon")
	d.logger.Info(fmt.Sprintf("Check interval: %s", d.config.Daemon.CheckInterval))
	d.logger.Info(fmt.Sprintf("Renewal threshold: %.0f%%", d.config.Daemon.RenewThreshold*100))
	d.logger.Info(fmt.Sprintf("Minimum TTL for renewal: %s", d.config.Daemon.MinRenewTTL))

	// Write PID file for daemon tracking (with file locking to prevent race conditions)
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("failed to acquire daemon lock (another instance may be starting): %w", err)
	}
	defer d.removePIDFile()

	// Start health server if configured
	if d.healthServer != nil {
		if err := d.healthServer.Start(); err != nil {
			d.logger.Warn(fmt.Sprintf("failed to start health server: %v", err))
		} else {
			d.logger.Info("Health server started")
			defer func() {
				if err := d.healthServer.Stop(); err != nil {
					d.logger.Warn(fmt.Sprintf("failed to stop health server: %v", err))
				}
			}()
		}
	}

	// Close logger on exit
	defer func() {
		if err := d.logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close logger: %v\n", err)
		}
	}()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Create a ticker for periodic checks
	ticker := time.NewTicker(d.config.Daemon.CheckInterval)
	defer ticker.Stop()

	// Do an initial check
	d.checkAndRenewTokens(ctx)

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Context canceled, shutting down")
			return ctx.Err()
		case <-d.stopChan:
			d.logger.Info("Stop signal received, shutting down")
			return nil
		case sig := <-sigChan:
			d.logger.Info(fmt.Sprintf("Received signal %v, shutting down", sig))
			return nil
		case <-ticker.C:
			d.logger.Info("Starting token renewal check")
			d.checkAndRenewTokens(ctx)
		}
	}
}

// Stop signals the daemon to stop.
func (d *Daemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running && d.stopChan != nil {
		close(d.stopChan)
	}
}

// IsRunning returns whether the daemon is running.
func (d *Daemon) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

// reloadConfig reloads the configuration from disk.
// Returns the loaded config or the previous config if reload fails.
func (d *Daemon) reloadConfig() *config.Config {
	newCfg, err := config.LoadFrom(d.configPath)
	if err != nil {
		d.logger.Warn(fmt.Sprintf("Failed to reload config: %v, using previous config", err))
		return d.config
	}

	// Update notifier if notification settings changed
	if newCfg.Daemon.Notifications != d.config.Daemon.Notifications {
		d.notifier = notify.New(newCfg.Daemon.Notifications)
		d.logger.Debug("Notification settings updated")
	}

	// Log if profiles were added or removed
	oldProfileNames := make(map[string]bool, len(d.config.Connections))
	for _, conn := range d.config.Connections {
		oldProfileNames[conn.Name] = true
	}

	for _, conn := range newCfg.Connections {
		if !oldProfileNames[conn.Name] {
			d.logger.Info(fmt.Sprintf("Detected new profile: %s", conn.Name))
		}
		delete(oldProfileNames, conn.Name) // Remove found profiles
	}

	// Remaining profiles in oldProfileNames were removed
	for name := range oldProfileNames {
		d.logger.Info(fmt.Sprintf("Profile removed from config: %s", name))
	}

	d.config = newCfg
	return newCfg
}

// checkAndRenewTokens checks all stored tokens and renews those that need it.
func (d *Daemon) checkAndRenewTokens(ctx context.Context) {
	// Reload config to detect new profiles
	cfg := d.reloadConfig()

	tokensChecked := 0
	tokensRenewed := 0
	tokensSkipped := 0
	profilesWithoutTokens := 0

	// Create TokenManager for token operations
	tm := token.NewTokenManager(ctx, d.store, vault.NewTokenExecutor())

	for _, conn := range cfg.Connections {
		prof := types.FromConnection(&conn)

		// Check if token exists for this profile
		if !tm.HasToken(prof) {
			d.logger.Debug(fmt.Sprintf("Profile %s: no token stored, skipping", conn.Name))
			profilesWithoutTokens++
			continue
		}

		tokensChecked++

		// Look up token to get current TTL
		tok, err := tm.Lookup(prof)
		if err != nil {
			d.logger.Error(fmt.Sprintf("Profile %s: error looking up token: %v", conn.Name, err))
			if d.healthServer != nil {
				d.healthServer.RecordError()
			}
			continue
		}

		// Check if token needs renewal
		minTTL := cfg.Daemon.MinRenewTTL
		threshold := cfg.Daemon.RenewThreshold
		needsRenewal := tok.NeedsRenewal(threshold, minTTL)

		// Log token status
		ttlDuration := time.Duration(tok.LeaseDuration) * time.Second
		if !needsRenewal {
			d.logger.Info(fmt.Sprintf("Profile %s: token OK (TTL: %s, renewable: %v)", conn.Name, ttlDuration, tok.Renewable))
			tokensSkipped++
			continue
		}

		// Token needs renewal
		if !tok.Renewable {
			d.logger.Warn(fmt.Sprintf("Profile %s: token needs renewal but is not renewable (TTL: %s)",
				conn.Name, ttlDuration))
			tokensSkipped++
			continue
		}

		// Check if we should skip due to backoff from previous failures
		if d.shouldSkipDueToBackoff(conn.Name) {
			backoff := d.getBackoff(conn.Name)
			retryIn := time.Until(backoff.nextRetry).Round(time.Second)
			d.logger.Info(fmt.Sprintf("Profile %s: skipping renewal due to backoff (retry in %s, TTL: %s)",
				conn.Name, retryIn, ttlDuration))
			tokensSkipped++
			continue
		}

		d.logger.Info(fmt.Sprintf("Profile %s: renewing token (current TTL: %s)", conn.Name, ttlDuration))

		_, err = tm.Renew(prof, "")
		if err != nil {
			d.logger.Error(fmt.Sprintf("Profile %s: renewal failed: %v", conn.Name, err))
			d.recordRenewalFailure(conn.Name)
			if d.healthServer != nil {
				d.healthServer.RecordError()
			}
			// Send failure notification
			if notifyErr := d.notifier.NotifyFailure(conn.Name, err); notifyErr != nil {
				d.logger.Debug(fmt.Sprintf("Failed to send notification: %v", notifyErr))
			}
			continue
		}

		// Success - reset backoff state
		d.resetBackoff(conn.Name)

		// Get new TTL for notification
		newTTL := ttlDuration // Use current TTL as estimate
		if newTok, err := tm.Lookup(prof); err == nil {
			newTTL = time.Duration(newTok.LeaseDuration) * time.Second
		}

		d.logger.Info(fmt.Sprintf("Profile %s: token renewed successfully (new TTL: %s)", conn.Name, newTTL))
		tokensRenewed++
		if d.healthServer != nil {
			d.healthServer.RecordRenewal()
		}

		// Send success notification
		if notifyErr := d.notifier.NotifyRenewal(conn.Name, newTTL); notifyErr != nil {
			d.logger.Debug(fmt.Sprintf("Failed to send notification: %v", notifyErr))
		}
	}

	// Log summary of the check
	d.logger.Info(fmt.Sprintf("Check complete: %d tokens checked, %d renewed, %d skipped, %d profiles without tokens",
		tokensChecked, tokensRenewed, tokensSkipped, profilesWithoutTokens))

	// Record the check with the health server
	if d.healthServer != nil {
		d.healthServer.RecordCheck(tokensChecked)
	}
}

// getBackoff returns the backoff state for a connection, creating it if needed.
func (d *Daemon) getBackoff(connName string) *connectionBackoff {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.backoffState[connName] == nil {
		d.backoffState[connName] = &connectionBackoff{}
	}
	return d.backoffState[connName]
}

// shouldSkipDueToBackoff checks if renewal should be skipped due to backoff.
func (d *Daemon) shouldSkipDueToBackoff(connName string) bool {
	backoff := d.getBackoff(connName)
	if backoff.failureCount == 0 {
		return false
	}
	return time.Now().Before(backoff.nextRetry)
}

// recordRenewalFailure records a renewal failure and calculates next retry time.
func (d *Daemon) recordRenewalFailure(connName string) {
	backoff := d.getBackoff(connName)
	backoff.failureCount++

	// Calculate backoff duration: initialBackoff * 2^(failureCount-1)
	initialBackoff := d.config.Daemon.InitialRetryBackoff
	if initialBackoff == 0 {
		initialBackoff = 30 * time.Second
	}
	maxBackoff := d.config.Daemon.MaxRetryBackoff
	if maxBackoff == 0 {
		maxBackoff = 15 * time.Minute
	}

	// Calculate exponential backoff
	multiplier := 1 << (backoff.failureCount - 1) // 2^(n-1)
	backoffDuration := time.Duration(multiplier) * initialBackoff
	if backoffDuration > maxBackoff {
		backoffDuration = maxBackoff
	}

	backoff.nextRetry = time.Now().Add(backoffDuration)
	d.logger.Warn(fmt.Sprintf("Renewal failed for %s (attempt %d), will retry in %s",
		connName, backoff.failureCount, backoffDuration))
}

// resetBackoff resets the backoff state for a connection after successful renewal.
func (d *Daemon) resetBackoff(connName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.backoffState, connName)
}

// writePIDFile writes the current process ID to the configured PID file.
// It uses exclusive file creation to prevent multiple instances from starting simultaneously.
func (d *Daemon) writePIDFile() error {
	pidFile := d.config.Daemon.PIDFile
	if pidFile == "" {
		paths := config.GetPaths()
		pidFile = filepath.Join(paths.DataDir, "patrol.pid")
	}

	// Ensure directory exists
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Retry logic: try up to 3 times to handle race conditions
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Try to create the PID file exclusively (fails if it already exists)
		// #nosec G304 - pidFile is from config paths (controlled)
		file, err := os.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			if !os.IsExist(err) {
				return fmt.Errorf("failed to create PID file: %w", err)
			}

			// PID file exists - check if the process is still running
			existingPID, readErr := GetPID(d.config)
			if readErr != nil {
				// PID file exists but can't read it - remove and retry
				_ = os.Remove(pidFile)
				continue
			}

			// Check if the existing process is still running
			if IsRunningFromPID(d.config) {
				return fmt.Errorf("daemon is already running (PID: %d)", existingPID)
			}

			// Process is not running - remove stale PID file and retry
			_ = os.Remove(pidFile)
			continue
		}

		// Successfully created the file - write PID
		defer file.Close()

		pid := os.Getpid()
		if _, err := file.WriteString(strconv.Itoa(pid)); err != nil {
			_ = os.Remove(pidFile)
			return fmt.Errorf("failed to write PID: %w", err)
		}

		// Sync to disk to ensure the file is written
		if err := file.Sync(); err != nil {
			_ = os.Remove(pidFile)
			return fmt.Errorf("failed to sync PID file: %w", err)
		}

		return nil
	}

	return fmt.Errorf("failed to acquire daemon lock after %d attempts", maxRetries)
}

// removePIDFile removes the PID file.
func (d *Daemon) removePIDFile() {
	pidFile := d.config.Daemon.PIDFile
	if pidFile == "" {
		paths := config.GetPaths()
		pidFile = filepath.Join(paths.DataDir, "patrol.pid")
	}
	_ = os.Remove(pidFile)
}

// GetPID reads the PID from the PID file, if it exists.
func GetPID(cfg *config.Config) (int, error) {
	pidFile := cfg.Daemon.PIDFile
	if pidFile == "" {
		paths := config.GetPaths()
		pidFile = filepath.Join(paths.DataDir, "patrol.pid")
	}

	// #nosec G304 - pidFile is from config paths (controlled)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(data))
}

// IsRunningFromPID checks if a daemon is running based on the PID file.
func IsRunningFromPID(cfg *config.Config) bool {
	pid, err := GetPID(cfg)
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
