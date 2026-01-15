package snapshot

import (
	"fmt"
	"sort"
)

// ChangeType represents the type of change between snapshots.
type ChangeType string

const (
	ChangeAdded      ChangeType = "added"      // Package was installed
	ChangeRemoved    ChangeType = "removed"    // Package was uninstalled
	ChangeUpgraded   ChangeType = "upgraded"   // Package version changed (newer)
	ChangeDowngraded ChangeType = "downgraded" // Package version changed (older)
)

// Change represents a single package change between snapshots.
type Change struct {
	Type       ChangeType `json:"type"`
	Package    string     `json:"package"`
	Source     string     `json:"source"`
	OldVersion string     `json:"old_version,omitempty"`
	NewVersion string     `json:"new_version,omitempty"`
}

// String returns a human-readable description of the change.
func (c Change) String() string {
	switch c.Type {
	case ChangeAdded:
		return fmt.Sprintf("+ %s (%s) [%s]", c.Package, c.NewVersion, c.Source)
	case ChangeRemoved:
		return fmt.Sprintf("- %s (%s) [%s]", c.Package, c.OldVersion, c.Source)
	case ChangeUpgraded:
		return fmt.Sprintf("^ %s: %s -> %s [%s]", c.Package, c.OldVersion, c.NewVersion, c.Source)
	case ChangeDowngraded:
		return fmt.Sprintf("v %s: %s -> %s [%s]", c.Package, c.OldVersion, c.NewVersion, c.Source)
	default:
		return fmt.Sprintf("? %s [%s]", c.Package, c.Source)
	}
}

// Diff represents the difference between two snapshots.
type Diff struct {
	From    string   `json:"from"` // ID of the older snapshot
	To      string   `json:"to"`   // ID of the newer snapshot
	Changes []Change `json:"changes"`
}

// IsEmpty returns true if there are no changes.
func (d *Diff) IsEmpty() bool {
	return len(d.Changes) == 0
}

// Added returns all packages that were added.
func (d *Diff) Added() []Change {
	var result []Change
	for _, c := range d.Changes {
		if c.Type == ChangeAdded {
			result = append(result, c)
		}
	}
	return result
}

// Removed returns all packages that were removed.
func (d *Diff) Removed() []Change {
	var result []Change
	for _, c := range d.Changes {
		if c.Type == ChangeRemoved {
			result = append(result, c)
		}
	}
	return result
}

// Upgraded returns all packages that were upgraded.
func (d *Diff) Upgraded() []Change {
	var result []Change
	for _, c := range d.Changes {
		if c.Type == ChangeUpgraded {
			result = append(result, c)
		}
	}
	return result
}

// Downgraded returns all packages that were downgraded.
func (d *Diff) Downgraded() []Change {
	var result []Change
	for _, c := range d.Changes {
		if c.Type == ChangeDowngraded {
			result = append(result, c)
		}
	}
	return result
}

// BySource groups changes by their source manager.
func (d *Diff) BySource() map[string][]Change {
	result := make(map[string][]Change)
	for _, c := range d.Changes {
		result[c.Source] = append(result[c.Source], c)
	}
	return result
}

// Summary returns a brief summary of the diff.
func (d *Diff) Summary() string {
	added := len(d.Added())
	removed := len(d.Removed())
	upgraded := len(d.Upgraded())
	downgraded := len(d.Downgraded())

	if d.IsEmpty() {
		return "No changes"
	}

	parts := []string{}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d added", added))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("-%d removed", removed))
	}
	if upgraded > 0 {
		parts = append(parts, fmt.Sprintf("^%d upgraded", upgraded))
	}
	if downgraded > 0 {
		parts = append(parts, fmt.Sprintf("v%d downgraded", downgraded))
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

// Compare computes the difference between two snapshots.
// 'from' is the older/previous snapshot, 'to' is the newer/current snapshot.
func Compare(from, to *Snapshot) *Diff {
	diff := &Diff{
		From:    from.ID,
		To:      to.ID,
		Changes: []Change{},
	}

	// Build lookup maps for efficient comparison
	fromMap := make(map[string]PackageState)
	toMap := make(map[string]PackageState)

	for _, pkg := range from.Packages {
		key := pkg.Source + "/" + pkg.Name
		fromMap[key] = pkg
	}

	for _, pkg := range to.Packages {
		key := pkg.Source + "/" + pkg.Name
		toMap[key] = pkg
	}

	// Find added and changed packages
	for key, toPkg := range toMap {
		fromPkg, exists := fromMap[key]
		if !exists {
			// Package was added
			diff.Changes = append(diff.Changes, Change{
				Type:       ChangeAdded,
				Package:    toPkg.Name,
				Source:     toPkg.Source,
				NewVersion: toPkg.Version,
			})
		} else if fromPkg.Version != toPkg.Version {
			// Package version changed
			changeType := ChangeUpgraded
			if compareVersions(fromPkg.Version, toPkg.Version) > 0 {
				changeType = ChangeDowngraded
			}
			diff.Changes = append(diff.Changes, Change{
				Type:       changeType,
				Package:    toPkg.Name,
				Source:     toPkg.Source,
				OldVersion: fromPkg.Version,
				NewVersion: toPkg.Version,
			})
		}
	}

	// Find removed packages
	for key, fromPkg := range fromMap {
		if _, exists := toMap[key]; !exists {
			diff.Changes = append(diff.Changes, Change{
				Type:       ChangeRemoved,
				Package:    fromPkg.Name,
				Source:     fromPkg.Source,
				OldVersion: fromPkg.Version,
			})
		}
	}

	// Sort changes for consistent output
	sort.Slice(diff.Changes, func(i, j int) bool {
		// Sort by type first (added, removed, upgraded, downgraded)
		if diff.Changes[i].Type != diff.Changes[j].Type {
			order := map[ChangeType]int{
				ChangeAdded:      1,
				ChangeRemoved:    2,
				ChangeUpgraded:   3,
				ChangeDowngraded: 4,
			}
			return order[diff.Changes[i].Type] < order[diff.Changes[j].Type]
		}
		// Then by source
		if diff.Changes[i].Source != diff.Changes[j].Source {
			return diff.Changes[i].Source < diff.Changes[j].Source
		}
		// Then by package name
		return diff.Changes[i].Package < diff.Changes[j].Package
	})

	return diff
}

// Invert returns a diff that would undo this diff's changes.
// Added packages become removed, removed become added, etc.
func (d *Diff) Invert() *Diff {
	inverted := &Diff{
		From:    d.To,
		To:      d.From,
		Changes: make([]Change, len(d.Changes)),
	}

	for i, c := range d.Changes {
		switch c.Type {
		case ChangeAdded:
			inverted.Changes[i] = Change{
				Type:       ChangeRemoved,
				Package:    c.Package,
				Source:     c.Source,
				OldVersion: c.NewVersion,
			}
		case ChangeRemoved:
			inverted.Changes[i] = Change{
				Type:       ChangeAdded,
				Package:    c.Package,
				Source:     c.Source,
				NewVersion: c.OldVersion,
			}
		case ChangeUpgraded:
			inverted.Changes[i] = Change{
				Type:       ChangeDowngraded,
				Package:    c.Package,
				Source:     c.Source,
				OldVersion: c.NewVersion,
				NewVersion: c.OldVersion,
			}
		case ChangeDowngraded:
			inverted.Changes[i] = Change{
				Type:       ChangeUpgraded,
				Package:    c.Package,
				Source:     c.Source,
				OldVersion: c.NewVersion,
				NewVersion: c.OldVersion,
			}
		}
	}

	return inverted
}

// compareVersions does a simple version comparison.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// This is a simple lexicographic comparison; for more accurate
// version comparison, we'd need a more sophisticated algorithm.
func compareVersions(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// DiffFromCurrent creates a diff between a snapshot and the current system state.
func DiffFromCurrent(from *Snapshot, current *Snapshot) *Diff {
	return Compare(from, current)
}

// DiffToRestore creates a diff that would restore the system to a previous state.
func DiffToRestore(target *Snapshot, current *Snapshot) *Diff {
	return Compare(current, target)
}
