package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Winget implements the Manager interface for Windows Package Manager (winget).
type Winget struct {
	*BaseManager
}

// NewWinget creates a new Winget manager instance.
func NewWinget() *Winget {
	return &Winget{
		BaseManager: NewBaseManager("winget", "Windows Package Manager", "winget", false),
	}
}

// Install installs one or more packages.
func (w *Winget) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	for _, pkg := range packages {
		args := []string{"install", pkg}

		if opts.AutoConfirm {
			args = append(args, "--accept-package-agreements", "--accept-source-agreements")
		}

		if opts.DryRun {
			w.SetDryRun(true)
			defer w.SetDryRun(false)
		}

		if err := w.Executor().Run(ctx, w.Binary(), args...); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall removes one or more packages.
func (w *Winget) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	for _, pkg := range packages {
		args := []string{"uninstall", pkg}

		if opts.AutoConfirm {
			args = append(args, "--silent")
		}

		if opts.DryRun {
			w.SetDryRun(true)
			defer w.SetDryRun(false)
		}

		if err := w.Executor().Run(ctx, w.Binary(), args...); err != nil {
			return err
		}
	}

	return nil
}

// Update refreshes the package database.
func (w *Winget) Update(ctx context.Context) error {
	// Winget doesn't have a separate update command
	return nil
}

// Upgrade upgrades installed packages.
func (w *Winget) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if len(opts.Packages) == 0 {
		args = append(args, "--all")
	} else {
		args = append(args, opts.Packages...)
	}

	if opts.AutoConfirm {
		args = append(args, "--accept-package-agreements", "--accept-source-agreements")
	}

	if opts.DryRun {
		w.SetDryRun(true)
		defer w.SetDryRun(false)
	}

	return w.Executor().Run(ctx, w.Binary(), args...)
}

// Search finds packages matching the query.
func (w *Winget) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := w.Executor().Output(ctx, w.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return w.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses winget search output.
func (w *Winget) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	headerPassed := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip until we find the separator line
		if strings.HasPrefix(line, "-") {
			headerPassed = true
			continue
		}

		if !headerPassed {
			continue
		}

		// Parse table format: Name  Id  Version  Match  Source
		// Use fixed-width parsing since columns align
		if len(line) < 20 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Try to find the version (typically 3rd or 4th field)
		name := fields[0]
		id := ""
		version := ""

		// Look for a field that looks like a version
		for i, f := range fields {
			if i == 0 {
				continue
			}
			// Version usually contains dots and numbers
			if strings.ContainsAny(f, ".") && len(f) > 0 && (f[0] >= '0' && f[0] <= '9' || f[0] == 'v') {
				version = f
				if i > 1 {
					id = fields[1]
				}
				break
			}
		}

		packages = append(packages, manager.Package{
			Name:    name,
			Version: version,
			Source:  "winget",
		})

		// Store ID in description if we found it
		if id != "" {
			packages[len(packages)-1].Description = "ID: " + id
		}

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (w *Winget) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := w.Executor().Output(ctx, w.Binary(), "show", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return w.parsePackageInfo(output), nil
}

// parsePackageInfo parses winget show output.
func (w *Winget) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "winget",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			info.Name = value
		case "Version":
			info.Version = value
		case "Description":
			info.Description = value
		case "Publisher":
			info.Maintainer = value
		case "License":
			info.License = value
		case "Homepage":
			info.URL = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (w *Winget) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := w.Executor().Output(ctx, w.Binary(), "list")
	if err != nil {
		return nil, err
	}

	packages := w.parseSearchOutput(output, opts.Limit)

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
func (w *Winget) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := w.Executor().Output(ctx, w.Binary(), "list", pkg)
	if err != nil {
		return false, nil
	}
	return strings.Contains(output, pkg), nil
}

// Clean removes cached package files.
func (w *Winget) Clean(ctx context.Context, opts manager.CleanOpts) error {
	// Winget doesn't have a cache clean command
	return nil
}

// Autoremove removes orphaned packages.
func (w *Winget) Autoremove(ctx context.Context) error {
	// Windows doesn't have automatic dependency tracking in winget
	return nil
}
