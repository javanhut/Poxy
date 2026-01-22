//go:build !windows

package executor

import (
	"os"
	"os/exec"
)

// isRoot returns true if the current process is running as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// hasSudo returns true if sudo is available on the system.
func hasSudo() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}
