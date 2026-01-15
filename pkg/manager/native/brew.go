package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Brew implements the Manager interface for Homebrew (macOS and Linux).
type Brew struct {
	*BaseManager
}

// NewBrew creates a new Homebrew manager instance.
func NewBrew() *Brew {
	return &Brew{
		BaseManager: NewBaseManager("brew", "Homebrew", "brew", false),
	}
}

// Install installs one or more packages.
func (b *Brew) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}

	if opts.Reinstall {
		args = append(args, "--force")
	}

	args = append(args, packages...)

	if opts.DryRun {
		b.SetDryRun(true)
		defer b.SetDryRun(false)
	}

	return b.Executor().Run(ctx, b.Binary(), args...)
}

// Uninstall removes one or more packages.
func (b *Brew) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"uninstall"}

	if opts.Recursive {
		// Remove dependencies too (not directly supported, but we can try)
		args = append(args, "--ignore-dependencies")
	}

	args = append(args, packages...)

	if opts.DryRun {
		b.SetDryRun(true)
		defer b.SetDryRun(false)
	}

	return b.Executor().Run(ctx, b.Binary(), args...)
}

// Update refreshes the package database.
func (b *Brew) Update(ctx context.Context) error {
	return b.Executor().Run(ctx, b.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (b *Brew) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		b.SetDryRun(true)
		defer b.SetDryRun(false)
	}

	return b.Executor().Run(ctx, b.Binary(), args...)
}

// Search finds packages matching the query.
func (b *Brew) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return b.searchInstalled(ctx, query, opts)
	}

	output, err := b.Executor().Output(ctx, b.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return b.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed packages.
func (b *Brew) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := b.Executor().Output(ctx, b.Binary(), "list", "--formula")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	queryLower := strings.ToLower(query)

	for _, name := range strings.Fields(output) {
		if !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Source:    "brew",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses brew search output.
func (b *Brew) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package

	for _, name := range strings.Fields(output) {
		// Skip section headers
		if strings.HasPrefix(name, "==>") {
			continue
		}

		packages = append(packages, manager.Package{
			Name:   name,
			Source: "brew",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (b *Brew) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := b.Executor().Output(ctx, b.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return b.parsePackageInfo(output), nil
}

// parsePackageInfo parses brew info output.
func (b *Brew) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "brew",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		switch lineNum {
		case 1:
			// First line: name: stable version (bottled)
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 1 {
				info.Name = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				// Parse version from "stable X.Y.Z"
				versionPart := strings.TrimSpace(parts[1])
				fields := strings.Fields(versionPart)
				if len(fields) > 1 {
					info.Version = fields[1]
				}
			}
		case 2:
			// Second line: URL
			info.URL = strings.TrimSpace(line)
		default:
			// Look for description
			if strings.HasPrefix(line, "==>") && strings.Contains(line, "Description") {
				continue
			}
			if info.Description == "" && !strings.HasPrefix(line, "==>") && !strings.HasPrefix(line, "http") {
				info.Description = strings.TrimSpace(line)
			}
			if strings.Contains(line, "License:") {
				info.License = strings.TrimPrefix(line, "License: ")
			}
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (b *Brew) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := b.Executor().Output(ctx, b.Binary(), "list", "--formula", "--versions")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		name := fields[0]
		version := ""
		if len(fields) > 1 {
			version = fields[1]
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "brew",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (b *Brew) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := b.Executor().Output(ctx, b.Binary(), "list", "--formula")
	if err != nil {
		return false, nil
	}

	for _, name := range strings.Fields(output) {
		if name == pkg {
			return true, nil
		}
	}

	return false, nil
}

// Clean removes cached package files.
func (b *Brew) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		b.SetDryRun(true)
		defer b.SetDryRun(false)
	}

	args := []string{"cleanup"}
	if opts.All {
		args = append(args, "-s") // Scrub the cache
	}

	return b.Executor().Run(ctx, b.Binary(), args...)
}

// Autoremove removes orphaned packages.
func (b *Brew) Autoremove(ctx context.Context) error {
	return b.Executor().Run(ctx, b.Binary(), "autoremove")
}
