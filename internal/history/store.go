package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"poxy/internal/config"

	"go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

const (
	bucketHistory = "history"
	bucketMeta    = "meta"
	keyLastOp     = "last_operation"
)

// Store manages operation history using BoltDB.
type Store struct {
	db *bbolt.DB
}

// Open opens or creates the history database.
func Open() (*Store, error) {
	if err := config.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := config.HistoryPath()

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open history database: %w", err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketHistory)); err != nil {
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

// Record saves a new history entry.
func (s *Store) Record(entry *Entry) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return fmt.Errorf("history bucket not found")
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal entry: %w", err)
		}

		// Use timestamp as key for chronological ordering
		key := []byte(entry.Timestamp.Format(time.RFC3339Nano))
		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to save entry: %w", err)
		}

		// Update last operation reference
		metaBucket := tx.Bucket([]byte(bucketMeta))
		if metaBucket != nil {
			_ = metaBucket.Put([]byte(keyLastOp), key) //nolint:errcheck
		}

		return nil
	})
}

// List returns the most recent history entries.
func (s *Store) List(limit int) ([]Entry, error) {
	var entries []Entry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()

		// Start from the end (most recent) and go backwards
		for k, v := cursor.Last(); k != nil && (limit <= 0 || len(entries) < limit); k, v = cursor.Prev() {
			var entry Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue // Skip malformed entries
			}
			entries = append(entries, entry)
		}

		return nil
	})

	return entries, err
}

// Get retrieves a specific entry by ID.
func (s *Store) Get(id string) (*Entry, error) {
	var entry *Entry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return fmt.Errorf("history bucket not found")
		}

		cursor := bucket.Cursor()

		// Search for entry with matching ID
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var e Entry
			if err := json.Unmarshal(v, &e); err != nil {
				continue
			}
			if e.ID == id {
				entry = &e
				return nil
			}
		}

		return fmt.Errorf("entry not found: %s", id)
	})

	return entry, err
}

// Last returns the most recent entry.
func (s *Store) Last() (*Entry, error) {
	var entry *Entry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		k, v := cursor.Last()
		if k == nil {
			return nil
		}

		var e Entry
		if err := json.Unmarshal(v, &e); err != nil {
			return err
		}
		entry = &e
		return nil
	})

	return entry, err
}

// LastReversible returns the most recent reversible entry.
func (s *Store) LastReversible() (*Entry, error) {
	entries, err := s.List(50) // Check recent entries
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.CanRollback() {
			return &e, nil
		}
	}

	return nil, fmt.Errorf("no reversible operations found")
}

// Count returns the total number of entries.
func (s *Store) Count() (int, error) {
	var count int

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return nil
		}

		count = bucket.Stats().KeyN
		return nil
	})

	return count, err
}

// Clear removes all history entries.
func (s *Store) Clear() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucketHistory)); err != nil && !errors.Is(err, berrors.ErrBucketNotFound) {
			return err
		}
		_, err := tx.CreateBucket([]byte(bucketHistory))
		return err
	})
}

// Prune removes entries older than the given duration.
func (s *Store) Prune(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	var deleted int

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketHistory))
		if bucket == nil {
			return nil
		}

		var toDelete [][]byte
		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var e Entry
			if err := json.Unmarshal(v, &e); err != nil {
				continue
			}
			if e.Timestamp.Before(cutoff) {
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
