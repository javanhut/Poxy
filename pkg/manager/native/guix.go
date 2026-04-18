package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Guix implements the Manager interface for GNU Guix (System and Home).
// `guix install` operates on the current user's profile and needs no sudo.
type Guix struct {
	*BaseManager
}

// NewGuix creates a new Guix manager instance.
func NewGuix() *Guix {
	// needsSudo=false: guix install/remove target the user profile by default.
	return &Guix{
		BaseManager: NewBaseManager("guix", "GNU Guix", "guix", false),
	}
}

// Install installs one or more packages.
func (g *Guix) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}
	args = append(args, packages...)

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	_, err := g.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Uninstall removes one or more packages.
func (g *Guix) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"remove"}
	args = append(args, packages...)

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	_, err := g.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Update refreshes the package collection via `guix pull`.
func (g *Guix) Update(ctx context.Context) error {
	return g.Executor().Run(ctx, g.Binary(), "pull")
}

// Upgrade upgrades installed packages.
func (g *Guix) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	_, err := g.RunOpCaptured(ctx, opts.OutputSink, args...)
	return err
}

// Search finds packages matching the query.
func (g *Guix) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := g.Executor().Output(ctx, g.Binary(), "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}
	return g.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses `guix search` output. Records are separated by
// blank lines, and each record has "name: foo", "version: 1.2.3", "synopsis:",
// etc. We extract name, version, and synopsis.
func (g *Guix) parseSearchOutput(output string, limit int) []manager.Package {
	var pkgs []manager.Package
	records := strings.Split(output, "\n\n")
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		pkg := manager.Package{Source: "guix"}
		scanner := bufio.NewScanner(strings.NewReader(rec))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "name: ") {
				pkg.Name = strings.TrimPrefix(line, "name: ")
			} else if strings.HasPrefix(line, "version: ") {
				pkg.Version = strings.TrimPrefix(line, "version: ")
			} else if strings.HasPrefix(line, "synopsis: ") {
				pkg.Description = strings.TrimPrefix(line, "synopsis: ")
			}
		}
		if pkg.Name == "" {
			continue
		}
		pkgs = append(pkgs, pkg)
		if limit > 0 && len(pkgs) >= limit {
			break
		}
	}
	return pkgs
}

// Info returns detailed information about a package.
func (g *Guix) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := g.Executor().Output(ctx, g.Binary(), "show", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	info := &manager.PackageInfo{
		Package: manager.Package{Source: "guix", Name: pkg},
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "name: "):
			info.Name = strings.TrimPrefix(line, "name: ")
		case strings.HasPrefix(line, "version: "):
			info.Version = strings.TrimPrefix(line, "version: ")
		case strings.HasPrefix(line, "synopsis: "):
			info.Description = strings.TrimPrefix(line, "synopsis: ")
		case strings.HasPrefix(line, "homepage: "):
			info.URL = strings.TrimPrefix(line, "homepage: ")
		case strings.HasPrefix(line, "license: "):
			info.License = strings.TrimPrefix(line, "license: ")
		}
	}
	return info, nil
}

// ListInstalled returns all installed packages (current user profile).
func (g *Guix) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := g.Executor().Output(ctx, g.Binary(), "package", "--list-installed")
	if err != nil {
		return nil, err
	}

	var pkgs []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: name<tab>version<tab>output<tab>store-path
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
			Version:   fields[1],
			Source:    "guix",
			Installed: true,
		})
		if opts.Limit > 0 && len(pkgs) >= opts.Limit {
			break
		}
	}
	return pkgs, nil
}

// IsInstalled checks if a package is installed in the user profile.
func (g *Guix) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := g.Executor().Output(ctx, g.Binary(), "package", "--list-installed="+pkg)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(output) != "", nil
}

// Clean removes old generations and collects garbage.
func (g *Guix) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		return g.Executor().Run(ctx, g.Binary(), "gc", "--list-dead")
	}
	return g.Executor().Run(ctx, g.Binary(), "gc")
}

// Autoremove — Guix reclaims unreferenced items via `guix gc`.
func (g *Guix) Autoremove(ctx context.Context) error {
	return g.Executor().Run(ctx, g.Binary(), "gc")
}
