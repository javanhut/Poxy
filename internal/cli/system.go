package cli

import (
	"github.com/spf13/cobra"
	"poxy/internal/ui"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Show system information",
	Long: `Display information about the detected system and available
package managers.

Examples:
  poxy system               # Show system info`,
	RunE: runSystem,
}

func runSystem(cmd *cobra.Command, args []string) error {
	sysInfo := registry.SystemInfo()
	if sysInfo == nil {
		ui.WarningMsg("System information not available")
		return nil
	}

	// Get native manager name
	nativeManager := ""
	if native := registry.Native(); native != nil {
		nativeManager = native.DisplayName()
	}

	// Get all available managers
	available := registry.Available()
	managerNames := make([]string, len(available))
	for i, mgr := range available {
		managerNames[i] = mgr.Name()
	}

	ui.PrintSystemInfo(
		string(sysInfo.OS),
		sysInfo.Arch,
		sysInfo.Distribution,
		sysInfo.PrettyName,
		nativeManager,
		managerNames,
	)

	return nil
}
