//go:build windows

package executor

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// isRoot returns true if the current process is running with administrator privileges on Windows.
func isRoot() bool {
	var sid *windows.SID

	// Get the SID for the Administrators group
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	// Check if the current process token is a member of the Administrators group
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

// hasSudo returns true if sudo-like functionality is available on Windows.
// On Windows, we check for gsudo or sudo (Windows 11+).
func hasSudo() bool {
	// Check for Windows 11+ built-in sudo
	if path := os.Getenv("PATH"); path != "" {
		for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
			// Check for sudo.exe (Windows 11+) or gsudo.exe
			if _, err := os.Stat(dir + "\\sudo.exe"); err == nil {
				return true
			}
			if _, err := os.Stat(dir + "\\gsudo.exe"); err == nil {
				return true
			}
		}
	}
	return false
}
