package cli

import (
	"fmt"
	"os"
	"os/exec"
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
  1. As a simple background process: Use 'daemon start' to run it manually
  2. As a system service: Use 'daemon install' to install it as a system service
     (launchd on macOS, systemd on Linux, Task Scheduler on Windows)

Examples:
  # Run the daemon in the foreground (for testing)
  patrol daemon run

  # Start the daemon as a background process
  patrol daemon start

  # Install as a system service (starts automatically on login)
  patrol daemon install

  # Check daemon status
  patrol daemon status

  # Stop the daemon
  patrol daemon stop

  # Uninstall the system service
  patrol daemon uninstall`,
	}

	cmd.AddCommand(
		cli.newDaemonRunCmd(),
		cli.newDaemonStartCmd(),
		cli.newDaemonStopCmd(),
		cli.newDaemonStatusCmd(),
		cli.newDaemonInstallCmd(),
		cli.newDaemonUninstallCmd(),
		cli.newDaemonServiceStartCmd(),
		cli.newDaemonServiceStopCmd(),
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
			d := daemon.New(cli.Config, cli.Keyring)
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

// newDaemonStartCmd creates the daemon start command.
func (cli *CLI) newDaemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the daemon in the background",
		Long: `Start the token renewal daemon as a background process.

The daemon will detach from the terminal and run independently.
Use 'patrol daemon stop' to stop it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already running
			if daemon.IsRunningFromPID(cli.Config) {
				return fmt.Errorf("daemon is already running")
			}

			// Get the executable path
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// Set up log file
			paths := config.GetPaths()
			logFile := filepath.Join(paths.CacheDir, "daemon.log")

			// Ensure cache directory exists
			if err := os.MkdirAll(paths.CacheDir, 0700); err != nil {
				return fmt.Errorf("failed to create cache directory: %w", err)
			}

			// Start the daemon process
			// #nosec G204 - executable is from os.Executable() (trusted), args are static strings, logFile is from config paths
			daemonCmd := exec.Command(executable, "daemon", "run", "--log", logFile)
			daemonCmd.Stdout = nil
			daemonCmd.Stderr = nil
			daemonCmd.Stdin = nil

			// Detach from parent process (Unix only - Setpgid is not available on Windows)
			setUnixProcessAttributes(daemonCmd)

			if err := daemonCmd.Start(); err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			// Capture PID before releasing the process handle
			pid := daemonCmd.Process.Pid

			// Detach and let it run
			if err := daemonCmd.Process.Release(); err != nil {
				return fmt.Errorf("failed to detach daemon: %w", err)
			}

			fmt.Printf("Daemon started (PID: %d)\n", pid)
			fmt.Printf("Log file: %s\n", logFile)

			return nil
		},
	}
}

// newDaemonStopCmd creates the daemon stop command.
func (cli *CLI) newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := daemon.GetPID(cli.Config)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Daemon is not running (no PID file found)")
					return nil
				}
				return fmt.Errorf("failed to get daemon PID: %w", err)
			}

			// Find the process
			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("failed to find process %d: %w", pid, err)
			}

			// Send termination signal
			if err := process.Signal(getTermSignal()); err != nil {
				if err.Error() == "os: process already finished" {
					fmt.Println("Daemon was not running (stale PID file)")
					// Clean up stale PID file
					paths := config.GetPaths()
					pidFile := filepath.Join(paths.DataDir, "patrol.pid")
					_ = os.Remove(pidFile)
					return nil
				}
				return fmt.Errorf("failed to stop daemon: %w", err)
			}

			fmt.Printf("Sent stop signal to daemon (PID: %d)\n", pid)
			return nil
		},
	}
}

// newDaemonStatusCmd creates the daemon status command.
func (cli *CLI) newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the daemon is running",
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

			// Get service status if available
			var serviceInfo *ServiceInfoOutput
			mgr, err := cli.getServiceManager()
			if err == nil {
				status, err := mgr.Status()
				if err == nil {
					serviceInfo = &ServiceInfoOutput{
						Platform:    daemon.ServicePlatformName(),
						ServiceFile: mgr.ServiceFilePath(),
						Installed:   status.Installed,
						Running:     status.Running,
						PID:         status.PID,
					}
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
				Service: serviceInfo,
			}

			output := NewOutputWriter(format)
			return output.Write(statusOutput, func() {
				if running {
					fmt.Printf("Daemon is running (PID: %d)\n", pid)
				} else {
					fmt.Println("Daemon is not running")
				}

				if serviceInfo != nil {
					fmt.Println()
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
						fmt.Printf("  Run 'patrol daemon install' to install as a system service\n")
					}
				}

				fmt.Printf("\nDaemon configuration:\n")
				fmt.Printf("  Check interval:     %s\n", cli.Config.Daemon.CheckInterval)
				fmt.Printf("  Renewal threshold:  %.0f%%\n", cli.Config.Daemon.RenewThreshold*100)
				fmt.Printf("  Min TTL for renewal: %s\n", cli.Config.Daemon.MinRenewTTL)
			})
		},
	}
}

// newDaemonInstallCmd creates the daemon install command.
func (cli *CLI) newDaemonInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Patrol as a user service",
		Long: `Install Patrol as a user-level service that starts automatically on login.

The service will run 'patrol daemon run' in the background, automatically
renewing your Vault tokens before they expire.

Supported platforms:
  - macOS: launchd user agent
  - Linux: systemd user service
  - Windows: Scheduled Task`,
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

			fmt.Println("Service installed successfully!")
			fmt.Printf("  Service file: %s\n", mgr.ServiceFilePath())

			// Auto-start the service after installation
			if err := mgr.Start(); err != nil {
				fmt.Printf("\nWarning: Failed to start service: %v\n", err)
				fmt.Println("You can manually start it with 'patrol daemon service-start'")
			} else {
				fmt.Println("  Service started automatically")
			}

			fmt.Println()
			fmt.Println("The daemon will now start automatically when you log in.")
			fmt.Println("Use 'patrol daemon status' to check the service status.")

			return nil
		},
	}
}

// newDaemonUninstallCmd creates the daemon uninstall command.
func (cli *CLI) newDaemonUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the Patrol service",
		Long: `Uninstall the Patrol system service.

This removes the service installation but does not stop a currently running
daemon process. Use 'patrol daemon stop' to stop a running daemon.`,
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

// newDaemonServiceStartCmd creates the daemon service-start command.
func (cli *CLI) newDaemonServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "service-start",
		Short: "Start the installed system service",
		Long: `Start the installed system service.

This command only works if the service has been installed with
'patrol daemon install'. For starting a simple background process,
use 'patrol daemon start' instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			installed, installErr := mgr.IsInstalled()
			if installErr != nil || !installed {
				return fmt.Errorf("service is not installed; run 'patrol daemon install' first")
			}

			if err := mgr.Start(); err != nil {
				return fmt.Errorf("failed to start service: %w", err)
			}

			fmt.Println("Service started")
			return nil
		},
	}
}

// newDaemonServiceStopCmd creates the daemon service-stop command.
func (cli *CLI) newDaemonServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "service-stop",
		Short: "Stop the installed system service",
		Long: `Stop the installed system service.

This command only works if the service has been installed with
'patrol daemon install'. For stopping a simple background process,
use 'patrol daemon stop' instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cli.getServiceManager()
			if err != nil {
				return err
			}

			installed, installErr := mgr.IsInstalled()
			if installErr != nil || !installed {
				return fmt.Errorf("service is not installed")
			}

			if err := mgr.Stop(); err != nil {
				return fmt.Errorf("failed to stop service: %w", err)
			}

			fmt.Println("Service stopped")
			return nil
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
