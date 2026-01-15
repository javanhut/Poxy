package cli

import (
	"context"

	"github.com/spf13/cobra"
	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
)

var cleanAll bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean package cache",
	Long: `Remove cached package files to free up disk space.

Examples:
  poxy clean                # Clean outdated cache
  poxy clean --all          # Clean all cached data`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "clean all cached data")
}

func runClean(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	ui.InfoMsg("Cleaning package cache using %s", mgr.DisplayName())

	// Create history entry
	entry := history.NewEntry(history.OpClean, mgr.Name(), nil)

	opts := manager.CleanOpts{
		DryRun: cfg.General.DryRun,
		All:    cleanAll,
	}

	// Execute clean
	err = mgr.Clean(ctx, opts)

	// Update history
	if err != nil {
		entry.MarkFailed(err)
		ui.ErrorMsg("Clean failed: %v", err)
	} else {
		entry.MarkSuccess()
		ui.SuccessMsg("Cache cleaned successfully")
	}

	// Record in history (ignore errors)
	if store, storeErr := history.Open(); storeErr == nil {
		store.Record(entry)
		store.Close()
	}

	return err
}
