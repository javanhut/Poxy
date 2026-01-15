package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Chocolatey implements the Manager interface for Chocolatey (Windows).
type Chocolatey struct {
	*BaseManager
}

// NewChocolatey creates a new Chocolatey manager instance.
func NewChocolatey() *Chocolatey {
	return &Chocolatey{
		BaseManager: NewBaseManager("chocolatey", "Chocolatey", "choco", false),
	}
}

// Install installs one or more packages.
func (c *Chocolatey) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}
	args = append(args, packages...)

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if opts.Reinstall {
		args = append(args, "--force")
	}

	if opts.DryRun {
		c.SetDryRun(true)
		defer c.SetDryRun(false)
	}

	return c.Executor().Run(ctx, c.Binary(), args...)
}

// Uninstall removes one or more packages.
func (c *Chocolatey) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"uninstall"}
	args = append(args, packages...)

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if opts.Recursive {
		args = append(args, "-x") // Remove dependencies
	}

	if opts.DryRun {
		c.SetDryRun(true)
		defer c.SetDryRun(false)
	}

	return c.Executor().Run(ctx, c.Binary(), args...)
}

// Update refreshes the package database.
func (c *Chocolatey) Update(ctx context.Context) error {
	// Chocolatey doesn't have a separate update command
	return nil
}

// Upgrade upgrades installed packages.
func (c *Chocolatey) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if len(opts.Packages) == 0 {
		args = append(args, "all")
	} else {
		args = append(args, opts.Packages...)
	}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if opts.DryRun {
		c.SetDryRun(true)
		defer c.SetDryRun(false)
	}

	return c.Executor().Run(ctx, c.Binary(), args...)
}

// Search finds packages matching the query.
func (c *Chocolatey) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search", query}

	if opts.InstalledOnly {
		args = append(args, "-l") // Local only
	}

	output, err := c.Executor().Output(ctx, c.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}

	return c.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses choco search output.
func (c *Chocolatey) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip info lines
		if strings.HasPrefix(line, "Chocolatey") || strings.Contains(line, "packages found") {
			continue
		}

		// Format: package version
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		version := fields[1]

		packages = append(packages, manager.Package{
			Name:    name,
			Version: version,
			Source:  "chocolatey",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (c *Chocolatey) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := c.Executor().Output(ctx, c.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return c.parsePackageInfo(output), nil
}

// parsePackageInfo parses choco info output.
func (c *Chocolatey) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "chocolatey",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, " Title:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, " Title:"))
		} else if strings.HasPrefix(line, " Version:") {
			info.Version = strings.TrimSpace(strings.TrimPrefix(line, " Version:"))
		} else if strings.HasPrefix(line, " Summary:") {
			info.Description = strings.TrimSpace(strings.TrimPrefix(line, " Summary:"))
		} else if strings.HasPrefix(line, " License:") {
			info.License = strings.TrimSpace(strings.TrimPrefix(line, " License:"))
		} else if strings.HasPrefix(line, " Package Url:") {
			info.URL = strings.TrimSpace(strings.TrimPrefix(line, " Package Url:"))
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (c *Chocolatey) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := c.Executor().Output(ctx, c.Binary(), "list", "-l")
	if err != nil {
		return nil, err
	}

	packages := c.parseSearchOutput(output, opts.Limit)

	// Mark all as installed
	for i := range packages {
		packages[i].Installed = true
	}

	// Filter by pattern
	if opts.Pattern != "" {
		patternLower := strings.ToLower(opts.Pattern)
		var filtered []manager.Package
		for _, pkg := range packages {
			if strings.Contains(strings.ToLower(pkg.Name), patternLower) {
				filtered = append(filtered, pkg)
			}
		}
		packages = filtered
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (c *Chocolatey) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := c.Executor().Output(ctx, c.Binary(), "list", "-l", pkg)
	if err != nil {
		return false, nil
	}
	return strings.Contains(output, pkg), nil
}

// Clean removes cached package files.
func (c *Chocolatey) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		c.SetDryRun(true)
		defer c.SetDryRun(false)
	}

	// Remove cached packages
	return c.Executor().Run(ctx, c.Binary(), "cache", "remove")
}

// Autoremove removes orphaned packages.
func (c *Chocolatey) Autoremove(ctx context.Context) error {
	// Chocolatey doesn't have automatic dependency tracking
	return nil
}
