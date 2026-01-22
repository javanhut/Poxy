package cli

import (
	"context"

	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/snapshot"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [packages...]",
	Short: "Upgrade installed packages",
	Long: `Upgrade installed packages to their latest versions.

If no packages are specified, all installed packages will be upgraded.

Examples:
  poxy upgrade              # Upgrade all packages
  poxy upgrade vim git      # Upgrade specific packages
  poxy upgrade -y           # Upgrade all without confirmation`,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// Resolve aliases if specific packages given
	packages := resolvePackages(args)

	if len(packages) > 0 {
		ui.InfoMsg("Upgrading %d package(s) using %s", len(packages), mgr.DisplayName())
		for _, pkg := range packages {
			ui.MutedMsg("  - %s", pkg)
		}
	} else {
		ui.InfoMsg("Upgrading all packages using %s", mgr.DisplayName())
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with upgrade?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Capture pre-operation snapshot
	capturePreOperationSnapshot(ctx, snapshot.TriggerUpgrade, packages)

	// Create history entry
	entry := history.NewEntry(history.OpUpgrade, mgr.Name(), packages)

	// Build options
	opts := manager.UpgradeOpts{
		AutoConfirm: cfg.General.AutoConfirm,
		DryRun:      cfg.General.DryRun,
		Packages:    packages,
	}

	// Execute upgrade
	err = mgr.Upgrade(ctx, opts)

	// Update history
	if err != nil {
		entry.MarkFailed(err)
		ui.ErrorMsg("Upgrade failed: %v", err)
	} else {
		entry.MarkSuccess()
		ui.SuccessMsg("Upgrade completed successfully")
	}

	// Record in history (ignore errors)
	if store, storeErr := history.Open(); storeErr == nil {
		_ = store.Record(entry) //nolint:errcheck
		_ = store.Close()       //nolint:errcheck
	}

	return err
}
