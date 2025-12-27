package daemon

import (
	"runtime"
	"testing"
)

func TestServicePlatformName(t *testing.T) {
	name := ServicePlatformName()

	switch runtime.GOOS {
	case "darwin":
		if name != "launchd" {
			t.Errorf("ServicePlatformName() = %q, want %q on darwin", name, "launchd")
		}
	case "linux":
		if name != "systemd" {
			t.Errorf("ServicePlatformName() = %q, want %q on linux", name, "systemd")
		}
	case "windows":
		if name != "Task Scheduler" {
			t.Errorf("ServicePlatformName() = %q, want %q on windows", name, "Task Scheduler")
		}
	default:
		if name != "unknown" {
			t.Errorf("ServicePlatformName() = %q, want %q on %s", name, "unknown", runtime.GOOS)
		}
	}
}

func TestNewServiceManager(t *testing.T) {
	cfg := ServiceConfig{
		ExecutablePath: "/usr/local/bin/patrol",
		LogPath:        "/tmp/patrol.log",
	}

	mgr, err := NewServiceManager(cfg)

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		if err != nil {
			t.Errorf("NewServiceManager() error = %v on supported platform %s", err, runtime.GOOS)
		}
		if mgr == nil {
			t.Error("NewServiceManager() returned nil on supported platform")
		}
	default:
		if err == nil {
			t.Error("NewServiceManager() expected error on unsupported platform")
		}
	}
}

func TestServiceStatus(t *testing.T) {
	status := ServiceStatus{
		Installed: true,
		Running:   true,
		PID:       12345,
	}

	if !status.Installed {
		t.Error("ServiceStatus.Installed should be true")
	}
	if !status.Running {
		t.Error("ServiceStatus.Running should be true")
	}
	if status.PID != 12345 {
		t.Errorf("ServiceStatus.PID = %d, want 12345", status.PID)
	}
}
