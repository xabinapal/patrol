package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/daemon"
)

// DaemonStatusOutput represents daemon status for JSON output.
type DaemonStatusOutput struct {
	Running       bool               `json:"running"`
	PID           int                `json:"pid,omitempty"`
	Configuration DaemonConfigOutput `json:"configuration"`
	Service       *ServiceInfoOutput `json:"service,omitempty"`
}

// DaemonConfigOutput represents daemon configuration for JSON output.
type DaemonConfigOutput struct {
	CheckInterval  string  `json:"check_interval"`
	RenewThreshold float64 `json:"renew_threshold"`
	MinRenewTTL    string  `json:"min_renew_ttl"`
}

// ServiceInfoOutput represents service installation status for JSON output.
type ServiceInfoOutput struct {
	Platform    string `json:"platform"`
	ServiceFile string `json:"service_file"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	PID         int    `json:"pid,omitempty"`
}

// newDaemonCmd creates the daemon command group.
func (cli *CLI) newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the token renewal background service",
		Long: `Manage the Patrol background service that automatically renews
Vault tokens before they expire.

The daemon runs in the background and periodically checks all stored tokens.
When a token is approaching expiration, the daemon will attempt to renew it
automatically, keeping you logged in without manual intervention.

You can run the daemon in two ways:
  1. Run directly in foreground: Use 'daemon run' to run it manually
  2. As a system service: Use 'daemon service install' to install it as a system service
     (launchd on macOS, systemd on Linux, Task Scheduler on Windows)

Examples:
  # Run the daemon in the foreground (for testing or manual execution)
  patrol daemon run

  # Install as a system service (starts automatically on login)
  patrol daemon service install

  # Check daemon status
  patrol daemon status

  # Restart the system service
  patrol daemon service restart

  # Uninstall the system service
  patrol daemon service uninstall`,
	}

	cmd.AddCommand(
		cli.newDaemonRunCmd(),
		cli.newDaemonStatusCmd(),
		cli.newDaemonServiceCmd(),
	)

	return cmd
}

// newDaemonRunCmd creates the daemon run command.
func (cli *CLI) newDaemonRunCmd() *cobra.Command {
	var (
		logFile    string
		logLevel   string
		logJSON    bool
		healthAddr string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the daemon in the foreground",
		Long: `Run the token renewal daemon in the foreground.

This is useful for testing or when running under a process manager
like systemd or launchd that handles daemonization.

Examples:
  # Run with debug logging
  patrol daemon run --log-level=debug

  # Run with JSON logging
  patrol daemon run --log-json

  # Run with health endpoint
  patrol daemon run --health-addr=localhost:9090`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse log level
			level, err := daemon.ParseLogLevel(logLevel)
			if err != nil {
				return err
			}

			// Get max log size from config (in MB, convert to bytes)
			maxSize := int64(cli.Config.Daemon.LogMaxSize) * 1024 * 1024

			// Set up logging with new logger
			loggerCfg := daemon.LoggerConfig{
				Level:    level,
				FilePath: logFile,
				JSONMode: logJSON,
				MaxSize:  maxSize,
			}

			logger, err := daemon.NewLogger(loggerCfg)
			if err != nil {
				return fmt.Errorf("failed to create logger: %w", err)
			}

			// Create daemon
			d := daemon.New(cli.Config, cli.Store)
			d.SetLogger(logger)

			// Set up health server if configured
			if healthAddr != "" {
				healthServer := daemon.NewHealthServer(healthAddr)
				d.SetHealthServer(healthServer)
				fmt.Printf("Health endpoint will be available at http://%s/health\n", healthAddr)
			}

			return d.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&logFile, "log", "", "Log file path (default: stderr)")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().BoolVar(&logJSON, "log-json", false, "Output logs as JSON")
	cmd.Flags().StringVar(&healthAddr, "health-addr", "", "Health endpoint address (e.g., localhost:9090)")

	return cmd
}

// newDaemonStatusCmd creates the daemon status command.
func (cli *CLI) newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the daemon process is running",
		Long: `Check if the daemon process is currently running.

This command checks for a running daemon process by looking for the PID file.
It does not check the system service status - use 'patrol daemon service status'
for that information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			running := daemon.IsRunningFromPID(cli.Config)
			pid := 0
			if running {
				var pidErr error
				pid, pidErr = daemon.GetPID(cli.Config)
				if pidErr != nil {
					// PID file might be stale, but daemon is running
					pid = 0
				}
			}

			statusOutput := DaemonStatusOutput{
				Running: running,
				PID:     pid,
				Configuration: DaemonConfigOutput{
					CheckInterval:  cli.Config.Daemon.CheckInterval.String(),
					RenewThreshold: cli.Config.Daemon.RenewThreshold,
					MinRenewTTL:    cli.Config.Daemon.MinRenewTTL.String(),
				},
			}

			output := NewOutputWriter(format)
			return output.Write(statusOutput, func() {
				if running {
					fmt.Printf("Daemon is running (PID: %d)\n", pid)
				} else {
					fmt.Println("Daemon is not running")
				}

				fmt.Printf("\nDaemon configuration:\n")
				fmt.Printf("  Check interval:     %s\n", cli.Config.Daemon.CheckInterval)
				fmt.Printf("  Renewal threshold:  %.0f%%\n", cli.Config.Daemon.RenewThreshold*100)
				fmt.Printf("  Min TTL for renewal: %s\n", cli.Config.Daemon.MinRenewTTL)
			})
		},
	}
}

// newDaemonServiceCmd creates the daemon service command group.
func (cli *CLI) newDaemonServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the Patrol system service",
		Long: `Manage the Patrol system service installation and lifecycle.

The service runs 'patrol daemon run' in the background, automatically
renewing your Vault tokens before they expire.

Supported platforms:
  - macOS: launchd user agent
  - Linux: systemd user service
  - Windows: Scheduled Task`,
	}

	cmd.AddCommand(
		cli.newDaemonServiceInstallCmd(),
		cli.newDaemonServiceUninstallCmd(),
		cli.newDaemonServiceRestartCmd(),
		cli.newDaemonServiceStatusCmd(),
	)

	return cmd
}

// newDaemonServiceInstallCmd creates the daemon service install command.
func (cli *CLI) newDaemonServiceInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Patrol as a user service",
		Long: `Install Patrol as a user-level service that starts automatically on login.

The service will run 'patrol daemon run' in the background, automatically
renewing your Vault tokens before they expire.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			// Check if already installed
			installed, installErr := mgr.IsInstalled()
			if installErr == nil && installed {
				fmt.Println("Service is already installed.")
				fmt.Printf("Service file: %s\n", mgr.ServiceFilePath())
				return nil
			}

			fmt.Printf("Installing Patrol service (%s)...\n", daemon.ServicePlatformName())

			if err := mgr.Install(); err != nil {
				return fmt.Errorf("failed to install service: %w", err)
			}

			fmt.Println("Service installed and started successfully!")
			fmt.Printf("  Service file: %s\n", mgr.ServiceFilePath())
			fmt.Println()
			fmt.Println("The daemon will now start automatically when you log in.")
			fmt.Println("Use 'patrol daemon status' to check the service status.")

			return nil
		},
	}
}

// newDaemonServiceUninstallCmd creates the daemon service uninstall command.
func (cli *CLI) newDaemonServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the Patrol service",
		Long: `Uninstall the Patrol system service.

This stops and removes the service installation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			installed, installErr := mgr.IsInstalled()
			if installErr == nil && !installed {
				fmt.Println("Service is not installed.")
				return nil
			}

			fmt.Printf("Uninstalling Patrol service (%s)...\n", daemon.ServicePlatformName())

			if err := mgr.Uninstall(); err != nil {
				return fmt.Errorf("failed to uninstall service: %w", err)
			}

			fmt.Println("Service uninstalled successfully!")
			return nil
		},
	}
}

// newDaemonServiceRestartCmd creates the daemon service restart command.
func (cli *CLI) newDaemonServiceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the installed system service",
		Long: `Restart the installed system service.

This command only works if the service has been installed with
'patrol daemon service install'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			installed, installErr := mgr.IsInstalled()
			if installErr != nil || !installed {
				return fmt.Errorf("service is not installed; run 'patrol daemon service install' first")
			}

			if err := mgr.Restart(); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}

			fmt.Println("Service restarted")
			return nil
		},
	}
}

// newDaemonServiceStatusCmd creates the daemon service status command.
func (cli *CLI) newDaemonServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the system service status",
		Long: `Check the status of the Patrol system service.

This command shows whether the service is installed and running.
It does not check the daemon process status - use 'patrol daemon status'
for that information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := ParseOutputFormat(cli.outputFlag)
			if err != nil {
				return err
			}

			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			status, err := mgr.Status()
			if err != nil {
				return fmt.Errorf("failed to get service status: %w", err)
			}

			serviceInfo := &ServiceInfoOutput{
				Platform:    daemon.ServicePlatformName(),
				ServiceFile: mgr.ServiceFilePath(),
				Installed:   status.Installed,
				Running:     status.Running,
				PID:         status.PID,
			}

			statusOutput := DaemonStatusOutput{
				Service: serviceInfo,
			}

			output := NewOutputWriter(format)
			return output.Write(statusOutput, func() {
				fmt.Printf("System service (%s):\n", serviceInfo.Platform)
				if serviceInfo.Installed {
					fmt.Printf("  Installed: yes\n")
					fmt.Printf("  Service file: %s\n", serviceInfo.ServiceFile)
					if serviceInfo.Running {
						fmt.Printf("  Running: yes (PID: %d)\n", serviceInfo.PID)
					} else {
						fmt.Printf("  Running: no\n")
					}
				} else {
					fmt.Printf("  Installed: no\n")
					fmt.Printf("  Run 'patrol daemon service install' to install as a system service\n")
				}
			})
		},
	}
}

// getServiceManager creates a service manager instance.
func (cli *CLI) getServiceManager() (daemon.ServiceManager, error) {
	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Get log path
	paths := config.GetPaths()
	logPath := filepath.Join(paths.CacheDir, "daemon.log")

	cfg := daemon.ServiceConfig{
		ExecutablePath: execPath,
		LogPath:        logPath,
	}

	return daemon.NewServiceManager(cfg)
}
