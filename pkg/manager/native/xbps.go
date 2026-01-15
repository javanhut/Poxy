package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// XBPS implements the Manager interface for Void Linux's XBPS package manager.
type XBPS struct {
	*BaseManager
}

// NewXBPS creates a new XBPS manager instance.
func NewXBPS() *XBPS {
	return &XBPS{
		BaseManager: NewBaseManager("xbps", "XBPS (Void Linux)", "xbps-install", true),
	}
}

// Install installs one or more packages.
func (x *XBPS) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"-S"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		x.SetDryRun(true)
		defer x.SetDryRun(false)
	}

	return x.Executor().RunSudo(ctx, "xbps-install", args...)
}

// Uninstall removes one or more packages.
func (x *XBPS) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{}

	if opts.Recursive {
		args = append(args, "-R")
	}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		x.SetDryRun(true)
		defer x.SetDryRun(false)
	}

	return x.Executor().RunSudo(ctx, "xbps-remove", args...)
}

// Update refreshes the package database.
func (x *XBPS) Update(ctx context.Context) error {
	return x.Executor().RunSudo(ctx, "xbps-install", "-S")
}

// Upgrade upgrades installed packages.
func (x *XBPS) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-Su"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		args = []string{"-S"}
		if opts.AutoConfirm {
			args = append(args, "-y")
		}
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		x.SetDryRun(true)
		defer x.SetDryRun(false)
	}

	return x.Executor().RunSudo(ctx, "xbps-install", args...)
}

// Search finds packages matching the query.
func (x *XBPS) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	var output string
	var err error

	if opts.InstalledOnly {
		output, err = x.Executor().Output(ctx, "xbps-query", "-l")
	} else {
		output, err = x.Executor().Output(ctx, "xbps-query", "-Rs", query)
	}

	if err != nil {
		return []manager.Package{}, nil
	}

	return x.parseSearchOutput(output, query, opts), nil
}

// parseSearchOutput parses xbps-query output.
func (x *XBPS) parseSearchOutput(output, query string, opts manager.SearchOpts) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)

	for scanner.Scan() {
		line := scanner.Text()

		// Format: [*] package-version description
		// or: package-version - description
		var name, version, description string
		var installed bool

		if strings.HasPrefix(line, "[*]") {
			installed = true
			line = strings.TrimPrefix(line, "[*]")
		} else if strings.HasPrefix(line, "[-]") {
			installed = false
			line = strings.TrimPrefix(line, "[-]")
		}

		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}

		// Parse name-version
		nameVersion := parts[0]
		lastDash := strings.LastIndex(nameVersion, "-")
		if lastDash > 0 {
			name = nameVersion[:lastDash]
			version = nameVersion[lastDash+1:]
		} else {
			name = nameVersion
		}

		if len(parts) > 1 {
			description = strings.TrimPrefix(parts[1], "- ")
		}

		// Filter by query for installed search
		if opts.InstalledOnly && !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:        name,
			Version:     version,
			Description: description,
			Source:      "xbps",
			Installed:   installed,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (x *XBPS) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	// Try remote first
	output, err := x.Executor().Output(ctx, "xbps-query", "-R", pkg)
	if err != nil {
		// Try local
		output, err = x.Executor().Output(ctx, "xbps-query", pkg)
		if err != nil {
			return nil, fmt.Errorf("package '%s' not found", pkg)
		}
	}

	return x.parsePackageInfo(output), nil
}

// parsePackageInfo parses xbps-query output.
func (x *XBPS) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "xbps",
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
		case "pkgname":
			info.Name = value
		case "version":
			info.Version = value
		case "short_desc":
			info.Description = value
		case "license":
			info.License = value
		case "repository":
			info.Repository = value
		case "installed_size":
			info.Size = value
		case "homepage":
			info.URL = value
		case "maintainer":
			info.Maintainer = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (x *XBPS) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := x.Executor().Output(ctx, "xbps-query", "-l")
	if err != nil {
		return nil, err
	}

	packages := x.parseSearchOutput(output, "", manager.SearchOpts{Limit: opts.Limit})

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
func (x *XBPS) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := x.Executor().Run(ctx, "xbps-query", pkg)
	return err == nil, nil
}

// Clean removes cached package files.
func (x *XBPS) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		x.SetDryRun(true)
		defer x.SetDryRun(false)
	}

	return x.Executor().RunSudo(ctx, "xbps-remove", "-O")
}

// Autoremove removes orphaned packages.
func (x *XBPS) Autoremove(ctx context.Context) error {
	return x.Executor().RunSudo(ctx, "xbps-remove", "-o", "-y")
}
