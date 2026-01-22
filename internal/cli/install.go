package cli

import (
	"context"
	"fmt"
	"strings"

	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/manager/native"
	"poxy/pkg/snapshot"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [packages...]",
	Short: "Install one or more packages",
	Long: `Install packages using the detected system package manager
or a specified source.

Poxy will automatically search all available sources if a package
is not found in the native package manager.

Examples:
  poxy install vim git curl        # Install using native manager
  poxy install discord             # Auto-finds in AUR if not in repos
  poxy install firefox -s flatpak  # Explicitly install from Flatpak
  poxy install -y neovim           # Install without confirmation
  poxy install code                # Uses alias if configured`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve aliases
	packages := resolvePackages(args)

	// If source is explicitly specified, use that manager directly
	if source != "" {
		return installFromSource(ctx, packages, source)
	}

	// Smart install: try native first, then search other sources
	return smartInstall(ctx, packages)
}

// installFromSource installs packages from a specific source.
func installFromSource(ctx context.Context, packages []string, sourceName string) error {
	mgr, err := registry.GetManagerForSource(sourceName)
	if err != nil {
		return err
	}

	return doInstall(ctx, mgr, packages)
}

// smartInstall tries to find the best source for each package.
func smartInstall(ctx context.Context, packages []string) error {
	native := registry.Native()
	if native == nil {
		return ErrNoManager
	}

	// Group packages by their best source
	type packageSource struct {
		pkg    string
		mgr    manager.Manager
		reason string
	}

	var toInstall []packageSource
	var notFound []string

	for _, pkg := range packages {
		mgr, reason := findBestSource(ctx, pkg)
		if mgr != nil {
			toInstall = append(toInstall, packageSource{pkg: pkg, mgr: mgr, reason: reason})
		} else {
			notFound = append(notFound, pkg)
		}
	}

	// Report not found packages
	if len(notFound) > 0 {
		ui.WarningMsg("Could not find the following packages in any source:")
		for _, pkg := range notFound {
			ui.MutedMsg("  - %s", pkg)
		}
		if len(toInstall) == 0 {
			return fmt.Errorf("no packages found")
		}
	}

	// Group by manager for efficient installation
	byManager := make(map[string][]string)
	managerMap := make(map[string]manager.Manager)

	for _, ps := range toInstall {
		mgrName := ps.mgr.Name()
		byManager[mgrName] = append(byManager[mgrName], ps.pkg)
		managerMap[mgrName] = ps.mgr
	}

	// Show installation plan
	ui.InfoMsg("Installation plan:")
	for _, ps := range toInstall {
		ui.MutedMsg("  - %s from %s (%s)", ps.pkg, ps.mgr.DisplayName(), ps.reason)
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with installation?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Capture pre-operation snapshot
	allPackages := make([]string, 0, len(toInstall))
	for _, ps := range toInstall {
		allPackages = append(allPackages, ps.pkg)
	}
	capturePreOperationSnapshot(ctx, snapshot.TriggerInstall, allPackages)

	// Install from each manager
	var lastErr error
	for mgrName, pkgs := range byManager {
		mgr := managerMap[mgrName]
		if err := doInstallQuiet(ctx, mgr, pkgs); err != nil {
			ui.ErrorMsg("Failed to install from %s: %v", mgr.DisplayName(), err)
			lastErr = err
		}
	}

	return lastErr
}

// findBestSource finds the best source for a package.
// Returns the manager and a reason string.
func findBestSource(ctx context.Context, pkg string) (manager.Manager, string) {
	pkgLower := strings.ToLower(pkg)

	// Check package mappings first for known packages
	if searchEngine != nil {
		if mappedMgr, mappedName := findMappedPackage(ctx, pkg); mappedMgr != nil {
			if mappedName != pkg {
				return mappedMgr, fmt.Sprintf("mapped to '%s' in %s", mappedName, mappedMgr.DisplayName())
			}
			return mappedMgr, fmt.Sprintf("known package in %s", mappedMgr.DisplayName())
		}
	}

	// First, check if it's in the native repos
	native := registry.Native()
	if native != nil {
		// Try exact match via Info first (faster and more accurate)
		if info, err := native.Info(ctx, pkg); err == nil && info != nil {
			return native, "in official repos"
		}

		// Fall back to search
		results, err := native.Search(ctx, pkg, manager.SearchOpts{Limit: 50})
		if err == nil {
			for _, r := range results {
				if strings.ToLower(r.Name) == pkgLower {
					return native, "in official repos"
				}
			}
		}
	}

	// Search all available sources
	available := registry.Available()

	// Priority order: native > aur > flatpak > snap > others
	priority := map[string]int{
		"pacman":  1,
		"apt":     1,
		"dnf":     1,
		"brew":    1,
		"aur":     2,
		"flatpak": 3,
		"snap":    4,
	}

	type match struct {
		mgr      manager.Manager
		pkgName  string
		priority int
		score    int // 0 = exact, 1 = starts with, 2 = contains
	}

	var matches []match

	for _, mgr := range available {
		if native != nil && mgr.Name() == native.Name() {
			continue // Already checked native
		}

		// Try exact match via Info first (especially important for AUR)
		if info, err := mgr.Info(ctx, pkg); err == nil && info != nil {
			p := priority[mgr.Name()]
			if p == 0 {
				p = 10
			}
			matches = append(matches, match{
				mgr:      mgr,
				pkgName:  info.Name,
				priority: p,
				score:    0, // Exact match
			})
			continue
		}

		// Fall back to search with larger limit
		results, err := mgr.Search(ctx, pkg, manager.SearchOpts{Limit: 100})
		if err != nil {
			continue
		}

		for _, r := range results {
			rNameLower := strings.ToLower(r.Name)
			var score int = -1

			// Exact match
			if rNameLower == pkgLower {
				score = 0
			} else if strings.HasPrefix(rNameLower, pkgLower+"-") || strings.HasPrefix(rNameLower, pkgLower+"_") {
				// Starts with package name followed by separator (e.g., "spotify-bin")
				score = 1
			} else if strings.HasPrefix(rNameLower, pkgLower) {
				// Starts with package name
				score = 2
			}

			if score >= 0 {
				p := priority[mgr.Name()]
				if p == 0 {
					p = 10
				}
				matches = append(matches, match{
					mgr:      mgr,
					pkgName:  r.Name,
					priority: p,
					score:    score,
				})

				// If we found an exact match, no need to continue searching this source
				if score == 0 {
					break
				}
			}
		}
	}

	if len(matches) == 0 {
		return nil, ""
	}

	// Find best match:
	// 1. Prefer exact matches (score 0)
	// 2. Then prefer by priority (lower is better)
	// 3. Then prefer shorter names (usually the main package)
	best := matches[0]
	for _, m := range matches[1:] {
		// Better score always wins
		if m.score < best.score {
			best = m
		} else if m.score == best.score {
			// Same score: prefer lower priority (native > aur > flatpak)
			if m.priority < best.priority {
				best = m
			} else if m.priority == best.priority {
				// Same priority: prefer shorter name
				if len(m.pkgName) < len(best.pkgName) {
					best = m
				}
			}
		}
	}

	reason := fmt.Sprintf("found in %s", best.mgr.DisplayName())
	if best.pkgName != pkg {
		reason = fmt.Sprintf("'%s' in %s", best.pkgName, best.mgr.DisplayName())
	}
	return best.mgr, reason
}

// findMappedPackage checks if a package has a known mapping and finds it.
func findMappedPackage(ctx context.Context, pkg string) (manager.Manager, string) {
	mappings := searchEngine.GetMappings()
	if mappings == nil {
		return nil, ""
	}

	// Check if this is a canonical name
	mapping := mappings.GetByCanonical(pkg)
	if mapping == nil {
		// Maybe it's a source-specific name, try to find canonical
		for _, mgr := range registry.Available() {
			if m := mappings.GetBySourceName(mgr.Name(), pkg); m != nil {
				mapping = m
				break
			}
		}
	}

	if mapping == nil {
		return nil, ""
	}

	// Find the best available source for this mapping
	// Priority: native > aur > flatpak > snap
	native := registry.Native()

	// Try native first
	if native != nil {
		if mappedName, ok := mapping.Sources[native.Name()]; ok {
			// Verify it exists
			if info, err := native.Info(ctx, mappedName); err == nil && info != nil {
				return native, mappedName
			}
		}
	}

	// Try other sources in priority order
	priorities := []string{"aur", "flatpak", "snap", "brew", "winget"}
	for _, source := range priorities {
		if mappedName, ok := mapping.Sources[source]; ok {
			if mgr, ok := registry.Get(source); ok {
				// Verify it exists
				if info, err := mgr.Info(ctx, mappedName); err == nil && info != nil {
					return mgr, mappedName
				}
			}
		}
	}

	return nil, ""
}

// doInstall performs the installation with full UI feedback.
func doInstall(ctx context.Context, mgr manager.Manager, packages []string) error {
	ui.InfoMsg("Installing %d package(s) using %s", len(packages), mgr.DisplayName())
	for _, pkg := range packages {
		ui.MutedMsg("  - %s", pkg)
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with installation?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	return doInstallQuiet(ctx, mgr, packages)
}

// doInstallQuiet performs the installation without extra prompts.
func doInstallQuiet(ctx context.Context, mgr manager.Manager, packages []string) error {
	// Create history entry
	entry := history.NewEntry(history.OpInstall, mgr.Name(), packages)

	// Build options - always set AutoConfirm since poxy already confirmed with user
	opts := manager.InstallOpts{
		AutoConfirm: true,
		DryRun:      cfg.General.DryRun,
	}

	// Execute installation
	err := mgr.Install(ctx, packages, opts)

	// Check for pacman dependency conflicts and offer to help
	if err != nil {
		if handled, handledErr := handlePacmanConflict(ctx, mgr, packages, opts, err); handled {
			if handledErr == nil {
				entry.MarkSuccess()
				ui.SuccessMsg("Successfully installed %v from %s", packages, mgr.DisplayName())
			} else {
				entry.MarkFailed(handledErr)
			}
			// Record in history (ignore errors)
			if store, storeErr := history.Open(); storeErr == nil {
				_ = store.Record(entry) //nolint:errcheck
				_ = store.Close()       //nolint:errcheck
			}
			return handledErr
		}
	}

	// Update history
	if err != nil {
		entry.MarkFailed(err)
		ui.ErrorMsg("Installation failed: %v", err)
	} else {
		entry.MarkSuccess()
		ui.SuccessMsg("Successfully installed %v from %s", packages, mgr.DisplayName())
	}

	// Record in history (ignore errors)
	if store, storeErr := history.Open(); storeErr == nil {
		_ = store.Record(entry) //nolint:errcheck
		_ = store.Close()       //nolint:errcheck
	}

	return err
}

// handlePacmanConflict checks if the error is a pacman dependency conflict and offers
// to upgrade the system and retry. Returns (handled, error) where handled indicates
// whether this function handled the error (whether successfully or not).
func handlePacmanConflict(ctx context.Context, mgr manager.Manager, packages []string, opts manager.InstallOpts, err error) (bool, error) {
	pacErr, ok := native.IsPacmanDependencyConflict(err)
	if !ok {
		return false, nil
	}

	// Display the conflict message
	ui.WarningMsg(native.FormatDependencyConflictMessage(pacErr))

	// If not interactive (auto-confirm mode), just return the error
	if cfg.General.AutoConfirm {
		return false, nil
	}

	// Ask user if they want to upgrade and retry
	confirmed, confirmErr := ui.Confirm("Update system and retry installation?", true)
	if confirmErr != nil || !confirmed {
		return true, err // Return original error
	}

	// Run system upgrade
	ui.InfoMsg("Updating system...")
	upgradeOpts := manager.UpgradeOpts{
		AutoConfirm: true, // We already got confirmation
	}

	if upgradeErr := mgr.Upgrade(ctx, upgradeOpts); upgradeErr != nil {
		ui.ErrorMsg("System upgrade failed: %v", upgradeErr)
		return true, upgradeErr
	}

	ui.SuccessMsg("System updated successfully")

	// Retry installation
	ui.InfoMsg("Retrying installation...")
	retryErr := mgr.Install(ctx, packages, opts)
	if retryErr != nil {
		ui.ErrorMsg("Retry failed: %v", retryErr)
		return true, retryErr
	}

	return true, nil
}
