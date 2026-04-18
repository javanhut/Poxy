package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// PkgBSD implements the Manager interface for FreeBSD's pkg(8).
type PkgBSD struct {
	*BaseManager
}

// NewPkgBSD creates a new FreeBSD pkg manager instance.
func NewPkgBSD() *PkgBSD {
	return &PkgBSD{
		BaseManager: NewBaseManager("pkg", "pkg (FreeBSD)", "pkg", true),
	}
}

// Install installs one or more packages.
func (p *PkgBSD) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}
	if opts.AutoConfirm {
		args = append(args, "-y")
	}
	if opts.Reinstall {
		args = append(args, "-f")
	}
	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	_, err := p.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Uninstall removes one or more packages.
func (p *PkgBSD) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"delete"}
	if opts.AutoConfirm {
		args = append(args, "-y")
	}
	if opts.Recursive {
		args = append(args, "-R")
	}
	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	_, err := p.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Update refreshes the package catalog.
func (p *PkgBSD) Update(ctx context.Context) error {
	return p.Executor().RunSudo(ctx, p.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (p *PkgBSD) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}
	if opts.AutoConfirm {
		args = append(args, "-y")
	}
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	_, err := p.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Search finds packages matching the query.
func (p *PkgBSD) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search"}
	if opts.InstalledOnly {
		return p.searchInstalled(ctx, query, opts)
	}
	args = append(args, query)

	output, err := p.Executor().Output(ctx, p.Binary(), args...)
	if err != nil {
		return []manager.Package{}, nil
	}
	return p.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed packages.
func (p *PkgBSD) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, p.Binary(), "info", "-a")
	if err != nil {
		return nil, err
	}
	return p.parseListOutput(output, query, opts.Limit, true), nil
}

// parseSearchOutput parses `pkg search` output.
// Format per line: "name-version  short description"
func (p *PkgBSD) parseSearchOutput(output string, limit int) []manager.Package {
	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Two-column format with variable whitespace.
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		nameVer := fields[0]
		name, version := splitNameVersion(nameVer)
		desc := ""
		if len(fields) > 1 {
			desc = strings.Join(fields[1:], " ")
		}
		pkgs = append(pkgs, manager.Package{
			Name:        name,
			Version:     version,
			Description: desc,
			Source:      "pkg",
		})
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// parseListOutput parses the name-version format produced by `pkg info -a`.
func (p *PkgBSD) parseListOutput(output, filter string, limit int, installed bool) []manager.Package {
	var pkgs []manager.Package
	filterLower := strings.ToLower(filter)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		name, version := splitNameVersion(fields[0])
		if filter != "" && !strings.Contains(strings.ToLower(name), filterLower) {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "pkg",
			Installed: installed,
		})
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// splitNameVersion splits a "name-version" string at the last dash that
// precedes something starting with a digit. Falls back to returning the
// whole string as name.
func splitNameVersion(s string) (name, version string) {
	for i := len(s) - 1; i > 0; i-- {
		if s[i] == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// Info returns detailed information about a package.
func (p *PkgBSD) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := p.Executor().Output(ctx, p.Binary(), "info", pkg)
	if err != nil {
		// Try remote query
		output, err = p.Executor().Output(ctx, p.Binary(), "rquery", "%n %v %c %w %l %e", pkg)
		if err != nil {
			return nil, fmt.Errorf("package '%s' not found", pkg)
		}
	}

	info := &manager.PackageInfo{
		Package: manager.Package{Source: "pkg", Name: pkg},
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
		case "Comment":
			info.Description = value
		case "WWW":
			info.URL = value
		case "Licenses":
			info.License = value
		case "Maintainer":
			info.Maintainer = value
		case "Flat size":
			info.Size = value
		}
	}
	return info, nil
}

// ListInstalled returns all installed packages.
func (p *PkgBSD) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, p.Binary(), "info", "-a")
	if err != nil {
		return nil, err
	}
	return p.parseListOutput(output, opts.Pattern, opts.Limit, true), nil
}

// IsInstalled checks if a package is installed.
func (p *PkgBSD) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := p.Executor().Run(ctx, p.Binary(), "info", "-e", pkg)
	return err == nil, nil
}

// ListUpgradable returns installed packages with newer versions available.
func (p *PkgBSD) ListUpgradable(ctx context.Context) ([]manager.Package, error) {
	// pkg version -vIL= prints lines for packages where local != remote.
	output, err := p.Executor().OutputQuiet(ctx, p.Binary(), "version", "-vIL=")
	if err != nil && output == "" {
		return []manager.Package{}, nil
	}

	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "name-version    status    (reason)"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Only lines marked with "<" indicate an older local version
		if fields[1] != "<" {
			continue
		}
		name, version := splitNameVersion(fields[0])
		pkgs = append(pkgs, manager.Package{
			Name:             name,
			InstalledVersion: version,
			Source:           "pkg",
			Installed:        true,
		})
	}
	return pkgs, nil
}

// Clean removes cached package files.
func (p *PkgBSD) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}
	args := []string{"clean", "-y"}
	if opts.All {
		args = append(args, "-a")
	}
	return p.Executor().RunSudo(ctx, p.Binary(), args...)
}

// Autoremove removes orphaned packages.
func (p *PkgBSD) Autoremove(ctx context.Context) error {
	return p.Executor().RunSudo(ctx, p.Binary(), "autoremove", "-y")
}
