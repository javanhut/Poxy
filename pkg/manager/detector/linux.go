package detector

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

// LinuxInfo contains information parsed from /etc/os-release.
type LinuxInfo struct {
	ID         string   // Distribution ID (e.g., "ubuntu", "arch", "fedora")
	IDLike     []string // Related distributions
	VersionID  string   // Version number (e.g., "22.04", "39")
	PrettyName string   // Human-readable name
	Name       string   // Distribution name
}

// DetectLinux detects the Linux distribution by reading /etc/os-release.
func DetectLinux() (*LinuxInfo, error) {
	info := &LinuxInfo{}

	// Try /etc/os-release first (most common)
	if err := parseOSRelease(info); err == nil {
		return info, nil
	}

	// Fall back to lsb_release command
	if err := parseLSBRelease(info); err == nil {
		return info, nil
	}

	// Fall back to checking specific release files
	if err := parseReleaseFiles(info); err == nil {
		return info, nil
	}

	// Return unknown if nothing works
	info.ID = "unknown"
	info.PrettyName = "Unknown Linux"
	return info, nil
}

// parseOSRelease parses /etc/os-release file.
func parseOSRelease(info *LinuxInfo) error {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "ID":
			info.ID = value
		case "ID_LIKE":
			info.IDLike = strings.Fields(value)
		case "VERSION_ID":
			info.VersionID = value
		case "PRETTY_NAME":
			info.PrettyName = value
		case "NAME":
			info.Name = value
		}
	}

	return scanner.Err()
}

// parseLSBRelease uses the lsb_release command as a fallback.
func parseLSBRelease(info *LinuxInfo) error {
	cmd := exec.Command("lsb_release", "-a")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Distributor ID":
			info.ID = strings.ToLower(value)
		case "Release":
			info.VersionID = value
		case "Description":
			info.PrettyName = value
		}
	}

	return nil
}

// parseReleaseFiles checks distribution-specific release files.
func parseReleaseFiles(info *LinuxInfo) error {
	releaseFiles := []struct {
		path   string
		distro string
	}{
		{"/etc/arch-release", "arch"},
		{"/etc/debian_version", "debian"},
		{"/etc/fedora-release", "fedora"},
		{"/etc/centos-release", "centos"},
		{"/etc/redhat-release", "rhel"},
		{"/etc/gentoo-release", "gentoo"},
		{"/etc/alpine-release", "alpine"},
		{"/etc/slackware-version", "slackware"},
	}

	for _, rf := range releaseFiles {
		if _, err := os.Stat(rf.path); err == nil {
			info.ID = rf.distro
			info.PrettyName = strings.Title(rf.distro) + " Linux"
			return nil
		}
	}

	return os.ErrNotExist
}

// distroManagerMap maps distribution IDs to their native package managers.
var distroManagerMap = map[string]string{
	// Debian family
	"debian":     "apt",
	"ubuntu":     "apt",
	"linuxmint":  "apt",
	"pop":        "apt",
	"elementary": "apt",
	"zorin":      "apt",
	"kali":       "apt",
	"parrot":     "apt",
	"mx":         "apt",
	"raspbian":   "apt",

	// Red Hat family
	"fedora":    "dnf",
	"rhel":      "dnf",
	"centos":    "dnf",
	"rocky":     "dnf",
	"almalinux": "dnf",
	"nobara":    "dnf",

	// Arch family
	"arch":        "pacman",
	"manjaro":     "pacman",
	"endeavouros": "pacman",
	"garuda":      "pacman",
	"arcolinux":   "pacman",
	"artix":       "pacman",
	"cachyos":     "pacman",

	// SUSE family
	"opensuse":            "zypper",
	"opensuse-leap":       "zypper",
	"opensuse-tumbleweed": "zypper",
	"sles":                "zypper",

	// Others
	"void":           "xbps",
	"alpine":         "apk",
	"gentoo":         "emerge",
	"funtoo":         "emerge",
	"solus":          "eopkg",
	"nixos":          "nix",
	"slackware":      "slackpkg",
	"clear-linux-os": "swupd",
}

// GetNativeManager returns the native package manager for a distribution ID.
func GetNativeManager(distroID string) string {
	if mgr, ok := distroManagerMap[distroID]; ok {
		return mgr
	}
	return ""
}

// GetNativeManagerForFamily checks the distribution ID and its family for a native manager.
func GetNativeManagerForFamily(distroID string, idLike []string) string {
	// Check direct ID first
	if mgr := GetNativeManager(distroID); mgr != "" {
		return mgr
	}

	// Check ID_LIKE family
	for _, family := range idLike {
		if mgr := GetNativeManager(family); mgr != "" {
			return mgr
		}
	}

	return ""
}
