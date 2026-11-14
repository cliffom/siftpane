package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceIsolationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs", "config.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Workspace("T1").Included["C1"] = true
	s.Workspace("T1").Read["C1"] = "42.0001"
	s.Workspace("T2").Included["C2"] = true
	if err = s.Save(); err != nil {
		t.Fatal(err)
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Workspace("T2").Included["C1"] || !r.Workspace("T1").Included["C1"] || r.Workspace("T1").Read["C1"] != "42.0001" {
		t.Fatal(r.Config)
	}
	delete(r.Workspace("T1").Included, "C1")
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Workspace("T1").Included["C1"] {
		t.Fatal("remove not saved")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatal(info.Mode())
	}
}

func TestCorruptConfigIsNotOverwritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(p); err == nil {
		t.Fatal("corrupt config ignored")
	}
}
