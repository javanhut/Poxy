package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// APK implements the Manager interface for Alpine Linux's apk package manager.
type APK struct {
	*BaseManager
}

// NewAPK creates a new APK manager instance.
func NewAPK() *APK {
	return &APK{
		BaseManager: NewBaseManager("apk", "APK (Alpine Linux)", "apk", true),
	}
}

// Install installs one or more packages.
func (a *APK) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"add"}

	// APK doesn't have a -y flag, it's non-interactive by default
	args = append(args, packages...)

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), args...)
}

// Uninstall removes one or more packages.
func (a *APK) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"del"}

	if opts.Purge {
		args = append(args, "--purge")
	}

	args = append(args, packages...)

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), args...)
}

// Update refreshes the package database.
func (a *APK) Update(ctx context.Context) error {
	return a.Executor().RunSudo(ctx, a.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (a *APK) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), args...)
}

// Search finds packages matching the query.
func (a *APK) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search"}

	if opts.SearchInDesc {
		args = append(args, "-d")
	}

	args = append(args, query)

	output, err := a.Executor().Output(ctx, a.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}

	return a.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses apk search output.
func (a *APK) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: package-name-version
		// Find the last dash followed by a version number
		name := line
		version := ""

		// Parse name and version
		lastDash := strings.LastIndex(line, "-")
		if lastDash > 0 {
			possibleVersion := line[lastDash+1:]
			// Check if it looks like a version (starts with digit)
			if len(possibleVersion) > 0 && possibleVersion[0] >= '0' && possibleVersion[0] <= '9' {
				name = line[:lastDash]
				version = possibleVersion
			}
		}

		packages = append(packages, manager.Package{
			Name:    name,
			Version: version,
			Source:  "apk",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (a *APK) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := a.Executor().Output(ctx, a.Binary(), "info", "-a", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return a.parsePackageInfo(output), nil
}

// parsePackageInfo parses apk info output.
func (a *APK) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "apk",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentSection string

	for scanner.Scan() {
		line := scanner.Text()

		// Section headers end with ":"
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			currentSection = strings.TrimSuffix(line, ":")
			continue
		}

		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}

		switch currentSection {
		case "description":
			info.Description = value
		case "webpage":
			info.URL = value
		case "license":
			info.License = value
		case "installed-size":
			info.Size = value
		case "maintainer":
			info.Maintainer = value
		}

		// Package name and version from first line
		if info.Name == "" && strings.Contains(line, " ") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				info.Name = parts[0]
			}
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (a *APK) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := a.Executor().Output(ctx, a.Binary(), "info", "-v")
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

		// Parse name-version
		name := line
		version := ""
		lastDash := strings.LastIndex(line, "-")
		if lastDash > 0 {
			possibleVersion := line[lastDash+1:]
			if len(possibleVersion) > 0 && possibleVersion[0] >= '0' && possibleVersion[0] <= '9' {
				name = line[:lastDash]
				version = possibleVersion
			}
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "apk",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (a *APK) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := a.Executor().Run(ctx, a.Binary(), "info", "-e", pkg)
	return err == nil, nil
}

// Clean removes cached package files.
func (a *APK) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), "cache", "clean")
}

// Autoremove removes orphaned packages.
func (a *APK) Autoremove(ctx context.Context) error {
	// APK handles this automatically, but we can try to clean up
	return a.Executor().RunSudo(ctx, a.Binary(), "autoremove")
}
