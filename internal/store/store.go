// Package store persists non-secret preferences only. Message bodies stay in memory.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Workspace stores only inclusion and local read positions for one workspace.
type Workspace struct {
	Included map[string]bool   `json:"included"`
	Read     map[string]string `json:"local_read"`
}

// Config isolates preferences by Slack workspace ID.
type Config struct {
	Workspaces map[string]*Workspace `json:"workspaces"`
}

// Store owns preferences and their path; an empty path disables disk writes.
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
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, privateDirectoryMode); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.Config, "", "  ")
	if err != nil {
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
	return os.Rename(name, s.Path)
}

const privateDirectoryMode = 0o700 // Owner-only preference directory.
