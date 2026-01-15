package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the complete poxy configuration.
type Config struct {
	General  GeneralConfig            `toml:"general"`
	Output   OutputConfig             `toml:"output"`
	Managers map[string]ManagerConfig `toml:"managers"`
	Aliases  map[string]string        `toml:"aliases"`
}

// GeneralConfig contains general poxy settings.
type GeneralConfig struct {
	// SourcePriority defines the order in which package sources are searched/preferred.
	// Valid values: "native", "flatpak", "snap", "aur", "brew"
	SourcePriority []string `toml:"source_priority"`

	// AutoConfirm skips confirmation prompts when true (like -y flag).
	AutoConfirm bool `toml:"auto_confirm"`

	// DryRun shows what would happen without executing when true.
	DryRun bool `toml:"dry_run"`

	// Snapshots enables automatic snapshot creation before operations.
	Snapshots bool `toml:"snapshots"`

	// SmartSearch enables TF-IDF based intelligent search with relevance ranking.
	// When disabled, falls back to native package manager search.
	SmartSearch bool `toml:"smart_search"`
}

// OutputConfig contains output formatting settings.
type OutputConfig struct {
	// Color enables colored output (respects NO_COLOR env var).
	Color bool `toml:"color"`

	// Unicode enables unicode symbols in output.
	Unicode bool `toml:"unicode"`

	// Verbose enables detailed output.
	Verbose bool `toml:"verbose"`
}

// ManagerConfig contains per-manager settings.
type ManagerConfig struct {
	// AURHelper specifies which AUR helper to use (yay, paru). Pacman only.
	AURHelper string `toml:"aur_helper"`

	// UseNala uses nala instead of apt if available. APT only.
	UseNala bool `toml:"use_nala"`

	// DefaultRemote specifies the default remote for Flatpak.
	DefaultRemote string `toml:"default_remote"`

	// AllowClassic allows classic confinement for Snap packages.
	AllowClassic bool `toml:"allow_classic"`

	// UseNative uses poxy's native AUR builder instead of helpers. AUR only.
	UseNative bool `toml:"use_native"`

	// ReviewPKGBUILD shows PKGBUILD review before building. AUR only.
	ReviewPKGBUILD bool `toml:"review_pkgbuild"`

	// UseSandbox runs AUR builds in a bubblewrap sandbox. AUR only.
	UseSandbox bool `toml:"use_sandbox"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		General: GeneralConfig{
			SourcePriority: []string{"native", "flatpak", "snap", "aur"},
			AutoConfirm:    false,
			DryRun:         false,
			Snapshots:      true, // Enable snapshots by default
			SmartSearch:    true, // Enable TF-IDF search by default
		},
		Output: OutputConfig{
			Color:   true,
			Unicode: true,
			Verbose: false,
		},
		Managers: map[string]ManagerConfig{
			"pacman": {
				AURHelper: "yay",
			},
			"apt": {
				UseNala: false,
			},
			"flatpak": {
				DefaultRemote: "flathub",
			},
			"snap": {
				AllowClassic: false,
			},
			"aur": {
				UseNative:      true, // Use native builder by default
				ReviewPKGBUILD: true, // Show security review by default
				UseSandbox:     true, // Use sandbox if available
			},
		},
		Aliases: map[string]string{},
	}
}

// Load loads the configuration from the default path.
// If the config file doesn't exist, it returns the default configuration.
func Load() (*Config, error) {
	return LoadFrom(ConfigPath())
}

// LoadFrom loads the configuration from a specific path.
// If the config file doesn't exist, it returns the default configuration.
func LoadFrom(path string) (*Config, error) {
	cfg := Default()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	// Parse the config file
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to the default path.
func (c *Config) Save() error {
	return c.SaveTo(ConfigPath())
}

// SaveTo writes the configuration to a specific path.
func (c *Config) SaveTo(path string) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(c)
}

// ResolveAlias returns the actual package name for an alias, or the original name if no alias exists.
func (c *Config) ResolveAlias(pkg string) string {
	if alias, ok := c.Aliases[pkg]; ok {
		return alias
	}
	return pkg
}

// ResolveAliases resolves all aliases in a list of package names.
func (c *Config) ResolveAliases(packages []string) []string {
	resolved := make([]string, len(packages))
	for i, pkg := range packages {
		resolved[i] = c.ResolveAlias(pkg)
	}
	return resolved
}

// GetManagerConfig returns the configuration for a specific manager.
// Returns an empty config if no configuration exists for the manager.
func (c *Config) GetManagerConfig(name string) ManagerConfig {
	if cfg, ok := c.Managers[name]; ok {
		return cfg
	}
	return ManagerConfig{}
}

// ShouldUseColor returns true if colored output should be used.
// Respects the NO_COLOR environment variable.
func (c *Config) ShouldUseColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return c.Output.Color
}
