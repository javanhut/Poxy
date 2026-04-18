package ui

import "testing"

func TestParsePackageSummaryPacmanInstall(t *testing.T) {
	output := `resolving dependencies...
looking for conflicting packages...

Packages (1) tldr-3.4.4-1

Total Installed Size:  0.12 MiB

:: Proceed with installation? [Y/n]
(1/1) checking keys in keyring
(1/1) installing tldr
`
	s := ParsePackageSummary(output)
	if s.Size != "0.12 MiB" {
		t.Errorf("Size = %q, want %q", s.Size, "0.12 MiB")
	}
	if s.VersionTag != "tldr-3.4.4-1" {
		t.Errorf("VersionTag = %q, want %q", s.VersionTag, "tldr-3.4.4-1")
	}
}

func TestParsePackageSummaryPacmanRemove(t *testing.T) {
	output := `checking dependencies...

Packages (1) tldr-3.4.4-1

Total Removed Size:  0.12 MiB

:: Do you want to remove these packages? [Y/n]
`
	s := ParsePackageSummary(output)
	if s.Size != "0.12 MiB" {
		t.Errorf("Size = %q, want %q", s.Size, "0.12 MiB")
	}
	if s.VersionTag != "tldr-3.4.4-1" {
		t.Errorf("VersionTag = %q, want %q", s.VersionTag, "tldr-3.4.4-1")
	}
}

func TestParsePackageSummaryAptInstall(t *testing.T) {
	output := `Reading package lists... Done
After this operation, 1234 kB of additional disk space will be used.
`
	s := ParsePackageSummary(output)
	if s.Size != "1234 kB" {
		t.Errorf("Size = %q, want %q", s.Size, "1234 kB")
	}
}

func TestParsePackageSummaryAptRemove(t *testing.T) {
	output := `Reading package lists... Done
After this operation, 512 kB disk space will be freed.
`
	s := ParsePackageSummary(output)
	if s.Size != "512 kB" {
		t.Errorf("Size = %q, want %q", s.Size, "512 kB")
	}
}

func TestParsePackageSummaryEmpty(t *testing.T) {
	s := ParsePackageSummary("nothing matches here")
	if s.Size != "" || s.VersionTag != "" {
		t.Errorf("expected zero-value summary, got %+v", s)
	}
}
