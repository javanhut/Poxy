package cli

import (
	"context"

	"github.com/spf13/cobra"
	"poxy/internal/ui"
	"poxy/pkg/manager"
)

var (
	listLimit   int
	listPattern string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Long: `List all installed packages from the system package manager
or a specific source.

Examples:
  poxy list                     # List all installed packages
  poxy list -s flatpak          # List installed Flatpaks
  poxy list -l 20               # List first 20 packages
  poxy list -p vim              # List packages matching 'vim'`,
	RunE: runList,
}

func init() {
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 0, "limit number of results")
	listCmd.Flags().StringVarP(&listPattern, "pattern", "p", "", "filter by name pattern")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	ui.InfoMsg("Listing installed packages from %s", mgr.DisplayName())

	opts := manager.ListOpts{
		Limit:         listLimit,
		InstalledOnly: true,
		Pattern:       listPattern,
	}

	packages, err := mgr.ListInstalled(ctx, opts)
	if err != nil {
		return err
	}

	ui.PrintPackages(packages)
	ui.MutedMsg("\nTotal: %d packages", len(packages))

	return nil
}
