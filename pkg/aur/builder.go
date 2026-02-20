package aur

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"poxy/pkg/sandbox"
)

var (
	// ErrPackageNotFound is returned when a package cannot be found in the AUR
	ErrPackageNotFound = errors.New("package not found in AUR")

	// ErrBuildFailed is returned when makepkg fails
	ErrBuildFailed = errors.New("package build failed")

	// ErrInstallFailed is returned when pacman -U fails
	ErrInstallFailed = errors.New("package installation failed")

	// ErrMissingDependencies is returned when dependencies cannot be resolved
	ErrMissingDependencies = errors.New("missing dependencies")
)

// BuildOptions configures the build process.
type BuildOptions struct {
	// CleanBuild removes $srcdir before building
	CleanBuild bool

	// Force rebuilds even if package is up to date
	Force bool

	// SkipPGPCheck skips PGP signature verification
	SkipPGPCheck bool

	// NoConfirm automatically answers yes to prompts
	NoConfirm bool

	// UseSandbox runs the build in a bubblewrap sandbox
	UseSandbox bool

	// KeepSources keeps sources after building
	KeepSources bool

	// InstallDeps automatically installs missing dependencies
	InstallDeps bool

	// AsDeps marks installed packages as dependencies
	AsDeps bool

	// Verbose enables verbose output
	Verbose bool

	// ReviewPKGBUILD prompts for PKGBUILD review before building
	ReviewPKGBUILD bool

	// OnReview is called when PKGBUILD review is needed
	// Return true to continue, false to abort
	OnReview func(pkg *Package, pkgbuild *PKGBUILD) bool

	// OnProgress is called with progress updates
	OnProgress func(stage string, message string)
}

// DefaultBuildOptions returns sensible default options.
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		CleanBuild:     true,
		UseSandbox:     sandbox.IsAvailable(),
		InstallDeps:    true,
		ReviewPKGBUILD: true,
	}
}

// Builder handles building AUR packages.
type Builder struct {
	client   *Client
	cacheDir string
	options  BuildOptions
}

// NewBuilder creates a new AUR builder.
func NewBuilder(cacheDir string) *Builder {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		cacheDir = filepath.Join(home, ".cache", "poxy", "aur")
	}

	return &Builder{
		client:   NewClient(),
		cacheDir: cacheDir,
		options:  DefaultBuildOptions(),
	}
}

// SetOptions sets the build options.
func (b *Builder) SetOptions(opts BuildOptions) {
	b.options = opts
}

// CacheDir returns the cache directory.
func (b *Builder) CacheDir() string {
	return b.cacheDir
}

// Build builds an AUR package and returns the path to the built package(s).
func (b *Builder) Build(ctx context.Context, pkgName string) ([]string, error) {
	b.progress("fetch", "Fetching package info from AUR...")

	// Get package info from AUR
	pkg, err := b.client.GetPackage(ctx, pkgName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPackageNotFound, pkgName)
	}

	// Clone or update the package
	pkgDir := filepath.Join(b.cacheDir, pkg.PackageBase)
	if err := b.fetchPackage(ctx, pkg, pkgDir); err != nil {
		return nil, err
	}

	// Parse PKGBUILD
	pkgbuildPath := filepath.Join(pkgDir, "PKGBUILD")
	pkgbuild, err := ParsePKGBUILD(pkgbuildPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKGBUILD: %w", err)
	}

	// Review PKGBUILD if enabled
	if b.options.ReviewPKGBUILD && b.options.OnReview != nil {
		if !b.options.OnReview(pkg, pkgbuild) {
			return nil, fmt.Errorf("build aborted by user")
		}
	}

	// Check and install dependencies
	if b.options.InstallDeps {
		if err := b.installDependencies(ctx, pkgbuild); err != nil {
			return nil, err
		}
	}

	// Build the package
	b.progress("build", "Building package...")
	builtPkgs, err := b.runMakepkg(ctx, pkgDir)
	if err != nil {
		return nil, err
	}

	return builtPkgs, nil
}

// BuildAndInstall builds and installs an AUR package.
func (b *Builder) BuildAndInstall(ctx context.Context, pkgName string) error {
	builtPkgs, err := b.Build(ctx, pkgName)
	if err != nil {
		return err
	}

	// Install the built packages
	b.progress("install", "Installing package...")
	return b.installPackages(ctx, builtPkgs)
}

// fetchPackage clones or updates the AUR git repository.
func (b *Builder) fetchPackage(ctx context.Context, pkg *Package, pkgDir string) error {
	gitURL := pkg.GitCloneURL()

	// Check if already cloned
	if _, err := os.Stat(filepath.Join(pkgDir, ".git")); err == nil {
		// Update existing repo
		b.progress("fetch", "Updating package repository...")
		cmd := exec.CommandContext(ctx, "git", "pull", "--rebase")
		cmd.Dir = pkgDir
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard

		if err := cmd.Run(); err != nil {
			// Try fresh clone on failure
			os.RemoveAll(pkgDir)
		} else {
			return nil
		}
	}

	// Clone the repository
	b.progress("fetch", "Cloning package repository...")
	if err := os.MkdirAll(filepath.Dir(pkgDir), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", gitURL, pkgDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

// installDependencies installs missing dependencies using pacman.
func (b *Builder) installDependencies(ctx context.Context, pkgbuild *PKGBUILD) error {
	// Collect all dependencies
	deps := pkgbuild.AllDependencies()
	makedeps := pkgbuild.AllBuildDependencies()
	allDeps := append(deps, makedeps...)

	if len(allDeps) == 0 {
		return nil
	}

	// Filter to only missing dependencies
	missing := b.filterMissingDeps(ctx, allDeps)
	if len(missing) == 0 {
		return nil
	}

	b.progress("deps", fmt.Sprintf("Installing %d dependencies...", len(missing)))

	// Install with pacman
	args := []string{"-S", "--needed"}
	if b.options.NoConfirm {
		args = append(args, "--noconfirm")
	}
	if b.options.AsDeps {
		args = append(args, "--asdeps")
	}
	args = append(args, missing...)

	cmd := exec.CommandContext(ctx, "sudo", append([]string{"pacman"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrMissingDependencies, err)
	}

	return nil
}

// filterMissingDeps returns dependencies that are not installed.
func (b *Builder) filterMissingDeps(ctx context.Context, deps []string) []string {
	var missing []string

	for _, dep := range deps {
		// Extract package name (remove version constraints)
		pkgName := dep
		for _, sep := range []string{">=", "<=", "=", ">", "<"} {
			if idx := strings.Index(pkgName, sep); idx != -1 {
				pkgName = pkgName[:idx]
			}
		}

		// Check if installed
		cmd := exec.CommandContext(ctx, "pacman", "-Q", pkgName)
		if err := cmd.Run(); err != nil {
			missing = append(missing, dep)
		}
	}

	return missing
}

// runMakepkg runs makepkg to build the package.
func (b *Builder) runMakepkg(ctx context.Context, pkgDir string) ([]string, error) {
	args := []string{"-f"} // Force rebuild

	if b.options.CleanBuild {
		args = append(args, "-c") // Clean up work files
	}
	if b.options.SkipPGPCheck {
		args = append(args, "--skippgpcheck")
	}
	if b.options.NoConfirm {
		args = append(args, "--noconfirm")
	}

	var cmd *exec.Cmd

	if b.options.UseSandbox && sandbox.IsAvailable() {
		// Run in sandbox
		sb, err := sandbox.BuildSandbox(pkgDir, b.cacheDir)
		if err != nil {
			// Fall back to direct execution
			b.progress("build", "Sandbox unavailable, building directly...")
			cmd = exec.CommandContext(ctx, "makepkg", args...)
			cmd.Dir = pkgDir
		} else {
			// Configure sandbox
			sb.SetVerbose(b.options.Verbose)
			sb.Profile().AllowNetwork() // Need network for sources

			if err := sb.Run(ctx, "makepkg", args...); err != nil {
				// Only retry without sandbox if the sandbox itself failed.
				// If the command ran inside the sandbox and failed, retrying
				// without sandbox won't help (e.g. GLIBC mismatch, build errors).
				if errors.Is(err, sandbox.ErrCommandFailed) {
					return nil, fmt.Errorf("%w: %v", ErrBuildFailed, err)
				}
				b.progress("build", "Sandbox setup failed, retrying without sandbox...")
				cmd = exec.CommandContext(ctx, "makepkg", args...)
				cmd.Dir = pkgDir
			} else {
				return b.findBuiltPackages(pkgDir)
			}
		}
	} else {
		cmd = exec.CommandContext(ctx, "makepkg", args...)
		cmd.Dir = pkgDir
	}

	if cmd != nil {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBuildFailed, err)
		}
	}

	return b.findBuiltPackages(pkgDir)
}

// findBuiltPackages finds the built .pkg.tar.* files in the directory.
func (b *Builder) findBuiltPackages(pkgDir string) ([]string, error) {
	patterns := []string{
		"*.pkg.tar.zst",
		"*.pkg.tar.xz",
		"*.pkg.tar.gz",
		"*.pkg.tar",
	}

	var packages []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(pkgDir, pattern))
		if err != nil {
			continue
		}
		packages = append(packages, matches...)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no built packages found in %s", pkgDir)
	}

	return packages, nil
}

// installPackages installs built packages using pacman -U.
func (b *Builder) installPackages(ctx context.Context, packages []string) error {
	args := []string{"-U"}
	if b.options.NoConfirm {
		args = append(args, "--noconfirm")
	}
	if b.options.AsDeps {
		args = append(args, "--asdeps")
	}
	args = append(args, packages...)

	cmd := exec.CommandContext(ctx, "sudo", append([]string{"pacman"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	return nil
}

// progress reports progress if a handler is set.
func (b *Builder) progress(stage, message string) {
	if b.options.OnProgress != nil {
		b.options.OnProgress(stage, message)
	}
}

// Clean removes cached data for a package.
func (b *Builder) Clean(pkgName string) error {
	pkgDir := filepath.Join(b.cacheDir, pkgName)
	return os.RemoveAll(pkgDir)
}

// CleanAll removes all cached data.
func (b *Builder) CleanAll() error {
	return os.RemoveAll(b.cacheDir)
}

// ListCached returns a list of cached packages.
func (b *Builder) ListCached() ([]string, error) {
	entries, err := os.ReadDir(b.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var packages []string
	for _, entry := range entries {
		if entry.IsDir() {
			packages = append(packages, entry.Name())
		}
	}

	return packages, nil
}

// GetCachedPKGBUILD returns the PKGBUILD for a cached package.
func (b *Builder) GetCachedPKGBUILD(pkgName string) (*PKGBUILD, error) {
	pkgDir := filepath.Join(b.cacheDir, pkgName)
	pkgbuildPath := filepath.Join(pkgDir, "PKGBUILD")

	if _, err := os.Stat(pkgbuildPath); err != nil {
		return nil, fmt.Errorf("package not cached: %s", pkgName)
	}

	return ParsePKGBUILD(pkgbuildPath)
}
