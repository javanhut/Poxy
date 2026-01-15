package cli

import (
	"context"
	"sync"
	"time"
)

// IndexBuilder handles background index loading and refreshing.
type IndexBuilder struct {
	engine *SearchEngine

	mu       sync.Mutex
	loading  bool
	lastLoad time.Time
	loadErr  error
}

// NewIndexBuilder creates a new index builder.
func NewIndexBuilder(engine *SearchEngine) *IndexBuilder {
	return &IndexBuilder{
		engine: engine,
	}
}

// LoadAsync starts loading the index in the background.
// Returns immediately; use IsLoading() to check status.
func (b *IndexBuilder) LoadAsync() {
	b.mu.Lock()
	if b.loading {
		b.mu.Unlock()
		return
	}
	b.loading = true
	b.mu.Unlock()

	go func() {
		err := b.engine.LoadIndex()

		b.mu.Lock()
		b.loading = false
		b.loadErr = err
		if err == nil {
			b.lastLoad = time.Now()
		}
		b.mu.Unlock()
	}()
}

// LoadSync loads the index synchronously.
func (b *IndexBuilder) LoadSync() error {
	b.mu.Lock()
	b.loading = true
	b.mu.Unlock()

	err := b.engine.LoadIndex()

	b.mu.Lock()
	b.loading = false
	b.loadErr = err
	if err == nil {
		b.lastLoad = time.Now()
	}
	b.mu.Unlock()

	return err
}

// BuildAsync rebuilds the index from live data in the background.
func (b *IndexBuilder) BuildAsync(ctx context.Context) {
	b.mu.Lock()
	if b.loading {
		b.mu.Unlock()
		return
	}
	b.loading = true
	b.mu.Unlock()

	go func() {
		err := b.engine.BuildIndex(ctx)

		b.mu.Lock()
		b.loading = false
		b.loadErr = err
		if err == nil {
			b.lastLoad = time.Now()
		}
		b.mu.Unlock()
	}()
}

// BuildSync rebuilds the index from live data synchronously.
func (b *IndexBuilder) BuildSync(ctx context.Context) error {
	b.mu.Lock()
	b.loading = true
	b.mu.Unlock()

	err := b.engine.BuildIndex(ctx)

	b.mu.Lock()
	b.loading = false
	b.loadErr = err
	if err == nil {
		b.lastLoad = time.Now()
	}
	b.mu.Unlock()

	return err
}

// IsLoading returns true if the index is currently being loaded.
func (b *IndexBuilder) IsLoading() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loading
}

// LastError returns the error from the last load attempt.
func (b *IndexBuilder) LastError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loadErr
}

// LastLoadTime returns when the index was last successfully loaded.
func (b *IndexBuilder) LastLoadTime() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastLoad
}

// NeedsRefresh returns true if the index should be refreshed.
// By default, considers the index stale after 24 hours.
func (b *IndexBuilder) NeedsRefresh(maxAge time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.lastLoad.IsZero() {
		return true
	}

	return time.Since(b.lastLoad) > maxAge
}

// WaitForLoad blocks until the index is loaded or times out.
func (b *IndexBuilder) WaitForLoad(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if b.engine.IsReady() {
			return true
		}

		b.mu.Lock()
		loading := b.loading
		b.mu.Unlock()

		if !loading {
			// Not loading and not ready means load failed or never started
			return b.engine.IsReady()
		}

		time.Sleep(50 * time.Millisecond)
	}

	return b.engine.IsReady()
}
