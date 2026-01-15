package native

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"poxy/pkg/manager"
)

// Emerge implements the Manager interface for Gentoo's portage/emerge package manager.
type Emerge struct {
	*BaseManager
}

// NewEmerge creates a new Emerge manager instance.
func NewEmerge() *Emerge {
	return &Emerge{
		BaseManager: NewBaseManager("emerge", "Portage (Gentoo)", "emerge", true),
	}
}

// Install installs one or more packages.
func (e *Emerge) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{}

	if opts.DryRun {
		args = append(args, "--pretend")
	}

	args = append(args, packages...)

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Uninstall removes one or more packages.
func (e *Emerge) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"--depclean"}

	if opts.DryRun {
		args = append(args, "--pretend")
	}

	args = append(args, packages...)

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Update refreshes the package database (syncs portage tree).
func (e *Emerge) Update(ctx context.Context) error {
	return e.Executor().RunSudo(ctx, e.Binary(), "--sync")
}

// Upgrade upgrades installed packages.
func (e *Emerge) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-auDN"}

	if opts.DryRun {
		args = append(args, "--pretend")
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	} else {
		args = append(args, "@world")
	}

	return e.Executor().RunSudo(ctx, e.Binary(), args...)
}

// Search finds packages matching the query.
func (e *Emerge) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"-s", query}

	if opts.SearchInDesc {
		args = []string{"-S", query}
	}

	output, err := e.Executor().Output(ctx, e.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}

	return e.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses emerge search output.
func (e *Emerge) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	var currentPkg *manager.Package

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		// Package line starts with "*"
		if strings.HasPrefix(line, "*") {
			if currentPkg != nil {
				packages = append(packages, *currentPkg)
				if limit > 0 && len(packages) >= limit {
					break
				}
			}

			// Format: * category/package
			name := strings.TrimPrefix(line, "* ")
			name = strings.TrimSpace(name)

			currentPkg = &manager.Package{
				Name:   name,
				Source: "emerge",
			}
		} else if currentPkg != nil {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Latest version available:") {
				currentPkg.Version = strings.TrimPrefix(line, "Latest version available:")
				currentPkg.Version = strings.TrimSpace(currentPkg.Version)
			} else if strings.HasPrefix(line, "Description:") {
				currentPkg.Description = strings.TrimPrefix(line, "Description:")
				currentPkg.Description = strings.TrimSpace(currentPkg.Description)
			}
		}
	}

	// Don't forget the last package
	if currentPkg != nil && (limit <= 0 || len(packages) < limit) {
		packages = append(packages, *currentPkg)
	}

	return packages
}

// Info returns detailed information about a package.
func (e *Emerge) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := e.Executor().Output(ctx, e.Binary(), "-pv", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	info := &manager.PackageInfo{
		Package: manager.Package{
			Name:   pkg,
			Source: "emerge",
		},
	}

	// Parse basic info from emerge -pv output
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, pkg) {
			// Extract version from line like [ebuild  N    ] category/package-1.0.0
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.Contains(p, pkg) {
					// Extract version from package-version format
					lastDash := strings.LastIndex(p, "-")
					if lastDash > 0 {
						info.Version = p[lastDash+1:]
					}
					break
				}
			}
		}
	}

	return info, nil
}

// ListInstalled returns all installed packages.
func (e *Emerge) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	// Use qlist if available (faster), otherwise fall back to equery
	var output string
	var err error

	if _, lookErr := exec.LookPath("qlist"); lookErr == nil {
		output, err = e.Executor().Output(ctx, "qlist", "-ICv")
	} else if _, lookErr := exec.LookPath("equery"); lookErr == nil {
		output, err = e.Executor().Output(ctx, "equery", "list", "*")
	} else {
		// Fall back to reading /var/db/pkg
		output, err = e.Executor().Output(ctx, "ls", "/var/db/pkg/*/")
	}

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

		// Format: category/package-version
		name := line
		version := ""

		// Parse version
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
			Source:    "emerge",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (e *Emerge) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	// Use qlist if available
	if _, err := exec.LookPath("qlist"); err == nil {
		output, err := e.Executor().Output(ctx, "qlist", "-IC", pkg)
		return err == nil && strings.TrimSpace(output) != "", nil
	}

	// Fall back to equery
	if _, err := exec.LookPath("equery"); err == nil {
		err := e.Executor().Run(ctx, "equery", "list", pkg)
		return err == nil, nil
	}

	return false, nil
}

// Clean removes cached package files.
func (e *Emerge) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		e.SetDryRun(true)
		defer e.SetDryRun(false)
	}

	// Use eclean if available
	if _, err := exec.LookPath("eclean"); err == nil {
		args := []string{"distfiles"}
		if opts.All {
			args = append(args, "-d")
		}
		return e.Executor().RunSudo(ctx, "eclean", args...)
	}

	return fmt.Errorf("eclean not found; install app-portage/gentoolkit")
}

// Autoremove removes orphaned packages.
func (e *Emerge) Autoremove(ctx context.Context) error {
	return e.Executor().RunSudo(ctx, e.Binary(), "--depclean")
}
