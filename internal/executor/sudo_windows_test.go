//go:build windows

package executor

import (
	"testing"
)

func TestIsRoot(t *testing.T) {
	// On Windows, just test that IsRoot() doesn't panic and returns a boolean
	// We can't easily verify the result without the same Windows API calls
	result := IsRoot()
	t.Logf("IsRoot() returned: %v", result)
}
