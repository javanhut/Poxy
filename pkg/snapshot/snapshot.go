// Package snapshot provides system state capture and rollback functionality.
// It captures the installed packages across all package managers to enable
// easy recovery from failed or unwanted changes.
package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"poxy/internal/config"
	"poxy/pkg/manager"

	"go.etcd.io/bbolt"
)

const (
	bucketSnapshots = "snapshots"
	bucketMeta      = "snapshot_meta"
	keyLatest       = "latest_id"
	keyAutoPrefix   = "auto_"

	// MaxSnapshots is the default maximum number of snapshots to keep.
	MaxSnapshots = 50

	// MaxAutoSnapshots is the maximum number of automatic snapshots to keep.
	MaxAutoSnapshots = 20
)

// Trigger represents what caused the snapshot to be created.
type Trigger string

const (
	TriggerManual    Trigger = "manual"    // User explicitly created snapshot
	TriggerInstall   Trigger = "install"   // Before package installation
	TriggerUninstall Trigger = "uninstall" // Before package removal
	TriggerUpgrade   Trigger = "upgrade"   // Before system upgrade
	TriggerUpdate    Trigger = "update"    // Before package database update
	TriggerScheduled Trigger = "scheduled" // Scheduled/periodic snapshot
)

// PackageState represents a single installed package.
type PackageState struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"` // Package manager that installed it
}

// Snapshot represents the system state at a point in time.
type Snapshot struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Description string         `json:"description,omitempty"`
	Trigger     Trigger        `json:"trigger"`
	Packages    []PackageState `json:"packages"`

	// Metadata about the operation that triggered this snapshot
	Operation string   `json:"operation,omitempty"` // install, uninstall, upgrade
	Targets   []string `json:"targets,omitempty"`   // Packages being operated on
}

// NewSnapshot creates a new snapshot with the given trigger.
func NewSnapshot(trigger Trigger, description string) *Snapshot {
	return &Snapshot{
		ID:          generateSnapshotID(),
		Timestamp:   time.Now(),
		Description: description,
		Trigger:     trigger,
		Packages:    []PackageState{},
	}
}

// generateSnapshotID creates a unique snapshot ID.
func generateSnapshotID() string {
	return time.Now().Format("20060102-150405")
}

// FormatTime returns a human-readable timestamp.
func (s *Snapshot) FormatTime() string {
	return s.Timestamp.Format("2006-01-02 15:04:05")
}

// PackageCount returns the total number of packages in the snapshot.
func (s *Snapshot) PackageCount() int {
	return len(s.Packages)
}

// PackagesBySource returns packages grouped by their source manager.
func (s *Snapshot) PackagesBySource() map[string][]PackageState {
	result := make(map[string][]PackageState)
	for _, pkg := range s.Packages {
		result[pkg.Source] = append(result[pkg.Source], pkg)
	}
	return result
}

// HasPackage checks if a package is in this snapshot.
func (s *Snapshot) HasPackage(name, source string) bool {
	for _, pkg := range s.Packages {
		if pkg.Name == name && pkg.Source == source {
			return true
		}
	}
	return false
}

// GetPackage returns a package by name and source, or nil if not found.
func (s *Snapshot) GetPackage(name, source string) *PackageState {
	for i := range s.Packages {
		if s.Packages[i].Name == name && s.Packages[i].Source == source {
			return &s.Packages[i]
		}
	}
	return nil
}

// Summary returns a brief description of the snapshot.
func (s *Snapshot) Summary() string {
	desc := s.Description
	if desc == "" {
		desc = string(s.Trigger)
	}
	return fmt.Sprintf("%s - %s (%d packages)", s.ID, desc, len(s.Packages))
}

// Store manages snapshot storage using BoltDB.
type Store struct {
	db *bbolt.DB
}

// OpenStore opens or creates the snapshot database.
func OpenStore() (*Store, error) {
	if err := config.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := config.SnapshotPath()

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot database: %w", err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketSnapshots)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMeta)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Save saves a snapshot to the database.
func (s *Store) Save(snap *Snapshot) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return fmt.Errorf("snapshots bucket not found")
		}

		data, err := json.Marshal(snap)
		if err != nil {
			return fmt.Errorf("failed to marshal snapshot: %w", err)
		}

		key := []byte(snap.ID)
		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}

		// Update latest reference
		metaBucket := tx.Bucket([]byte(bucketMeta))
		if metaBucket != nil {
			_ = metaBucket.Put([]byte(keyLatest), key) //nolint:errcheck
		}

		return nil
	})
}

// Get retrieves a snapshot by ID.
func (s *Store) Get(id string) (*Snapshot, error) {
	var snap *Snapshot

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return fmt.Errorf("snapshots bucket not found")
		}

		data := bucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("snapshot not found: %s", id)
		}

		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}
		snap = &snapshot
		return nil
	})

	return snap, err
}

// Latest returns the most recent snapshot.
func (s *Store) Latest() (*Snapshot, error) {
	var snap *Snapshot

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		k, v := cursor.Last()
		if k == nil {
			return nil
		}

		var snapshot Snapshot
		if err := json.Unmarshal(v, &snapshot); err != nil {
			return err
		}
		snap = &snapshot
		return nil
	})

	return snap, err
}

// List returns snapshots, optionally limited and filtered by trigger.
func (s *Store) List(limit int, trigger Trigger) ([]Snapshot, error) {
	var snapshots []Snapshot

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()

		// Start from the end (most recent) and go backwards
		for k, v := cursor.Last(); k != nil && (limit <= 0 || len(snapshots) < limit); k, v = cursor.Prev() {
			var snap Snapshot
			if err := json.Unmarshal(v, &snap); err != nil {
				continue // Skip malformed entries
			}

			// Filter by trigger if specified
			if trigger != "" && snap.Trigger != trigger {
				continue
			}

			snapshots = append(snapshots, snap)
		}

		return nil
	})

	return snapshots, err
}

// Delete removes a snapshot by ID.
func (s *Store) Delete(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return fmt.Errorf("snapshots bucket not found")
		}

		return bucket.Delete([]byte(id))
	})
}

// Count returns the total number of snapshots.
func (s *Store) Count() (int, error) {
	var count int

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return nil
		}
		count = bucket.Stats().KeyN
		return nil
	})

	return count, err
}

// Prune removes old snapshots, keeping only the most recent ones.
func (s *Store) Prune(keepCount int, keepAutoCount int) (int, error) {
	var deleted int

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return nil
		}

		// Collect all snapshots
		var all []Snapshot
		var auto []Snapshot

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var snap Snapshot
			if err := json.Unmarshal(v, &snap); err != nil {
				continue
			}
			all = append(all, snap)
			if snap.Trigger != TriggerManual {
				auto = append(auto, snap)
			}
		}

		// Sort by timestamp (newest first)
		sort.Slice(all, func(i, j int) bool {
			return all[i].Timestamp.After(all[j].Timestamp)
		})
		sort.Slice(auto, func(i, j int) bool {
			return auto[i].Timestamp.After(auto[j].Timestamp)
		})

		// Determine which to delete
		toDelete := make(map[string]bool)

		// Mark old snapshots beyond keepCount for deletion
		if len(all) > keepCount {
			for _, snap := range all[keepCount:] {
				toDelete[snap.ID] = true
			}
		}

		// Also enforce auto snapshot limit
		if len(auto) > keepAutoCount {
			for _, snap := range auto[keepAutoCount:] {
				toDelete[snap.ID] = true
			}
		}

		// Delete marked snapshots
		for id := range toDelete {
			if err := bucket.Delete([]byte(id)); err != nil {
				return err
			}
			deleted++
		}

		return nil
	})

	return deleted, err
}

// PruneByAge removes snapshots older than the given duration.
func (s *Store) PruneByAge(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	var deleted int

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketSnapshots))
		if bucket == nil {
			return nil
		}

		var toDelete [][]byte
		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var snap Snapshot
			if err := json.Unmarshal(v, &snap); err != nil {
				continue
			}
			// Don't delete manual snapshots by age
			if snap.Trigger != TriggerManual && snap.Timestamp.Before(cutoff) {
				toDelete = append(toDelete, k)
			}
		}

		for _, k := range toDelete {
			if err := bucket.Delete(k); err != nil {
				return err
			}
			deleted++
		}

		return nil
	})

	return deleted, err
}

// Capture creates a snapshot of the current system state using the provided managers.
func Capture(ctx context.Context, trigger Trigger, description string, managers []manager.Manager) (*Snapshot, error) {
	snap := NewSnapshot(trigger, description)

	for _, mgr := range managers {
		if !mgr.IsAvailable() {
			continue
		}

		packages, err := mgr.ListInstalled(ctx, manager.ListOpts{})
		if err != nil {
			// Log but continue - don't fail the whole snapshot for one manager
			continue
		}

		for _, pkg := range packages {
			snap.Packages = append(snap.Packages, PackageState{
				Name:    pkg.Name,
				Version: pkg.Version,
				Source:  mgr.Name(),
			})
		}
	}

	// Sort packages for consistent ordering
	sort.Slice(snap.Packages, func(i, j int) bool {
		if snap.Packages[i].Source != snap.Packages[j].Source {
			return snap.Packages[i].Source < snap.Packages[j].Source
		}
		return snap.Packages[i].Name < snap.Packages[j].Name
	})

	return snap, nil
}

// CaptureAndSave captures the current state and saves it to the store.
func CaptureAndSave(ctx context.Context, trigger Trigger, description string, managers []manager.Manager) (*Snapshot, error) {
	snap, err := Capture(ctx, trigger, description, managers)
	if err != nil {
		return nil, err
	}

	store, err := OpenStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	if err := store.Save(snap); err != nil {
		return nil, err
	}

	// Auto-prune old snapshots
	_, _ = store.Prune(MaxSnapshots, MaxAutoSnapshots) //nolint:errcheck

	return snap, nil
}
