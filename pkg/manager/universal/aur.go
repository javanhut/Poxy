package universal

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"poxy/internal/executor"
	"poxy/pkg/manager"
)

// AUR implements the Manager interface for Arch User Repository helpers (yay, paru).
type AUR struct {
	name        string
	displayName string
	binary      string
	helper      string // "yay" or "paru"
	exec        *executor.Executor
}

// NewAUR creates a new AUR helper manager instance.
// It auto-detects which helper is available (prefers yay, then paru).
func NewAUR(preferredHelper string) *AUR {
	helper := detectAURHelper(preferredHelper)
	if helper == "" {
		return nil
	}

	displayName := "AUR"
	switch helper {
	case "yay":
		displayName = "Yay (AUR)"
	case "paru":
		displayName = "Paru (AUR)"
	}

	return &AUR{
		name:        "aur",
		displayName: displayName,
		binary:      helper,
		helper:      helper,
		exec:        executor.New(false, false),
	}
}

// detectAURHelper finds an available AUR helper.
func detectAURHelper(preferred string) string {
	// Check preferred first
	if preferred != "" {
		if _, err := exec.LookPath(preferred); err == nil {
			return preferred
		}
	}

	// Check common AUR helpers
	helpers := []string{"yay", "paru", "trizen", "aurman"}
	for _, h := range helpers {
		if _, err := exec.LookPath(h); err == nil {
			return h
		}
	}

	return ""
}

// Name returns the short identifier.
func (a *AUR) Name() string {
	return a.name
}

// DisplayName returns the human-readable name.
func (a *AUR) DisplayName() string {
	return a.displayName
}

// Type returns the manager type.
func (a *AUR) Type() manager.ManagerType {
	return manager.TypeAUR
}

// IsAvailable returns true if an AUR helper is installed.
func (a *AUR) IsAvailable() bool {
	if a == nil || a.helper == "" {
		return false
	}
	_, err := exec.LookPath(a.helper)
	return err == nil
}

// NeedsSudo returns false (AUR helpers manage sudo themselves).
func (a *AUR) NeedsSudo() bool {
	return false
}

// Install installs one or more AUR packages.
func (a *AUR) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"-S"}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	args = append(args, packages...)

	if opts.DryRun {
		a.exec.SetDryRun(true)
		defer a.exec.SetDryRun(false)
	}

	return a.exec.Run(ctx, a.binary, args...)
}

// Uninstall removes one or more packages.
func (a *AUR) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"-R"}

	if opts.Recursive {
		args = []string{"-Rs"}
	}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	args = append(args, packages...)

	if opts.DryRun {
		a.exec.SetDryRun(true)
		defer a.exec.SetDryRun(false)
	}

	return a.exec.Run(ctx, a.binary, args...)
}

// Update refreshes the package database.
func (a *AUR) Update(ctx context.Context) error {
	return a.exec.Run(ctx, a.binary, "-Sy")
}

// Upgrade upgrades all packages including AUR.
func (a *AUR) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-Syu"}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	if len(opts.Packages) > 0 {
		args = []string{"-S"}
		if opts.AutoConfirm {
			args = append(args, "--noconfirm")
		}
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		a.exec.SetDryRun(true)
		defer a.exec.SetDryRun(false)
	}

	return a.exec.Run(ctx, a.binary, args...)
}

// Search finds packages matching the query (includes AUR).
func (a *AUR) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return a.searchInstalled(ctx, query, opts)
	}

	// Search AUR specifically
	output, err := a.exec.Output(ctx, a.binary, "-Ssa", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return a.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed AUR packages.
func (a *AUR) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	// Get foreign packages (AUR packages)
	output, err := a.exec.Output(ctx, a.binary, "-Qm")
	if err != nil {
		return []manager.Package{}, nil
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		version := fields[1]

		if query != "" && !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "aur",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses AUR helper search output.
func (a *AUR) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	lines := strings.Split(output, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Package line format: aur/package-name version
		if strings.HasPrefix(line, "aur/") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}

			// Parse aur/package-name
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

			packages = append(packages, manager.Package{
				Name:        name,
				Version:     version,
				Description: description,
				Source:      "aur",
			})

			if limit > 0 && len(packages) >= limit {
				break
			}
		}
	}

	return packages
}

// Info returns detailed information about an AUR package.
func (a *AUR) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	// Try remote first
	output, err := a.exec.Output(ctx, a.binary, "-Si", pkg)
	if err != nil {
		// Try local
		output, err = a.exec.Output(ctx, a.binary, "-Qi", pkg)
		if err != nil {
			return nil, fmt.Errorf("package '%s' not found", pkg)
		}
	}

	return a.parsePackageInfo(output), nil
}

// parsePackageInfo parses pacman-style info output.
func (a *AUR) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "aur",
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
		case "Maintainer", "Packager":
			info.Maintainer = value
		case "Installed Size":
			info.Size = value
		}
	}

	return info
}

// ListInstalled returns all installed AUR packages.
func (a *AUR) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	// Get foreign packages (AUR packages)
	output, err := a.exec.Output(ctx, a.binary, "-Qm")
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
			Source:    "aur",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if an AUR package is installed.
func (a *AUR) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := a.exec.Run(ctx, a.binary, "-Qi", pkg)
	return err == nil, nil
}

// Clean removes cached package files.
func (a *AUR) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		a.exec.SetDryRun(true)
		defer a.exec.SetDryRun(false)
	}

	args := []string{"-Sc", "--noconfirm"}
	if opts.All {
		args = []string{"-Scc", "--noconfirm"}
	}

	return a.exec.Run(ctx, a.binary, args...)
}

// Autoremove removes orphaned packages.
func (a *AUR) Autoremove(ctx context.Context) error {
	// Get list of orphans
	output, err := a.exec.Output(ctx, a.binary, "-Qdtq")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	orphans := strings.Fields(output)
	args := append([]string{"-Rs", "--noconfirm"}, orphans...)

	return a.exec.Run(ctx, a.binary, args...)
}
