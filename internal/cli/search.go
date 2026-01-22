package cli

import (
	"context"
	"fmt"

	"poxy/internal/ui"
	"poxy/pkg/manager"

	"github.com/spf13/cobra"
)

var (
	searchInstalled bool
	searchLimit     int
	searchNative    bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for packages",
	Long: `Search for packages across all available package sources.

By default, uses intelligent TF-IDF search for better relevance ranking.
Results are sorted by relevance score, with exact matches first.

Use --source to search a specific source only.
Use --native to bypass TF-IDF and use native package manager search.

Examples:
  poxy search firefox           # Smart search across all sources
  poxy search vim -s apt        # Search only apt
  poxy search --installed vim   # Search installed packages only
  poxy search -l 10 editor      # Limit to 10 results
  poxy search --native firefox  # Use native search (no TF-IDF)`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&searchInstalled, "installed", false, "search installed packages only")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 0, "limit results (0 = default 50)")
	searchCmd.Flags().BoolVar(&searchNative, "native", false, "use native search instead of TF-IDF")
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	query := args[0]

	// Determine if we should use smart search
	useSmartSearch := searchEngine != nil && !searchNative && cfg.General.SmartSearch

	// If source specified, search only that source
	if source != "" {
		return searchSingleSource(ctx, query, source)
	}

	// Smart search with TF-IDF
	if useSmartSearch {
		return searchSmart(ctx, query)
	}

	// Fallback to native search
	return searchNativeAll(ctx, query)
}

// searchSingleSource searches a specific package source.
func searchSingleSource(ctx context.Context, query, sourceName string) error {
	mgr, err := registry.GetManagerForSource(sourceName)
	if err != nil {
		return err
	}

	ui.InfoMsg("Searching for '%s' in %s...", query, mgr.DisplayName())

	opts := manager.SearchOpts{
		Limit:         searchLimit,
		InstalledOnly: searchInstalled,
	}

	results, err := mgr.Search(ctx, query, opts)
	if err != nil {
		return err
	}

	printSearchResults(results)
	return offerInstall(ctx, results)
}

// searchSmart performs TF-IDF based intelligent search.
func searchSmart(ctx context.Context, query string) error {
	ui.InfoMsg("Searching for '%s' (smart search)...", query)

	opts := SearchOptions{
		Limit:         searchLimit,
		InstalledOnly: searchInstalled,
		NativeFirst:   true,
	}

	if opts.Limit == 0 {
		opts.Limit = 50
	}

	results, err := searchEngine.Search(ctx, query, opts)
	if err != nil {
		ui.WarningMsg("Smart search error, falling back to native: %v", err)
		return searchNativeAll(ctx, query)
	}

	if len(results) == 0 {
		ui.InfoMsg("No packages found matching '%s'", query)
		return nil
	}

	// Print results with scores if verbose
	printSmartResults(results)

	// Convert to manager.Package for install prompt
	packages := make([]manager.Package, len(results))
	for i, r := range results {
		packages[i] = r.Package
	}

	return offerInstall(ctx, packages)
}

// searchNativeAll searches using native package managers.
func searchNativeAll(ctx context.Context, query string) error {
	ui.InfoMsg("Searching for '%s' across all sources...", query)

	opts := manager.SearchOpts{
		Limit:         searchLimit,
		InstalledOnly: searchInstalled,
	}

	results, err := registry.SearchAll(ctx, query, opts)
	if err != nil {
		ui.WarningMsg("Some sources returned errors: %v", err)
	}

	printSearchResults(results)
	return offerInstall(ctx, results)
}

// printSearchResults prints search results in standard format.
func printSearchResults(results []manager.Package) {
	if len(results) == 0 {
		ui.InfoMsg("No packages found")
		return
	}

	ui.PrintSearchResults(results)
}

// printSmartResults prints smart search results with relevance info.
func printSmartResults(results []SearchResult) {
	if len(results) == 0 {
		ui.InfoMsg("No packages found")
		return
	}

	ui.HeaderMsg("Search Results (%d)", len(results))
	ui.Println("")

	for i, r := range results {
		// Format: [rank] name version [source] - description
		// In verbose mode, also show score and match reason

		rank := fmt.Sprintf("%2d.", i+1)

		installed := ""
		if r.Installed {
			installed = ui.Green(" [installed]")
		}

		if verbose {
			// Verbose output with score
			ui.Println("%s %s %s [%s] (%.1f - %s)%s",
				rank,
				ui.Bold(r.Name),
				ui.Green(r.Version),
				ui.Cyan(r.Source),
				r.Score,
				r.MatchReason,
				installed,
			)
		} else {
			// Normal output
			ui.Println("%s %s %s [%s]%s",
				rank,
				ui.Bold(r.Name),
				ui.Green(r.Version),
				ui.Cyan(r.Source),
				installed,
			)
		}

		// Description (truncated)
		if r.Description != "" {
			desc := r.Description
			if len(desc) > 70 {
				desc = desc[:67] + "..."
			}
			ui.MutedMsg("    %s", desc)
		}
	}

	// Show index stats if verbose
	if verbose && searchEngine != nil {
		ui.Println("")
		ui.MutedMsg("Index: %d packages indexed", searchEngine.IndexSize())
	}
}

// offerInstall offers to install a selected package.
func offerInstall(ctx context.Context, results []manager.Package) error {
	if len(results) == 0 {
		return nil
	}

	pkg, err := ui.SelectPackage(results, "Select a package to install")
	if err != nil || pkg == nil {
		return nil
	}

	mgr, ok := registry.Get(pkg.Source)
	if !ok {
		return fmt.Errorf("package manager not available: %s", pkg.Source)
	}

	prompt := fmt.Sprintf("Install %s from %s?", pkg.Name, pkg.Source)
	confirmed, _ := ui.Confirm(prompt, true) //nolint:errcheck
	if confirmed {
		return runInstallPackage(ctx, mgr, pkg.Name)
	}

	return nil
}

// runInstallPackage is a helper to install a single package.
func runInstallPackage(ctx context.Context, mgr manager.Manager, pkg string) error {
	opts := manager.InstallOpts{
		AutoConfirm: cfg.General.AutoConfirm,
		DryRun:      cfg.General.DryRun,
	}

	err := mgr.Install(ctx, []string{pkg}, opts)
	if err != nil {
		ui.ErrorMsg("Installation failed: %v", err)
		return err
	}

	ui.SuccessMsg("Successfully installed %s", pkg)
	return nil
}
