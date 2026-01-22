// Package database provides a package metadata cache with smart search capabilities.
package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"poxy/internal/config"
	"poxy/pkg/manager"

	"go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

const (
	bucketPackages = "packages"
	bucketMeta     = "meta"
	bucketMappings = "mappings"

	keyLastUpdate = "last_update"
	keyVersion    = "version"
)

// PackageEntry represents a cached package with metadata.
type PackageEntry struct {
	manager.Package
	LastSeen time.Time `json:"last_seen"`
	Keywords []string  `json:"keywords,omitempty"`
}

// Store manages the package metadata cache using BoltDB.
type Store struct {
	db *bbolt.DB
}

// Open opens or creates the package database.
func Open() (*Store, error) {
	if err := config.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := config.DataDir() + "/packages.db"

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open package database: %w", err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketPackages)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMeta)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMappings)); err != nil {
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

// AddPackage adds or updates a package in the cache.
func (s *Store) AddPackage(pkg manager.Package) error {
	entry := PackageEntry{
		Package:  pkg,
		LastSeen: time.Now(),
		Keywords: tokenize(pkg.Name + " " + pkg.Description),
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return fmt.Errorf("packages bucket not found")
		}

		// Create source sub-bucket if needed
		sourceBucket, err := bucket.CreateBucketIfNotExists([]byte(pkg.Source))
		if err != nil {
			return err
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal package: %w", err)
		}

		return sourceBucket.Put([]byte(pkg.Name), data)
	})
}

// AddPackages adds multiple packages to the cache.
func (s *Store) AddPackages(packages []manager.Package) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return fmt.Errorf("packages bucket not found")
		}

		now := time.Now()

		for _, pkg := range packages {
			entry := PackageEntry{
				Package:  pkg,
				LastSeen: now,
				Keywords: tokenize(pkg.Name + " " + pkg.Description),
			}

			sourceBucket, err := bucket.CreateBucketIfNotExists([]byte(pkg.Source))
			if err != nil {
				return err
			}

			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}

			if err := sourceBucket.Put([]byte(pkg.Name), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetPackage retrieves a package from the cache.
func (s *Store) GetPackage(source, name string) (*PackageEntry, error) {
	var entry *PackageEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		sourceBucket := bucket.Bucket([]byte(source))
		if sourceBucket == nil {
			return nil
		}

		data := sourceBucket.Get([]byte(name))
		if data == nil {
			return nil
		}

		entry = &PackageEntry{}
		return json.Unmarshal(data, entry)
	})

	return entry, err
}

// GetAllPackages retrieves all packages from the cache.
func (s *Store) GetAllPackages() ([]PackageEntry, error) {
	var packages []PackageEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(sourceName, _ []byte) error {
			sourceBucket := bucket.Bucket(sourceName)
			if sourceBucket == nil {
				return nil
			}

			return sourceBucket.ForEach(func(_, data []byte) error {
				var entry PackageEntry
				if err := json.Unmarshal(data, &entry); err != nil {
					return nil // Skip malformed entries
				}
				packages = append(packages, entry)
				return nil
			})
		})
	})

	return packages, err
}

// GetPackagesBySource retrieves all packages from a specific source.
func (s *Store) GetPackagesBySource(source string) ([]PackageEntry, error) {
	var packages []PackageEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		sourceBucket := bucket.Bucket([]byte(source))
		if sourceBucket == nil {
			return nil
		}

		return sourceBucket.ForEach(func(_, data []byte) error {
			var entry PackageEntry
			if err := json.Unmarshal(data, &entry); err != nil {
				return nil
			}
			packages = append(packages, entry)
			return nil
		})
	})

	return packages, err
}

// DeletePackage removes a package from the cache.
func (s *Store) DeletePackage(source, name string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		sourceBucket := bucket.Bucket([]byte(source))
		if sourceBucket == nil {
			return nil
		}

		return sourceBucket.Delete([]byte(name))
	})
}

// ClearSource removes all packages from a specific source.
func (s *Store) ClearSource(source string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		return bucket.DeleteBucket([]byte(source))
	})
}

// Clear removes all cached packages.
func (s *Store) Clear() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucketPackages)); err != nil && !errors.Is(err, berrors.ErrBucketNotFound) {
			return err
		}
		_, err := tx.CreateBucket([]byte(bucketPackages))
		return err
	})
}

// SetLastUpdate sets the last update time for a source.
func (s *Store) SetLastUpdate(source string, t time.Time) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketMeta))
		if bucket == nil {
			return nil
		}

		key := keyLastUpdate + ":" + source
		return bucket.Put([]byte(key), []byte(t.Format(time.RFC3339)))
	})
}

// GetLastUpdate returns the last update time for a source.
func (s *Store) GetLastUpdate(source string) (time.Time, error) {
	var t time.Time

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketMeta))
		if bucket == nil {
			return nil
		}

		key := keyLastUpdate + ":" + source
		data := bucket.Get([]byte(key))
		if data == nil {
			return nil
		}

		var err error
		t, err = time.Parse(time.RFC3339, string(data))
		return err
	})

	return t, err
}

// Count returns the number of cached packages.
func (s *Store) Count() (int, error) {
	var count int

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(_, _ []byte) error {
			sourceBucket := bucket.Bucket([]byte(bucketPackages))
			if sourceBucket != nil {
				count += sourceBucket.Stats().KeyN
			}
			return nil
		})
	})

	return count, err
}

// CountBySource returns the number of cached packages per source.
func (s *Store) CountBySource() (map[string]int, error) {
	counts := make(map[string]int)

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketPackages))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(sourceName, _ []byte) error {
			sourceBucket := bucket.Bucket(sourceName)
			if sourceBucket != nil {
				counts[string(sourceName)] = sourceBucket.Stats().KeyN
			}
			return nil
		})
	})

	return counts, err
}

// tokenize splits text into searchable tokens.
func tokenize(text string) []string {
	// Simple tokenization - split on non-alphanumeric characters
	var tokens []string
	var current []rune

	for _, r := range text {
		if isAlphanumeric(r) {
			current = append(current, toLower(r))
		} else if len(current) > 0 {
			tokens = append(tokens, string(current))
			current = current[:0]
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	return tokens
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
