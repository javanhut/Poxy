package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	appName      = "poxy"
	configFile   = "config.toml"
	historyFile  = "history.db"
	snapshotFile = "snapshots.db"
)

// ConfigDir returns the platform-specific configuration directory for poxy.
func ConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir() //nolint:errcheck
		return filepath.Join(home, "Library", "Application Support", appName)
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), appName)
	default: // linux and others
		// Respect XDG_CONFIG_HOME if set
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, appName)
		}
		home, _ := os.UserHomeDir() //nolint:errcheck
		return filepath.Join(home, ".config", appName)
	}
}

// DataDir returns the platform-specific data directory for poxy.
func DataDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir() //nolint:errcheck
		return filepath.Join(home, "Library", "Application Support", appName)
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), appName)
	default: // linux and others
		// Respect XDG_DATA_HOME if set
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, appName)
		}
		home, _ := os.UserHomeDir() //nolint:errcheck
		return filepath.Join(home, ".local", "share", appName)
	}
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), configFile)
}

// HistoryPath returns the full path to the history database.
func HistoryPath() string {
	return filepath.Join(DataDir(), historyFile)
}

// SnapshotPath returns the full path to the snapshot database.
func SnapshotPath() string {
	return filepath.Join(DataDir(), snapshotFile)
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0755)
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() error {
	return os.MkdirAll(DataDir(), 0755)
}
