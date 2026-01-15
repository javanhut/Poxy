package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Zypper implements the Manager interface for openSUSE's zypper package manager.
type Zypper struct {
	*BaseManager
}

// NewZypper creates a new Zypper manager instance.
func NewZypper() *Zypper {
	return &Zypper{
		BaseManager: NewBaseManager("zypper", "Zypper (openSUSE)", "zypper", true),
	}
}

// Install installs one or more packages.
func (z *Zypper) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		z.SetDryRun(true)
		defer z.SetDryRun(false)
	}

	return z.Executor().RunSudo(ctx, z.Binary(), args...)
}

// Uninstall removes one or more packages.
func (z *Zypper) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"remove"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		z.SetDryRun(true)
		defer z.SetDryRun(false)
	}

	return z.Executor().RunSudo(ctx, z.Binary(), args...)
}

// Update refreshes the package database.
func (z *Zypper) Update(ctx context.Context) error {
	return z.Executor().RunSudo(ctx, z.Binary(), "refresh")
}

// Upgrade upgrades installed packages.
func (z *Zypper) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"update"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		z.SetDryRun(true)
		defer z.SetDryRun(false)
	}

	return z.Executor().RunSudo(ctx, z.Binary(), args...)
}

// Search finds packages matching the query.
func (z *Zypper) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search"}

	if opts.InstalledOnly {
		args = append(args, "-i")
	}

	args = append(args, query)

	output, err := z.Executor().Output(ctx, z.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}

	return z.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses zypper search output.
func (z *Zypper) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	inTable := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip until we find the table header separator
		if strings.HasPrefix(line, "--") {
			inTable = true
			continue
		}

		if !inTable {
			continue
		}

		// Parse table row: Status | Name | Summary | Type
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		status := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		summary := strings.TrimSpace(parts[2])

		installed := strings.Contains(status, "i")

		packages = append(packages, manager.Package{
			Name:        name,
			Description: summary,
			Source:      "zypper",
			Installed:   installed,
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (z *Zypper) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := z.Executor().Output(ctx, z.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return z.parsePackageInfo(output), nil
}

// parsePackageInfo parses zypper info output.
func (z *Zypper) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "zypper",
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
		case "Summary":
			info.Description = value
		case "License":
			info.License = value
		case "Repository":
			info.Repository = value
		case "Installed Size":
			info.Size = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (z *Zypper) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := z.Executor().Output(ctx, z.Binary(), "search", "-i")
	if err != nil {
		return nil, err
	}

	packages := z.parseSearchOutput(output, opts.Limit)

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
func (z *Zypper) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := z.Executor().Output(ctx, "rpm", "-q", pkg)
	if err != nil {
		return false, nil
	}
	return !strings.Contains(output, "not installed"), nil
}

// Clean removes cached package files.
func (z *Zypper) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		z.SetDryRun(true)
		defer z.SetDryRun(false)
	}

	return z.Executor().RunSudo(ctx, z.Binary(), "clean", "--all")
}

// Autoremove removes orphaned packages.
func (z *Zypper) Autoremove(ctx context.Context) error {
	// Zypper doesn't have a direct autoremove, but we can remove unneeded packages
	return z.Executor().RunSudo(ctx, z.Binary(), "remove", "--clean-deps")
}
