package cli

import (
	"github.com/spf13/cobra"
	"poxy/internal/history"
	"poxy/internal/tui"
	"poxy/internal/ui"
	"poxy/pkg/database"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal user interface",
	Long: `Launch the interactive terminal user interface (TUI) for poxy.

The TUI provides a visual way to:
  - Browse installed packages
  - Search for new packages
  - View package details
  - Install and remove packages
  - View operation history
  - Check system information

Navigation:
  - Use arrow keys or j/k to navigate
  - Press 1-5 to switch tabs
  - Press / to search
  - Press i to install, r to remove
  - Press ? for help
  - Press q to quit`,
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Open history store
	historyStore, err := history.Open()
	if err != nil {
		ui.WarningMsg("Could not open history: %v", err)
		// Continue without history
	}
	defer func() {
		if historyStore != nil {
			historyStore.Close()
		}
	}()

	// Get search index if available
	var searchIndex *database.Index
	if searchEngine != nil {
		searchIndex = searchEngine.GetIndex()
	}

	// Launch TUI
	return tui.Run(registry, cfg, historyStore, searchIndex)
}
