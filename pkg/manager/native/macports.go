package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// MacPorts implements the Manager interface for macOS's MacPorts
// package manager (the `port` CLI).
type MacPorts struct {
	*BaseManager
}

// NewMacPorts creates a new MacPorts manager instance.
func NewMacPorts() *MacPorts {
	return &MacPorts{
		BaseManager: NewBaseManager("macports", "MacPorts (macOS)", "port", true),
	}
}

// Install installs one or more packages.
func (m *MacPorts) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}
	args = append(args, packages...)

	if opts.DryRun {
		m.SetDryRun(true)
		defer m.SetDryRun(false)
	}

	_, err := m.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Uninstall removes one or more packages.
func (m *MacPorts) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"uninstall"}
	if opts.Recursive {
		args = append(args, "--follow-dependencies")
	}
	args = append(args, packages...)

	if opts.DryRun {
		m.SetDryRun(true)
		defer m.SetDryRun(false)
	}

	_, err := m.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Update refreshes the package database via `port selfupdate`.
func (m *MacPorts) Update(ctx context.Context) error {
	return m.Executor().RunSudo(ctx, m.Binary(), "selfupdate")
}

// Upgrade upgrades installed packages.
func (m *MacPorts) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	} else {
		args = append(args, "outdated")
	}

	if opts.DryRun {
		m.SetDryRun(true)
		defer m.SetDryRun(false)
	}

	_, err := m.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Search finds packages matching the query.
func (m *MacPorts) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := m.Executor().Output(ctx, m.Binary(), "search", "--name", query)
	if err != nil {
		return []manager.Package{}, nil
	}
	return m.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses `port search` output.
// Format: "name @version (category) description"
func (m *MacPorts) parseSearchOutput(output string, limit int) []manager.Package {
	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "No match") {
			continue
		}
		// "<name> @<version> (<category>)" — description is on next indented line
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "@") {
			continue
		}
		pkg := manager.Package{
			Name:    fields[0],
			Version: strings.TrimPrefix(fields[1], "@"),
			Source:  "macports",
		}
		// Capture the description that follows on the next indented line.
		if scanner.Scan() {
			desc := strings.TrimSpace(scanner.Text())
			pkg.Description = desc
		}
		pkgs = append(pkgs, pkg)
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// Info returns detailed information about a package.
func (m *MacPorts) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := m.Executor().Output(ctx, m.Binary(), "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	info := &manager.PackageInfo{
		Package: manager.Package{Source: "macports", Name: pkg},
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
		case "Description":
			info.Description = value
		case "Homepage":
			info.URL = value
		case "License":
			info.License = value
		case "Maintainers":
			info.Maintainer = value
		case "Version":
			info.Version = value
		}
	}
	return info, nil
}

// ListInstalled returns all installed packages.
func (m *MacPorts) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := m.Executor().Output(ctx, m.Binary(), "installed")
	if err != nil {
		return nil, err
	}

	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "The following ports") || strings.HasPrefix(line, "None of the") {
			continue
		}
		// "name @version (active)" or "name @version"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      name,
			Version:   strings.TrimPrefix(fields[1], "@"),
			Source:    "macports",
			Installed: true,
		})
		if opts.Limit > 0 && len(pkgs) >= opts.Limit {
			break
		}
	}
	return pkgs, nil
}

// IsInstalled checks if a package is installed.
func (m *MacPorts) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := m.Executor().OutputQuiet(ctx, m.Binary(), "installed", pkg)
	if err != nil {
		return false, nil
	}
	return !strings.Contains(output, "None of the specified ports are installed"), nil
}

// ListUpgradable returns installed packages with newer versions available.
func (m *MacPorts) ListUpgradable(ctx context.Context) ([]manager.Package, error) {
	output, err := m.Executor().OutputQuiet(ctx, m.Binary(), "outdated")
	if err != nil && output == "" {
		return []manager.Package{}, nil
	}

	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "The following") || strings.HasPrefix(line, "No installed ports") {
			continue
		}
		// "name <oldversion > <newversion" style output
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      fields[0],
			Source:    "macports",
			Installed: true,
		})
	}
	return pkgs, nil
}

// Clean removes cached package files.
func (m *MacPorts) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		m.SetDryRun(true)
		defer m.SetDryRun(false)
	}
	args := []string{"clean", "--all", "installed"}
	if opts.All {
		args = []string{"clean", "--all", "--dist", "--work", "--logs", "installed"}
	}
	return m.Executor().RunSudo(ctx, m.Binary(), args...)
}

// Autoremove removes leaf ports (no dependents).
func (m *MacPorts) Autoremove(ctx context.Context) error {
	return m.Executor().RunSudo(ctx, m.Binary(), "uninstall", "leaves")
}
