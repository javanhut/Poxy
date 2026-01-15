package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Scoop implements the Manager interface for Scoop (Windows).
type Scoop struct {
	*BaseManager
}

// NewScoop creates a new Scoop manager instance.
func NewScoop() *Scoop {
	return &Scoop{
		BaseManager: NewBaseManager("scoop", "Scoop", "scoop", false),
	}
}

// Install installs one or more packages.
func (s *Scoop) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}
	args = append(args, packages...)

	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().Run(ctx, s.Binary(), args...)
}

// Uninstall removes one or more packages.
func (s *Scoop) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"uninstall"}
	args = append(args, packages...)

	if opts.Purge {
		args = append(args, "-p") // Remove persisted data
	}

	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().Run(ctx, s.Binary(), args...)
}

// Update refreshes the package database.
func (s *Scoop) Update(ctx context.Context) error {
	return s.Executor().Run(ctx, s.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (s *Scoop) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"update"}

	if len(opts.Packages) == 0 {
		args = append(args, "*")
	} else {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().Run(ctx, s.Binary(), args...)
}

// Search finds packages matching the query.
func (s *Scoop) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return s.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses scoop search output.
func (s *Scoop) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	inResults := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip header lines
		if strings.Contains(line, "bucket") || strings.HasPrefix(line, "-") {
			inResults = true
			continue
		}

		if !inResults {
			continue
		}

		// Format: name (version)
		name := line
		version := ""

		if idx := strings.Index(line, " ("); idx > 0 {
			name = line[:idx]
			version = strings.Trim(line[idx:], " ()")
		}

		packages = append(packages, manager.Package{
			Name:    name,
			Version: version,
			Source:  "scoop",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (s *Scoop) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return s.parsePackageInfo(output), nil
}

// parsePackageInfo parses scoop info output.
func (s *Scoop) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "scoop",
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
		case "License":
			info.License = value
		case "Website":
			info.URL = value
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (s *Scoop) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "list")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)
	headerPassed := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip header
		if strings.HasPrefix(line, "Name") || strings.HasPrefix(line, "-") {
			headerPassed = true
			continue
		}

		if !headerPassed {
			continue
		}

		// Format: Name Version Source Updated Info
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		version := fields[1]

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "scoop",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (s *Scoop) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "list")
	if err != nil {
		return false, nil
	}
	return strings.Contains(output, pkg), nil
}

// Clean removes cached package files.
func (s *Scoop) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	args := []string{"cleanup", "*"}
	if opts.All {
		args = append(args, "-k") // Remove old versions too
	}

	return s.Executor().Run(ctx, s.Binary(), args...)
}

// Autoremove removes orphaned packages.
func (s *Scoop) Autoremove(ctx context.Context) error {
	// Scoop doesn't have automatic dependency tracking
	return nil
}
