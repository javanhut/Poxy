package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
)

var rollbackID string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Undo last operation",
	Long: `Undo the last reversible package operation.

Only install and uninstall operations can be rolled back.
Update, upgrade, and clean operations cannot be undone.

Examples:
  poxy rollback             # Undo last reversible operation
  poxy rollback --id=xyz    # Undo specific operation by ID`,
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().StringVar(&rollbackID, "id", "", "specific operation ID to rollback")
}

func runRollback(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := history.Open()
	if err != nil {
		return fmt.Errorf("failed to open history: %w", err)
	}
	defer store.Close()

	// Find the operation to rollback
	var entry *history.Entry

	if rollbackID != "" {
		entry, err = store.Get(rollbackID)
		if err != nil {
			return fmt.Errorf("operation not found: %s", rollbackID)
		}
	} else {
		entry, err = store.LastReversible()
		if err != nil {
			return fmt.Errorf("no reversible operations found")
		}
	}

	if !entry.CanRollback() {
		return fmt.Errorf("operation cannot be rolled back: %s %v", entry.Operation, entry.Packages)
	}

	// Get the package manager
	mgr, ok := registry.Get(entry.Source)
	if !ok {
		return fmt.Errorf("package manager not available: %s", entry.Source)
	}

	// Show what we're doing
	ui.HeaderMsg("Rolling back: %s", entry.Summary())
	ui.InfoMsg("Reverse operation: %s", entry.ReverseOp)
	for _, pkg := range entry.Packages {
		ui.MutedMsg("  - %s", pkg)
	}

	// Confirm
	if !cfg.General.AutoConfirm {
		confirmed, err := ui.Confirm("Proceed with rollback?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Execute rollback
	switch entry.ReverseOp {
	case history.OpInstall:
		opts := manager.InstallOpts{
			AutoConfirm: cfg.General.AutoConfirm,
			DryRun:      cfg.General.DryRun,
		}
		err = mgr.Install(ctx, entry.Packages, opts)

	case history.OpUninstall:
		opts := manager.UninstallOpts{
			AutoConfirm: cfg.General.AutoConfirm,
			DryRun:      cfg.General.DryRun,
		}
		err = mgr.Uninstall(ctx, entry.Packages, opts)

	default:
		return fmt.Errorf("unsupported reverse operation: %s", entry.ReverseOp)
	}

	if err != nil {
		ui.ErrorMsg("Rollback failed: %v", err)
		return err
	}

	ui.SuccessMsg("Rollback completed successfully")
	return nil
}
