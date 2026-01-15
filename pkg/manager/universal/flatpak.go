// Package universal implements cross-distribution package managers.
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

// Flatpak implements the Manager interface for Flatpak.
type Flatpak struct {
	name          string
	displayName   string
	binary        string
	defaultRemote string
	exec          *executor.Executor
}

// NewFlatpak creates a new Flatpak manager instance.
func NewFlatpak(defaultRemote string) *Flatpak {
	if defaultRemote == "" {
		defaultRemote = "flathub"
	}
	return &Flatpak{
		name:          "flatpak",
		displayName:   "Flatpak",
		binary:        "flatpak",
		defaultRemote: defaultRemote,
		exec:          executor.New(false, false),
	}
}

// Name returns the short identifier.
func (f *Flatpak) Name() string {
	return f.name
}

// DisplayName returns the human-readable name.
func (f *Flatpak) DisplayName() string {
	return f.displayName
}

// Type returns the manager type.
func (f *Flatpak) Type() manager.ManagerType {
	return manager.TypeUniversal
}

// IsAvailable returns true if Flatpak is installed.
func (f *Flatpak) IsAvailable() bool {
	_, err := exec.LookPath(f.binary)
	return err == nil
}

// NeedsSudo returns true if this manager needs root privileges.
func (f *Flatpak) NeedsSudo() bool {
	return false // User-level installations don't need sudo
}

// Install installs one or more Flatpak applications.
func (f *Flatpak) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	for _, pkg := range packages {
		args := []string{"install"}

		if opts.AutoConfirm {
			args = append(args, "-y")
		}

		// Add remote if package doesn't include it
		if !strings.Contains(pkg, "/") {
			args = append(args, f.defaultRemote)
		}

		args = append(args, pkg)

		if opts.DryRun {
			f.exec.SetDryRun(true)
			defer f.exec.SetDryRun(false)
		}

		if err := f.exec.Run(ctx, f.binary, args...); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall removes one or more Flatpak applications.
func (f *Flatpak) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"uninstall"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		f.exec.SetDryRun(true)
		defer f.exec.SetDryRun(false)
	}

	err := f.exec.Run(ctx, f.binary, args...)
	if err != nil {
		return err
	}

	if opts.Recursive {
		return f.Autoremove(ctx)
	}

	return nil
}

// Update for Flatpak is a no-op (Flatpak updates remotes automatically).
func (f *Flatpak) Update(ctx context.Context) error {
	return nil
}

// Upgrade updates all installed Flatpak applications.
func (f *Flatpak) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"update"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		f.exec.SetDryRun(true)
		defer f.exec.SetDryRun(false)
	}

	return f.exec.Run(ctx, f.binary, args...)
}

// Search finds Flatpak applications matching the query.
func (f *Flatpak) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return f.searchInstalled(ctx, query, opts)
	}

	output, err := f.exec.Output(ctx, f.binary, "search", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return f.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed applications.
func (f *Flatpak) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := f.exec.Output(ctx, f.binary, "list", "--columns=name,application,version")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		appID := fields[1]
		version := ""
		if len(fields) > 2 {
			version = fields[2]
		}

		if !strings.Contains(strings.ToLower(name), queryLower) &&
			!strings.Contains(strings.ToLower(appID), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      appID,
			Version:   version,
			Source:    "flatpak",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses flatpak search output.
func (f *Flatpak) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		// Flatpak search output: Name\tDescription\tApplication ID\tVersion\tRemotes
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		description := ""
		appID := ""
		version := ""

		if len(fields) > 1 {
			description = fields[1]
		}
		if len(fields) > 2 {
			appID = fields[2]
		}
		if len(fields) > 3 {
			version = fields[3]
		}

		packages = append(packages, manager.Package{
			Name:        appID,
			Version:     version,
			Description: name + ": " + description,
			Source:      "flatpak",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a Flatpak application.
func (f *Flatpak) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := f.exec.Output(ctx, f.binary, "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return f.parsePackageInfo(output), nil
}

// parsePackageInfo parses flatpak info output.
func (f *Flatpak) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "flatpak",
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
		case "ID", "Ref":
			if info.Name == "" {
				info.Name = value
			}
		case "Version":
			info.Version = value
		case "License":
			info.License = value
		case "Origin":
			info.Repository = value
		case "Installation":
			// This tells us if it's system or user installation
		case "Installed":
			info.Size = value
		}
	}

	return info
}

// ListInstalled returns all installed Flatpak applications.
func (f *Flatpak) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := f.exec.Output(ctx, f.binary, "list", "--columns=name,application,version")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		appID := fields[1]
		version := ""
		if len(fields) > 2 {
			version = fields[2]
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(appID), patternLower) &&
			!strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:        appID,
			Version:     version,
			Description: name,
			Source:      "flatpak",
			Installed:   true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a Flatpak application is installed.
func (f *Flatpak) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := f.exec.Run(ctx, f.binary, "info", pkg)
	return err == nil, nil
}

// Clean removes unused Flatpak data.
func (f *Flatpak) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		f.exec.SetDryRun(true)
		defer f.exec.SetDryRun(false)
	}

	return f.exec.Run(ctx, f.binary, "uninstall", "--unused", "-y")
}

// Autoremove removes unused runtimes and extensions.
func (f *Flatpak) Autoremove(ctx context.Context) error {
	return f.exec.Run(ctx, f.binary, "uninstall", "--unused", "-y")
}
