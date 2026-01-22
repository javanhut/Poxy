package cli

import (
	"context"
	"sort"
	"strings"
	"sync"

	"poxy/pkg/database"
	"poxy/pkg/manager"
)

// SearchEngine provides intelligent package search with TF-IDF ranking.
// It combines cached index search with live manager queries for best results.
type SearchEngine struct {
	mu sync.RWMutex

	index    *database.Index
	store    *database.Store
	mappings *database.MappingStore
	registry *manager.Registry

	// State
	indexReady bool
	indexSize  int
}

// SearchResult represents a search result with relevance information.
type SearchResult struct {
	manager.Package
	Score       float64 // Relevance score (higher is better)
	MatchReason string  // Why this package matched
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Limit         int    // Maximum results (0 = default 50)
	SourceFilter  string // Only search this source
	InstalledOnly bool   // Only return installed packages
	NativeFirst   bool   // Boost native packages in ranking
}

// NewSearchEngine creates a new search engine.
func NewSearchEngine(registry *manager.Registry) *SearchEngine {
	return &SearchEngine{
		index:    database.NewIndex(),
		mappings: database.NewMappingStore(),
		registry: registry,
	}
}

// IsReady returns true if the index has been loaded and has data.
func (e *SearchEngine) IsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.indexReady && e.indexSize > 0
}

// IndexSize returns the number of packages in the index.
func (e *SearchEngine) IndexSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.indexSize
}

// Search performs an intelligent search using TF-IDF with fallback.
func (e *SearchEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit == 0 {
		opts.Limit = 50
	}

	// Determine native source for boosting
	nativeSource := ""
	if native := e.registry.Native(); native != nil {
		nativeSource = native.Name()
	}

	// Try TF-IDF search first if index is ready
	if e.IsReady() {
		indexOpts := database.SearchOptions{
			Limit:          opts.Limit * 2, // Get more for merging
			SourceFilter:   opts.SourceFilter,
			InstalledOnly:  opts.InstalledOnly,
			BoostInstalled: true,
			NativeSource:   nativeSource,
		}

		indexResults := e.index.Search(query, indexOpts)

		if len(indexResults) > 0 {
			// Convert to our SearchResult type
			results := make([]SearchResult, 0, len(indexResults))
			for _, r := range indexResults {
				results = append(results, SearchResult{
					Package:     r.Package,
					Score:       r.Score,
					MatchReason: r.MatchReason,
				})
			}

			// Optionally merge with live results for freshness
			liveResults, _ := e.searchLive(ctx, query, opts) //nolint:errcheck
			results = e.mergeResults(results, liveResults, opts.Limit)

			return results, nil
		}
	}

	// Fallback to live search
	return e.searchLive(ctx, query, opts)
}

// searchLive performs a live search across all managers.
func (e *SearchEngine) searchLive(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	mgrOpts := manager.SearchOpts{
		Limit:         opts.Limit,
		InstalledOnly: opts.InstalledOnly,
	}

	var packages []manager.Package
	var err error

	if opts.SourceFilter != "" {
		// Search specific source
		mgr, mgrErr := e.registry.GetManagerForSource(opts.SourceFilter)
		if mgrErr != nil {
			return nil, mgrErr
		}
		packages, err = mgr.Search(ctx, query, mgrOpts)
	} else {
		// Search all sources
		packages, err = e.registry.SearchAll(ctx, query, mgrOpts)
	}

	if err != nil {
		return nil, err
	}

	// Convert to SearchResult with basic scoring
	results := make([]SearchResult, 0, len(packages))
	queryLower := strings.ToLower(query)

	for _, pkg := range packages {
		score := e.calculateBasicScore(pkg, queryLower)
		reason := e.getMatchReason(pkg, queryLower)

		results = append(results, SearchResult{
			Package:     pkg,
			Score:       score,
			MatchReason: reason,
		})
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// calculateBasicScore calculates a simple relevance score.
func (e *SearchEngine) calculateBasicScore(pkg manager.Package, queryLower string) float64 {
	nameLower := strings.ToLower(pkg.Name)
	score := 1.0

	// Exact name match
	if nameLower == queryLower {
		score = 100.0
	} else if strings.HasPrefix(nameLower, queryLower) {
		// Prefix match
		score = 50.0
	} else if strings.Contains(nameLower, queryLower) {
		// Contains in name
		score = 25.0
	} else if strings.Contains(strings.ToLower(pkg.Description), queryLower) {
		// Description match
		score = 10.0
	}

	// Boost installed packages
	if pkg.Installed {
		score *= 1.5
	}

	// Boost native source
	if native := e.registry.Native(); native != nil && pkg.Source == native.Name() {
		score *= 1.2
	}

	return score
}

// getMatchReason returns a human-readable match reason.
func (e *SearchEngine) getMatchReason(pkg manager.Package, queryLower string) string {
	nameLower := strings.ToLower(pkg.Name)

	if nameLower == queryLower {
		return "Exact match"
	}
	if strings.HasPrefix(nameLower, queryLower) {
		return "Name prefix"
	}
	if strings.Contains(nameLower, queryLower) {
		return "Name contains"
	}
	if strings.Contains(strings.ToLower(pkg.Description), queryLower) {
		return "Description"
	}
	return "Keyword"
}

// mergeResults merges TF-IDF results with live results, deduplicating.
func (e *SearchEngine) mergeResults(indexed, live []SearchResult, limit int) []SearchResult {
	// Build a set of already-seen packages
	seen := make(map[string]bool)
	results := make([]SearchResult, 0, len(indexed)+len(live))

	// Add indexed results first (they have better scores)
	for _, r := range indexed {
		key := r.Source + ":" + r.Name
		if !seen[key] {
			seen[key] = true
			results = append(results, r)
		}
	}

	// Add live results that aren't duplicates
	for _, r := range live {
		key := r.Source + ":" + r.Name
		if !seen[key] {
			seen[key] = true
			// Reduce score for live-only results
			r.Score *= 0.8
			results = append(results, r)
		}
	}

	// Re-sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// LoadIndex loads packages from the database store into the TF-IDF index.
func (e *SearchEngine) LoadIndex() error {
	store, err := database.Open()
	if err != nil {
		return err
	}
	defer store.Close()

	e.store = store

	// Load all cached packages
	entries, err := store.GetAllPackages()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	// Convert to Package slice
	packages := make([]manager.Package, len(entries))
	for i, entry := range entries {
		packages[i] = entry.Package
	}

	// Add to index
	e.index.AddBatch(packages)

	// Load mappings
	e.mappings.AddBatch(database.CommonMappings())

	e.mu.Lock()
	e.indexReady = true
	e.indexSize = len(packages)
	e.mu.Unlock()

	return nil
}

// BuildIndex fetches packages from all managers and builds the index.
func (e *SearchEngine) BuildIndex(ctx context.Context) error {
	store, err := database.Open()
	if err != nil {
		return err
	}
	defer store.Close()

	var allPackages []manager.Package

	// Fetch from all available managers
	for _, mgr := range e.registry.Available() {
		// Get installed packages
		installed, err := mgr.ListInstalled(ctx, manager.ListOpts{})
		if err != nil {
			continue
		}

		for i := range installed {
			installed[i].Installed = true
		}
		allPackages = append(allPackages, installed...)

		// For some managers, we could also search for popular packages
		// This is optional and could be expensive
	}

	if len(allPackages) == 0 {
		return nil
	}

	// Store in database
	if err := store.AddPackages(allPackages); err != nil {
		return err
	}

	// Add to index
	e.index.AddBatch(allPackages)

	// Load mappings
	e.mappings.AddBatch(database.CommonMappings())

	e.mu.Lock()
	e.indexReady = true
	e.indexSize = len(allPackages)
	e.mu.Unlock()

	return nil
}

// GetMappings returns the package mapping store.
func (e *SearchEngine) GetMappings() *database.MappingStore {
	return e.mappings
}

// FindEquivalent finds equivalent packages across sources for a given package.
func (e *SearchEngine) FindEquivalent(source, name string) map[string]string {
	return e.mappings.FindEquivalent(source, name)
}

// GetMappedName returns the package name for a specific source, using mappings.
func (e *SearchEngine) GetMappedName(canonical, source string) string {
	return e.mappings.GetNameForSource(canonical, source)
}

// GetIndex returns the underlying TF-IDF index.
func (e *SearchEngine) GetIndex() *database.Index {
	return e.index
}
