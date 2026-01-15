package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// DNF implements the Manager interface for Fedora/RHEL's DNF package manager.
type DNF struct {
	*BaseManager
}

// NewDNF creates a new DNF manager instance.
func NewDNF() *DNF {
	return &DNF{
		BaseManager: NewBaseManager("dnf", "DNF (Fedora/RHEL)", "dnf", true),
	}
}

// Install installs one or more packages.
func (d *DNF) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if opts.Reinstall {
		args = append(args, "--reinstall")
	}

	args = append(args, packages...)

	if opts.DryRun {
		d.SetDryRun(true)
		defer d.SetDryRun(false)
	}

	return d.Executor().RunSudo(ctx, d.Binary(), args...)
}

// Uninstall removes one or more packages.
func (d *DNF) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"remove"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		d.SetDryRun(true)
		defer d.SetDryRun(false)
	}

	err := d.Executor().RunSudo(ctx, d.Binary(), args...)
	if err != nil {
		return err
	}

	if opts.Recursive {
		return d.Autoremove(ctx)
	}

	return nil
}

// Update refreshes the package database.
func (d *DNF) Update(ctx context.Context) error {
	return d.Executor().RunSudo(ctx, d.Binary(), "makecache")
}

// Upgrade upgrades installed packages.
func (d *DNF) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		d.SetDryRun(true)
		defer d.SetDryRun(false)
	}

	return d.Executor().RunSudo(ctx, d.Binary(), args...)
}

// Search finds packages matching the query.
func (d *DNF) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search"}

	if opts.InstalledOnly {
		return d.searchInstalled(ctx, query, opts)
	}

	args = append(args, query)

	output, err := d.Executor().Output(ctx, d.Binary(), args...)
	if err != nil {
		// DNF returns error if no results, but we want empty slice
		return []manager.Package{}, nil
	}

	return d.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed packages.
func (d *DNF) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := d.Executor().Output(ctx, d.Binary(), "list", "installed")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)
	skipHeader := true

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header line
		if skipHeader {
			if strings.Contains(line, "Installed Packages") {
				skipHeader = false
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// DNF format: package-name.arch version repo
		nameParts := strings.Split(fields[0], ".")
		name := nameParts[0]
		version := fields[1]

		if !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "dnf",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses dnf search output.
func (d *DNF) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentPkg *manager.Package

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and headers
		if line == "" || strings.HasPrefix(line, "=") || strings.HasPrefix(line, "Last metadata") {
			continue
		}

		// Package name line (contains .arch : description)
		if strings.Contains(line, " : ") {
			if currentPkg != nil {
				packages = append(packages, *currentPkg)
				if limit > 0 && len(packages) >= limit {
					break
				}
			}

			parts := strings.SplitN(line, " : ", 2)
			if len(parts) < 2 {
				continue
			}

			nameParts := strings.Split(strings.TrimSpace(parts[0]), ".")
			name := nameParts[0]

			currentPkg = &manager.Package{
				Name:        name,
				Description: strings.TrimSpace(parts[1]),
				Source:      "dnf",
			}
		} else if currentPkg != nil && strings.HasPrefix(line, " ") {
			// Continuation of description
			currentPkg.Description += " " + strings.TrimSpace(line)
		}
	}

	// Don't forget the last package
	if currentPkg != nil && (limit <= 0 || len(packages) < limit) {
		packages = append(packages, *currentPkg)
	}

	return packages
}

// Info returns detailed information about a package.
func (d *DNF) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := d.Executor().Output(ctx, d.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return d.parsePackageInfo(output), nil
}

// parsePackageInfo parses dnf info output.
func (d *DNF) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "dnf",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " : ", 2)
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
		case "Release":
			if info.Version != "" {
				info.Version = info.Version + "-" + value
			}
		case "Summary":
			info.Description = value
		case "License":
			info.License = value
		case "URL":
			info.URL = value
		case "Repository":
			info.Repository = value
		case "Size":
			info.Size = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (d *DNF) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := d.Executor().Output(ctx, d.Binary(), "list", "installed")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)
	skipHeader := true

	for scanner.Scan() {
		line := scanner.Text()

		if skipHeader {
			if strings.Contains(line, "Installed Packages") {
				skipHeader = false
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		nameParts := strings.Split(fields[0], ".")
		name := nameParts[0]
		version := fields[1]

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "dnf",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (d *DNF) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := d.Executor().Run(ctx, "rpm", "-q", pkg)
	return err == nil, nil
}

// Clean removes cached package files.
func (d *DNF) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		d.SetDryRun(true)
		defer d.SetDryRun(false)
	}

	return d.Executor().RunSudo(ctx, d.Binary(), "clean", "all")
}

// Autoremove removes orphaned packages.
func (d *DNF) Autoremove(ctx context.Context) error {
	return d.Executor().RunSudo(ctx, d.Binary(), "autoremove", "-y")
}
