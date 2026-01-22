package cli

import (
	"context"

	"poxy/internal/ui"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [package]",
	Short: "Show package information",
	Long: `Display detailed information about a specific package.

Examples:
  poxy info vim               # Show info from native manager
  poxy info firefox -s flatpak # Show Flatpak info`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	pkg := resolvePackages(args)[0]

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// Get package info
	info, err := mgr.Info(ctx, pkg)
	if err != nil {
		return err
	}

	// Display info
	ui.PrintPackageInfo(info)

	// Check if installed
	installed, _ := mgr.IsInstalled(ctx, pkg) //nolint:errcheck
	if installed {
		ui.SuccessMsg("Package is installed")
	} else {
		ui.MutedMsg("Package is not installed")
	}

	return nil
}
