// Package cli implements the command-line interface for poxy.
package cli

import (
	"poxy/internal/config"
	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/manager/native"
	"poxy/pkg/manager/universal"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile string
	source  string
	dryRun  bool
	yes     bool
	verbose bool
	noColor bool

	// Global state
	cfg          *config.Config
	registry     *manager.Registry
	searchEngine *SearchEngine
	indexBuilder *IndexBuilder
)

// Build metadata - set at build time via ldflags
var (
	Version   = "0.2.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "poxy",
	Short: "Universal package manager for any operating system",
	Long: `Poxy unifies package management across Linux distributions,
macOS, and Windows. Use familiar commands regardless of your system's
native package manager.

Supported package managers:
  Linux:    apt, dnf, pacman, zypper, xbps, apk, emerge, eopkg, nix, slackpkg, swupd
  macOS:    brew
  Windows:  winget, chocolatey, scoop
  Universal: flatpak, snap, AUR helpers (yay, paru)

Examples:
  poxy install vim                    # Install using native package manager
  poxy install firefox -s flatpak     # Install from Flatpak
  poxy search vscode                  # Search all sources
  poxy upgrade                        # Upgrade all packages`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeApp()
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVarP(&source, "source", "s", "", "package source (apt, flatpak, snap, etc.)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would happen without executing")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "assume yes to all prompts")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(autoremoveCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(undoCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(selfUpdateCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// initializeApp sets up the application state.
func initializeApp() error {
	// Load configuration
	var err error
	if cfgFile != "" {
		cfg, err = config.LoadFrom(cfgFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return err
	}

	// Apply global flag overrides
	if yes {
		cfg.General.AutoConfirm = true
	}
	if dryRun {
		cfg.General.DryRun = true
	}
	if verbose {
		cfg.Output.Verbose = true
	}
	if noColor {
		cfg.Output.Color = false
	}

	// Initialize UI
	ui.Init(cfg.ShouldUseColor(), cfg.Output.Unicode)

	// Initialize registry
	registry = manager.NewRegistry(cfg)
	registerManagers()

	// Detect system and available managers
	if err := registry.Detect(); err != nil {
		// Non-fatal: we can still work with explicitly specified sources
		if verbose {
			ui.WarningMsg("System detection warning: %v", err)
		}
	}

	// Initialize search engine if smart search is enabled
	if cfg.General.SmartSearch {
		searchEngine = NewSearchEngine(registry)
		indexBuilder = NewIndexBuilder(searchEngine)

		// Load index in background (non-blocking)
		indexBuilder.LoadAsync()
	}

	return nil
}

// registerManagers registers all available package managers.
func registerManagers() {
	// Native Linux managers
	registry.Register(native.NewAPT(cfg.GetManagerConfig("apt").UseNala))
	registry.Register(native.NewDNF())
	registry.Register(native.NewPacman())
	registry.Register(native.NewZypper())
	registry.Register(native.NewXBPS())
	registry.Register(native.NewAPK())
	registry.Register(native.NewEmerge())
	registry.Register(native.NewEopkg())
	registry.Register(native.NewNix())
	registry.Register(native.NewSlackpkg())
	registry.Register(native.NewSwupd())

	// Homebrew (macOS + Linux)
	registry.Register(native.NewBrew())

	// Windows managers
	registry.Register(native.NewWinget())
	registry.Register(native.NewChocolatey())
	registry.Register(native.NewScoop())

	// Universal managers
	registry.Register(universal.NewFlatpak(cfg.GetManagerConfig("flatpak").DefaultRemote))
	registry.Register(universal.NewSnap(cfg.GetManagerConfig("snap").AllowClassic))

	// AUR support - prefer native builder, fall back to helper (yay/paru)
	aurConfig := cfg.GetManagerConfig("aur")
	if aurConfig.UseNative {
		// Use poxy's native AUR builder
		nativeAUR := universal.NewNativeAUR(aurConfig.ReviewPKGBUILD)
		if nativeAUR.IsAvailable() {
			registry.Register(nativeAUR)
		}
	} else {
		// Use AUR helper (yay, paru, etc.)
		aurHelper := cfg.GetManagerConfig("pacman").AURHelper
		if aur := universal.NewAUR(aurHelper); aur != nil {
			registry.Register(aur)
		}
	}
}

// getManager returns the appropriate manager based on flags and detection.
func getManager() (manager.Manager, error) {
	if source != "" {
		return registry.GetManagerForSource(source)
	}

	native := registry.Native()
	if native == nil {
		return nil, ErrNoManager
	}

	return native, nil
}

// resolvePackages resolves aliases in package names.
func resolvePackages(packages []string) []string {
	return cfg.ResolveAliases(packages)
}

// Version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print poxy version",
	Run: func(cmd *cobra.Command, args []string) {
		ui.InfoMsg("poxy version %s", Version)
		if Commit != "unknown" {
			ui.MutedMsg("  Commit: %s", Commit)
		}
		if BuildTime != "unknown" {
			ui.MutedMsg("  Built:  %s", BuildTime)
		}
	},
}
