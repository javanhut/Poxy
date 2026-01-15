// Package manager provides the core abstraction for package managers across different operating systems.
package manager

import "time"

// ManagerType represents the category of package manager.
type ManagerType string

const (
	// TypeNative represents system-native package managers (apt, dnf, pacman, etc.)
	TypeNative ManagerType = "native"
	// TypeUniversal represents cross-distribution package managers (flatpak, snap)
	TypeUniversal ManagerType = "universal"
	// TypeAUR represents Arch User Repository helpers (yay, paru)
	TypeAUR ManagerType = "aur"
)

// Package represents a software package from any source.
type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Source      string `json:"source"`    // Manager name: "apt", "flatpak", etc.
	Installed   bool   `json:"installed"` // Whether the package is currently installed
	Size        string `json:"size"`      // Optional: download/install size
}

// PackageInfo contains detailed information about a package.
type PackageInfo struct {
	Package
	Repository   string    `json:"repository"`
	Maintainer   string    `json:"maintainer"`
	License      string    `json:"license"`
	URL          string    `json:"url"`
	Dependencies []string  `json:"dependencies"`
	InstallDate  time.Time `json:"install_date"` // If installed
}

// InstallOpts contains options for package installation.
type InstallOpts struct {
	AutoConfirm bool // Automatically confirm prompts
	DryRun      bool // Show what would happen without executing
	Reinstall   bool // Reinstall if already installed
}

// UninstallOpts contains options for package removal.
type UninstallOpts struct {
	AutoConfirm bool // Automatically confirm prompts
	DryRun      bool // Show what would happen without executing
	Purge       bool // Remove configuration files too
	Recursive   bool // Remove unused dependencies
}

// UpgradeOpts contains options for package upgrades.
type UpgradeOpts struct {
	AutoConfirm bool     // Automatically confirm prompts
	DryRun      bool     // Show what would happen without executing
	Packages    []string // Specific packages to upgrade (empty = upgrade all)
}

// SearchOpts contains options for package search.
type SearchOpts struct {
	Limit         int  // Maximum number of results
	InstalledOnly bool // Only show installed packages
	SearchInDesc  bool // Search in package descriptions too
	ExactMatch    bool // Require exact name match
}

// CleanOpts contains options for cache cleaning.
type CleanOpts struct {
	DryRun bool // Show what would happen without executing
	All    bool // Clean all cached data, not just old versions
}

// ListOpts contains options for listing packages.
type ListOpts struct {
	Limit         int    // Maximum number of results
	InstalledOnly bool   // Only show installed packages
	Upgradable    bool   // Only show packages with available upgrades
	Pattern       string // Filter by name pattern
}
