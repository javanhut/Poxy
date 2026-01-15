package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"poxy/internal/ui"
	"poxy/pkg/snapshot"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage system state snapshots",
	Long: `Manage system state snapshots for rollback and recovery.

Snapshots capture the list of installed packages from all package managers.
They are automatically created before install/uninstall/upgrade operations
and can be used to restore the system to a previous state.

Examples:
  poxy snapshot list                # List available snapshots
  poxy snapshot create              # Create a manual snapshot
  poxy snapshot show <id>           # Show details of a snapshot
  poxy snapshot diff <id1> <id2>    # Compare two snapshots
  poxy snapshot delete <id>         # Delete a snapshot
  poxy snapshot prune               # Remove old snapshots`,
}

func init() {
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotShowCmd)
	snapshotCmd.AddCommand(snapshotDiffCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotCmd.AddCommand(snapshotPruneCmd)
}

// snapshotListCmd lists available snapshots
var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available snapshots",
	Long: `List all available snapshots, showing the most recent first.

Use --limit to control how many snapshots to show.
Use --trigger to filter by trigger type (manual, install, uninstall, upgrade).`,
	RunE: runSnapshotList,
}

var (
	snapshotListLimit   int
	snapshotListTrigger string
)

func init() {
	snapshotListCmd.Flags().IntVarP(&snapshotListLimit, "limit", "l", 20, "maximum number of snapshots to list")
	snapshotListCmd.Flags().StringVarP(&snapshotListTrigger, "trigger", "t", "", "filter by trigger type")
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	store, err := snapshot.OpenStore()
	if err != nil {
		return fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	trigger := snapshot.Trigger(snapshotListTrigger)
	snapshots, err := store.List(snapshotListLimit, trigger)
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		ui.InfoMsg("No snapshots available")
		ui.MutedMsg("Snapshots are created automatically before install/uninstall/upgrade operations.")
		ui.MutedMsg("Create a manual snapshot with: poxy snapshot create")
		return nil
	}

	ui.HeaderMsg("Available Snapshots")
	ui.Println("")

	for _, snap := range snapshots {
		triggerStr := string(snap.Trigger)
		desc := snap.Description
		if desc == "" {
			desc = "-"
		}

		// Color code by trigger type
		switch snap.Trigger {
		case snapshot.TriggerManual:
			ui.Println("  %s  %-10s  %4d pkgs  %s",
				ui.Cyan(snap.ID), ui.Green("manual"), snap.PackageCount(), desc)
		case snapshot.TriggerInstall:
			ui.Println("  %s  %-10s  %4d pkgs  %s",
				ui.Cyan(snap.ID), "install", snap.PackageCount(), desc)
		case snapshot.TriggerUninstall:
			ui.Println("  %s  %-10s  %4d pkgs  %s",
				ui.Cyan(snap.ID), ui.Yellow("uninstall"), snap.PackageCount(), desc)
		case snapshot.TriggerUpgrade:
			ui.Println("  %s  %-10s  %4d pkgs  %s",
				ui.Cyan(snap.ID), "upgrade", snap.PackageCount(), desc)
		default:
			ui.Println("  %s  %-10s  %4d pkgs  %s",
				ui.Cyan(snap.ID), triggerStr, snap.PackageCount(), desc)
		}
	}

	count, _ := store.Count()
	ui.MutedMsg("")
	ui.MutedMsg("Showing %d of %d total snapshots", len(snapshots), count)

	return nil
}

// snapshotCreateCmd creates a manual snapshot
var snapshotCreateCmd = &cobra.Command{
	Use:   "create [description]",
	Short: "Create a manual snapshot",
	Long: `Create a manual snapshot of the current system state.

Manual snapshots are not automatically pruned and must be deleted manually.`,
	RunE: runSnapshotCreate,
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	managers := getAvailableManagers()
	if len(managers) == 0 {
		return ErrNoManager
	}

	description := "manual snapshot"
	if len(args) > 0 {
		description = args[0]
	}

	ui.InfoMsg("Creating snapshot...")

	snap, err := snapshot.CaptureAndSave(ctx, snapshot.TriggerManual, description, managers)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	ui.SuccessMsg("Created snapshot %s with %d packages", snap.ID, snap.PackageCount())

	// Show breakdown by source
	bySource := snap.PackagesBySource()
	for source, pkgs := range bySource {
		ui.MutedMsg("  %s: %d packages", source, len(pkgs))
	}

	return nil
}

// snapshotShowCmd shows details of a snapshot
var snapshotShowCmd = &cobra.Command{
	Use:   "show <snapshot-id>",
	Short: "Show details of a snapshot",
	Long:  `Show detailed information about a specific snapshot.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotShow,
}

func runSnapshotShow(cmd *cobra.Command, args []string) error {
	store, err := snapshot.OpenStore()
	if err != nil {
		return fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	snap, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("snapshot not found: %s", args[0])
	}

	ui.HeaderMsg("Snapshot: %s", snap.ID)
	ui.Println("")
	ui.Println("  Timestamp:   %s", snap.FormatTime())
	ui.Println("  Trigger:     %s", snap.Trigger)
	ui.Println("  Description: %s", snap.Description)
	ui.Println("  Packages:    %d total", snap.PackageCount())
	ui.Println("")

	// Show packages by source
	bySource := snap.PackagesBySource()
	for source, pkgs := range bySource {
		ui.InfoMsg("%s (%d packages)", source, len(pkgs))
		for _, pkg := range pkgs {
			ui.MutedMsg("  %s %s", pkg.Name, ui.Green(pkg.Version))
		}
		ui.Println("")
	}

	return nil
}

// snapshotDiffCmd compares two snapshots
var snapshotDiffCmd = &cobra.Command{
	Use:   "diff <snapshot-id-1> <snapshot-id-2>",
	Short: "Compare two snapshots",
	Long: `Compare two snapshots and show the differences.

The first snapshot is treated as the "from" state and the second as "to".
This shows what changed between the two states.`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshotDiff,
}

func runSnapshotDiff(cmd *cobra.Command, args []string) error {
	store, err := snapshot.OpenStore()
	if err != nil {
		return fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	snap1, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("snapshot not found: %s", args[0])
	}

	snap2, err := store.Get(args[1])
	if err != nil {
		return fmt.Errorf("snapshot not found: %s", args[1])
	}

	diff := snapshot.Compare(snap1, snap2)

	ui.HeaderMsg("Diff: %s -> %s", snap1.ID, snap2.ID)
	ui.Println("")

	if diff.IsEmpty() {
		ui.SuccessMsg("No differences - snapshots are identical")
		return nil
	}

	ui.InfoMsg(diff.Summary())
	ui.Println("")

	// Show added packages
	if added := diff.Added(); len(added) > 0 {
		ui.InfoMsg("Added packages:")
		for _, c := range added {
			ui.Println("  %s %s [%s]", ui.Green("+"), c.Package, c.Source)
		}
		ui.Println("")
	}

	// Show removed packages
	if removed := diff.Removed(); len(removed) > 0 {
		ui.InfoMsg("Removed packages:")
		for _, c := range removed {
			ui.Println("  %s %s [%s]", ui.Red("-"), c.Package, c.Source)
		}
		ui.Println("")
	}

	// Show upgraded packages
	if upgraded := diff.Upgraded(); len(upgraded) > 0 {
		ui.InfoMsg("Upgraded packages:")
		for _, c := range upgraded {
			ui.Println("  %s %s: %s -> %s [%s]",
				ui.Cyan("^"), c.Package, c.OldVersion, c.NewVersion, c.Source)
		}
		ui.Println("")
	}

	// Show downgraded packages
	if downgraded := diff.Downgraded(); len(downgraded) > 0 {
		ui.InfoMsg("Downgraded packages:")
		for _, c := range downgraded {
			ui.Println("  %s %s: %s -> %s [%s]",
				ui.Yellow("v"), c.Package, c.OldVersion, c.NewVersion, c.Source)
		}
		ui.Println("")
	}

	return nil
}

// snapshotDeleteCmd deletes a snapshot
var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete <snapshot-id>",
	Short: "Delete a snapshot",
	Long:  `Delete a specific snapshot from the store.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotDelete,
}

func runSnapshotDelete(cmd *cobra.Command, args []string) error {
	store, err := snapshot.OpenStore()
	if err != nil {
		return fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	// Verify snapshot exists
	snap, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("snapshot not found: %s", args[0])
	}

	// Confirm deletion
	if !cfg.General.AutoConfirm {
		ui.WarningMsg("About to delete snapshot: %s (%s)", snap.ID, snap.Description)
		confirmed, err := ui.Confirm("Delete this snapshot?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	if err := store.Delete(args[0]); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	ui.SuccessMsg("Deleted snapshot %s", args[0])
	return nil
}

// snapshotPruneCmd removes old snapshots
var snapshotPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old snapshots",
	Long: `Remove old automatic snapshots to save space.

By default, keeps the 50 most recent snapshots overall and
20 most recent automatic snapshots. Manual snapshots are never
automatically pruned.`,
	RunE: runSnapshotPrune,
}

var (
	pruneKeep     int
	pruneAutoKeep int
)

func init() {
	snapshotPruneCmd.Flags().IntVar(&pruneKeep, "keep", 50, "number of total snapshots to keep")
	snapshotPruneCmd.Flags().IntVar(&pruneAutoKeep, "keep-auto", 20, "number of automatic snapshots to keep")
}

func runSnapshotPrune(cmd *cobra.Command, args []string) error {
	store, err := snapshot.OpenStore()
	if err != nil {
		return fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	beforeCount, _ := store.Count()

	deleted, err := store.Prune(pruneKeep, pruneAutoKeep)
	if err != nil {
		return fmt.Errorf("failed to prune snapshots: %w", err)
	}

	if deleted == 0 {
		ui.InfoMsg("No snapshots to prune (keeping %d of %d)", beforeCount, beforeCount)
	} else {
		ui.SuccessMsg("Pruned %d old snapshot(s)", deleted)
	}

	afterCount, _ := store.Count()
	ui.MutedMsg("Remaining snapshots: %d", afterCount)

	return nil
}
