package store

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/cliffom/siftpane/internal/core"
)

// Search metadata is disposable and account-isolated; it proves neither current
// access nor complete directory coverage. Per-entry ages survive partial scans.
const (
	directoryCacheKind = "directory"
	directoryEntries   = 4096
	directoryBytes     = 2 << 20
	directoryMaxAge    = 24 * time.Hour
	directoryTextRunes = 256
	directoryVersion   = 1
)

type directoryEntry struct {
	conversationSummary
	User    string    `json:"user,omitempty"`
	Checked time.Time `json:"checked"`
}

type directoryFile struct {
	Version int              `json:"version"`
	Entries []directoryEntry `json:"entries"`
}

func (s *Store) directoryEntries(id core.Identity) []directoryEntry {
	if s.Path == "" {
		return nil
	}
	f, err := os.Open(s.cachePath(id, directoryCacheKind))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() // Disposable read cache; close cannot change the result.
	b, err := io.ReadAll(io.LimitReader(f, directoryBytes+1))
	if err != nil || len(b) > directoryBytes {
		return nil
	}
	var cache directoryFile
	if json.Unmarshal(b, &cache) != nil || cache.Version != directoryVersion || len(cache.Entries) > directoryEntries {
		return nil
	}
	now := time.Now()
	out := []directoryEntry{}
	seen := map[string]bool{}
	for _, c := range cache.Entries {
		age := now.Sub(c.Checked)
		if c.ID == "" || c.Name == "" || !validKind(c.Kind) || seen[c.ID] || age < 0 || age > directoryMaxAge {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	return out
}

// Directory returns cached search suggestions without restoring inclusion or cursors.
func (s *Store) Directory(id core.Identity) []core.Conversation {
	out := []core.Conversation{}
	for _, c := range s.directoryEntries(id) {
		out = append(out, core.Conversation{ID: c.ID, Name: c.Name, Handle: c.Handle, Kind: c.Kind, User: c.User, Members: c.Members})
	}
	return out
}

// SaveDirectory merges freshly observed summaries without renewing other entries.
func (s *Store) SaveDirectory(id core.Identity, cs []core.Conversation) error {
	if s.Path == "" {
		return nil
	}
	byID := map[string]directoryEntry{}
	for _, c := range s.directoryEntries(id) {
		byID[c.ID] = c
	}
	now := time.Now()
	for _, c := range cs {
		if c.ID == "" || c.Name == "" || !validKind(c.Kind) || utf8.RuneCountInString(c.Name) > directoryTextRunes || utf8.RuneCountInString(c.Handle) > directoryTextRunes {
			continue
		}
		byID[c.ID] = directoryEntry{conversationSummary: conversationSummary{ID: c.ID, Name: c.Name, Handle: c.Handle, Kind: c.Kind, Members: c.Members}, User: c.User, Checked: now}
	}
	entries := make([]directoryEntry, 0, len(byID))
	for _, c := range byID {
		entries = append(entries, c)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Checked.Equal(entries[j].Checked) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Checked.After(entries[j].Checked)
	})
	entries = entries[:min(len(entries), directoryEntries)]
	cache := directoryFile{Version: directoryVersion, Entries: []directoryEntry{}}
	envelope, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	used := len(envelope)
	for _, entry := range entries {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if used+len(encoded)+1 > directoryBytes {
			break
		}
		cache.Entries = append(cache.Entries, entry)
		used += len(encoded) + 1
	}
	b, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return writePrivate(s.cachePath(id, directoryCacheKind), b)
}

// RemoveDirectory discards a denied search hint while preserving other entries' ages.
func (s *Store) RemoveDirectory(id core.Identity, cid string) error {
	if s.Path == "" {
		return nil
	}
	entries := s.directoryEntries(id)
	kept := entries[:0]
	for _, c := range entries {
		if c.ID != cid {
			kept = append(kept, c)
		}
	}
	b, err := json.Marshal(directoryFile{Version: directoryVersion, Entries: kept})
	if err != nil {
		return err
	}
	return writePrivate(s.cachePath(id, directoryCacheKind), b)
}
