package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_history.db")

	// Override the history path
	originalXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)

	store, err := Open()
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Setenv("XDG_DATA_HOME", originalXDG)
		os.Remove(dbPath)
	}

	return store, cleanup
}

func TestOpen(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if store == nil {
		t.Fatal("Open() returned nil")
	}
}

func TestRecord(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	entry := NewEntry(OpInstall, "apt", []string{"vim", "git"})
	entry.MarkSuccess()

	err := store.Record(entry)
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Verify it was recorded
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestList(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Record multiple entries
	for i := 0; i < 5; i++ {
		entry := NewEntry(OpInstall, "apt", []string{"pkg" + string(rune('a'+i))})
		entry.MarkSuccess()
		store.Record(entry)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// List all
	entries, err := store.List(0)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}

	// List with limit
	limitedEntries, err := store.List(3)
	if err != nil {
		t.Fatalf("List(3) error: %v", err)
	}
	if len(limitedEntries) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(limitedEntries))
	}

	// Entries should be in reverse chronological order (newest first)
	if len(entries) >= 2 {
		if entries[0].Timestamp.Before(entries[1].Timestamp) {
			t.Error("List() should return entries in reverse chronological order")
		}
	}
}

func TestGet(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Record an entry
	entry := NewEntry(OpInstall, "apt", []string{"vim"})
	entry.MarkSuccess()
	store.Record(entry)

	// Get by ID
	retrieved, err := store.Get(entry.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if retrieved.ID != entry.ID {
		t.Errorf("Get() returned wrong entry: %s != %s", retrieved.ID, entry.ID)
	}

	// Get non-existent
	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should error for non-existent ID")
	}
}

func TestLast(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Empty store
	entry, err := store.Last()
	// Note: empty store returns nil entry without error
	_ = err // Explicitly ignore - no error expected for empty store
	if entry != nil {
		t.Error("Last() should return nil for empty store")
	}

	// Add entries
	entry1 := NewEntry(OpInstall, "apt", []string{"vim"})
	store.Record(entry1)
	time.Sleep(1 * time.Millisecond)

	entry2 := NewEntry(OpUninstall, "apt", []string{"git"})
	store.Record(entry2)

	// Get last
	last, err := store.Last()
	if err != nil {
		t.Fatalf("Last() error: %v", err)
	}

	if last.ID != entry2.ID {
		t.Errorf("Last() returned wrong entry: %s != %s", last.ID, entry2.ID)
	}
}

func TestLastReversible(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add non-reversible entry
	entry1 := NewEntry(OpUpdate, "apt", nil)
	entry1.MarkSuccess()
	store.Record(entry1)
	time.Sleep(1 * time.Millisecond)

	// Add reversible entry
	entry2 := NewEntry(OpInstall, "apt", []string{"vim"})
	entry2.MarkSuccess()
	store.Record(entry2)

	// Get last reversible
	reversible, err := store.LastReversible()
	if err != nil {
		t.Fatalf("LastReversible() error: %v", err)
	}

	if reversible.ID != entry2.ID {
		t.Errorf("LastReversible() returned wrong entry")
	}
}

func TestCount(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Empty store
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 for empty store, got %d", count)
	}

	// Add entries
	for i := 0; i < 3; i++ {
		entry := NewEntry(OpInstall, "apt", []string{"pkg"})
		store.Record(entry)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestClear(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add entries
	for i := 0; i < 3; i++ {
		entry := NewEntry(OpInstall, "apt", []string{"pkg"})
		store.Record(entry)
	}

	// Clear
	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// Verify empty
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after Clear(), got %d", count)
	}
}

func TestPrune(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add an old entry (we'll manually set the timestamp)
	oldEntry := &Entry{
		ID:        "old-entry",
		Timestamp: time.Now().Add(-48 * time.Hour), // 2 days ago
		Operation: OpInstall,
		Source:    "apt",
		Packages:  []string{"old-pkg"},
		Success:   true,
	}
	store.Record(oldEntry)

	// Add a recent entry
	newEntry := NewEntry(OpInstall, "apt", []string{"new-pkg"})
	store.Record(newEntry)

	// Prune entries older than 24 hours
	deleted, err := store.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted entry, got %d", deleted)
	}

	// Verify only new entry remains
	count, _ := store.Count()
	if count != 1 {
		t.Errorf("expected 1 entry after prune, got %d", count)
	}
}

func TestClose(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// Close again should not error
	_ = store.Close()
	// May or may not error depending on implementation
}
