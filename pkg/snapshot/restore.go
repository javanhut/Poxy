package snapshot

import (
	"context"
	"fmt"
	"sort"

	"poxy/pkg/manager"
)

// RestoreOpts configures how a restore operation is performed.
type RestoreOpts struct {
	// DryRun only shows what would be done without making changes.
	DryRun bool

	// AutoConfirm skips confirmation prompts.
	AutoConfirm bool

	// SkipVersionCheck ignores version mismatches and just ensures packages exist.
	SkipVersionCheck bool

	// Sources limits the restore to specific package managers.
	Sources []string
}

// RestorePlan represents the operations needed to restore a snapshot.
type RestorePlan struct {
	Target   *Snapshot           // The snapshot we're restoring to
	Current  *Snapshot           // The current system state
	Diff     *Diff               // Difference between current and target
	ToAdd    map[string][]string // Packages to install, by source
	ToRemove map[string][]string // Packages to uninstall, by source
}

// IsEmpty returns true if no actions are needed.
func (p *RestorePlan) IsEmpty() bool {
	for _, pkgs := range p.ToAdd {
		if len(pkgs) > 0 {
			return false
		}
	}
	for _, pkgs := range p.ToRemove {
		if len(pkgs) > 0 {
			return false
		}
	}
	return true
}

// Summary returns a brief summary of the restore plan.
func (p *RestorePlan) Summary() string {
	addCount := 0
	removeCount := 0
	for _, pkgs := range p.ToAdd {
		addCount += len(pkgs)
	}
	for _, pkgs := range p.ToRemove {
		removeCount += len(pkgs)
	}

	if addCount == 0 && removeCount == 0 {
		return "No changes needed"
	}

	parts := []string{}
	if addCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to install", addCount))
	}
	if removeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to remove", removeCount))
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ", "
		}
		result += part
	}
	return result
}

// PlanRestore creates a plan to restore to a previous snapshot.
func PlanRestore(ctx context.Context, target *Snapshot, managers []manager.Manager, opts RestoreOpts) (*RestorePlan, error) {
	// Capture current state
	current, err := Capture(ctx, TriggerManual, "current state for restore", managers)
	if err != nil {
		return nil, fmt.Errorf("failed to capture current state: %w", err)
	}

	// Filter by source if specified
	filteredTarget := target
	filteredCurrent := current
	if len(opts.Sources) > 0 {
		sourceSet := make(map[string]bool)
		for _, s := range opts.Sources {
			sourceSet[s] = true
		}

		filteredTarget = filterSnapshot(target, sourceSet)
		filteredCurrent = filterSnapshot(current, sourceSet)
	}

	// Compute diff from current to target
	diff := DiffToRestore(filteredTarget, filteredCurrent)

	plan := &RestorePlan{
		Target:   target,
		Current:  current,
		Diff:     diff,
		ToAdd:    make(map[string][]string),
		ToRemove: make(map[string][]string),
	}

	// Process changes
	for _, change := range diff.Changes {
		switch change.Type {
		case ChangeAdded:
			// Package in target but not in current - need to install
			plan.ToAdd[change.Source] = append(plan.ToAdd[change.Source], change.Package)
		case ChangeRemoved:
			// Package in current but not in target - need to remove
			plan.ToRemove[change.Source] = append(plan.ToRemove[change.Source], change.Package)
		case ChangeDowngraded:
			// Version changed - for now, we just note this
			// Full version restore would require downgrade support
			// TODO: Add version mismatch handling when !opts.SkipVersionCheck
		}
	}

	// Sort packages for consistent ordering
	for source := range plan.ToAdd {
		sort.Strings(plan.ToAdd[source])
	}
	for source := range plan.ToRemove {
		sort.Strings(plan.ToRemove[source])
	}

	return plan, nil
}

// filterSnapshot returns a new snapshot with only packages from the specified sources.
func filterSnapshot(snap *Snapshot, sources map[string]bool) *Snapshot {
	filtered := &Snapshot{
		ID:          snap.ID,
		Timestamp:   snap.Timestamp,
		Description: snap.Description,
		Trigger:     snap.Trigger,
		Operation:   snap.Operation,
		Targets:     snap.Targets,
	}

	for _, pkg := range snap.Packages {
		if sources[pkg.Source] {
			filtered.Packages = append(filtered.Packages, pkg)
		}
	}

	return filtered
}

// Executor performs restore operations.
type Executor struct {
	managers map[string]manager.Manager
	opts     RestoreOpts
}

// NewExecutor creates a new restore executor.
func NewExecutor(managers []manager.Manager, opts RestoreOpts) *Executor {
	mgrMap := make(map[string]manager.Manager)
	for _, mgr := range managers {
		mgrMap[mgr.Name()] = mgr
	}

	return &Executor{
		managers: mgrMap,
		opts:     opts,
	}
}

// Execute performs the restore according to the plan.
// Returns the number of successful operations and any error.
func (e *Executor) Execute(ctx context.Context, plan *RestorePlan) (int, error) {
	if plan.IsEmpty() {
		return 0, nil
	}

	successful := 0
	var lastErr error

	// First, install missing packages (safer to install before removing)
	for source, packages := range plan.ToAdd {
		mgr, ok := e.managers[source]
		if !ok {
			lastErr = fmt.Errorf("package manager not available: %s", source)
			continue
		}

		if e.opts.DryRun {
			successful += len(packages)
			continue
		}

		opts := manager.InstallOpts{
			AutoConfirm: e.opts.AutoConfirm,
			DryRun:      e.opts.DryRun,
		}

		if err := mgr.Install(ctx, packages, opts); err != nil {
			lastErr = fmt.Errorf("failed to install packages from %s: %w", source, err)
		} else {
			successful += len(packages)
		}
	}

	// Then, remove extra packages
	for source, packages := range plan.ToRemove {
		mgr, ok := e.managers[source]
		if !ok {
			lastErr = fmt.Errorf("package manager not available: %s", source)
			continue
		}

		if e.opts.DryRun {
			successful += len(packages)
			continue
		}

		opts := manager.UninstallOpts{
			AutoConfirm: e.opts.AutoConfirm,
			DryRun:      e.opts.DryRun,
		}

		if err := mgr.Uninstall(ctx, packages, opts); err != nil {
			lastErr = fmt.Errorf("failed to remove packages from %s: %w", source, err)
		} else {
			successful += len(packages)
		}
	}

	return successful, lastErr
}

// Undo reverts the most recent operation by restoring the previous snapshot.
func Undo(ctx context.Context, managers []manager.Manager, opts RestoreOpts) (*RestorePlan, error) {
	store, err := OpenStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	// Get the two most recent snapshots
	snapshots, err := store.List(2, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snapshots) < 2 {
		return nil, fmt.Errorf("not enough snapshots to undo (need at least 2)")
	}

	// The most recent snapshot is the current state (post-operation)
	// The second-most-recent is the state before the operation
	target := &snapshots[1] // Restore to the state before

	return PlanRestore(ctx, target, managers, opts)
}

// RestoreToSnapshot restores the system to the specified snapshot.
func RestoreToSnapshot(ctx context.Context, snapshotID string, managers []manager.Manager, opts RestoreOpts) (*RestorePlan, error) {
	store, err := OpenStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot store: %w", err)
	}
	defer store.Close()

	target, err := store.Get(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	return PlanRestore(ctx, target, managers, opts)
}

// UndoResult contains the result of an undo operation.
type UndoResult struct {
	Plan       *RestorePlan
	Successful int
	Error      error
}

// PerformUndo executes an undo operation and returns the result.
func PerformUndo(ctx context.Context, managers []manager.Manager, opts RestoreOpts) (*UndoResult, error) {
	plan, err := Undo(ctx, managers, opts)
	if err != nil {
		return nil, err
	}

	executor := NewExecutor(managers, opts)
	successful, execErr := executor.Execute(ctx, plan)

	return &UndoResult{
		Plan:       plan,
		Successful: successful,
		Error:      execErr,
	}, nil
}
