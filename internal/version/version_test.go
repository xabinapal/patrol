package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	// GoVersion should match runtime
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %s, want %s", info.GoVersion, runtime.Version())
	}

	// Platform should contain OS and arch
	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if info.Platform != expectedPlatform {
		t.Errorf("Platform = %s, want %s", info.Platform, expectedPlatform)
	}

	// Version should be set (even if to "dev")
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestInfoString(t *testing.T) {
	info := Get()
	str := info.String()

	// Should contain "patrol"
	if !strings.Contains(str, "patrol") {
		t.Errorf("String() should contain 'patrol', got %s", str)
	}

	// Should contain version
	if !strings.Contains(str, info.Version) {
		t.Errorf("String() should contain version %s, got %s", info.Version, str)
	}

	// Should contain platform
	if !strings.Contains(str, info.Platform) {
		t.Errorf("String() should contain platform %s, got %s", info.Platform, str)
	}
}

func TestInfoShort(t *testing.T) {
	info := Get()
	short := info.Short()

	// Should start with "patrol"
	if !strings.HasPrefix(short, "patrol") {
		t.Errorf("Short() should start with 'patrol', got %s", short)
	}

	// Should contain version
	if !strings.Contains(short, info.Version) {
		t.Errorf("Short() should contain version %s, got %s", info.Version, short)
	}
}

func TestVersionVariables(t *testing.T) {
	// Default values
	if Version == "" {
		t.Error("Version variable should have a default value")
	}
	if Commit == "" {
		t.Error("Commit variable should have a default value")
	}
	if Date == "" {
		t.Error("Date variable should have a default value")
	}
}
