package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()

	if dir == "" {
		t.Error("ConfigDir() returned empty string")
	}

	// Should contain 'poxy' in the path
	if !strings.Contains(dir, "poxy") {
		t.Errorf("ConfigDir() should contain 'poxy': %s", dir)
	}

	// Platform-specific checks
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, "Library/Application Support") {
			t.Errorf("macOS ConfigDir() should be in Library/Application Support: %s", dir)
		}
	case "windows":
		// Check for APPDATA
		if !strings.Contains(strings.ToLower(dir), "appdata") {
			t.Errorf("Windows ConfigDir() should be in APPDATA: %s", dir)
		}
	default: // Linux
		if !strings.Contains(dir, ".config") && os.Getenv("XDG_CONFIG_HOME") == "" {
			t.Errorf("Linux ConfigDir() should be in .config: %s", dir)
		}
	}
}

func TestDataDir(t *testing.T) {
	dir := DataDir()

	if dir == "" {
		t.Error("DataDir() returned empty string")
	}

	// Should contain 'poxy' in the path
	if !strings.Contains(dir, "poxy") {
		t.Errorf("DataDir() should contain 'poxy': %s", dir)
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()

	if path == "" {
		t.Error("ConfigPath() returned empty string")
	}

	// Should end with config.toml
	if !strings.HasSuffix(path, "config.toml") {
		t.Errorf("ConfigPath() should end with 'config.toml': %s", path)
	}
}

func TestHistoryPath(t *testing.T) {
	path := HistoryPath()

	if path == "" {
		t.Error("HistoryPath() returned empty string")
	}

	// Should end with history.db
	if !strings.HasSuffix(path, "history.db") {
		t.Errorf("HistoryPath() should end with 'history.db': %s", path)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// Save original and use temp dir
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}

	// Check directory exists
	dir := ConfigDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Config directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("ConfigDir is not a directory")
	}
}

func TestEnsureDataDir(t *testing.T) {
	// Save original and use temp dir
	originalXDG := os.Getenv("XDG_DATA_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", originalXDG)

	err := EnsureDataDir()
	if err != nil {
		t.Fatalf("EnsureDataDir() error: %v", err)
	}

	// Check directory exists
	dir := DataDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Data directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("DataDir is not a directory")
	}
}

func TestXDGOverride(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("XDG not used on this platform")
	}

	tmpDir := t.TempDir()
	customConfig := filepath.Join(tmpDir, "custom_config")
	customData := filepath.Join(tmpDir, "custom_data")

	// Test XDG_CONFIG_HOME override
	originalConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", customConfig)

	configDir := ConfigDir()
	if !strings.HasPrefix(configDir, customConfig) {
		t.Errorf("ConfigDir should use XDG_CONFIG_HOME: %s", configDir)
	}
	os.Setenv("XDG_CONFIG_HOME", originalConfig)

	// Test XDG_DATA_HOME override
	originalData := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", customData)

	dataDir := DataDir()
	if !strings.HasPrefix(dataDir, customData) {
		t.Errorf("DataDir should use XDG_DATA_HOME: %s", dataDir)
	}
	os.Setenv("XDG_DATA_HOME", originalData)
}
