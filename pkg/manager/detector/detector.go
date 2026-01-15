// Package detector handles OS and distribution detection.
package detector

import (
	"runtime"
)

// OSType represents the detected operating system type.
type OSType string

const (
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "darwin"
	OSWindows OSType = "windows"
	OSUnknown OSType = "unknown"
)

// SystemInfo contains information about the detected system.
type SystemInfo struct {
	OS           OSType
	Arch         string
	Distribution string   // Linux distribution ID (e.g., "ubuntu", "arch")
	DistroFamily []string // Related distributions (from ID_LIKE)
	PrettyName   string   // Human-readable name
	VersionID    string   // Distribution version
}

// Detect detects the current system's OS and distribution.
func Detect() (*SystemInfo, error) {
	info := &SystemInfo{
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		info.OS = OSLinux
		linuxInfo, err := DetectLinux()
		if err != nil {
			return info, err
		}
		info.Distribution = linuxInfo.ID
		info.DistroFamily = linuxInfo.IDLike
		info.PrettyName = linuxInfo.PrettyName
		info.VersionID = linuxInfo.VersionID
	case "darwin":
		info.OS = OSDarwin
		info.Distribution = "macos"
		info.PrettyName = "macOS"
	case "windows":
		info.OS = OSWindows
		info.Distribution = "windows"
		info.PrettyName = "Windows"
	default:
		info.OS = OSUnknown
	}

	return info, nil
}

// MatchesDistro checks if the system matches any of the given distribution identifiers.
// It checks both the direct distribution ID and the ID_LIKE family.
func (s *SystemInfo) MatchesDistro(distros ...string) bool {
	for _, d := range distros {
		// Direct match
		if s.Distribution == d {
			return true
		}
		// Family match
		for _, family := range s.DistroFamily {
			if family == d {
				return true
			}
		}
	}
	return false
}

// IsLinux returns true if the system is running Linux.
func (s *SystemInfo) IsLinux() bool {
	return s.OS == OSLinux
}

// IsDarwin returns true if the system is running macOS.
func (s *SystemInfo) IsDarwin() bool {
	return s.OS == OSDarwin
}

// IsWindows returns true if the system is running Windows.
func (s *SystemInfo) IsWindows() bool {
	return s.OS == OSWindows
}
