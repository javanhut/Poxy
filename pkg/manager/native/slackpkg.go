package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Slackpkg implements the Manager interface for Slackware's slackpkg package manager.
type Slackpkg struct {
	*BaseManager
}

// NewSlackpkg creates a new Slackpkg manager instance.
func NewSlackpkg() *Slackpkg {
	return &Slackpkg{
		BaseManager: NewBaseManager("slackpkg", "Slackpkg (Slackware)", "slackpkg", true),
	}
}

// Install installs one or more packages.
func (s *Slackpkg) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	for _, pkg := range packages {
		if err := s.Executor().RunSudo(ctx, s.Binary(), "install", pkg); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall removes one or more packages.
func (s *Slackpkg) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	for _, pkg := range packages {
		if err := s.Executor().RunSudo(ctx, s.Binary(), "remove", pkg); err != nil {
			return err
		}
	}

	return nil
}

// Update refreshes the package database.
func (s *Slackpkg) Update(ctx context.Context) error {
	return s.Executor().RunSudo(ctx, s.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (s *Slackpkg) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	if len(opts.Packages) > 0 {
		for _, pkg := range opts.Packages {
			if err := s.Executor().RunSudo(ctx, s.Binary(), "upgrade", pkg); err != nil {
				return err
			}
		}
		return nil
	}

	return s.Executor().RunSudo(ctx, s.Binary(), "upgrade-all")
}

// Search finds packages matching the query.
func (s *Slackpkg) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return s.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses slackpkg search output.
func (s *Slackpkg) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format varies, try to extract package name
		name := line
		version := ""

		// Try to parse name-version format
		lastDash := strings.LastIndex(line, "-")
		if lastDash > 0 {
			possibleVersion := line[lastDash+1:]
			if len(possibleVersion) > 0 && possibleVersion[0] >= '0' && possibleVersion[0] <= '9' {
				name = line[:lastDash]
				version = possibleVersion
			}
		}

		packages = append(packages, manager.Package{
			Name:    name,
			Version: version,
			Source:  "slackpkg",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (s *Slackpkg) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return s.parsePackageInfo(output, pkg), nil
}

// parsePackageInfo parses slackpkg info output.
func (s *Slackpkg) parsePackageInfo(output, pkgName string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Name:   pkgName,
			Source: "slackpkg",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var description []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "PACKAGE NAME:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, "PACKAGE NAME:"))
		} else if strings.HasPrefix(line, "PACKAGE SIZE") {
			info.Size = strings.TrimSpace(strings.TrimPrefix(line, "PACKAGE SIZE"))
		} else if strings.HasPrefix(line, "PACKAGE DESCRIPTION:") {
			// Next lines are description
			continue
		} else if !strings.HasPrefix(line, "PACKAGE") && strings.TrimSpace(line) != "" {
			description = append(description, strings.TrimSpace(line))
		}
	}

	if len(description) > 0 {
		info.Description = strings.Join(description, " ")
	}

	return info
}

// ListInstalled returns all installed packages.
func (s *Slackpkg) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	// List packages from /var/log/packages
	output, err := s.Executor().Output(ctx, "ls", "/var/log/packages/")
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

		// Parse package name and version from filename
		name := line
		version := ""

		// Slackware format: name-version-arch-build
		parts := strings.Split(line, "-")
		if len(parts) >= 4 {
			name = strings.Join(parts[:len(parts)-3], "-")
			version = parts[len(parts)-3]
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "slackpkg",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (s *Slackpkg) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := s.Executor().Output(ctx, "ls", "/var/log/packages/")
	if err != nil {
		return false, nil
	}

	return strings.Contains(output, pkg), nil
}

// Clean removes cached package files.
func (s *Slackpkg) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().RunSudo(ctx, s.Binary(), "clean-system")
}

// Autoremove removes orphaned packages.
func (s *Slackpkg) Autoremove(ctx context.Context) error {
	// Slackware doesn't have automatic dependency tracking
	return nil
}
