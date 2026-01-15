package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"poxy/internal/history"
	"poxy/internal/ui"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show operation history",
	Long: `Display the history of package operations performed by poxy.

Examples:
  poxy history              # Show recent history
  poxy history -l 20        # Show last 20 operations`,
	RunE: runHistory,
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "l", 10, "number of entries to show")
}

func runHistory(cmd *cobra.Command, args []string) error {
	store, err := history.Open()
	if err != nil {
		return fmt.Errorf("failed to open history: %w", err)
	}
	defer store.Close()

	entries, err := store.List(historyLimit)
	if err != nil {
		return fmt.Errorf("failed to read history: %w", err)
	}

	if len(entries) == 0 {
		ui.MutedMsg("No history entries found")
		return nil
	}

	ui.HeaderMsg("Operation History")

	for i, entry := range entries {
		status := ui.Green("success")
		if !entry.Success {
			status = ui.Red("failed")
		}

		reverseIndicator := ""
		if entry.CanRollback() {
			reverseIndicator = " " + ui.Cyan("[reversible]")
		}

		fmt.Printf("%2d. %s %s %s [%s] (%s)%s\n",
			i+1,
			ui.Muted.Sprint(entry.FormatTime()),
			ui.Bold(string(entry.Operation)),
			formatPackages(entry.Packages),
			ui.Cyan(entry.Source),
			status,
			reverseIndicator,
		)

		if entry.Error != "" {
			ui.MutedMsg("    Error: %s", entry.Error)
		}
	}

	total, _ := store.Count()
	ui.MutedMsg("\nShowing %d of %d total entries", len(entries), total)

	return nil
}

// formatPackages formats a list of packages for display.
func formatPackages(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	if len(packages) == 1 {
		return packages[0]
	}
	if len(packages) <= 3 {
		return fmt.Sprintf("%v", packages)
	}
	return fmt.Sprintf("%s (+%d more)", packages[0], len(packages)-1)
}
