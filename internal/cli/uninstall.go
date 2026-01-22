package cli

import (
	"context"

	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/snapshot"

	"github.com/spf13/cobra"
)

var (
	uninstallPurge     bool
	uninstallRecursive bool
)

var uninstallCmd = &cobra.Command{
	Use:     "uninstall [packages...]",
	Aliases: []string{"remove", "rm"},
	Short:   "Remove one or more packages",
	Long: `Remove packages using the detected system package manager
or a specified source.

Examples:
  poxy uninstall vim                # Remove package
  poxy uninstall -y firefox         # Remove without confirmation
  poxy uninstall --purge nginx      # Remove including config files
  poxy uninstall -r package         # Remove with unused dependencies`,
	Args: cobra.MinimumNArgs(1),
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "remove configuration files too")
	uninstallCmd.Flags().BoolVarP(&uninstallRecursive, "recursive", "r", false, "remove unused dependencies")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// Resolve aliases
	packages := resolvePackages(args)

	// Show what we're doing
	ui.InfoMsg("Removing %d package(s) using %s", len(packages), mgr.DisplayName())
	for _, pkg := range packages {
		ui.MutedMsg("  - %s", pkg)
	}

	if uninstallPurge {
		ui.WarningMsg("Configuration files will also be removed")
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with removal?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Capture pre-operation snapshot
	capturePreOperationSnapshot(ctx, snapshot.TriggerUninstall, packages)

	// Create history entry
	entry := history.NewEntry(history.OpUninstall, mgr.Name(), packages)

	// Build options
	opts := manager.UninstallOpts{
		AutoConfirm: cfg.General.AutoConfirm,
		DryRun:      cfg.General.DryRun,
		Purge:       uninstallPurge,
		Recursive:   uninstallRecursive,
	}

	// Execute removal
	err = mgr.Uninstall(ctx, packages, opts)

	// Update history
	if err != nil {
		entry.MarkFailed(err)
		ui.ErrorMsg("Removal failed: %v", err)
	} else {
		entry.MarkSuccess()
		ui.SuccessMsg("Successfully removed %d package(s)", len(packages))
	}

	// Record in history (ignore errors)
	if store, storeErr := history.Open(); storeErr == nil {
		_ = store.Record(entry) //nolint:errcheck
		_ = store.Close()       //nolint:errcheck
	}

	return err
}
