package universal

import (
	"testing"

	"poxy/pkg/manager"
)

func TestFlatpakManager(t *testing.T) {
	flatpak := NewFlatpak("")

	if flatpak.Name() != "flatpak" {
		t.Errorf("expected name 'flatpak', got '%s'", flatpak.Name())
	}

	if flatpak.DisplayName() != "Flatpak" {
		t.Errorf("expected display name 'Flatpak', got '%s'", flatpak.DisplayName())
	}

	if flatpak.Type() != manager.TypeUniversal {
		t.Errorf("expected Type Universal, got %s", flatpak.Type())
	}

	// Flatpak doesn't need sudo for user installations
	if flatpak.NeedsSudo() {
		t.Error("Flatpak should not need sudo")
	}

	// IsAvailable just checks for binary
	_ = flatpak.IsAvailable()
}

func TestFlatpakWithRemote(t *testing.T) {
	flatpak := NewFlatpak("flathub")

	if flatpak.defaultRemote != "flathub" {
		t.Errorf("expected default remote 'flathub', got '%s'", flatpak.defaultRemote)
	}
}

func TestSnapManager(t *testing.T) {
	snap := NewSnap(false)

	if snap.Name() != "snap" {
		t.Errorf("expected name 'snap', got '%s'", snap.Name())
	}

	if snap.DisplayName() != "Snap" {
		t.Errorf("expected display name 'Snap', got '%s'", snap.DisplayName())
	}

	if snap.Type() != manager.TypeUniversal {
		t.Errorf("expected Type Universal, got %s", snap.Type())
	}

	// Snap needs sudo
	if !snap.NeedsSudo() {
		t.Error("Snap should need sudo")
	}
}

func TestSnapWithClassic(t *testing.T) {
	snap := NewSnap(true)

	if !snap.allowClassic {
		t.Error("expected allowClassic to be true")
	}
}

func TestAURManager(t *testing.T) {
	// Test with no helper available
	aur := NewAUR("")
	// May be nil if no helper is installed
	if aur != nil {
		if aur.Name() != "aur" {
			t.Errorf("expected name 'aur', got '%s'", aur.Name())
		}

		if aur.Type() != manager.TypeAUR {
			t.Errorf("expected Type AUR, got %s", aur.Type())
		}

		// AUR helpers manage sudo themselves
		if aur.NeedsSudo() {
			t.Error("AUR helper should not need sudo (manages it internally)")
		}
	}
}

func TestAURWithPreferredHelper(t *testing.T) {
	// Test with preferred helper (may not be installed)
	aur := NewAUR("yay")
	if aur != nil {
		if aur.helper != "yay" && aur.helper != "" {
			// It might fall back to another helper if yay isn't installed
			t.Logf("helper is: %s", aur.helper)
		}
	}
}

func TestDetectAURHelper(t *testing.T) {
	// This test just verifies the function doesn't panic
	// The actual result depends on system state
	helper := detectAURHelper("")
	t.Logf("detected AUR helper: %s", helper)

	// Test with specific preference
	helper = detectAURHelper("nonexistent-helper")
	// Should return empty or fall back to an available helper
	t.Logf("detected AUR helper with invalid preference: %s", helper)
}

func TestUniversalManagersInterface(t *testing.T) {
	managers := []manager.Manager{
		NewFlatpak(""),
		NewSnap(false),
	}

	// Only test if AUR is available
	if aur := NewAUR(""); aur != nil {
		managers = append(managers, aur)
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

			// Test Type (should not be TypeNative for universal managers)
			mgrType := mgr.Type()
			if mgrType != manager.TypeUniversal && mgrType != manager.TypeAUR {
				t.Errorf("unexpected type: %s", mgrType)
			}
		})
	}
}
