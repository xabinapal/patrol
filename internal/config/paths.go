// Package config provides configuration management for Patrol.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the application name used for directories.
	AppName = "patrol"
	// ConfigFileName is the default configuration file name.
	ConfigFileName = "config.yaml"
)

// Paths holds all the application paths.
type Paths struct {
	ConfigDir  string
	DataDir    string
	CacheDir   string
	ConfigFile string
}

// GetPaths returns the application paths following XDG Base Directory specification.
func GetPaths() Paths {
	return Paths{
		ConfigDir:  getConfigDir(),
		DataDir:    getDataDir(),
		CacheDir:   getCacheDir(),
		ConfigFile: filepath.Join(getConfigDir(), ConfigFileName),
	}
}

// getConfigDir returns the configuration directory path.
func getConfigDir() string {
	// Check for explicit override
	if dir := os.Getenv("PATROL_CONFIG_DIR"); dir != "" {
		return dir
	}

	switch runtime.GOOS {
	case "windows":
		// Use %APPDATA%\patrol on Windows
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, AppName)
		}
		// Fallback to user profile
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Roaming", AppName)
		}
	case "darwin":
		// macOS: prefer XDG, fallback to ~/Library/Application Support
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			// Check if ~/.config/patrol exists, use it if so
			xdgPath := filepath.Join(home, ".config", AppName)
			if _, err := os.Stat(xdgPath); err == nil {
				return xdgPath
			}
			// Otherwise use macOS standard location
			return filepath.Join(home, "Library", "Application Support", AppName)
		}
	default:
		// Linux and other Unix-like systems: follow XDG
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".config", AppName)
		}
	}

	// Last resort fallback
	return filepath.Join(".", "."+AppName)
}

// getDataDir returns the data directory path.
func getDataDir() string {
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, AppName)
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Local", AppName)
		}
	case "darwin":
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return filepath.Join(xdgData, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, "Library", "Application Support", AppName)
		}
	default:
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return filepath.Join(xdgData, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".local", "share", AppName)
		}
	}

	return filepath.Join(".", "."+AppName, "data")
}

// getCacheDir returns the cache directory path.
func getCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, AppName, "cache")
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Local", AppName, "cache")
		}
	case "darwin":
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, "Library", "Caches", AppName)
		}
	default:
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, AppName)
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".cache", AppName)
		}
	}

	return filepath.Join(".", "."+AppName, "cache")
}

// EnsureDirs creates all necessary directories if they don't exist.
func (p Paths) EnsureDirs() error {
	dirs := []string{p.ConfigDir, p.DataDir, p.CacheDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}
