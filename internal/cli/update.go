package cli

import (
	"context"

	"github.com/spf13/cobra"
	"poxy/internal/history"
	"poxy/internal/ui"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update package database",
	Long: `Refresh the package database/repository cache.

This downloads the latest package information from the repositories
but does not install or upgrade any packages.

Examples:
  poxy update               # Update native package manager
  poxy update -s flatpak    # Update Flatpak remotes`,
	RunE: runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	ui.InfoMsg("Updating package database using %s", mgr.DisplayName())

	// Create history entry
	entry := history.NewEntry(history.OpUpdate, mgr.Name(), nil)

	// Execute update
	err = mgr.Update(ctx)

	// Update history
	if err != nil {
		entry.MarkFailed(err)
		ui.ErrorMsg("Update failed: %v", err)
	} else {
		entry.MarkSuccess()
		ui.SuccessMsg("Package database updated successfully")
	}

	// Record in history (ignore errors)
	if store, storeErr := history.Open(); storeErr == nil {
		store.Record(entry)
		store.Close()
	}

	return err
}
