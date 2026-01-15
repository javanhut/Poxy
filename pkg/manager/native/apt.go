package native

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"poxy/pkg/manager"
)

// APT implements the Manager interface for Debian/Ubuntu's APT package manager.
type APT struct {
	*BaseManager
	useNala bool
}

// NewAPT creates a new APT manager instance.
func NewAPT(useNala bool) *APT {
	binary := "apt"
	displayName := "APT (Debian/Ubuntu)"

	// Check if nala is available and preferred
	if useNala {
		if _, err := exec.LookPath("nala"); err == nil {
			binary = "nala"
			displayName = "Nala (APT Frontend)"
		}
	}

	return &APT{
		BaseManager: NewBaseManager("apt", displayName, binary, true),
		useNala:     useNala && binary == "nala",
	}
}

// Install installs one or more packages.
func (a *APT) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	args := []string{"install"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if opts.Reinstall {
		args = append(args, "--reinstall")
	}

	args = append(args, packages...)

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), args...)
}

// Uninstall removes one or more packages.
func (a *APT) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	cmd := "remove"
	if opts.Purge {
		cmd = "purge"
	}

	args := []string{cmd}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	args = append(args, packages...)

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	err := a.Executor().RunSudo(ctx, a.Binary(), args...)
	if err != nil {
		return err
	}

	// Optionally remove unused dependencies
	if opts.Recursive {
		return a.Autoremove(ctx)
	}

	return nil
}

// Update refreshes the package database.
func (a *APT) Update(ctx context.Context) error {
	return a.Executor().RunSudo(ctx, a.Binary(), "update")
}

// Upgrade upgrades installed packages.
func (a *APT) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"upgrade"}

	if opts.AutoConfirm {
		args = append(args, "-y")
	}

	if len(opts.Packages) > 0 {
		// Upgrade specific packages
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	return a.Executor().RunSudo(ctx, a.Binary(), args...)
}

// Search finds packages matching the query.
func (a *APT) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	args := []string{"search"}

	if opts.InstalledOnly {
		// Use dpkg for installed packages
		return a.searchInstalled(ctx, query, opts)
	}

	args = append(args, query)

	output, err := a.Executor().Output(ctx, "apt-cache", "search", query)
	if err != nil {
		return nil, err
	}

	return a.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed packages.
func (a *APT) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := a.Executor().Output(ctx, "dpkg", "-l")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "ii") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[1]
		version := fields[2]

		// Filter by query
		if !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "apt",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses apt-cache search output.
func (a *APT) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) < 2 {
			continue
		}

		packages = append(packages, manager.Package{
			Name:        strings.TrimSpace(parts[0]),
			Description: strings.TrimSpace(parts[1]),
			Source:      "apt",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a package.
func (a *APT) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := a.Executor().Output(ctx, "apt-cache", "show", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return a.parsePackageInfo(output), nil
}

// parsePackageInfo parses apt-cache show output.
func (a *APT) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "apt",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentField string
	var descriptionLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Handle multi-line fields
		if strings.HasPrefix(line, " ") && currentField == "Description" {
			descriptionLines = append(descriptionLines, strings.TrimSpace(line))
			continue
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]
		currentField = key

		switch key {
		case "Package":
			info.Name = value
		case "Version":
			info.Version = value
		case "Description", "Description-en":
			info.Description = value
			descriptionLines = nil
		case "Maintainer":
			info.Maintainer = value
		case "Homepage":
			info.URL = value
		case "Depends":
			info.Dependencies = a.parseDependencies(value)
		case "Section":
			info.Repository = value
		case "Installed-Size":
			info.Size = value + " KB"
		}
	}

	// Append additional description lines
	if len(descriptionLines) > 0 {
		info.Description = info.Description + "\n" + strings.Join(descriptionLines, "\n")
	}

	return info
}

// parseDependencies parses the Depends field.
func (a *APT) parseDependencies(deps string) []string {
	var result []string
	re := regexp.MustCompile(`([a-zA-Z0-9._+-]+)`)

	for _, dep := range strings.Split(deps, ",") {
		matches := re.FindStringSubmatch(strings.TrimSpace(dep))
		if len(matches) > 0 {
			result = append(result, matches[1])
		}
	}

	return result
}

// ListInstalled returns all installed packages.
func (a *APT) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := a.Executor().Output(ctx, "dpkg-query", "-W", "-f=${Package}\\t${Version}\\t${Status}\\n")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}

		// Only show installed packages
		if !strings.Contains(fields[2], "installed") {
			continue
		}

		name := fields[0]
		version := fields[1]

		// Filter by pattern if specified
		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "apt",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (a *APT) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := a.Executor().Output(ctx, "dpkg-query", "-W", "-f=${Status}", pkg)
	if err != nil {
		return false, nil // Package not found = not installed
	}
	return strings.Contains(output, "installed"), nil
}

// Clean removes cached package files.
func (a *APT) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		a.SetDryRun(true)
		defer a.SetDryRun(false)
	}

	cmd := "autoclean"
	if opts.All {
		cmd = "clean"
	}

	return a.Executor().RunSudo(ctx, a.Binary(), cmd)
}

// Autoremove removes orphaned packages.
func (a *APT) Autoremove(ctx context.Context) error {
	return a.Executor().RunSudo(ctx, a.Binary(), "autoremove", "-y")
}
