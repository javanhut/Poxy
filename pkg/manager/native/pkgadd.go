package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// PkgAdd implements the Manager interface for OpenBSD's pkg_add/pkg_delete
// suite. Unlike most PMs, OpenBSD has no persistent package database to
// refresh — pkg_add reads PKG_PATH / mirror URLs directly each time.
type PkgAdd struct {
	*BaseManager
}

// NewPkgAdd creates a new OpenBSD pkg_add manager instance.
func NewPkgAdd() *PkgAdd {
	return &PkgAdd{
		BaseManager: NewBaseManager("pkg_add", "pkg_add (OpenBSD)", "pkg_add", true),
	}
}

// Install installs one or more packages.
func (p *PkgAdd) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{}
	if opts.AutoConfirm {
		args = append(args, "-I")
	}
	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	_, err := p.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Uninstall removes one or more packages via pkg_delete.
func (p *PkgAdd) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{}
	if opts.Recursive {
		args = append(args, "-X")
	}
	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	_, err := p.RunOpCapturedAs(ctx, opts.OutputSink, "pkg_delete", args...)
	return err
}

// Update is a no-op — OpenBSD has no local package index to refresh.
func (p *PkgAdd) Update(_ context.Context) error {
	return nil
}

// Upgrade upgrades installed packages via `pkg_add -u`.
func (p *PkgAdd) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-u"}
	if opts.AutoConfirm {
		args = append(args, "-I")
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

// Search finds packages matching the query via pkg_info.
func (p *PkgAdd) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return p.searchInstalled(ctx, query, opts)
	}
	// pkg_info -Q performs a remote prefix search.
	output, err := p.Executor().Output(ctx, "pkg_info", "-Q", query)
	if err != nil {
		return []manager.Package{}, nil
	}
	return p.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled filters `pkg_info -A` by the query.
func (p *PkgAdd) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, "pkg_info", "-A")
	if err != nil {
		return nil, err
	}
	return p.parseListOutput(output, query, opts.Limit, true), nil
}

// parseSearchOutput parses `pkg_info -Q` output.
// Format per line: "name-version[-flavor]".
func (p *PkgAdd) parseSearchOutput(output string, limit int) []manager.Package {
	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		name, version := splitNameVersion(line)
		pkgs = append(pkgs, manager.Package{
			Name:    name,
			Version: version,
			Source:  "pkg_add",
		})
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// parseListOutput parses `pkg_info -A` output, optionally filtering by query.
func (p *PkgAdd) parseListOutput(output, filter string, limit int, installed bool) []manager.Package {
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
		desc := ""
		if len(fields) > 1 {
			desc = strings.Join(fields[1:], " ")
		}
		pkgs = append(pkgs, manager.Package{
			Name:        name,
			Version:     version,
			Description: desc,
			Source:      "pkg_add",
			Installed:   installed,
		})
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// Info returns detailed information about a package.
func (p *PkgAdd) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := p.Executor().Output(ctx, "pkg_info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	info := &manager.PackageInfo{
		Package: manager.Package{Source: "pkg_add", Name: pkg},
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Comment:") {
			info.Description = strings.TrimSpace(strings.TrimPrefix(line, "Comment:"))
		} else if strings.HasPrefix(line, "URL:") {
			info.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
		} else if strings.HasPrefix(line, "Maintainer:") {
			info.Maintainer = strings.TrimSpace(strings.TrimPrefix(line, "Maintainer:"))
		}
	}
	return info, nil
}

// ListInstalled returns all installed packages.
func (p *PkgAdd) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, "pkg_info", "-A")
	if err != nil {
		return nil, err
	}
	return p.parseListOutput(output, opts.Pattern, opts.Limit, true), nil
}

// IsInstalled checks if a package is installed.
func (p *PkgAdd) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := p.Executor().Run(ctx, "pkg_info", "-e", pkg+"-*")
	return err == nil, nil
}

// ListUpgradable returns installed packages with newer versions available.
// Uses `pkg_add -un` (simulate update without installing).
func (p *PkgAdd) ListUpgradable(ctx context.Context) ([]manager.Package, error) {
	output, err := p.Executor().OutputQuiet(ctx, p.Binary(), "-un")
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
		// Lines of the form: "name-oldver -> name-newver"
		parts := strings.Split(line, "->")
		if len(parts) != 2 {
			continue
		}
		oldName, oldVer := splitNameVersion(strings.TrimSpace(parts[0]))
		_, newVer := splitNameVersion(strings.TrimSpace(parts[1]))
		pkgs = append(pkgs, manager.Package{
			Name:             oldName,
			InstalledVersion: oldVer,
			Version:          newVer,
			Source:           "pkg_add",
			Installed:        true,
		})
	}
	return pkgs, nil
}

// Clean has no equivalent in the OpenBSD pkg suite.
func (p *PkgAdd) Clean(_ context.Context, _ manager.CleanOpts) error {
	return nil
}

// Autoremove removes orphaned dependencies via `pkg_delete -a`.
func (p *PkgAdd) Autoremove(ctx context.Context) error {
	return p.Executor().RunSudo(ctx, "pkg_delete", "-a")
}
