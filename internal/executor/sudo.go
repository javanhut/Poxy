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

// IsRoot returns true if the current process is running as root (exported version).
func IsRoot() bool {
	return isRoot()
}

// HasSudo returns true if sudo is available on the system (exported version).
func HasSudo() bool {
	return hasSudo()
}

// CanElevate returns true if the process can elevate privileges.
func CanElevate() bool {
	return isRoot() || hasSudo()
}

// CheckPrivileges returns an error if privileges cannot be elevated when needed.
func CheckPrivileges(needsSudo bool) error {
	if !needsSudo {
		return nil
	}
	if !CanElevate() {
		return ErrNoPrivileges
	}
	return nil
}

// ErrNoPrivileges is returned when an operation requires root but cannot elevate.
type errNoPrivileges struct{}

func (e errNoPrivileges) Error() string {
	return "this operation requires root privileges, but neither running as root nor sudo is available"
}

// ErrNoPrivileges is the error returned when privileges cannot be elevated.
var ErrNoPrivileges = errNoPrivileges{}
