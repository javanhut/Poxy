package aur

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// SRCINFO represents a parsed .SRCINFO file.
type SRCINFO struct {
	// Package base information
	PkgBase string
	PkgDesc string
	PkgVer  string
	PkgRel  string
	Epoch   string
	URL     string
	Arch    []string
	License []string
	Groups  []string

	// Dependencies
	Depends      []string
	MakeDepends  []string
	CheckDepends []string
	OptDepends   []string
	Conflicts    []string
	Provides     []string
	Replaces     []string

	// Sources
	Source    []string
	NoExtract []string

	// Checksums (parallel arrays with Source)
	MD5Sums    []string
	SHA1Sums   []string
	SHA256Sums []string
	SHA384Sums []string
	SHA512Sums []string
	B2Sums     []string

	// Build options
	Options   []string
	Backup    []string
	Install   string
	Changelog string

	// Packages (for split packages)
	Packages []SRCINFOPackage
}

// SRCINFOPackage represents a package section in .SRCINFO (for split packages).
type SRCINFOPackage struct {
	PkgName    string
	PkgDesc    string
	Arch       []string
	URL        string
	License    []string
	Groups     []string
	Depends    []string
	OptDepends []string
	Provides   []string
	Conflicts  []string
	Replaces   []string
	Backup     []string
	Options    []string
	Install    string
}

// ParseSRCINFO parses a .SRCINFO file.
func ParseSRCINFO(path string) (*SRCINFO, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open .SRCINFO: %w", err)
	}
	defer file.Close()

	return ParseSRCINFOReader(bufio.NewScanner(file))
}

// ParseSRCINFOContent parses .SRCINFO content from a string.
func ParseSRCINFOContent(content string) (*SRCINFO, error) {
	return ParseSRCINFOReader(bufio.NewScanner(strings.NewReader(content)))
}

// ParseSRCINFOReader parses .SRCINFO from a scanner.
func ParseSRCINFOReader(scanner *bufio.Scanner) (*SRCINFO, error) {
	info := &SRCINFO{}
	var currentPkg *SRCINFOPackage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Check for new package section
		if key == "pkgname" {
			pkg := SRCINFOPackage{PkgName: value}
			info.Packages = append(info.Packages, pkg)
			currentPkg = &info.Packages[len(info.Packages)-1]
			continue
		}

		// Parse into base or current package
		if currentPkg != nil {
			parsePackageField(currentPkg, key, value)
		} else {
			parseBaseField(info, key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .SRCINFO: %w", err)
	}

	return info, nil
}

func parseBaseField(info *SRCINFO, key, value string) {
	switch key {
	case "pkgbase":
		info.PkgBase = value
	case "pkgdesc":
		info.PkgDesc = value
	case "pkgver":
		info.PkgVer = value
	case "pkgrel":
		info.PkgRel = value
	case "epoch":
		info.Epoch = value
	case "url":
		info.URL = value
	case "install":
		info.Install = value
	case "changelog":
		info.Changelog = value
	case "arch":
		info.Arch = append(info.Arch, value)
	case "license":
		info.License = append(info.License, value)
	case "groups":
		info.Groups = append(info.Groups, value)
	case "depends":
		info.Depends = append(info.Depends, value)
	case "makedepends":
		info.MakeDepends = append(info.MakeDepends, value)
	case "checkdepends":
		info.CheckDepends = append(info.CheckDepends, value)
	case "optdepends":
		info.OptDepends = append(info.OptDepends, value)
	case "conflicts":
		info.Conflicts = append(info.Conflicts, value)
	case "provides":
		info.Provides = append(info.Provides, value)
	case "replaces":
		info.Replaces = append(info.Replaces, value)
	case "source":
		info.Source = append(info.Source, value)
	case "noextract":
		info.NoExtract = append(info.NoExtract, value)
	case "options":
		info.Options = append(info.Options, value)
	case "backup":
		info.Backup = append(info.Backup, value)
	case "md5sums":
		info.MD5Sums = append(info.MD5Sums, value)
	case "sha1sums":
		info.SHA1Sums = append(info.SHA1Sums, value)
	case "sha256sums":
		info.SHA256Sums = append(info.SHA256Sums, value)
	case "sha384sums":
		info.SHA384Sums = append(info.SHA384Sums, value)
	case "sha512sums":
		info.SHA512Sums = append(info.SHA512Sums, value)
	case "b2sums":
		info.B2Sums = append(info.B2Sums, value)
	}
}

func parsePackageField(pkg *SRCINFOPackage, key, value string) {
	switch key {
	case "pkgdesc":
		pkg.PkgDesc = value
	case "url":
		pkg.URL = value
	case "install":
		pkg.Install = value
	case "arch":
		pkg.Arch = append(pkg.Arch, value)
	case "license":
		pkg.License = append(pkg.License, value)
	case "groups":
		pkg.Groups = append(pkg.Groups, value)
	case "depends":
		pkg.Depends = append(pkg.Depends, value)
	case "optdepends":
		pkg.OptDepends = append(pkg.OptDepends, value)
	case "provides":
		pkg.Provides = append(pkg.Provides, value)
	case "conflicts":
		pkg.Conflicts = append(pkg.Conflicts, value)
	case "replaces":
		pkg.Replaces = append(pkg.Replaces, value)
	case "backup":
		pkg.Backup = append(pkg.Backup, value)
	case "options":
		pkg.Options = append(pkg.Options, value)
	}
}

// FullVersion returns the full version string (epoch:pkgver-pkgrel).
func (s *SRCINFO) FullVersion() string {
	ver := s.PkgVer
	if s.PkgRel != "" {
		ver += "-" + s.PkgRel
	}
	if s.Epoch != "" {
		ver = s.Epoch + ":" + ver
	}
	return ver
}

// AllDepends returns all runtime dependencies.
func (s *SRCINFO) AllDepends() []string {
	return s.Depends
}

// AllBuildDepends returns all build-time dependencies.
func (s *SRCINFO) AllBuildDepends() []string {
	deps := make([]string, 0, len(s.MakeDepends)+len(s.CheckDepends))
	deps = append(deps, s.MakeDepends...)
	deps = append(deps, s.CheckDepends...)
	return deps
}

// PackageNames returns all package names defined in this SRCINFO.
func (s *SRCINFO) PackageNames() []string {
	names := make([]string, len(s.Packages))
	for i, pkg := range s.Packages {
		names[i] = pkg.PkgName
	}
	return names
}

// GetPackage returns a specific package by name.
func (s *SRCINFO) GetPackage(name string) *SRCINFOPackage {
	for i := range s.Packages {
		if s.Packages[i].PkgName == name {
			return &s.Packages[i]
		}
	}
	return nil
}
