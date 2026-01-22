package cli

import (
	"context"

	"poxy/internal/ui"
	"poxy/pkg/manager"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose system issues",
	Long: `Check for common issues with package managers and system
configuration.

Examples:
  poxy doctor               # Run diagnostics`,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	issues := 0

	ui.HeaderMsg("Running diagnostics...")

	// Check system detection
	sysInfo := registry.SystemInfo()
	if sysInfo == nil {
		ui.ErrorMsg("System detection failed")
		issues++
	} else {
		ui.SuccessMsg("System detected: %s (%s)", sysInfo.PrettyName, sysInfo.Arch)
	}

	// Check native manager
	native := registry.Native()
	if native == nil {
		ui.ErrorMsg("No native package manager detected")
		issues++
	} else {
		ui.SuccessMsg("Native package manager: %s", native.DisplayName())

		// Try to run a simple command
		if native.IsAvailable() {
			ui.SuccessMsg("Package manager is available and working")
		} else {
			ui.ErrorMsg("Package manager binary not found")
			issues++
		}
	}

	// Check available managers
	available := registry.Available()
	ui.InfoMsg("Available package managers: %d", len(available))
	for _, mgr := range available {
		status := ui.Green("OK")
		if mgr.NeedsSudo() {
			status = ui.Green("OK") + " (requires sudo)"
		}
		ui.MutedMsg("  - %s: %s", mgr.Name(), status)
	}

	// Check universal managers
	ui.HeaderMsg("Universal Package Managers")

	universalManagers := []string{"flatpak", "snap"}
	for _, name := range universalManagers {
		mgr, ok := registry.Get(name)
		if ok && mgr.IsAvailable() {
			ui.SuccessMsg("%s is available", name)
		} else {
			ui.MutedMsg("%s is not installed", name)
		}
	}

	// Check AUR helper (if on Arch)
	if sysInfo != nil && sysInfo.MatchesDistro("arch", "manjaro", "endeavouros") {
		ui.HeaderMsg("AUR Helper")
		aur, ok := registry.Get("aur")
		if ok && aur.IsAvailable() {
			ui.SuccessMsg("AUR helper available: %s", aur.DisplayName())
		} else {
			ui.WarningMsg("No AUR helper found (install yay or paru for AUR support)")
		}
	}

	// Check config
	ui.HeaderMsg("Configuration")
	ui.SuccessMsg("Config file: %s", cfg.General.SourcePriority)

	// Test basic operations
	ui.HeaderMsg("Testing Operations")

	if native != nil {
		// Try a search (non-destructive)
		_, err := native.Search(ctx, "test", defaultSearchOpts())
		if err != nil {
			ui.WarningMsg("Search test failed: %v", err)
		} else {
			ui.SuccessMsg("Search operation works")
		}
	}

	// Summary
	ui.HeaderMsg("Summary")
	if issues == 0 {
		ui.SuccessMsg("No issues found! Poxy is ready to use.")
	} else {
		ui.WarningMsg("Found %d issue(s). Some features may not work correctly.", issues)
	}

	return nil
}

func defaultSearchOpts() manager.SearchOpts {
	return manager.SearchOpts{
		Limit: 1,
	}
}
