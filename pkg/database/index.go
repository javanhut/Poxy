package database

import (
	"math"
	"sort"
	"strings"
	"sync"

	"poxy/pkg/manager"
)

// SearchResult represents a package with its relevance score.
type SearchResult struct {
	manager.Package
	Score       float64 // TF-IDF relevance score
	MatchReason string  // Why this package matched
}

// Index provides TF-IDF based search over packages.
type Index struct {
	mu sync.RWMutex

	// Document storage
	documents []document
	docByID   map[string]int // "source:name" -> index

	// Inverted index: term -> document IDs
	invertedIndex map[string][]int

	// IDF cache: term -> IDF value
	idfCache map[string]float64

	// Scoring boosts
	boostExactMatch   float64
	boostPrefixMatch  float64
	boostInstalled    float64
	boostNativeSource float64
}

type document struct {
	Package manager.Package
	Terms   map[string]int // term -> frequency
	Length  int            // Total terms (for normalization)
}

// NewIndex creates a new TF-IDF search index.
func NewIndex() *Index {
	return &Index{
		docByID:           make(map[string]int),
		invertedIndex:     make(map[string][]int),
		idfCache:          make(map[string]float64),
		boostExactMatch:   10.0,
		boostPrefixMatch:  5.0,
		boostInstalled:    1.5,
		boostNativeSource: 1.2,
	}
}

// SetBoosts configures scoring boost factors.
func (idx *Index) SetBoosts(exactMatch, prefixMatch, installed, nativeSource float64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.boostExactMatch = exactMatch
	idx.boostPrefixMatch = prefixMatch
	idx.boostInstalled = installed
	idx.boostNativeSource = nativeSource
}

// Add adds a package to the index.
func (idx *Index) Add(pkg manager.Package) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.addUnlocked(pkg)
}

// AddBatch adds multiple packages to the index.
func (idx *Index) AddBatch(packages []manager.Package) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, pkg := range packages {
		idx.addUnlocked(pkg)
	}

	// Rebuild IDF cache
	idx.rebuildIDFCache()
}

func (idx *Index) addUnlocked(pkg manager.Package) {
	docID := pkg.Source + ":" + pkg.Name

	// Check if already indexed
	if existingIdx, exists := idx.docByID[docID]; exists {
		// Update existing document
		idx.removeFromInvertedIndex(existingIdx)
		idx.documents[existingIdx] = idx.createDocument(pkg)
		idx.addToInvertedIndex(existingIdx, idx.documents[existingIdx].Terms)
		return
	}

	// Create new document
	doc := idx.createDocument(pkg)
	docIdx := len(idx.documents)
	idx.documents = append(idx.documents, doc)
	idx.docByID[docID] = docIdx

	// Add to inverted index
	idx.addToInvertedIndex(docIdx, doc.Terms)
}

func (idx *Index) createDocument(pkg manager.Package) document {
	// Combine name and description for indexing
	text := pkg.Name + " " + pkg.Description

	// Tokenize and count term frequencies
	terms := make(map[string]int)
	tokens := tokenize(text)
	for _, token := range tokens {
		if len(token) >= 2 { // Ignore single-character tokens
			terms[token]++
		}
	}

	// Add name as a special term for exact matching
	terms["__name:"+strings.ToLower(pkg.Name)] = 1

	return document{
		Package: pkg,
		Terms:   terms,
		Length:  len(tokens),
	}
}

func (idx *Index) addToInvertedIndex(docIdx int, terms map[string]int) {
	for term := range terms {
		idx.invertedIndex[term] = append(idx.invertedIndex[term], docIdx)
	}
}

func (idx *Index) removeFromInvertedIndex(docIdx int) {
	doc := idx.documents[docIdx]
	for term := range doc.Terms {
		postings := idx.invertedIndex[term]
		for i, d := range postings {
			if d == docIdx {
				idx.invertedIndex[term] = append(postings[:i], postings[i+1:]...)
				break
			}
		}
	}
}

func (idx *Index) rebuildIDFCache() {
	idx.idfCache = make(map[string]float64)
	n := float64(len(idx.documents))

	for term, postings := range idx.invertedIndex {
		df := float64(len(postings))
		idx.idfCache[term] = math.Log(n / df)
	}
}

// Search performs a TF-IDF search and returns ranked results.
func (idx *Index) Search(query string, opts SearchOptions) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.documents) == 0 {
		return nil
	}

	// Tokenize query
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	// Calculate query TF-IDF vector
	queryVec := make(map[string]float64)
	for _, term := range queryTerms {
		queryVec[term]++
	}
	for term := range queryVec {
		if idf, ok := idx.idfCache[term]; ok {
			queryVec[term] *= idf
		}
	}

	// Score all candidate documents
	candidates := idx.findCandidates(queryTerms)
	results := make([]SearchResult, 0, len(candidates))

	queryLower := strings.ToLower(query)

	for docIdx := range candidates {
		doc := idx.documents[docIdx]
		score := idx.scoreDocument(doc, queryVec, queryTerms, queryLower, opts)

		if score > 0 {
			results = append(results, SearchResult{
				Package:     doc.Package,
				Score:       score,
				MatchReason: idx.getMatchReason(doc, queryTerms, queryLower),
			})
		}
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results
}

func (idx *Index) findCandidates(queryTerms []string) map[int]bool {
	candidates := make(map[int]bool)

	for _, term := range queryTerms {
		// Exact term match
		if postings, ok := idx.invertedIndex[term]; ok {
			for _, docIdx := range postings {
				candidates[docIdx] = true
			}
		}

		// Prefix matching for partial queries
		for indexedTerm, postings := range idx.invertedIndex {
			if strings.HasPrefix(indexedTerm, term) {
				for _, docIdx := range postings {
					candidates[docIdx] = true
				}
			}
		}
	}

	return candidates
}

func (idx *Index) scoreDocument(doc document, queryVec map[string]float64, queryTerms []string, queryLower string, opts SearchOptions) float64 {
	// Calculate TF-IDF cosine similarity
	var dotProduct, docNorm float64

	for term, tf := range doc.Terms {
		if qWeight, ok := queryVec[term]; ok {
			// TF-IDF weight for document term
			idf := idx.idfCache[term]
			docWeight := float64(tf) * idf

			dotProduct += qWeight * docWeight
		}
		idf := idx.idfCache[term]
		docWeight := float64(tf) * idf
		docNorm += docWeight * docWeight
	}

	if docNorm == 0 {
		return 0
	}

	// Cosine similarity
	score := dotProduct / math.Sqrt(docNorm)

	// Apply boosts
	nameLower := strings.ToLower(doc.Package.Name)

	// Exact name match
	if nameLower == queryLower {
		score *= idx.boostExactMatch
	} else if strings.HasPrefix(nameLower, queryLower) {
		// Prefix match
		score *= idx.boostPrefixMatch
	} else if strings.Contains(nameLower, queryLower) {
		// Contains match
		score *= 2.0
	}

	// Boost installed packages
	if doc.Package.Installed && opts.BoostInstalled {
		score *= idx.boostInstalled
	}

	// Boost native source
	if opts.NativeSource != "" && doc.Package.Source == opts.NativeSource {
		score *= idx.boostNativeSource
	}

	// Filter by source if specified
	if opts.SourceFilter != "" && doc.Package.Source != opts.SourceFilter {
		return 0
	}

	// Filter installed-only
	if opts.InstalledOnly && !doc.Package.Installed {
		return 0
	}

	return score
}

func (idx *Index) getMatchReason(doc document, queryTerms []string, queryLower string) string {
	nameLower := strings.ToLower(doc.Package.Name)

	if nameLower == queryLower {
		return "Exact name match"
	}
	if strings.HasPrefix(nameLower, queryLower) {
		return "Name starts with query"
	}
	if strings.Contains(nameLower, queryLower) {
		return "Name contains query"
	}
	if strings.Contains(strings.ToLower(doc.Package.Description), queryLower) {
		return "Description match"
	}

	return "Keyword match"
}

// Remove removes a package from the index.
func (idx *Index) Remove(source, name string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := source + ":" + name
	docIdx, exists := idx.docByID[docID]
	if !exists {
		return
	}

	idx.removeFromInvertedIndex(docIdx)
	delete(idx.docByID, docID)

	// Note: We don't remove from documents slice to avoid reindexing
	// The document will just be unreachable
}

// Clear removes all packages from the index.
func (idx *Index) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.documents = nil
	idx.docByID = make(map[string]int)
	idx.invertedIndex = make(map[string][]int)
	idx.idfCache = make(map[string]float64)
}

// Size returns the number of indexed packages.
func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.docByID)
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Limit          int    // Maximum results (0 = unlimited)
	SourceFilter   string // Only return results from this source
	InstalledOnly  bool   // Only return installed packages
	BoostInstalled bool   // Boost installed packages in ranking
	NativeSource   string // The native package manager source (for boosting)
}

// DefaultSearchOptions returns sensible default search options.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit:          50,
		BoostInstalled: true,
	}
}
