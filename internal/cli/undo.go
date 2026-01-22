package cli

import (
	"context"

	"poxy/internal/ui"
	"poxy/pkg/snapshot"

	"github.com/spf13/cobra"
)

var (
	undoSnapshotID string
	undoShowPlan   bool
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last package operation",
	Long: `Undo the last package operation by restoring the previous system state.

Uses snapshots to determine what changed and reverses those changes.
By default, undoes the most recent operation. Use --snapshot to restore
to a specific snapshot.

Examples:
  poxy undo                          # Undo last operation
  poxy undo --snapshot=20240114-153045   # Restore to specific snapshot
  poxy undo --plan                   # Show what would be undone without doing it`,
	RunE: runUndo,
}

func init() {
	undoCmd.Flags().StringVar(&undoSnapshotID, "snapshot", "", "specific snapshot ID to restore to")
	undoCmd.Flags().BoolVar(&undoShowPlan, "plan", false, "show what would be undone without executing")
}

func runUndo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	managers := getAvailableManagers()
	if len(managers) == 0 {
		return ErrNoManager
	}

	opts := snapshot.RestoreOpts{
		DryRun:      cfg.General.DryRun || undoShowPlan,
		AutoConfirm: cfg.General.AutoConfirm,
	}

	var plan *snapshot.RestorePlan
	var err error

	if undoSnapshotID != "" {
		// Restore to specific snapshot
		ui.InfoMsg("Restoring to snapshot %s", undoSnapshotID)
		plan, err = snapshot.RestoreToSnapshot(ctx, undoSnapshotID, managers, opts)
	} else {
		// Undo last operation
		plan, err = snapshot.Undo(ctx, managers, opts)
	}

	if err != nil {
		return err
	}

	if plan.IsEmpty() {
		ui.SuccessMsg("No changes needed - system already matches target state")
		return nil
	}

	// Show plan
	ui.HeaderMsg("Undo Plan")
	ui.InfoMsg("Restoring from snapshot %s to %s", plan.Diff.From, plan.Diff.To)
	ui.MutedMsg(plan.Summary())
	ui.Println("")

	// Show packages to install
	if len(plan.ToAdd) > 0 {
		ui.InfoMsg("Packages to reinstall:")
		for source, pkgs := range plan.ToAdd {
			for _, pkg := range pkgs {
				ui.MutedMsg("  + %s [%s]", pkg, source)
			}
		}
	}

	// Show packages to remove
	if len(plan.ToRemove) > 0 {
		ui.InfoMsg("Packages to remove:")
		for source, pkgs := range plan.ToRemove {
			for _, pkg := range pkgs {
				ui.MutedMsg("  - %s [%s]", pkg, source)
			}
		}
	}

	// If just showing plan, stop here
	if undoShowPlan || cfg.General.DryRun {
		ui.MutedMsg("")
		ui.MutedMsg("(dry run - no changes made)")
		return nil
	}

	// Confirm
	if !cfg.General.AutoConfirm {
		confirmed, err := ui.Confirm("Proceed with undo?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Execute restore
	executor := snapshot.NewExecutor(managers, opts)
	successful, execErr := executor.Execute(ctx, plan)

	if execErr != nil {
		ui.WarningMsg("Some operations failed: %v", execErr)
		ui.InfoMsg("Successfully processed %d package(s)", successful)
		return execErr
	}

	ui.SuccessMsg("Undo completed - processed %d package(s)", successful)
	return nil
}
