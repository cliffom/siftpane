package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	profileEntries   = 512        // Successful display identities per authenticated workspace/user.
	profileBytes     = 256 * 1024 // Maximum disposable name-cache file size.
	profileMaxAge    = 7 * 24 * time.Hour
	profileVersion   = 1
	profileCacheKind = "profiles"
)

type profileSummary struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Handle  string    `json:"handle,omitempty"`
	Updated time.Time `json:"updated"`
}

type profileFile struct {
	Version  int              `json:"version"`
	Profiles []profileSummary `json:"profiles"`
}

// Profiles loads fresh display identities. Invalid cache data never blocks startup.
func (s *Store) Profiles(id core.Identity) []core.Profile {
	if s.Path == "" {
		return nil
	}
	f, err := os.Open(s.cachePath(id, profileCacheKind))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() // Read-only disposable cache.
	b, err := io.ReadAll(io.LimitReader(f, profileBytes+1))
	if err != nil || len(b) > profileBytes {
		return nil
	}
	var cache profileFile
	if json.Unmarshal(b, &cache) != nil || cache.Version != profileVersion || len(cache.Profiles) > profileEntries {
		return nil
	}
	out := []core.Profile{}
	seen := map[string]bool{}
	now := time.Now()
	for _, p := range cache.Profiles {
		age := now.Sub(p.Updated)
		if p.ID == "" || p.Name == "" || seen[p.ID] || age < 0 || age > profileMaxAge {
			continue
		}
		seen[p.ID] = true
		out = append(out, core.Profile{ID: p.ID, Name: p.Name, Handle: p.Handle, Updated: p.Updated})
	}
	return out
}

// SaveProfiles retains bounded successful names, never bodies or complete API profiles.
// Per-entry lookup timestamps prevent unrelated saves from renewing stale names.
func (s *Store) SaveProfiles(id core.Identity, ps []core.Profile) error {
	if s.Path == "" {
		return nil
	}
	ps = append([]core.Profile(nil), ps...)
	sort.Slice(ps, func(i, j int) bool { return ps[i].Updated.After(ps[j].Updated) })
	cache := profileFile{Version: profileVersion}
	for _, p := range ps {
		if len(cache.Profiles) >= profileEntries {
			break
		}
		if p.ID == "" || p.Name == "" {
			continue
		}
		cache.Profiles = append(cache.Profiles, profileSummary{ID: p.ID, Name: p.Name, Handle: p.Handle, Updated: p.Updated})
	}
	b, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if len(b) > profileBytes {
		return fmt.Errorf("name cache exceeds %d bytes", profileBytes)
	}
	return writePrivate(s.cachePath(id, profileCacheKind), b)
}
