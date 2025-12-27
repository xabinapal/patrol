package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetPaths(t *testing.T) {
	paths := GetPaths()

	// All paths should be non-empty
	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}
	if paths.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if paths.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
	if paths.ConfigFile == "" {
		t.Error("ConfigFile should not be empty")
	}

	// ConfigFile should be within ConfigDir
	if !strings.HasPrefix(paths.ConfigFile, paths.ConfigDir) {
		t.Errorf("ConfigFile %s should be within ConfigDir %s", paths.ConfigFile, paths.ConfigDir)
	}

	// ConfigFile should end with config.yaml
	if filepath.Base(paths.ConfigFile) != ConfigFileName {
		t.Errorf("ConfigFile should end with %s, got %s", ConfigFileName, filepath.Base(paths.ConfigFile))
	}
}

func TestGetPathsWithEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	oldEnv := os.Getenv("PATROL_CONFIG_DIR")
	os.Setenv("PATROL_CONFIG_DIR", tmpDir)
	defer os.Setenv("PATROL_CONFIG_DIR", oldEnv)

	paths := GetPaths()

	if paths.ConfigDir != tmpDir {
		t.Errorf("expected ConfigDir %s, got %s", tmpDir, paths.ConfigDir)
	}
}

func TestGetPathsXDGConfigHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG not applicable on Windows")
	}

	tmpDir := t.TempDir()
	oldEnv := os.Getenv("XDG_CONFIG_HOME")
	oldPatrol := os.Getenv("PATROL_CONFIG_DIR")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Unsetenv("PATROL_CONFIG_DIR")
	defer func() {
		os.Setenv("XDG_CONFIG_HOME", oldEnv)
		os.Setenv("PATROL_CONFIG_DIR", oldPatrol)
	}()

	paths := GetPaths()

	expected := filepath.Join(tmpDir, AppName)
	if paths.ConfigDir != expected {
		t.Errorf("expected ConfigDir %s, got %s", expected, paths.ConfigDir)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
		CacheDir:  filepath.Join(tmpDir, "cache"),
	}

	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() failed: %v", err)
	}

	// Verify directories exist
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.CacheDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s should exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s should be a directory", dir)
		}
		// Check permissions (Unix only)
		if runtime.GOOS != "windows" {
			perm := info.Mode().Perm()
			if perm != 0700 {
				t.Errorf("directory %s should have 0700 permissions, got %o", dir, perm)
			}
		}
	}
}

func TestGetConfigDir_Fallback(t *testing.T) {
	// Temporarily clear all env vars
	envVars := []string{"PATROL_CONFIG_DIR", "XDG_CONFIG_HOME", "HOME", "APPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	// Set HOME back for the test (otherwise we'd get a weird fallback)
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	paths := GetPaths()

	// Should use HOME-based path
	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty even without XDG vars")
	}
}

func TestGetConfigDirPlatformSpecific(t *testing.T) {
	// Save original env vars
	envVars := []string{"PATROL_CONFIG_DIR", "XDG_CONFIG_HOME", "HOME", "APPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	tests := []struct {
		name        string
		goos        string
		envSetup    map[string]string
		expectedDir string
	}{
		{
			name: "windows with APPDATA",
			goos: "windows",
			envSetup: map[string]string{
				"APPDATA": "C:\\Users\\test\\AppData\\Roaming",
			},
			expectedDir: filepath.Join("C:\\Users\\test\\AppData\\Roaming", AppName),
		},
		{
			name: "windows with USERPROFILE fallback",
			goos: "windows",
			envSetup: map[string]string{
				"USERPROFILE": "C:\\Users\\test",
			},
			expectedDir: filepath.Join("C:\\Users\\test", "AppData", "Roaming", AppName),
		},
		{
			name: "darwin with XDG_CONFIG_HOME",
			goos: "darwin",
			envSetup: map[string]string{
				"XDG_CONFIG_HOME": "/tmp/xdg",
			},
			expectedDir: filepath.Join("/tmp/xdg", AppName),
		},
		{
			name: "darwin with HOME only",
			goos: "darwin",
			envSetup: map[string]string{
				"HOME": "/Users/test",
			},
			expectedDir: filepath.Join("/Users/test", "Library", "Application Support", AppName),
		},
		{
			name: "linux with XDG_CONFIG_HOME",
			goos: "linux",
			envSetup: map[string]string{
				"XDG_CONFIG_HOME": "/home/test/.config",
			},
			expectedDir: filepath.Join("/home/test/.config", AppName),
		},
		{
			name: "linux with HOME fallback",
			goos: "linux",
			envSetup: map[string]string{
				"HOME": "/home/test",
			},
			expectedDir: filepath.Join("/home/test", ".config", AppName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if not running on the target platform
			if runtime.GOOS != tt.goos {
				t.Skipf("skipping %s test on %s", tt.goos, runtime.GOOS)
			}

			// Clear all env vars first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up test env vars
			for k, v := range tt.envSetup {
				os.Setenv(k, v)
			}

			dir := getConfigDir()
			if dir != tt.expectedDir {
				t.Errorf("getConfigDir() = %s, expected %s", dir, tt.expectedDir)
			}
		})
	}
}

func TestGetDataDirPlatformSpecific(t *testing.T) {
	// Save original env vars
	envVars := []string{"XDG_DATA_HOME", "HOME", "LOCALAPPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	tests := []struct {
		name        string
		goos        string
		envSetup    map[string]string
		expectedDir string
	}{
		{
			name: "windows with LOCALAPPDATA",
			goos: "windows",
			envSetup: map[string]string{
				"LOCALAPPDATA": "C:\\Users\\test\\AppData\\Local",
			},
			expectedDir: filepath.Join("C:\\Users\\test\\AppData\\Local", AppName),
		},
		{
			name: "windows with USERPROFILE fallback",
			goos: "windows",
			envSetup: map[string]string{
				"USERPROFILE": "C:\\Users\\test",
			},
			expectedDir: filepath.Join("C:\\Users\\test", "AppData", "Local", AppName),
		},
		{
			name: "darwin with XDG_DATA_HOME",
			goos: "darwin",
			envSetup: map[string]string{
				"XDG_DATA_HOME": "/tmp/xdg-data",
			},
			expectedDir: filepath.Join("/tmp/xdg-data", AppName),
		},
		{
			name: "darwin with HOME only",
			goos: "darwin",
			envSetup: map[string]string{
				"HOME": "/Users/test",
			},
			expectedDir: filepath.Join("/Users/test", "Library", "Application Support", AppName),
		},
		{
			name: "linux with XDG_DATA_HOME",
			goos: "linux",
			envSetup: map[string]string{
				"XDG_DATA_HOME": "/home/test/.local/share",
			},
			expectedDir: filepath.Join("/home/test/.local/share", AppName),
		},
		{
			name: "linux with HOME fallback",
			goos: "linux",
			envSetup: map[string]string{
				"HOME": "/home/test",
			},
			expectedDir: filepath.Join("/home/test", ".local", "share", AppName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if not running on the target platform
			if runtime.GOOS != tt.goos {
				t.Skipf("skipping %s test on %s", tt.goos, runtime.GOOS)
			}

			// Clear all env vars first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up test env vars
			for k, v := range tt.envSetup {
				os.Setenv(k, v)
			}

			dir := getDataDir()
			if dir != tt.expectedDir {
				t.Errorf("getDataDir() = %s, expected %s", dir, tt.expectedDir)
			}
		})
	}
}

func TestGetCacheDirPlatformSpecific(t *testing.T) {
	// Save original env vars
	envVars := []string{"XDG_CACHE_HOME", "HOME", "LOCALAPPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	tests := []struct {
		name        string
		goos        string
		envSetup    map[string]string
		expectedDir string
	}{
		{
			name: "windows with LOCALAPPDATA",
			goos: "windows",
			envSetup: map[string]string{
				"LOCALAPPDATA": "C:\\Users\\test\\AppData\\Local",
			},
			expectedDir: filepath.Join("C:\\Users\\test\\AppData\\Local", AppName, "cache"),
		},
		{
			name: "windows with USERPROFILE fallback",
			goos: "windows",
			envSetup: map[string]string{
				"USERPROFILE": "C:\\Users\\test",
			},
			expectedDir: filepath.Join("C:\\Users\\test", "AppData", "Local", AppName, "cache"),
		},
		{
			name: "darwin with XDG_CACHE_HOME",
			goos: "darwin",
			envSetup: map[string]string{
				"XDG_CACHE_HOME": "/tmp/xdg-cache",
			},
			expectedDir: filepath.Join("/tmp/xdg-cache", AppName),
		},
		{
			name: "darwin with HOME only",
			goos: "darwin",
			envSetup: map[string]string{
				"HOME": "/Users/test",
			},
			expectedDir: filepath.Join("/Users/test", "Library", "Caches", AppName),
		},
		{
			name: "linux with XDG_CACHE_HOME",
			goos: "linux",
			envSetup: map[string]string{
				"XDG_CACHE_HOME": "/home/test/.cache",
			},
			expectedDir: filepath.Join("/home/test/.cache", AppName),
		},
		{
			name: "linux with HOME fallback",
			goos: "linux",
			envSetup: map[string]string{
				"HOME": "/home/test",
			},
			expectedDir: filepath.Join("/home/test", ".cache", AppName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if not running on the target platform
			if runtime.GOOS != tt.goos {
				t.Skipf("skipping %s test on %s", tt.goos, runtime.GOOS)
			}

			// Clear all env vars first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up test env vars
			for k, v := range tt.envSetup {
				os.Setenv(k, v)
			}

			dir := getCacheDir()
			if dir != tt.expectedDir {
				t.Errorf("getCacheDir() = %s, expected %s", dir, tt.expectedDir)
			}
		})
	}
}

func TestGetConfigDirMacOSXDGPreference(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	// Save and clear env vars
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	oldHome := os.Getenv("HOME")
	oldPatrol := os.Getenv("PATROL_CONFIG_DIR")

	tmpHome := t.TempDir()
	xdgPath := filepath.Join(tmpHome, ".config", AppName)

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
		os.Setenv("HOME", oldHome)
		os.Setenv("PATROL_CONFIG_DIR", oldPatrol)
	}()

	// Test: when XDG path exists, it should be preferred
	os.Unsetenv("PATROL_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("HOME", tmpHome)

	// Create XDG path
	if err := os.MkdirAll(xdgPath, 0700); err != nil {
		t.Fatalf("failed to create XDG path: %v", err)
	}

	dir := getConfigDir()
	if dir != xdgPath {
		t.Errorf("getConfigDir() should prefer existing XDG path %s, got %s", xdgPath, dir)
	}
}

func TestGetConfigDirMacOSLibraryFallback(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	// Save and clear env vars
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	oldHome := os.Getenv("HOME")
	oldPatrol := os.Getenv("PATROL_CONFIG_DIR")

	tmpHome := t.TempDir()
	libraryPath := filepath.Join(tmpHome, "Library", "Application Support", AppName)

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
		os.Setenv("HOME", oldHome)
		os.Setenv("PATROL_CONFIG_DIR", oldPatrol)
	}()

	// Test: when XDG path doesn't exist, should use Library
	os.Unsetenv("PATROL_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("HOME", tmpHome)

	// Don't create XDG path - it shouldn't exist

	dir := getConfigDir()
	if dir != libraryPath {
		t.Errorf("getConfigDir() should use Library path %s when XDG doesn't exist, got %s", libraryPath, dir)
	}
}

func TestGetConfigDirUltimateFallback(t *testing.T) {
	// Save and clear ALL env vars
	envVars := []string{"PATROL_CONFIG_DIR", "XDG_CONFIG_HOME", "HOME", "APPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	dir := getConfigDir()
	expected := filepath.Join(".", "."+AppName)
	if dir != expected {
		t.Errorf("getConfigDir() should fall back to %s when all env vars missing, got %s", expected, dir)
	}
}

func TestGetDataDirUltimateFallback(t *testing.T) {
	// Save and clear ALL env vars
	envVars := []string{"XDG_DATA_HOME", "HOME", "LOCALAPPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	dir := getDataDir()
	expected := filepath.Join(".", "."+AppName, "data")
	if dir != expected {
		t.Errorf("getDataDir() should fall back to %s when all env vars missing, got %s", expected, dir)
	}
}

func TestGetCacheDirUltimateFallback(t *testing.T) {
	// Save and clear ALL env vars
	envVars := []string{"XDG_CACHE_HOME", "HOME", "LOCALAPPDATA", "USERPROFILE"}
	oldVals := make(map[string]string)
	for _, env := range envVars {
		oldVals[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range oldVals {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	dir := getCacheDir()
	expected := filepath.Join(".", "."+AppName, "cache")
	if dir != expected {
		t.Errorf("getCacheDir() should fall back to %s when all env vars missing, got %s", expected, dir)
	}
}

func TestEnsureDirsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests are unreliable on Windows")
	}

	tmpDir := t.TempDir()

	// Create a file where we'll try to create a directory
	filePath := filepath.Join(tmpDir, "blockingfile")
	if err := os.WriteFile(filePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	paths := Paths{
		// Try to create a directory where a file already exists
		ConfigDir: filepath.Join(filePath, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
		CacheDir:  filepath.Join(tmpDir, "cache"),
	}

	err := paths.EnsureDirs()
	if err == nil {
		t.Error("EnsureDirs() should fail when directory cannot be created")
	}
}

func TestEnsureDirsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
		CacheDir:  filepath.Join(tmpDir, "cache"),
	}

	// First call should succeed
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("first EnsureDirs() failed: %v", err)
	}

	// Second call should also succeed (idempotent)
	if err := paths.EnsureDirs(); err != nil {
		t.Errorf("second EnsureDirs() failed: %v", err)
	}

	// All directories should still exist
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.CacheDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s should exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s should be a directory", dir)
		}
	}
}
