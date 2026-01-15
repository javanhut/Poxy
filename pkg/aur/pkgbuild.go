package aur

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// PKGBUILD represents a parsed PKGBUILD file.
type PKGBUILD struct {
	// Source file path
	Path string

	// Basic package info
	PkgName []string // Can be multiple for split packages
	PkgBase string
	PkgVer  string
	PkgRel  string
	Epoch   string
	PkgDesc string
	URL     string
	License []string
	Arch    []string
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

	// Checksums
	MD5Sums    []string
	SHA256Sums []string
	SHA512Sums []string
	B2Sums     []string

	// Options
	Options []string
	Backup  []string
	Install string

	// Build functions present
	HasPrepare bool
	HasBuild   bool
	HasCheck   bool
	HasPackage bool

	// Raw content for security review
	RawContent string

	// Potentially dangerous commands found
	DangerousCommands []DangerousCommand
}

// DangerousCommand represents a potentially dangerous command in PKGBUILD.
type DangerousCommand struct {
	Line    int
	Command string
	Reason  string
}

// ParsePKGBUILD parses a PKGBUILD file.
// It uses bash to source the PKGBUILD and extract variables for accuracy.
func ParsePKGBUILD(path string) (*PKGBUILD, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PKGBUILD: %w", err)
	}

	pkg := &PKGBUILD{
		Path:       path,
		RawContent: string(content),
	}

	// Try parsing with bash first (most accurate)
	if err := pkg.parseWithBash(path); err != nil {
		// Fall back to regex parsing
		pkg.parseWithRegex(string(content))
	}

	// Detect build functions
	pkg.detectFunctions(string(content))

	// Scan for dangerous commands
	pkg.scanForDangerousCommands(string(content))

	return pkg, nil
}

// parseWithBash uses bash to source the PKGBUILD and extract variables.
func (p *PKGBUILD) parseWithBash(path string) error {
	// Script to extract all relevant variables
	script := `
source "$1" 2>/dev/null
echo "pkgname=${pkgname[@]}"
echo "pkgbase=$pkgbase"
echo "pkgver=$pkgver"
echo "pkgrel=$pkgrel"
echo "epoch=$epoch"
echo "pkgdesc=$pkgdesc"
echo "url=$url"
echo "license=${license[@]}"
echo "arch=${arch[@]}"
echo "groups=${groups[@]}"
echo "depends=${depends[@]}"
echo "makedepends=${makedepends[@]}"
echo "checkdepends=${checkdepends[@]}"
echo "optdepends=${optdepends[@]}"
echo "conflicts=${conflicts[@]}"
echo "provides=${provides[@]}"
echo "replaces=${replaces[@]}"
echo "source=${source[@]}"
echo "noextract=${noextract[@]}"
echo "options=${options[@]}"
echo "backup=${backup[@]}"
echo "install=$install"
echo "md5sums=${md5sums[@]}"
echo "sha256sums=${sha256sums[@]}"
echo "sha512sums=${sha512sums[@]}"
echo "b2sums=${b2sums[@]}"
`

	ctx, cancel := context.WithTimeout(context.Background(), 5*60)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", script, "--", path)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("bash parsing failed: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "pkgname":
			p.PkgName = splitBashArray(value)
		case "pkgbase":
			p.PkgBase = value
		case "pkgver":
			p.PkgVer = value
		case "pkgrel":
			p.PkgRel = value
		case "epoch":
			p.Epoch = value
		case "pkgdesc":
			p.PkgDesc = value
		case "url":
			p.URL = value
		case "license":
			p.License = splitBashArray(value)
		case "arch":
			p.Arch = splitBashArray(value)
		case "groups":
			p.Groups = splitBashArray(value)
		case "depends":
			p.Depends = splitBashArray(value)
		case "makedepends":
			p.MakeDepends = splitBashArray(value)
		case "checkdepends":
			p.CheckDepends = splitBashArray(value)
		case "optdepends":
			p.OptDepends = splitBashArray(value)
		case "conflicts":
			p.Conflicts = splitBashArray(value)
		case "provides":
			p.Provides = splitBashArray(value)
		case "replaces":
			p.Replaces = splitBashArray(value)
		case "source":
			p.Source = splitBashArray(value)
		case "noextract":
			p.NoExtract = splitBashArray(value)
		case "options":
			p.Options = splitBashArray(value)
		case "backup":
			p.Backup = splitBashArray(value)
		case "install":
			p.Install = value
		case "md5sums":
			p.MD5Sums = splitBashArray(value)
		case "sha256sums":
			p.SHA256Sums = splitBashArray(value)
		case "sha512sums":
			p.SHA512Sums = splitBashArray(value)
		case "b2sums":
			p.B2Sums = splitBashArray(value)
		}
	}

	return nil
}

// parseWithRegex uses regex to parse the PKGBUILD (fallback).
func (p *PKGBUILD) parseWithRegex(content string) {
	// Simple patterns for basic variables
	patterns := map[string]*regexp.Regexp{
		"pkgname": regexp.MustCompile(`(?m)^pkgname=["']?([^"'\n]+)["']?`),
		"pkgbase": regexp.MustCompile(`(?m)^pkgbase=["']?([^"'\n]+)["']?`),
		"pkgver":  regexp.MustCompile(`(?m)^pkgver=["']?([^"'\n]+)["']?`),
		"pkgrel":  regexp.MustCompile(`(?m)^pkgrel=["']?([^"'\n]+)["']?`),
		"epoch":   regexp.MustCompile(`(?m)^epoch=["']?([^"'\n]+)["']?`),
		"pkgdesc": regexp.MustCompile(`(?m)^pkgdesc=["']([^"']+)["']`),
		"url":     regexp.MustCompile(`(?m)^url=["']?([^"'\n]+)["']?`),
		"install": regexp.MustCompile(`(?m)^install=["']?([^"'\n]+)["']?`),
	}

	for key, pattern := range patterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 1 {
			switch key {
			case "pkgname":
				p.PkgName = []string{match[1]}
			case "pkgbase":
				p.PkgBase = match[1]
			case "pkgver":
				p.PkgVer = match[1]
			case "pkgrel":
				p.PkgRel = match[1]
			case "epoch":
				p.Epoch = match[1]
			case "pkgdesc":
				p.PkgDesc = match[1]
			case "url":
				p.URL = match[1]
			case "install":
				p.Install = match[1]
			}
		}
	}

	// Array patterns
	arrayPatterns := map[string]*regexp.Regexp{
		"arch":         regexp.MustCompile(`(?m)^arch=\(([^)]+)\)`),
		"license":      regexp.MustCompile(`(?m)^license=\(([^)]+)\)`),
		"depends":      regexp.MustCompile(`(?m)^depends=\(([^)]+)\)`),
		"makedepends":  regexp.MustCompile(`(?m)^makedepends=\(([^)]+)\)`),
		"checkdepends": regexp.MustCompile(`(?m)^checkdepends=\(([^)]+)\)`),
		"source":       regexp.MustCompile(`(?m)^source=\(([^)]+)\)`),
	}

	for key, pattern := range arrayPatterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 1 {
			values := parseArrayContent(match[1])
			switch key {
			case "arch":
				p.Arch = values
			case "license":
				p.License = values
			case "depends":
				p.Depends = values
			case "makedepends":
				p.MakeDepends = values
			case "checkdepends":
				p.CheckDepends = values
			case "source":
				p.Source = values
			}
		}
	}
}

// detectFunctions checks which build functions are defined.
func (p *PKGBUILD) detectFunctions(content string) {
	p.HasPrepare = regexp.MustCompile(`(?m)^prepare\s*\(\)`).MatchString(content)
	p.HasBuild = regexp.MustCompile(`(?m)^build\s*\(\)`).MatchString(content)
	p.HasCheck = regexp.MustCompile(`(?m)^check\s*\(\)`).MatchString(content)
	p.HasPackage = regexp.MustCompile(`(?m)^package\s*\(\)`).MatchString(content) ||
		regexp.MustCompile(`(?m)^package_\w+\s*\(\)`).MatchString(content)
}

// scanForDangerousCommands looks for potentially dangerous commands.
func (p *PKGBUILD) scanForDangerousCommands(content string) {
	dangerousPatterns := []struct {
		pattern *regexp.Regexp
		reason  string
	}{
		{regexp.MustCompile(`curl\s+[^|]*\|\s*(ba)?sh`), "Downloads and executes script"},
		{regexp.MustCompile(`wget\s+[^|]*\|\s*(ba)?sh`), "Downloads and executes script"},
		{regexp.MustCompile(`rm\s+-rf\s+/[^$]`), "Recursive deletion from root"},
		{regexp.MustCompile(`chmod\s+777`), "World-writable permissions"},
		{regexp.MustCompile(`eval\s+`), "Dynamic code execution"},
		{regexp.MustCompile(`\$\([^)]*curl[^)]*\)`), "Command substitution with curl"},
		{regexp.MustCompile(`\$\([^)]*wget[^)]*\)`), "Command substitution with wget"},
		{regexp.MustCompile(`sudo\s+`), "Explicit sudo usage"},
		{regexp.MustCompile(`/etc/passwd`), "Accesses passwd file"},
		{regexp.MustCompile(`/etc/shadow`), "Accesses shadow file"},
		{regexp.MustCompile(`\.ssh/`), "Accesses SSH directory"},
		{regexp.MustCompile(`nc\s+-[el]`), "Netcat listener"},
		{regexp.MustCompile(`ncat\s+-[el]`), "Ncat listener"},
		{regexp.MustCompile(`python.*-c.*socket`), "Python socket code"},
		{regexp.MustCompile(`base64\s+-d`), "Base64 decoding (obfuscation)"},
	}

	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		for _, dp := range dangerousPatterns {
			if dp.pattern.MatchString(line) {
				p.DangerousCommands = append(p.DangerousCommands, DangerousCommand{
					Line:    lineNum + 1,
					Command: strings.TrimSpace(line),
					Reason:  dp.reason,
				})
			}
		}
	}
}

// splitBashArray splits a bash array output into individual elements.
func splitBashArray(value string) []string {
	if value == "" {
		return nil
	}
	fields := strings.Fields(value)
	return fields
}

// parseArrayContent parses the content inside a bash array ().
func parseArrayContent(content string) []string {
	// Remove quotes and split
	content = strings.ReplaceAll(content, "'", "")
	content = strings.ReplaceAll(content, "\"", "")
	fields := strings.Fields(content)
	return fields
}

// FullVersion returns the full version string.
func (p *PKGBUILD) FullVersion() string {
	ver := p.PkgVer
	if p.PkgRel != "" {
		ver += "-" + p.PkgRel
	}
	if p.Epoch != "" {
		ver = p.Epoch + ":" + ver
	}
	return ver
}

// Name returns the primary package name.
func (p *PKGBUILD) Name() string {
	if len(p.PkgName) > 0 {
		return p.PkgName[0]
	}
	return p.PkgBase
}

// IsSplitPackage returns true if this PKGBUILD defines multiple packages.
func (p *PKGBUILD) IsSplitPackage() bool {
	return len(p.PkgName) > 1
}

// AllDependencies returns all runtime dependencies.
func (p *PKGBUILD) AllDependencies() []string {
	return p.Depends
}

// AllBuildDependencies returns all build-time dependencies.
func (p *PKGBUILD) AllBuildDependencies() []string {
	deps := make([]string, 0, len(p.MakeDepends)+len(p.CheckDepends))
	deps = append(deps, p.MakeDepends...)
	deps = append(deps, p.CheckDepends...)
	return deps
}

// HasDangerousCommands returns true if dangerous commands were detected.
func (p *PKGBUILD) HasDangerousCommands() bool {
	return len(p.DangerousCommands) > 0
}

// SourceURLs returns the URLs from the source array.
func (p *PKGBUILD) SourceURLs() []string {
	var urls []string
	for _, src := range p.Source {
		// Extract URL from "filename::url" format
		if idx := strings.Index(src, "::"); idx != -1 {
			urls = append(urls, src[idx+2:])
		} else if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") ||
			strings.HasPrefix(src, "ftp://") || strings.HasPrefix(src, "git+") {
			urls = append(urls, src)
		}
	}
	return urls
}

// GenerateSRCINFO generates .SRCINFO content from the PKGBUILD.
func (p *PKGBUILD) GenerateSRCINFO(ctx context.Context) (string, error) {
	dir := filepath.Dir(p.Path)
	cmd := exec.CommandContext(ctx, "makepkg", "--printsrcinfo")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate .SRCINFO: %w", err)
	}

	return string(output), nil
}
