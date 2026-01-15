package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Eopkg implements the Manager interface for Solus's eopkg package manager.
type Eopkg struct {
	*BaseManager
}

// NewEopkg creates a new Eopkg manager instance.
func NewEopkg() *Eopkg {
	return &Eopkg{
		BaseManager: NewBaseManager("eopkg", "eopkg (Solus)", "eopkg", true),
	}
}

// Install installs one or more packages.
func (e *Eopkg) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		e.SetDryRun(true)
		defer e.SetDryRun(false)
	}

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Uninstall removes one or more packages.
func (e *Eopkg) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"remove"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		e.SetDryRun(true)
		defer e.SetDryRun(false)
	}

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Update refreshes the package database.
func (e *Eopkg) Update(ctx context.Context) error {
	return e.Executor().RunSudo(ctx, e.Binary(), "update-repo")
}

// Upgrade upgrades installed packages.
func (e *Eopkg) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		e.SetDryRun(true)
		defer e.SetDryRun(false)
	}

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Search finds packages matching the query.
func (e *Eopkg) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := e.Executor().Output(ctx, e.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return e.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses eopkg search output.
func (e *Eopkg) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: package - description
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) < 1 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		description := ""
		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		packages = append(packages, manager.Package{
			Name:        name,
			Description: description,
			Source:      "eopkg",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (e *Eopkg) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := e.Executor().Output(ctx, e.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return e.parsePackageInfo(output), nil
}

// parsePackageInfo parses eopkg info output.
func (e *Eopkg) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "eopkg",
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
		case "Size":
			info.Size = value
		case "Source":
			info.Repository = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (e *Eopkg) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := e.Executor().Output(ctx, e.Binary(), "list-installed")
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

		// Format: package - version
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) < 1 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		version := ""
		if len(parts) > 1 {
			version = strings.TrimSpace(parts[1])
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "eopkg",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (e *Eopkg) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := e.Executor().Output(ctx, e.Binary(), "list-installed")
	if err != nil {
		return false, nil
	}
	return strings.Contains(output, pkg), nil
}

// Clean removes cached package files.
func (e *Eopkg) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		e.SetDryRun(true)
		defer e.SetDryRun(false)
	}

	return e.Executor().RunSudo(ctx, e.Binary(), "delete-cache")
}

// Autoremove removes orphaned packages.
func (e *Eopkg) Autoremove(ctx context.Context) error {
	return e.Executor().RunSudo(ctx, e.Binary(), "remove-orphans", "-y")
}
