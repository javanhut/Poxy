package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Swupd implements the Manager interface for Clear Linux's swupd package manager.
type Swupd struct {
	*BaseManager
}

// NewSwupd creates a new Swupd manager instance.
func NewSwupd() *Swupd {
	return &Swupd{
		BaseManager: NewBaseManager("swupd", "swupd (Clear Linux)", "swupd", true),
	}
}

// Install installs one or more bundles.
func (s *Swupd) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"bundle-add"}
	args = append(args, packages...)

	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().RunSudo(ctx, s.Binary(), args...)
}

// Uninstall removes one or more bundles.
func (s *Swupd) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"bundle-remove"}
	args = append(args, packages...)

	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().RunSudo(ctx, s.Binary(), args...)
}

// Update refreshes the package database (checks for updates).
func (s *Swupd) Update(ctx context.Context) error {
	return s.Executor().RunSudo(ctx, s.Binary(), "check-update")
}

// Upgrade upgrades the system.
func (s *Swupd) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	return s.Executor().RunSudo(ctx, s.Binary(), "update")
}

// Search finds bundles matching the query.
func (s *Swupd) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search", query}

	output, err := s.Executor().Output(ctx, s.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}

	return s.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses swupd search output.
func (s *Swupd) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip informational lines
		if strings.HasPrefix(line, "Searching") || strings.HasPrefix(line, "Bundle") {
			continue
		}

		// Parse bundle name
		name := line
		if strings.Contains(line, " ") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				name = parts[0]
			}
		}

		packages = append(packages, manager.Package{
			Name:   name,
			Source: "swupd",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a bundle.
func (s *Swupd) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "bundle-info", pkg)
	if err != nil {
		return nil, fmt.Errorf("bundle '%s' not found", pkg)
	}

	return s.parsePackageInfo(output, pkg), nil
}

// parsePackageInfo parses swupd bundle-info output.
func (s *Swupd) parsePackageInfo(output, pkgName string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Name:   pkgName,
			Source: "swupd",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Description:") {
			info.Description = strings.TrimSpace(strings.TrimPrefix(line, "Description:"))
		} else if strings.HasPrefix(line, "Status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			info.Installed = strings.Contains(status, "installed")
		} else if strings.HasPrefix(line, "Size:") {
			info.Size = strings.TrimSpace(strings.TrimPrefix(line, "Size:"))
		}
	}

	return info
}

// ListInstalled returns all installed bundles.
func (s *Swupd) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "bundle-list")
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

		// Skip header/info lines
		if strings.HasPrefix(line, "Installed") || strings.HasPrefix(line, "---") {
			continue
		}

		name := line

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Source:    "swupd",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a bundle is installed.
func (s *Swupd) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := s.Executor().Output(ctx, s.Binary(), "bundle-list")
	if err != nil {
		return false, nil
	}

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == pkg {
			return true, nil
		}
	}

	return false, nil
}

// Clean removes cached package files.
func (s *Swupd) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		s.SetDryRun(true)
		defer s.SetDryRun(false)
	}

	// swupd doesn't have a dedicated clean command, but verify can fix issues
	return nil
}

// Autoremove removes orphaned packages.
func (s *Swupd) Autoremove(ctx context.Context) error {
	// Clear Linux bundles don't have traditional dependency orphaning
	return nil
}
