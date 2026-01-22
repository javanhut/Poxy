//go:build !windows

package executor

import (
	"os"
	"testing"
)

func TestIsRoot(t *testing.T) {
	// Test that IsRoot returns the correct value based on os.Geteuid()
	result := IsRoot()

	// If we're not running as root (which is the common case in tests)
	if os.Geteuid() != 0 && result {
		t.Error("IsRoot() should return false when not running as root")
	}

	if os.Geteuid() == 0 && !result {
		t.Error("IsRoot() should return true when running as root")
	}
}
