package universal

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"poxy/internal/executor"
	"poxy/pkg/aur"
	"poxy/pkg/manager"
)

// NativeAUR implements the Manager interface using poxy's native AUR support.
// It builds packages directly without requiring yay/paru.
type NativeAUR struct {
	name           string
	displayName    string
	client         *aur.Client
	builder        *aur.Builder
	exec           *executor.Executor
	reviewPKGBUILD bool
}

// NewNativeAUR creates a new native AUR manager.
func NewNativeAUR(reviewPKGBUILD bool) *NativeAUR {
	return &NativeAUR{
		name:           "aur",
		displayName:    "AUR (Native)",
		client:         aur.NewClient(),
		builder:        aur.NewBuilder(""),
		exec:           executor.New(false, false),
		reviewPKGBUILD: reviewPKGBUILD,
	}
}

// Name returns the short identifier.
func (a *NativeAUR) Name() string {
	return a.name
}

// DisplayName returns the human-readable name.
func (a *NativeAUR) DisplayName() string {
	return a.displayName
}

// Type returns the manager type.
func (a *NativeAUR) Type() manager.ManagerType {
	return manager.TypeAUR
}

// IsAvailable returns true if we can build AUR packages.
// Requires: git, makepkg, pacman
func (a *NativeAUR) IsAvailable() bool {
	// Check for required tools
	required := []string{"git", "makepkg", "pacman"}
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			return false
		}
	}
	return true
}

// NeedsSudo returns false (we handle sudo internally for pacman -U).
func (a *NativeAUR) NeedsSudo() bool {
	return false
}

// Install builds and installs one or more AUR packages.
func (a *NativeAUR) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	buildOpts := aur.DefaultBuildOptions()
	buildOpts.NoConfirm = opts.AutoConfirm
	buildOpts.ReviewPKGBUILD = a.reviewPKGBUILD && !opts.AutoConfirm

	if a.reviewPKGBUILD && !opts.AutoConfirm {
		buildOpts.OnReview = aur.CreateReviewCallback(true)
	}

	a.builder.SetOptions(buildOpts)

	if opts.DryRun {
		fmt.Printf("Would build and install from AUR: %s\n", strings.Join(packages, ", "))
		return nil
	}

	for _, pkg := range packages {
		if err := a.builder.BuildAndInstall(ctx, pkg); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}

	return nil
}

// Uninstall removes one or more packages using pacman.
func (a *NativeAUR) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"-R"}

	if opts.Recursive {
		args = []string{"-Rs"}
	}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	args = append(args, packages...)

	if opts.DryRun {
		fmt.Printf("Would run: sudo pacman %s\n", strings.Join(args, " "))
		return nil
	}

	return a.exec.RunSudo(ctx, "pacman", args...)
}

// Update refreshes the package database.
func (a *NativeAUR) Update(ctx context.Context) error {
	return a.exec.RunSudo(ctx, "pacman", "-Sy")
}

// Upgrade upgrades all packages including AUR.
func (a *NativeAUR) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	// For AUR upgrades, we'd need to compare installed versions with AUR
	// For now, just upgrade official packages via pacman
	args := []string{"-Syu"}

	if opts.AutoConfirm {
		args = append(args, "--noconfirm")
	}

	if opts.DryRun {
		fmt.Printf("Would run: sudo pacman %s\n", strings.Join(args, " "))
		return nil
	}

	return a.exec.RunSudo(ctx, "pacman", args...)
}

// Search finds packages matching the query in the AUR.
func (a *NativeAUR) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return a.searchInstalled(ctx, query, opts)
	}

	// Search AUR API
	aurPkgs, err := a.client.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	for i, pkg := range aurPkgs {
		if opts.Limit > 0 && i >= opts.Limit {
			break
		}

		packages = append(packages, manager.Package{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Description: pkg.Description,
			Source:      "aur",
		})
	}

	return packages, nil
}

// searchInstalled searches installed foreign (AUR) packages.
func (a *NativeAUR) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := a.exec.Output(ctx, "pacman", "-Qm")
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

// Info returns detailed information about an AUR package.
func (a *NativeAUR) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	// Try AUR API first
	aurPkg, err := a.client.GetPackage(ctx, pkg)
	if err == nil {
		return &manager.PackageInfo{
			Package: manager.Package{
				Name:        aurPkg.Name,
				Version:     aurPkg.Version,
				Description: aurPkg.Description,
				Source:      "aur",
			},
			URL:        aurPkg.URL,
			Maintainer: aurPkg.Maintainer,
			License:    strings.Join(aurPkg.License, ", "),
		}, nil
	}

	// Fall back to local query (quiet to suppress errors)
	output, err := a.exec.OutputQuiet(ctx, "pacman", "-Qi", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return parsePackageInfo(output, "aur"), nil
}

// ListInstalled returns all installed AUR (foreign) packages.
func (a *NativeAUR) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := a.exec.Output(ctx, "pacman", "-Qm")
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
func (a *NativeAUR) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	err := a.exec.Run(ctx, "pacman", "-Qi", pkg)
	return err == nil, nil
}

// Clean removes cached AUR build files.
func (a *NativeAUR) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		fmt.Printf("Would clean AUR cache at %s\n", a.builder.CacheDir())
		return nil
	}

	if opts.All {
		return a.builder.CleanAll()
	}

	// Clean old builds (not all)
	return nil
}

// Autoremove removes orphaned packages.
func (a *NativeAUR) Autoremove(ctx context.Context) error {
	output, err := a.exec.Output(ctx, "pacman", "-Qdtq")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	orphans := strings.Fields(output)
	args := append([]string{"-Rs", "--noconfirm"}, orphans...)

	return a.exec.RunSudo(ctx, "pacman", args...)
}

// parsePackageInfo parses pacman -Qi output.
func parsePackageInfo(output, source string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: source,
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
