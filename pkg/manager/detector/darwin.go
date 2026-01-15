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

// GetDarwinManager returns the recommended package manager for macOS.
// Currently only Homebrew is supported.
func GetDarwinManager() string {
	return "brew"
}
