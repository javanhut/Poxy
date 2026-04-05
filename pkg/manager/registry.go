package manager

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"poxy/internal/config"
	"poxy/pkg/manager/detector"
)

// Registry manages all available package managers and provides unified access.
type Registry struct {
	managers map[string]Manager
	native   Manager
	sysInfo  *detector.SystemInfo
	cfg      *config.Config
	mu       sync.RWMutex
}

// NewRegistry creates a new package manager registry.
func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		managers: make(map[string]Manager),
		cfg:      cfg,
	}
}

// Register adds a manager to the registry.
func (r *Registry) Register(mgr Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.managers[mgr.Name()] = mgr
}

// Detect detects the system and identifies available package managers.
func (r *Registry) Detect() error {
	info, err := detector.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect system: %w", err)
	}
	r.sysInfo = info

	// Determine native package manager based on OS
	var nativeName string
	switch info.OS {
	case detector.OSLinux:
		nativeName = detector.GetNativeManagerForFamily(info.Distribution, info.DistroFamily)
	case detector.OSDarwin:
		nativeName = detector.GetDarwinManager()
	case detector.OSWindows:
		nativeName = detector.GetWindowsManager()
	}

	if nativeName != "" {
		if mgr, ok := r.managers[nativeName]; ok && mgr.IsAvailable() {
			r.native = mgr
		}
	}

	return nil
}

// Native returns the detected native package manager for this system.
func (r *Registry) Native() Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.native
}

// Get returns a specific manager by name.
func (r *Registry) Get(name string) (Manager, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mgr, ok := r.managers[name]
	return mgr, ok
}

// Available returns all available (installed) package managers.
func (r *Registry) Available() []Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []Manager
	for _, mgr := range r.managers {
		if mgr.IsAvailable() {
			available = append(available, mgr)
		}
	}

	// Sort by priority from config
	r.sortByPriority(available)
	return available
}

// AvailableByType returns available managers of a specific type.
func (r *Registry) AvailableByType(t ManagerType) []Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []Manager
	for _, mgr := range r.managers {
		if mgr.IsAvailable() && mgr.Type() == t {
			available = append(available, mgr)
		}
	}

	r.sortByPriority(available)
	return available
}

// All returns all registered managers (including unavailable ones).
func (r *Registry) All() []Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	managers := make([]Manager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	return managers
}

// SystemInfo returns the detected system information.
func (r *Registry) SystemInfo() *detector.SystemInfo {
	return r.sysInfo
}

// SearchAll searches for packages across all available managers concurrently.
func (r *Registry) SearchAll(ctx context.Context, query string, opts SearchOpts) ([]Package, error) {
	available := r.Available()
	if len(available) == 0 {
		return nil, fmt.Errorf("no package managers available")
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		results  []Package
		firstErr error
	)

	for _, mgr := range available {
		wg.Add(1)
		go func(m Manager) {
			defer wg.Done()

			pkgs, err := m.Search(ctx, query, opts)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", m.Name(), err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results = append(results, pkgs...)
			mu.Unlock()
		}(mgr)
	}

	wg.Wait()

	// Sort results by source priority
	r.sortPackagesByPriority(results)

	return results, firstErr
}

// GetManagerForSource returns the appropriate manager for a source string.
// Source can be a manager name (e.g., "apt") or a type (e.g., "native").
func (r *Registry) GetManagerForSource(source string) (Manager, error) {
	// Check if it's a direct manager name
	if mgr, ok := r.Get(source); ok {
		if !mgr.IsAvailable() {
			return nil, fmt.Errorf("package manager '%s' is not available on this system", source)
		}
		return mgr, nil
	}

	// Check if it's a type or alias
	switch source {
	case "native":
		if r.native == nil {
			return nil, fmt.Errorf("no native package manager detected")
		}
		return r.native, nil
	case "universal":
		managers := r.AvailableByType(TypeUniversal)
		if len(managers) == 0 {
			return nil, fmt.Errorf("no universal package managers available")
		}
		return managers[0], nil
	case "aur", "yay", "paru", "trizen", "aurman":
		managers := r.AvailableByType(TypeAUR)
		if len(managers) == 0 {
			return nil, fmt.Errorf("no AUR helpers available")
		}
		return managers[0], nil
	}

	return nil, fmt.Errorf("unknown package source: %s", source)
}

// sortByPriority sorts managers based on the configured priority order.
func (r *Registry) sortByPriority(managers []Manager) {
	if r.cfg == nil {
		return
	}

	priority := make(map[string]int)
	for i, name := range r.cfg.General.SourcePriority {
		priority[name] = i
	}

	sort.SliceStable(managers, func(i, j int) bool {
		// Get priority for each manager (type or name)
		pi := r.getPriority(managers[i], priority)
		pj := r.getPriority(managers[j], priority)
		return pi < pj
	})
}

// sortPackagesByPriority sorts packages based on their source manager's priority.
func (r *Registry) sortPackagesByPriority(packages []Package) {
	if r.cfg == nil {
		return
	}

	priority := make(map[string]int)
	for i, name := range r.cfg.General.SourcePriority {
		priority[name] = i
	}

	sort.SliceStable(packages, func(i, j int) bool {
		pi := priority[packages[i].Source]
		pj := priority[packages[j].Source]
		if pi != pj {
			return pi < pj
		}
		// Secondary sort by package name
		return packages[i].Name < packages[j].Name
	})
}

// getPriority returns the priority index for a manager.
func (r *Registry) getPriority(mgr Manager, priorityMap map[string]int) int {
	// Check by name first
	if p, ok := priorityMap[mgr.Name()]; ok {
		return p
	}

	// Check by type
	switch mgr.Type() {
	case TypeNative:
		if p, ok := priorityMap["native"]; ok {
			return p
		}
	case TypeUniversal:
		if p, ok := priorityMap["universal"]; ok {
			return p
		}
		if p, ok := priorityMap["flatpak"]; ok && mgr.Name() == "flatpak" {
			return p
		}
		if p, ok := priorityMap["snap"]; ok && mgr.Name() == "snap" {
			return p
		}
	case TypeAUR:
		if p, ok := priorityMap["aur"]; ok {
			return p
		}
	}

	// Default to lowest priority
	return 999
}
