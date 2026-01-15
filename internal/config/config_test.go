package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Check default source priority
	if len(cfg.General.SourcePriority) != 4 {
		t.Errorf("expected 4 source priorities, got %d", len(cfg.General.SourcePriority))
	}

	// Check default output settings
	if !cfg.Output.Color {
		t.Error("expected Color to be true by default")
	}
	if !cfg.Output.Unicode {
		t.Error("expected Unicode to be true by default")
	}
	if cfg.Output.Verbose {
		t.Error("expected Verbose to be false by default")
	}

	// Check general settings
	if cfg.General.AutoConfirm {
		t.Error("expected AutoConfirm to be false by default")
	}
	if cfg.General.DryRun {
		t.Error("expected DryRun to be false by default")
	}
}

func TestResolveAlias(t *testing.T) {
	cfg := &Config{
		Aliases: map[string]string{
			"vim":  "neovim",
			"code": "visual-studio-code",
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"vim", "neovim"},
		{"code", "visual-studio-code"},
		{"git", "git"}, // No alias, returns original
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cfg.ResolveAlias(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveAlias(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveAliases(t *testing.T) {
	cfg := &Config{
		Aliases: map[string]string{
			"vim": "neovim",
		},
	}

	input := []string{"vim", "git", "curl"}
	expected := []string{"neovim", "git", "curl"}

	result := cfg.ResolveAliases(input)

	if len(result) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(result))
	}

	for i, r := range result {
		if r != expected[i] {
			t.Errorf("result[%d] = %s, want %s", i, r, expected[i])
		}
	}
}

func TestGetManagerConfig(t *testing.T) {
	cfg := &Config{
		Managers: map[string]ManagerConfig{
			"pacman": {AURHelper: "yay"},
			"apt":    {UseNala: true},
		},
	}

	// Existing manager
	pacmanCfg := cfg.GetManagerConfig("pacman")
	if pacmanCfg.AURHelper != "yay" {
		t.Errorf("expected AURHelper 'yay', got '%s'", pacmanCfg.AURHelper)
	}

	// Non-existing manager returns empty config
	dnfCfg := cfg.GetManagerConfig("dnf")
	if dnfCfg.AURHelper != "" {
		t.Errorf("expected empty AURHelper, got '%s'", dnfCfg.AURHelper)
	}
}

func TestShouldUseColor(t *testing.T) {
	cfg := &Config{
		Output: OutputConfig{Color: true},
	}

	// Should return true when Color is true and NO_COLOR is not set
	os.Unsetenv("NO_COLOR")
	if !cfg.ShouldUseColor() {
		t.Error("expected ShouldUseColor() to return true")
	}

	// Should return false when NO_COLOR is set
	os.Setenv("NO_COLOR", "1")
	if cfg.ShouldUseColor() {
		t.Error("expected ShouldUseColor() to return false when NO_COLOR is set")
	}
	os.Unsetenv("NO_COLOR")

	// Should return false when Color is false
	cfg.Output.Color = false
	if cfg.ShouldUseColor() {
		t.Error("expected ShouldUseColor() to return false when Color is false")
	}
}

func TestLoadSaveConfig(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create and save config
	cfg := Default()
	cfg.Aliases["test"] = "test-package"

	err := cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Load config
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	// Verify loaded config
	if loaded.ResolveAlias("test") != "test-package" {
		t.Error("loaded config doesn't have expected alias")
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Loading non-existent file should return default config
	cfg, err := LoadFrom("/non/existent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadFrom() should not error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadFrom() should return default config for non-existent file")
	}

	// Should have default values
	if !cfg.Output.Color {
		t.Error("expected default Color to be true")
	}
}
