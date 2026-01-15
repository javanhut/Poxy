package database

import (
	"encoding/json"
	"strings"
	"sync"

	"go.etcd.io/bbolt"
)

// Mapping represents a cross-source package mapping.
// For example, "firefox" in pacman might be "org.mozilla.firefox" in flatpak.
type Mapping struct {
	// Canonical is the standard/common name for the package
	Canonical string `json:"canonical"`

	// Sources maps source names to package names in that source
	// e.g., {"pacman": "firefox", "flatpak": "org.mozilla.firefox", "apt": "firefox"}
	Sources map[string]string `json:"sources"`

	// Category for the package (e.g., "browser", "editor", "media")
	Category string `json:"category,omitempty"`
}

// MappingStore manages cross-source package mappings.
type MappingStore struct {
	mu       sync.RWMutex
	mappings map[string]*Mapping // canonical -> mapping
	reverse  map[string]string   // "source:name" -> canonical
}

// NewMappingStore creates a new mapping store.
func NewMappingStore() *MappingStore {
	return &MappingStore{
		mappings: make(map[string]*Mapping),
		reverse:  make(map[string]string),
	}
}

// Add adds a mapping to the store.
func (ms *MappingStore) Add(mapping *Mapping) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.mappings[mapping.Canonical] = mapping

	// Update reverse index
	for source, name := range mapping.Sources {
		key := source + ":" + strings.ToLower(name)
		ms.reverse[key] = mapping.Canonical
	}
}

// AddBatch adds multiple mappings to the store.
func (ms *MappingStore) AddBatch(mappings []*Mapping) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, mapping := range mappings {
		ms.mappings[mapping.Canonical] = mapping
		for source, name := range mapping.Sources {
			key := source + ":" + strings.ToLower(name)
			ms.reverse[key] = mapping.Canonical
		}
	}
}

// GetByCanonical retrieves a mapping by its canonical name.
func (ms *MappingStore) GetByCanonical(canonical string) *Mapping {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.mappings[canonical]
}

// GetBySourceName retrieves a mapping by source and package name.
func (ms *MappingStore) GetBySourceName(source, name string) *Mapping {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	key := source + ":" + strings.ToLower(name)
	if canonical, ok := ms.reverse[key]; ok {
		return ms.mappings[canonical]
	}
	return nil
}

// GetNameForSource returns the package name for a specific source.
func (ms *MappingStore) GetNameForSource(canonical, source string) string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if mapping, ok := ms.mappings[canonical]; ok {
		if name, ok := mapping.Sources[source]; ok {
			return name
		}
	}
	return ""
}

// FindEquivalent finds equivalent packages across sources.
func (ms *MappingStore) FindEquivalent(source, name string) map[string]string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	key := source + ":" + strings.ToLower(name)
	if canonical, ok := ms.reverse[key]; ok {
		if mapping, ok := ms.mappings[canonical]; ok {
			// Return a copy of the sources map
			result := make(map[string]string)
			for s, n := range mapping.Sources {
				result[s] = n
			}
			return result
		}
	}
	return nil
}

// GetAllMappings returns all mappings.
func (ms *MappingStore) GetAllMappings() []*Mapping {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make([]*Mapping, 0, len(ms.mappings))
	for _, mapping := range ms.mappings {
		result = append(result, mapping)
	}
	return result
}

// Clear removes all mappings.
func (ms *MappingStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.mappings = make(map[string]*Mapping)
	ms.reverse = make(map[string]string)
}

// Size returns the number of mappings.
func (ms *MappingStore) Size() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.mappings)
}

// SaveToDB saves mappings to the database.
func (ms *MappingStore) SaveToDB(db *bbolt.DB) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketMappings))
		if err != nil {
			return err
		}

		for canonical, mapping := range ms.mappings {
			data, err := json.Marshal(mapping)
			if err != nil {
				continue
			}
			if err := bucket.Put([]byte(canonical), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// LoadFromDB loads mappings from the database.
func (ms *MappingStore) LoadFromDB(db *bbolt.DB) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketMappings))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(_, data []byte) error {
			var mapping Mapping
			if err := json.Unmarshal(data, &mapping); err != nil {
				return nil // Skip malformed entries
			}

			ms.mappings[mapping.Canonical] = &mapping
			for source, name := range mapping.Sources {
				key := source + ":" + strings.ToLower(name)
				ms.reverse[key] = mapping.Canonical
			}

			return nil
		})
	})
}

// CommonMappings returns a set of commonly known package mappings.
func CommonMappings() []*Mapping {
	return []*Mapping{
		// Browsers
		{
			Canonical: "firefox",
			Category:  "browser",
			Sources: map[string]string{
				"pacman":  "firefox",
				"apt":     "firefox",
				"dnf":     "firefox",
				"flatpak": "org.mozilla.firefox",
				"brew":    "firefox",
				"winget":  "Mozilla.Firefox",
			},
		},
		{
			Canonical: "chromium",
			Category:  "browser",
			Sources: map[string]string{
				"pacman":  "chromium",
				"apt":     "chromium-browser",
				"dnf":     "chromium",
				"flatpak": "org.chromium.Chromium",
				"brew":    "chromium",
			},
		},
		{
			Canonical: "google-chrome",
			Category:  "browser",
			Sources: map[string]string{
				"aur":     "google-chrome",
				"apt":     "google-chrome-stable",
				"flatpak": "com.google.Chrome",
				"winget":  "Google.Chrome",
			},
		},

		// Editors
		{
			Canonical: "vscode",
			Category:  "editor",
			Sources: map[string]string{
				"aur":     "visual-studio-code-bin",
				"apt":     "code",
				"flatpak": "com.visualstudio.code",
				"brew":    "visual-studio-code",
				"winget":  "Microsoft.VisualStudioCode",
			},
		},
		{
			Canonical: "vim",
			Category:  "editor",
			Sources: map[string]string{
				"pacman": "vim",
				"apt":    "vim",
				"dnf":    "vim-enhanced",
				"brew":   "vim",
				"winget": "vim.vim",
			},
		},
		{
			Canonical: "neovim",
			Category:  "editor",
			Sources: map[string]string{
				"pacman":  "neovim",
				"apt":     "neovim",
				"dnf":     "neovim",
				"flatpak": "io.neovim.nvim",
				"brew":    "neovim",
			},
		},

		// Communication
		{
			Canonical: "discord",
			Category:  "communication",
			Sources: map[string]string{
				"aur":     "discord",
				"flatpak": "com.discordapp.Discord",
				"brew":    "discord",
				"winget":  "Discord.Discord",
			},
		},
		{
			Canonical: "slack",
			Category:  "communication",
			Sources: map[string]string{
				"aur":     "slack-desktop",
				"flatpak": "com.slack.Slack",
				"brew":    "slack",
				"winget":  "SlackTechnologies.Slack",
			},
		},
		{
			Canonical: "telegram",
			Category:  "communication",
			Sources: map[string]string{
				"pacman":  "telegram-desktop",
				"apt":     "telegram-desktop",
				"flatpak": "org.telegram.desktop",
				"brew":    "telegram",
				"winget":  "Telegram.TelegramDesktop",
			},
		},

		// Media
		{
			Canonical: "vlc",
			Category:  "media",
			Sources: map[string]string{
				"pacman":  "vlc",
				"apt":     "vlc",
				"dnf":     "vlc",
				"flatpak": "org.videolan.VLC",
				"brew":    "vlc",
				"winget":  "VideoLAN.VLC",
			},
		},
		{
			Canonical: "spotify",
			Category:  "media",
			Sources: map[string]string{
				"aur":     "spotify",
				"flatpak": "com.spotify.Client",
				"brew":    "spotify",
				"winget":  "Spotify.Spotify",
			},
		},

		// Development
		{
			Canonical: "git",
			Category:  "development",
			Sources: map[string]string{
				"pacman": "git",
				"apt":    "git",
				"dnf":    "git",
				"brew":   "git",
				"winget": "Git.Git",
			},
		},
		{
			Canonical: "docker",
			Category:  "development",
			Sources: map[string]string{
				"pacman": "docker",
				"apt":    "docker.io",
				"dnf":    "docker",
				"brew":   "docker",
				"winget": "Docker.DockerDesktop",
			},
		},
		{
			Canonical: "nodejs",
			Category:  "development",
			Sources: map[string]string{
				"pacman": "nodejs",
				"apt":    "nodejs",
				"dnf":    "nodejs",
				"brew":   "node",
				"winget": "OpenJS.NodeJS",
			},
		},

		// Utilities
		{
			Canonical: "htop",
			Category:  "utility",
			Sources: map[string]string{
				"pacman": "htop",
				"apt":    "htop",
				"dnf":    "htop",
				"brew":   "htop",
			},
		},
		{
			Canonical: "curl",
			Category:  "utility",
			Sources: map[string]string{
				"pacman": "curl",
				"apt":    "curl",
				"dnf":    "curl",
				"brew":   "curl",
			},
		},
	}
}
