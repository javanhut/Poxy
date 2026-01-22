package cli

import (
	"context"

	"poxy/internal/ui"

	"github.com/spf13/cobra"
)

var autoremoveCmd = &cobra.Command{
	Use:     "autoremove",
	Aliases: []string{"orphans"},
	Short:   "Remove orphaned packages",
	Long: `Remove packages that were installed as dependencies
but are no longer required by any installed package.

Examples:
  poxy autoremove           # Remove orphaned packages
  poxy autoremove -y        # Remove without confirmation`,
	RunE: runAutoremove,
}

func runAutoremove(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	ui.InfoMsg("Removing orphaned packages using %s", mgr.DisplayName())

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm {
		confirmed, err := ui.Confirm("Remove orphaned packages?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Execute autoremove
	err = mgr.Autoremove(ctx)
	if err != nil {
		ui.ErrorMsg("Autoremove failed: %v", err)
		return err
	}

	ui.SuccessMsg("Orphaned packages removed")
	return nil
}
