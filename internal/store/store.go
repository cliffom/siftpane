// Package store persists preferences, attention and explicitly opted-in disposable display caches.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cliffom/siftpane/internal/core"
)

// Workspace stores only inclusion and local read positions for one account.
type Workspace struct {
	Included map[string]bool   `json:"included"`
	Read     map[string]string `json:"local_read"`
}

// Config isolates account preferences and stores nonsecret credential identities.
type Config struct {
	Workspaces         map[string]*Workspace    `json:"workspaces"`
	Accounts           map[string]core.Identity `json:"accounts,omitempty"`
	AccountPreferences map[string]*Workspace    `json:"account_preferences,omitempty"`
	DefaultAccount     string                   `json:"default_account,omitempty"`
}

// Store owns preferences and disposable display metadata; an empty path disables disk I/O.
type Store struct {
	Path   string
	Config Config
}

// DefaultPath returns the platform-specific Siftpane configuration path.
func DefaultPath() (string, error) {
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "siftpane", "config.json"), nil
}

// Open loads preferences, accepting a missing file and rejecting corrupt contents.
func Open(path string) (*Store, error) {
	s := &Store{Path: path, Config: Config{Workspaces: map[string]*Workspace{}}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &s.Config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if s.Config.Workspaces == nil {
		s.Config.Workspaces = map[string]*Workspace{}
	}
	return s, nil
}

// Workspace returns initialized preferences for the given workspace ID.
func (s *Store) Workspace(id string) *Workspace {
	if s.Config.Workspaces == nil {
		s.Config.Workspaces = map[string]*Workspace{}
	}
	w := s.Config.Workspaces[id]
	if w == nil {
		w = &Workspace{}
		s.Config.Workspaces[id] = w
	}
	if w.Included == nil {
		w.Included = map[string]bool{}
	}
	if w.Read == nil {
		w.Read = map[string]string{}
	}
	return w
}

// Save atomically replaces the preference file with private file permissions.
func (s *Store) Save() error {
	if s.Path == "" {
		return nil
	} // isolated demo
	b, err := json.MarshalIndent(s.Config, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(s.Path, b)
}

func writePrivate(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, privateDirectoryMode); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		// Cleanup is best effort; Save reports write/sync/close/rename failures.
		_ = os.Remove(name)
	}()
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // Preserve the write/sync error that prevented saving.
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close() // Preserve the write/sync error that prevented saving.
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

const privateDirectoryMode = 0o700 // Owner-only preference directory.

// AccountKey identifies an authenticated account independently of display names.
func AccountKey(id core.Identity) string { return id.Workspace + "/" + id.User }

// AccountWorkspace claims legacy workspace preferences once, then isolates users.
// Old entries are moved, rather than copied into every user of the workspace.
func (s *Store) AccountWorkspace(id core.Identity) *Workspace {
	if s.Config.AccountPreferences == nil {
		s.Config.AccountPreferences = map[string]*Workspace{}
	}
	key := AccountKey(id)
	if w := s.Config.AccountPreferences[key]; w != nil {
		if w.Included == nil {
			w.Included = map[string]bool{}
		}
		if w.Read == nil {
			w.Read = map[string]string{}
		}
		return w
	}
	w := s.Workspace(id.Workspace)
	delete(s.Config.Workspaces, id.Workspace)
	s.Config.AccountPreferences[key] = w
	return w
}
