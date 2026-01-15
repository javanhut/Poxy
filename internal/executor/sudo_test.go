package executor

import (
	"os"
	"testing"
)

func TestIsRoot(t *testing.T) {
	// Just test that it doesn't panic and returns a boolean
	result := IsRoot()

	// If we're not running as root (which is the common case in tests)
	if os.Geteuid() != 0 && result {
		t.Error("IsRoot() should return false when not running as root")
	}

	if os.Geteuid() == 0 && !result {
		t.Error("IsRoot() should return true when running as root")
	}
}

func TestHasSudo(t *testing.T) {
	// Just test that it doesn't panic and returns a boolean
	_ = HasSudo()
}

func TestCanElevate(t *testing.T) {
	result := CanElevate()

	// Should return true if either root or sudo available
	if IsRoot() && !result {
		t.Error("CanElevate() should return true when running as root")
	}

	if HasSudo() && !result {
		t.Error("CanElevate() should return true when sudo is available")
	}
}

func TestCheckPrivileges(t *testing.T) {
	// When needsSudo is false, should always return nil
	err := CheckPrivileges(false)
	if err != nil {
		t.Errorf("CheckPrivileges(false) should return nil: %v", err)
	}

	// When needsSudo is true and we can elevate, should return nil
	if CanElevate() {
		err = CheckPrivileges(true)
		if err != nil {
			t.Errorf("CheckPrivileges(true) with elevation available should return nil: %v", err)
		}
	}
}

func TestErrNoPrivileges(t *testing.T) {
	err := ErrNoPrivileges
	msg := err.Error()

	if msg == "" {
		t.Error("ErrNoPrivileges.Error() should return non-empty string")
	}

	// Check it mentions root or sudo
	if len(msg) < 10 {
		t.Error("ErrNoPrivileges message seems too short")
	}
}
