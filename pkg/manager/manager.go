package manager

import "context"

// Manager defines the interface that all package managers must implement.
// This provides a unified API across all supported package managers.
type Manager interface {
	// Metadata returns information about this package manager.

	// Name returns the short identifier for this manager (e.g., "apt", "pacman").
	Name() string

	// DisplayName returns a human-readable name (e.g., "APT (Debian/Ubuntu)").
	DisplayName() string

	// Type returns the category of this manager (native, universal, aur).
	Type() ManagerType

	// Availability checks.

	// IsAvailable returns true if this package manager is installed and usable.
	IsAvailable() bool

	// NeedsSudo returns true if this manager requires root privileges for most operations.
	NeedsSudo() bool

	// Core package operations.

	// Install installs one or more packages.
	Install(ctx context.Context, packages []string, opts InstallOpts) error

	// Uninstall removes one or more packages.
	Uninstall(ctx context.Context, packages []string, opts UninstallOpts) error

	// Update refreshes the package database/repository cache.
	Update(ctx context.Context) error

	// Upgrade upgrades installed packages to their latest versions.
	Upgrade(ctx context.Context, opts UpgradeOpts) error

	// Package discovery.

	// Search finds packages matching the query.
	Search(ctx context.Context, query string, opts SearchOpts) ([]Package, error)

	// Info returns detailed information about a specific package.
	Info(ctx context.Context, pkg string) (*PackageInfo, error)

	// ListInstalled returns all installed packages.
	ListInstalled(ctx context.Context, opts ListOpts) ([]Package, error)

	// IsInstalled checks if a specific package is installed.
	IsInstalled(ctx context.Context, pkg string) (bool, error)

	// Maintenance operations.

	// Clean removes cached package files.
	Clean(ctx context.Context, opts CleanOpts) error

	// Autoremove removes orphaned packages (dependencies no longer needed).
	Autoremove(ctx context.Context) error
}

// ManagerInfo provides static information about a manager without requiring instantiation.
type ManagerInfo struct {
	Name        string
	DisplayName string
	Type        ManagerType
	Binary      string   // Primary binary to check for availability
	Distros     []string // Linux distributions this manager is native to
}
