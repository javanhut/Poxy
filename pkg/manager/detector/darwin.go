package detector

import (
	"os/exec"
	"strings"
)

// DarwinInfo contains information about a macOS system.
type DarwinInfo struct {
	ProductName    string // e.g., "macOS"
	ProductVersion string // e.g., "14.0"
	BuildVersion   string // e.g., "23A344"
}

// DetectDarwin detects macOS version information.
func DetectDarwin() (*DarwinInfo, error) {
	info := &DarwinInfo{
		ProductName: "macOS",
	}

	// Get macOS version using sw_vers
	if version, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		info.ProductVersion = strings.TrimSpace(string(version))
	}

	if build, err := exec.Command("sw_vers", "-buildVersion").Output(); err == nil {
		info.BuildVersion = strings.TrimSpace(string(build))
	}

	return info, nil
}

// GetDarwinManager returns the recommended package manager for macOS,
// preferring whichever is actually installed. Homebrew takes precedence
// over MacPorts when both are available.
func GetDarwinManager() string {
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew"
	}
	if _, err := exec.LookPath("port"); err == nil {
		return "macports"
	}
	return "brew"
}
