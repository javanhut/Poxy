package native

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"poxy/pkg/manager"
)

// Nix implements the Manager interface for NixOS/Nix package manager.
type Nix struct {
	*BaseManager
}

// NewNix creates a new Nix manager instance.
func NewNix() *Nix {
	return &Nix{
		BaseManager: NewBaseManager("nix", "Nix (NixOS)", "nix-env", false),
	}
}

// Install installs one or more packages.
func (n *Nix) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	// Convert package names to nix attribute paths
	args := []string{"-iA"}

	for _, pkg := range packages {
		// If not already prefixed, add nixpkgs prefix
		if !strings.Contains(pkg, ".") {
			pkg = "nixpkgs." + pkg
		}
		args = append(args, pkg)
	}

	if opts.DryRun {
		n.SetDryRun(true)
		defer n.SetDryRun(false)
	}

	return n.Executor().Run(ctx, n.Binary(), args...)
}

// Uninstall removes one or more packages.
func (n *Nix) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"-e"}
	args = append(args, packages...)

	if opts.DryRun {
		n.SetDryRun(true)
		defer n.SetDryRun(false)
	}

	return n.Executor().Run(ctx, n.Binary(), args...)
}

// Update refreshes the package database (updates channels).
func (n *Nix) Update(ctx context.Context) error {
	return n.Executor().Run(ctx, "nix-channel", "--update")
}

// Upgrade upgrades installed packages.
func (n *Nix) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"-u"}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		n.SetDryRun(true)
		defer n.SetDryRun(false)
	}

	return n.Executor().Run(ctx, n.Binary(), args...)
}

// Search finds packages matching the query.
func (n *Nix) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	// Try new nix search first
	output, err := n.Executor().Output(ctx, "nix", "search", "nixpkgs", query, "--json")
	if err == nil && output != "" {
		return n.parseNewSearchOutput(output, opts.Limit), nil
	}

	// Fall back to nix-env -qaP
	output, err = n.Executor().Output(ctx, n.Binary(), "-qaP", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return n.parseSearchOutput(output, opts.Limit), nil
}

// parseSearchOutput parses nix-env -qaP output.
func (n *Nix) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: attribute-path  package-name-version
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		attrPath := fields[0]
		nameVersion := fields[1]

		// Parse version from nameVersion
		version := ""
		lastDash := strings.LastIndex(nameVersion, "-")
		if lastDash > 0 {
			possibleVersion := nameVersion[lastDash+1:]
			if len(possibleVersion) > 0 && possibleVersion[0] >= '0' && possibleVersion[0] <= '9' {
				version = possibleVersion
			}
		}

		// Use attribute path as the name for installation
		packages = append(packages, manager.Package{
			Name:    attrPath,
			Version: version,
			Source:  "nix",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// parseNewSearchOutput parses nix search --json output.
func (n *Nix) parseNewSearchOutput(output string, limit int) []manager.Package {
	// For simplicity, just parse line by line for now
	// A proper implementation would use encoding/json
	var packages []manager.Package

	// Basic parsing - nix search --json output is complex
	// This is a simplified version
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "\"pname\"") {
			// Extract package info from JSON line
			// This is a simplified parser
			continue
		}
	}

	if len(packages) == 0 {
		// Fallback to line-based parsing
		return n.parseSearchOutput(output, limit)
	}

	return packages
}

// Info returns detailed information about a package.
func (n *Nix) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	// Add nixpkgs prefix if not present
	if !strings.Contains(pkg, ".") {
		pkg = "nixpkgs." + pkg
	}

	output, err := n.Executor().Output(ctx, n.Binary(), "-qaA", pkg, "--description")
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	info := &manager.PackageInfo{
		Package: manager.Package{
			Name:        pkg,
			Description: strings.TrimSpace(output),
			Source:      "nix",
		},
	}

	return info, nil
}

// ListInstalled returns all installed packages.
func (n *Nix) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := n.Executor().Output(ctx, n.Binary(), "-q")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse name-version
		name := line
		version := ""
		lastDash := strings.LastIndex(line, "-")
		if lastDash > 0 {
			possibleVersion := line[lastDash+1:]
			if len(possibleVersion) > 0 && possibleVersion[0] >= '0' && possibleVersion[0] <= '9' {
				name = line[:lastDash]
				version = possibleVersion
			}
		}

		if opts.Pattern != "" && !strings.Contains(strings.ToLower(name), patternLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "nix",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a package is installed.
func (n *Nix) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := n.Executor().Output(ctx, n.Binary(), "-q")
	if err != nil {
		return false, nil
	}

	pkgLower := strings.ToLower(pkg)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), pkgLower) {
			return true, nil
		}
	}

	return false, nil
}

// Clean removes cached package files (garbage collection).
func (n *Nix) Clean(ctx context.Context, opts manager.CleanOpts) error {
	if opts.DryRun {
		n.SetDryRun(true)
		defer n.SetDryRun(false)
	}

	args := []string{"-d"}
	if opts.All {
		// Delete all old generations
		args = append(args, "--delete-older-than", "1d")
	}

	return n.Executor().Run(ctx, "nix-collect-garbage", args...)
}

// Autoremove removes orphaned packages.
func (n *Nix) Autoremove(ctx context.Context) error {
	// Nix handles this through garbage collection
	return n.Clean(ctx, manager.CleanOpts{})
}
