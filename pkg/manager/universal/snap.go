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

// Snap implements the Manager interface for Snap.
type Snap struct {
	name         string
	displayName  string
	binary       string
	allowClassic bool
	exec         *executor.Executor
}

// NewSnap creates a new Snap manager instance.
func NewSnap(allowClassic bool) *Snap {
	return &Snap{
		name:         "snap",
		displayName:  "Snap",
		binary:       "snap",
		allowClassic: allowClassic,
		exec:         executor.New(false, false),
	}
}

// Name returns the short identifier.
func (s *Snap) Name() string {
	return s.name
}

// DisplayName returns the human-readable name.
func (s *Snap) DisplayName() string {
	return s.displayName
}

// Type returns the manager type.
func (s *Snap) Type() manager.ManagerType {
	return manager.TypeUniversal
}

// IsAvailable returns true if Snap is installed.
func (s *Snap) IsAvailable() bool {
	_, err := exec.LookPath(s.binary)
	return err == nil
}

// NeedsSudo returns true if this manager needs root privileges.
func (s *Snap) NeedsSudo() bool {
	return true // Snap typically requires sudo
}

// Install installs one or more Snap packages.
func (s *Snap) Install(ctx context.Context, packages []string, opts manager.InstallOpts) error {
	for _, pkg := range packages {
		args := []string{"install", pkg}

		if s.allowClassic {
			args = append(args, "--classic")
		}

		if opts.DryRun {
			s.exec.SetDryRun(true)
			defer s.exec.SetDryRun(false)
		}

		if err := s.exec.RunSudo(ctx, s.binary, args...); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall removes one or more Snap packages.
func (s *Snap) Uninstall(ctx context.Context, packages []string, opts manager.UninstallOpts) error {
	args := []string{"remove"}
	args = append(args, packages...)

	if opts.Purge {
		args = append(args, "--purge")
	}

	if opts.DryRun {
		s.exec.SetDryRun(true)
		defer s.exec.SetDryRun(false)
	}

	return s.exec.RunSudo(ctx, s.binary, args...)
}

// Update for Snap is a no-op (Snap handles this automatically).
func (s *Snap) Update(ctx context.Context) error {
	return nil
}

// Upgrade refreshes all installed Snap packages.
func (s *Snap) Upgrade(ctx context.Context, opts manager.UpgradeOpts) error {
	args := []string{"refresh"}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	if opts.DryRun {
		s.exec.SetDryRun(true)
		defer s.exec.SetDryRun(false)
	}

	return s.exec.RunSudo(ctx, s.binary, args...)
}

// Search finds Snap packages matching the query.
func (s *Snap) Search(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	if opts.InstalledOnly {
		return s.searchInstalled(ctx, query, opts)
	}

	output, err := s.exec.Output(ctx, s.binary, "find", query)
	if err != nil {
		return []manager.Package{}, nil
	}

	return s.parseSearchOutput(output, opts.Limit), nil
}

// searchInstalled searches installed snaps.
func (s *Snap) searchInstalled(ctx context.Context, query string, opts manager.SearchOpts) ([]manager.Package, error) {
	output, err := s.exec.Output(ctx, s.binary, "list")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	queryLower := strings.ToLower(query)
	headerSkipped := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header
		if !headerSkipped {
			headerSkipped = true
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		version := fields[1]

		if !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		packages = append(packages, manager.Package{
			Name:      name,
			Version:   version,
			Source:    "snap",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// parseSearchOutput parses snap find output.
func (s *Snap) parseSearchOutput(output string, limit int) []manager.Package {
	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	headerSkipped := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header
		if !headerSkipped {
			headerSkipped = true
			continue
		}

		// Format: Name  Version  Publisher  Notes  Summary
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		version := fields[1]
		description := ""

		// Get the summary (last part after Notes column)
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}

		packages = append(packages, manager.Package{
			Name:        name,
			Version:     version,
			Description: description,
			Source:      "snap",
		})

		if limit > 0 && len(packages) >= limit {
			break
		}
	}

	return packages
}

// Info returns detailed information about a Snap package.
func (s *Snap) Info(ctx context.Context, pkg string) (*manager.PackageInfo, error) {
	output, err := s.exec.Output(ctx, s.binary, "info", pkg)
	if err != nil {
		return nil, fmt.Errorf("package '%s' not found", pkg)
	}

	return s.parsePackageInfo(output), nil
}

// parsePackageInfo parses snap info output.
func (s *Snap) parsePackageInfo(output string) *manager.PackageInfo {
	info := &manager.PackageInfo{
		Package: manager.Package{
			Source: "snap",
		},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var inDescription bool
	var descLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Multi-line description handling
		if inDescription {
			if strings.HasPrefix(line, "  ") || line == "" {
				descLines = append(descLines, strings.TrimSpace(line))
				continue
			}
			inDescription = false
			info.Description = strings.Join(descLines, " ")
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			info.Name = value
		case "version":
			info.Version = value
		case "summary":
			info.Description = value
		case "description":
			inDescription = true
			descLines = []string{}
		case "license":
			info.License = value
		case "publisher":
			info.Maintainer = value
		case "snap-id":
			// Could store this if needed
		case "installed":
			info.Size = value
		case "contact":
			info.URL = value
		}
	}

	return info
}

// ListInstalled returns all installed Snap packages.
func (s *Snap) ListInstalled(ctx context.Context, opts manager.ListOpts) ([]manager.Package, error) {
	output, err := s.exec.Output(ctx, s.binary, "list")
	if err != nil {
		return nil, err
	}

	var packages []manager.Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	patternLower := strings.ToLower(opts.Pattern)
	headerSkipped := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header
		if !headerSkipped {
			headerSkipped = true
			continue
		}

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
			Source:    "snap",
			Installed: true,
		})

		if opts.Limit > 0 && len(packages) >= opts.Limit {
			break
		}
	}

	return packages, nil
}

// IsInstalled checks if a Snap package is installed.
func (s *Snap) IsInstalled(ctx context.Context, pkg string) (bool, error) {
	output, err := s.exec.Output(ctx, s.binary, "list")
	if err != nil {
		return false, nil
	}

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == pkg {
			return true, nil
		}
	}

	return false, nil
}

// Clean is a no-op for Snap (automatic cleanup).
func (s *Snap) Clean(ctx context.Context, opts manager.CleanOpts) error {
	// Snap manages its own cleanup
	return nil
}

// Autoremove is a no-op for Snap.
func (s *Snap) Autoremove(ctx context.Context) error {
	// Snap doesn't have orphan package concept
	return nil
}
