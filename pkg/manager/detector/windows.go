package detector

import (
	"os/exec"
	"strings"
)

// WindowsInfo contains information about a Windows system.
type WindowsInfo struct {
	ProductName string
	Version     string
	Build       string
}

// DetectWindows detects Windows version information.
func DetectWindows() (*WindowsInfo, error) {
	info := &WindowsInfo{
		ProductName: "Windows",
	}

	// Try to get version info using PowerShell
	cmd := exec.Command("powershell", "-Command", "(Get-WmiObject -Class Win32_OperatingSystem).Caption")
	if output, err := cmd.Output(); err == nil {
		info.ProductName = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("powershell", "-Command", "(Get-WmiObject -Class Win32_OperatingSystem).Version")
	if output, err := cmd.Output(); err == nil {
		info.Version = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("powershell", "-Command", "(Get-WmiObject -Class Win32_OperatingSystem).BuildNumber")
	if output, err := cmd.Output(); err == nil {
		info.Build = strings.TrimSpace(string(output))
	}

	return info, nil
}

// GetWindowsManagers returns available package managers on Windows in priority order.
func GetWindowsManagers() []string {
	managers := []string{}

	// Check for winget (Windows Package Manager) - preferred
	if _, err := exec.LookPath("winget"); err == nil {
		managers = append(managers, "winget")
	}

	// Check for chocolatey
	if _, err := exec.LookPath("choco"); err == nil {
		managers = append(managers, "chocolatey")
	}

	// Check for scoop
	if _, err := exec.LookPath("scoop"); err == nil {
		managers = append(managers, "scoop")
	}

	return managers
}

// GetWindowsManager returns the preferred available package manager on Windows.
func GetWindowsManager() string {
	managers := GetWindowsManagers()
	if len(managers) > 0 {
		return managers[0]
	}
	return ""
}
