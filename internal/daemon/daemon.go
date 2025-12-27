// Package daemon provides the background token renewal service.
package daemon

import (
	"bytes"
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
	"github.com/xabinapal/patrol/internal/keyring"
	"github.com/xabinapal/patrol/internal/notify"
	"github.com/xabinapal/patrol/internal/proxy"
	"github.com/xabinapal/patrol/internal/token"
)

// connectionBackoff tracks the backoff state for a connection.
type connectionBackoff struct {
	failureCount int
	nextRetry    time.Time
}

// Daemon manages automatic token renewal.
type Daemon struct {
	config       *config.Config
	keyring      keyring.Store
	logger       *Logger
	healthServer *HealthServer
	notifier     notify.Notifier

	mu           sync.Mutex
	running      bool
	stopChan     chan struct{}
	backoffState map[string]*connectionBackoff // keyed by connection name
}

// New creates a new Daemon instance.
func New(cfg *config.Config, kr keyring.Store) *Daemon {
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

	return &Daemon{
		config:       cfg,
		keyring:      kr,
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

	// Check keyring availability
	if err := d.keyring.IsAvailable(); err != nil {
		return fmt.Errorf("keyring not available: %w", err)
	}

	d.logger.Info("Starting token renewal daemon")
	d.logger.Info(fmt.Sprintf("Check interval: %s", d.config.Daemon.CheckInterval))
	d.logger.Info(fmt.Sprintf("Renewal threshold: %.0f%%", d.config.Daemon.RenewThreshold*100))
	d.logger.Info(fmt.Sprintf("Minimum TTL for renewal: %s", d.config.Daemon.MinRenewTTL))

	// Write PID file for daemon tracking
	if err := d.writePIDFile(); err != nil {
		d.logger.Warn(fmt.Sprintf("failed to write PID file: %v", err))
	} else {
		defer d.removePIDFile()
	}

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

// checkAndRenewTokens checks all stored tokens and renews those that need it.
func (d *Daemon) checkAndRenewTokens(ctx context.Context) {
	tokensManaged := 0

	for _, conn := range d.config.Connections {
		// Get the stored token
		tokenStr, err := d.keyring.Get(conn.KeyringKey())
		if err != nil {
			if errors.Is(err, keyring.ErrTokenNotFound) {
				continue // No token for this profile
			}
			d.logger.Error(fmt.Sprintf("Error getting token for %s: %v", conn.Name, err))
			if d.healthServer != nil {
				d.healthServer.RecordError()
			}
			continue
		}

		tokensManaged++

		// Check if we can execute vault commands
		if !proxy.BinaryExists(&conn) {
			d.logger.Debug(fmt.Sprintf("Binary not found for %s, skipping renewal check", conn.Name))
			continue
		}

		// Look up token to get current TTL
		lookupData, err := d.lookupToken(ctx, &conn, tokenStr)
		if err != nil {
			d.logger.Error(fmt.Sprintf("Error looking up token for %s: %v", conn.Name, err))
			if d.healthServer != nil {
				d.healthServer.RecordError()
			}
			continue
		}

		d.logger.Debug(fmt.Sprintf("Token for %s: TTL=%ds, Renewable=%v", conn.Name, lookupData.TTL, lookupData.Renewable))

		// Check if token needs renewal
		if d.needsRenewal(lookupData) {
			if !lookupData.Renewable {
				d.logger.Warn(fmt.Sprintf("Token for %s needs renewal but is not renewable (TTL: %ds)",
					conn.Name, lookupData.TTL))
				continue
			}

			// Check if we should skip due to backoff from previous failures
			if d.shouldSkipDueToBackoff(conn.Name) {
				backoff := d.getBackoff(conn.Name)
				d.logger.Debug(fmt.Sprintf("Skipping renewal for %s due to backoff (retry in %s)",
					conn.Name, time.Until(backoff.nextRetry).Round(time.Second)))
				continue
			}

			d.logger.Info(fmt.Sprintf("Renewing token for %s (current TTL: %ds)", conn.Name, lookupData.TTL))

			if err := d.renewToken(ctx, &conn, tokenStr); err != nil {
				d.logger.Error(fmt.Sprintf("Error renewing token for %s: %v", conn.Name, err))
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
			newTTL := time.Duration(lookupData.CreationTTL) * time.Second // Use creation TTL as estimate
			if newLookup, err := d.lookupToken(ctx, &conn, tokenStr); err == nil {
				newTTL = time.Duration(newLookup.TTL) * time.Second
			}

			d.logger.Info(fmt.Sprintf("Successfully renewed token for %s", conn.Name))
			if d.healthServer != nil {
				d.healthServer.RecordRenewal()
			}

			// Send success notification
			if notifyErr := d.notifier.NotifyRenewal(conn.Name, newTTL); notifyErr != nil {
				d.logger.Debug(fmt.Sprintf("Failed to send notification: %v", notifyErr))
			}
		}
	}

	// Record the check with the health server
	if d.healthServer != nil {
		d.healthServer.RecordCheck(tokensManaged)
	}
}

// lookupToken queries Vault for token information.
func (d *Daemon) lookupToken(ctx context.Context, conn *config.Connection, tokenStr string) (*token.VaultTokenLookupData, error) {
	// Silent - we only need to parse JSON, not show output
	exec := proxy.NewExecutor(conn, proxy.WithToken(tokenStr))

	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"token", "lookup", "-format=json"}, &captureBuf)
	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("token lookup failed: %s", captureBuf.String())
	}

	return token.ParseLookupResponse(captureBuf.Bytes())
}

// needsRenewal checks if a token needs to be renewed based on configuration.
func (d *Daemon) needsRenewal(data *token.VaultTokenLookupData) bool {
	if data.TTL <= 0 {
		return false // Token doesn't expire or is already expired
	}

	// Check minimum TTL threshold
	minTTL := int(d.config.Daemon.MinRenewTTL.Seconds())
	if data.TTL < minTTL {
		return true
	}

	// Check percentage threshold
	if data.CreationTTL > 0 {
		elapsed := data.CreationTTL - data.TTL
		elapsedRatio := float64(elapsed) / float64(data.CreationTTL)
		if elapsedRatio >= d.config.Daemon.RenewThreshold {
			return true
		}
	}

	return false
}

// renewToken attempts to renew a token.
func (d *Daemon) renewToken(ctx context.Context, conn *config.Connection, tokenStr string) error {
	// Silent - we only need to parse JSON, not show output
	exec := proxy.NewExecutor(conn, proxy.WithToken(tokenStr))

	var captureBuf bytes.Buffer
	exitCode, err := exec.Execute(ctx, []string{"token", "renew", "-format=json"}, &captureBuf)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return fmt.Errorf("token renewal failed: %s", captureBuf.String())
	}

	// Parse the response to get the new token (in case it changed)
	tok, err := token.ParseLoginResponse(captureBuf.Bytes())
	if err != nil {
		// Token might not have changed, that's OK
		d.logger.Debug(fmt.Sprintf("Could not parse renewal response for %s: %v", conn.Name, err))
		return nil
	}

	// If the token changed, update it in the keyring
	if tok.ClientToken != "" && tok.ClientToken != tokenStr {
		if err := d.keyring.Set(conn.KeyringKey(), tok.ClientToken); err != nil {
			return fmt.Errorf("failed to update renewed token: %w", err)
		}
		d.logger.Debug(fmt.Sprintf("Token for %s was updated during renewal", conn.Name))
	}

	return nil
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

	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0600)
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
