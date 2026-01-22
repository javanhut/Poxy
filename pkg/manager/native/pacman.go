package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Pacman implements the Manager interface for Arch Linux's pacman package manager.
type Pacman struct {
	*BaseManager
}

// NewPacman creates a new Pacman manager instance.
func NewPacman() *Pacman {
	return &Pacman{
		BaseManager: NewBaseManager("pacman", "Pacman (Arch Linux)", "pacman", true),
	}
}

// Install installs one or more packages.
func (p *Pacman) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"-S"}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	if opts.Reinstall {
		args = append(args, "--needed")
	}

	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	stderr, err := p.Executor().RunSudoWithStderr(ctx, p.Binary(), args...)
	if err != nil {
		// Try to parse the error for better handling
		if pacErr := ParsePacmanError(stderr, err); pacErr != nil {
			return pacErr
		}
		return err
	}
	return nil
}

// Uninstall removes one or more packages.
func (p *Pacman) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"-R"}

	if opts.Recursive {
		args = []string{"-Rs"} // Remove with dependencies
	}

	if opts.Purge {
		args[0] = args[0] + "n" // -Rn or -Rsn to remove config files
	}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	args = append(args, packages...)

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	return p.Executor().RunSudo(ctx, p.Binary(), args...)
}

// Update refreshes the package database.
func (p *Pacman) Update(ctx context.Context) error {
	return p.Executor().RunSudo(ctx, p.Binary(), "-Sy")
}

// Upgrade upgrades installed packages.
func (p *Pacman) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-Syu"}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	if len(opts.Packages) > 0 {
		// Upgrade specific packages only
		args = []string{"-S"}
		if opts.AutoConfirm {
			args = append(args, "--noconfirm")
		}
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	return p.Executor().RunSudo(ctx, p.Binary(), args...)
}

// Search finds packages matching the query.
func (p *Pacman) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return p.searchInstalled(ctx, query, opts)
	}

	output, err := p.Executor().Output(ctx, p.Binary(), "-Ss", query)
	if err != nil {
		// Pacman returns error if no results
		return []manager.Package{}, nil
	}

	return p.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed packages.
func (p *Pacman) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, p.Binary(), "-Qs", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return p.parseLocalSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses pacman -Ss output.
func (p *Pacman) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	lines := strings.Split(output, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Repository/package line: repo/package version
		if strings.Contains(line, "/") && !strings.HasPrefix(line, " ") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}

			// Parse repo/package
			repoPkg := strings.SplitN(parts[0], "/", 2)
			if len(repoPkg) < 2 {
				continue
			}

			name := repoPkg[1]
			version := parts[1]

			// Get description from next line
			var description string
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], " ") {
				description = strings.TrimSpace(lines[i+1])
				i++
			}

			// Check if installed
			installed := false
			for _, p := range parts {
				if p == "[installed]" {
					installed = true
					break
				}
			}

			packages = append(packages, manager.Package{
				Name:        name,
				Version:     version,
				Description: description,
				Source:      "pacman",
				Installed:   installed,
			})

			if limit > 0 && len(packages) >= limit {
				break
			}
		}
	}

	return packages
}

// parseLocalSearchOutput parses pacman -Qs output.
func (p *Pacman) parseLocalSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	lines := strings.Split(output, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.Contains(line, "/") && !strings.HasPrefix(line, " ") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}

			repoPkg := strings.SplitN(parts[0], "/", 2)
			if len(repoPkg) < 2 {
				continue
			}

			name := repoPkg[1]
			version := parts[1]

			var description string
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], " ") {
				description = strings.TrimSpace(lines[i+1])
				i++
			}

			packages = append(packages, manager.Package{
				Name:        name,
				Version:     version,
				Description: description,
				Source:      "pacman",
				Installed:   true,
			})

			if limit > 0 && len(packages) >= limit {
				break
			}
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (p *Pacman) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	// Try remote first (quiet to suppress "not found" errors)
	output, err := p.Executor().OutputQuiet(ctx, p.Binary(), "-Si", pkg)
	if err != nil {
		// Try local
		output, err = p.Executor().OutputQuiet(ctx, p.Binary(), "-Qi", pkg)
		if err != nil {
			return nil, fmt.Errorf("package '%s' not found", pkg)
		}
	}

	return p.parsePackageInfo(output), nil
}

// parsePackageInfo parses pacman -Si/-Qi output.
func (p *Pacman) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "pacman",
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
		case "URL":
			info.URL = value
		case "Licenses":
			info.License = value
		case "Repository":
			info.Repository = value
		case "Installed Size":
			info.Size = value
		case "Packager":
			info.Maintainer = value
		case "Depends On":
			if value != "None" {
				info.Dependencies = strings.Fields(value)
			}
		}
	}

	return info
}

// ListInstalled returns all installed packages.
func (p *Pacman) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := p.Executor().Output(ctx, p.Binary(), "-Q")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := scanner.Text()
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
			Source:    "pacman",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (p *Pacman) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := p.Executor().Run(ctx, p.Binary(), "-Qi", pkg)
	return err == nil, nil
}

// Clean removes cached package files.
func (p *Pacman) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		p.SetDryRun(true)
		defer p.SetDryRun(false)
	}

	args := []string{"-Sc", "--noconfirm"}
	if opts.All {
		args = []string{"-Scc", "--noconfirm"}
	}

	return p.Executor().RunSudo(ctx, p.Binary(), args...)
}

// Autoremove removes orphaned packages.
func (p *Pacman) Autoremove(ctx context.Context) error {
	// Get list of orphans
	output, err := p.Executor().Output(ctx, p.Binary(), "-Qdtq")
	if err != nil || strings.TrimSpace(output) == "" {
		// No orphans
		return nil
	}

	orphans := strings.Fields(output)
	args := append([]string{"-Rs", "--noconfirm"}, orphans...)

	return p.Executor().RunSudo(ctx, p.Binary(), args...)
}
