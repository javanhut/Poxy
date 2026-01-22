package manager

import (
	"testing"
)

func TestManagerType(t *testing.T) {
	tests := []struct {
		name     string
		mtype    ManagerType
		expected string
	}{
		{"Native", TypeNative, "native"},
		{"Universal", TypeUniversal, "universal"},
		{"AUR", TypeAUR, "aur"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mtype) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.mtype)
			}
		})
	}
}

func TestPackage(t *testing.T) {
	pkg := Package{
		Name:        "vim",
		Version:     "9.0",
		Description: "Vi IMproved",
		Source:      "apt",
		Installed:   true,
	}

	if pkg.Name != "vim" {
		t.Errorf("expected Name 'vim', got '%s'", pkg.Name)
	}
	if pkg.Version != "9.0" {
		t.Errorf("expected Version '9.0', got '%s'", pkg.Version)
	}
	if pkg.Description != "Vi IMproved" {
		t.Errorf("expected Description 'Vi IMproved', got '%s'", pkg.Description)
	}
	if pkg.Source != "apt" {
		t.Errorf("expected Source 'apt', got '%s'", pkg.Source)
	}
	if !pkg.Installed {
		t.Error("expected Installed to be true")
	}
}

func TestInstallOpts(t *testing.T) {
	opts := InstallOpts{
		AutoConfirm: true,
		DryRun:      false,
		Reinstall:   true,
	}

	if !opts.AutoConfirm {
		t.Error("expected AutoConfirm to be true")
	}
	if opts.DryRun {
		t.Error("expected DryRun to be false")
	}
	if !opts.Reinstall {
		t.Error("expected Reinstall to be true")
	}
}

func TestUninstallOpts(t *testing.T) {
	opts := UninstallOpts{
		AutoConfirm: true,
		Purge:       true,
		Recursive:   true,
	}

	if !opts.AutoConfirm {
		t.Error("expected AutoConfirm to be true")
	}
	if !opts.Purge {
		t.Error("expected Purge to be true")
	}
	if !opts.Recursive {
		t.Error("expected Recursive to be true")
	}
}

func TestSearchOpts(t *testing.T) {
	opts := SearchOpts{
		Limit:         10,
		InstalledOnly: true,
		SearchInDesc:  true,
	}

	if opts.Limit != 10 {
		t.Errorf("expected Limit 10, got %d", opts.Limit)
	}
	if !opts.InstalledOnly {
		t.Error("expected InstalledOnly to be true")
	}
	if !opts.SearchInDesc {
		t.Error("expected SearchInDesc to be true")
	}
}
