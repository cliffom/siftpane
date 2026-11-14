// Package store persists non-secret preferences only. Message bodies stay in memory.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Workspace struct {
	Included map[string]bool   `json:"included"`
	Read     map[string]string `json:"local_read"`
}
type Config struct {
	Workspaces map[string]*Workspace `json:"workspaces"`
}
type Store struct {
	Path   string
	Config Config
}

func DefaultPath() (string, error) {
	p, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(p, "siftpane", "config.json"), nil
}
func Open(path string) (*Store, error) {
	s := &Store{Path: path, Config: Config{Workspaces: map[string]*Workspace{}}}
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &s.Config); e != nil {
		return nil, fmt.Errorf("invalid config: %w", e)
	}
	if s.Config.Workspaces == nil {
		s.Config.Workspaces = map[string]*Workspace{}
	}
	return s, nil
}
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
func (s *Store) Save() error {
	if s.Path == "" {
		return nil
	} // isolated demo
	dir := filepath.Dir(s.Path)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s.Config, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".config-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(name, s.Path)
}
