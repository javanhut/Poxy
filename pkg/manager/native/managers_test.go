package native

import (
	"testing"

	"poxy/pkg/manager"
)

// TestManagerInterface verifies all managers implement the Manager interface
func TestManagerInterface(t *testing.T) {
	managers := []manager.Manager{
		NewAPT(false),
		NewDNF(),
		NewPacman(),
		NewZypper(),
		NewXBPS(),
		NewAPK(),
		NewEmerge(),
		NewEopkg(),
		NewNix(),
		NewSlackpkg(),
		NewSwupd(),
		NewBrew(),
		NewWinget(),
		NewChocolatey(),
		NewScoop(),
	}

	for _, mgr := range managers {
		t.Run(mgr.Name(), func(t *testing.T) {
			// Test Name
			if mgr.Name() == "" {
				t.Error("Name() should not be empty")
			}

			// Test DisplayName
			if mgr.DisplayName() == "" {
				t.Error("DisplayName() should not be empty")
			}

			// Test Type
			if mgr.Type() != manager.TypeNative {
				t.Errorf("Type() should be TypeNative, got %s", mgr.Type())
			}

			// Test NeedsSudo returns a boolean (doesn't panic)
			_ = mgr.NeedsSudo()

			// Test IsAvailable returns a boolean (doesn't panic)
			_ = mgr.IsAvailable()
		})
	}
}

func TestAPTManager(t *testing.T) {
	apt := NewAPT(false)

	if apt.Name() != "apt" {
		t.Errorf("expected name 'apt', got '%s'", apt.Name())
	}

	if !apt.NeedsSudo() {
		t.Error("APT should need sudo")
	}

	// Test with nala preference (binary not found, should fall back to apt)
	aptNala := NewAPT(true)
	if aptNala.Binary() != "apt" && aptNala.Binary() != "nala" {
		t.Errorf("unexpected binary: %s", aptNala.Binary())
	}
}

func TestDNFManager(t *testing.T) {
	dnf := NewDNF()

	if dnf.Name() != "dnf" {
		t.Errorf("expected name 'dnf', got '%s'", dnf.Name())
	}

	if !dnf.NeedsSudo() {
		t.Error("DNF should need sudo")
	}
}

func TestPacmanManager(t *testing.T) {
	pacman := NewPacman()

	if pacman.Name() != "pacman" {
		t.Errorf("expected name 'pacman', got '%s'", pacman.Name())
	}

	if !pacman.NeedsSudo() {
		t.Error("Pacman should need sudo")
	}
}

func TestZypperManager(t *testing.T) {
	zypper := NewZypper()

	if zypper.Name() != "zypper" {
		t.Errorf("expected name 'zypper', got '%s'", zypper.Name())
	}
}

func TestXBPSManager(t *testing.T) {
	xbps := NewXBPS()

	if xbps.Name() != "xbps" {
		t.Errorf("expected name 'xbps', got '%s'", xbps.Name())
	}
}

func TestAPKManager(t *testing.T) {
	apk := NewAPK()

	if apk.Name() != "apk" {
		t.Errorf("expected name 'apk', got '%s'", apk.Name())
	}
}

func TestEmergeManager(t *testing.T) {
	emerge := NewEmerge()

	if emerge.Name() != "emerge" {
		t.Errorf("expected name 'emerge', got '%s'", emerge.Name())
	}
}

func TestEopkgManager(t *testing.T) {
	eopkg := NewEopkg()

	if eopkg.Name() != "eopkg" {
		t.Errorf("expected name 'eopkg', got '%s'", eopkg.Name())
	}
}

func TestNixManager(t *testing.T) {
	nix := NewNix()

	if nix.Name() != "nix" {
		t.Errorf("expected name 'nix', got '%s'", nix.Name())
	}

	// Nix doesn't need sudo for user installations
	if nix.NeedsSudo() {
		t.Error("Nix should not need sudo for user installations")
	}
}

func TestSlackpkgManager(t *testing.T) {
	slackpkg := NewSlackpkg()

	if slackpkg.Name() != "slackpkg" {
		t.Errorf("expected name 'slackpkg', got '%s'", slackpkg.Name())
	}
}

func TestSwupdManager(t *testing.T) {
	swupd := NewSwupd()

	if swupd.Name() != "swupd" {
		t.Errorf("expected name 'swupd', got '%s'", swupd.Name())
	}
}

func TestBrewManager(t *testing.T) {
	brew := NewBrew()

	if brew.Name() != "brew" {
		t.Errorf("expected name 'brew', got '%s'", brew.Name())
	}

	// Homebrew doesn't need sudo
	if brew.NeedsSudo() {
		t.Error("Homebrew should not need sudo")
	}
}

func TestWingetManager(t *testing.T) {
	winget := NewWinget()

	if winget.Name() != "winget" {
		t.Errorf("expected name 'winget', got '%s'", winget.Name())
	}

	// Winget doesn't need sudo (uses UAC)
	if winget.NeedsSudo() {
		t.Error("Winget should not need sudo")
	}
}

func TestChocolateyManager(t *testing.T) {
	choco := NewChocolatey()

	if choco.Name() != "chocolatey" {
		t.Errorf("expected name 'chocolatey', got '%s'", choco.Name())
	}
}

func TestScoopManager(t *testing.T) {
	scoop := NewScoop()

	if scoop.Name() != "scoop" {
		t.Errorf("expected name 'scoop', got '%s'", scoop.Name())
	}

	// Scoop doesn't need admin
	if scoop.NeedsSudo() {
		t.Error("Scoop should not need sudo")
	}
}

func TestBaseManager(t *testing.T) {
	base := NewBaseManager("test", "Test Manager", "test-bin", true)

	if base.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", base.Name())
	}

	if base.DisplayName() != "Test Manager" {
		t.Errorf("expected display name 'Test Manager', got '%s'", base.DisplayName())
	}

	if base.Binary() != "test-bin" {
		t.Errorf("expected binary 'test-bin', got '%s'", base.Binary())
	}

	if !base.NeedsSudo() {
		t.Error("expected NeedsSudo to be true")
	}

	// Test SetBinary
	base.SetBinary("new-bin")
	if base.Binary() != "new-bin" {
		t.Errorf("SetBinary failed, got '%s'", base.Binary())
	}

	// Test Type
	if base.Type() != manager.TypeNative {
		t.Errorf("expected Type Native, got %s", base.Type())
	}

	// Test Executor
	exec := base.Executor()
	if exec == nil {
		t.Error("Executor() should not return nil")
	}
}
