package ui

import "regexp"

// PackageSummary holds metadata parsed from a package manager's operation
// output (install, remove, upgrade). All fields are best-effort; callers
// must tolerate empty values.
type PackageSummary struct {
	Size       string // e.g. "0.12 MiB" — size of the change (installed or removed)
	VersionTag string // e.g. "tldr-3.4.4-1" (pacman-style versioned package name)
}

var (
	pacmanInstalledRE = regexp.MustCompile(`(?m)Total Installed Size:\s+([0-9.]+\s+[KMG]i?B)`)
	pacmanRemovedRE   = regexp.MustCompile(`(?m)Total Removed Size:\s+([0-9.]+\s+[KMG]i?B)`)
	pacmanDownloadRE  = regexp.MustCompile(`(?m)Total Download Size:\s+([0-9.]+\s+[KMG]i?B)`)
	aptUsedRE         = regexp.MustCompile(`(?m)After this operation,\s+([0-9.]+\s+[kMG]?B)\s+of additional disk space will be used`)
	aptFreedRE        = regexp.MustCompile(`(?m)After this operation,\s+([0-9.]+\s+[kMG]?B)\s+disk space will be freed`)
	dnfInstalledRE    = regexp.MustCompile(`(?m)Installed size:\s+([0-9.]+\s+[kMG])`)
	dnfDownloadRE     = regexp.MustCompile(`(?m)Total download size:\s+([0-9.]+\s+[kMG])`)
	dnfFreedRE        = regexp.MustCompile(`(?m)Freed space:\s+([0-9.]+\s+[kMG])`)
	zypperUsedRE      = regexp.MustCompile(`(?m)After the operation, additional\s+([0-9.]+\s+[KMG]i?B)`)
	zypperFreedRE     = regexp.MustCompile(`(?m)After the operation,\s+([0-9.]+\s+[KMG]i?B)\s+will be freed`)
	pacmanPkgsRE      = regexp.MustCompile(`(?m)^Packages \(\d+\)\s+(\S+)`)
)

// ParsePackageSummary best-effort extracts size/version info from a package
// manager's install, remove, or upgrade output. Returns a zero-valued
// summary if nothing matches.
func ParsePackageSummary(output string) PackageSummary {
	var s PackageSummary
	for _, re := range []*regexp.Regexp{
		pacmanInstalledRE, pacmanRemovedRE,
		aptUsedRE, aptFreedRE,
		dnfInstalledRE, dnfFreedRE, dnfDownloadRE,
		zypperUsedRE, zypperFreedRE,
		pacmanDownloadRE,
	} {
		if m := re.FindStringSubmatch(output); len(m) == 2 {
			s.Size = m[1]
			break
		}
	}
	if m := pacmanPkgsRE.FindStringSubmatch(output); len(m) == 2 {
		s.VersionTag = m[1]
	}
	return s
}
