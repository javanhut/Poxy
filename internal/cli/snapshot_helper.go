package cli

import (
	"context"
	"fmt"

	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/snapshot"
)

// capturePreOperationSnapshot captures a snapshot before a package operation.
// Returns the snapshot or nil if snapshotting is disabled or fails.
func capturePreOperationSnapshot(ctx context.Context, trigger snapshot.Trigger, targets []string) *snapshot.Snapshot {
	// Check if snapshots are enabled
	if cfg != nil && !cfg.General.Snapshots {
		return nil
	}

	// Get all available managers
	managers := getAvailableManagers()
	if len(managers) == 0 {
		return nil
	}

	// Build description
	description := fmt.Sprintf("before %s", trigger)
	if len(targets) > 0 {
		if len(targets) == 1 {
			description = fmt.Sprintf("before %s %s", trigger, targets[0])
		} else {
			description = fmt.Sprintf("before %s %d packages", trigger, len(targets))
		}
	}

	// Capture snapshot
	snap, err := snapshot.CaptureAndSave(ctx, trigger, description, managers)
	if err != nil {
		if verbose {
			ui.WarningMsg("Failed to capture snapshot: %v", err)
		}
		return nil
	}

	if verbose {
		ui.MutedMsg("Captured snapshot %s (%d packages)", snap.ID, snap.PackageCount())
	}

	// Store operation metadata in the snapshot
	snap.Operation = string(trigger)
	snap.Targets = targets

	// Update the snapshot with metadata
	store, err := snapshot.OpenStore()
	if err == nil {
		_ = store.Save(snap) //nolint:errcheck
		_ = store.Close()    //nolint:errcheck
	}

	return snap
}

// getAvailableManagers returns all currently available package managers.
func getAvailableManagers() []manager.Manager {
	if registry == nil {
		return nil
	}

	return registry.Available()
}
