package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceIsolationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs", "config.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	s.Workspace("T1").Included["C1"] = true
	s.Workspace("T1").Read["C1"] = "42.0001"
	s.Workspace("T2").Included["C2"] = true
	if e = s.Save(); e != nil {
		t.Fatal(e)
	}
	r, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if r.Workspace("T2").Included["C1"] || !r.Workspace("T1").Included["C1"] || r.Workspace("T1").Read["C1"] != "42.0001" {
		t.Fatal(r.Config)
	}
	delete(r.Workspace("T1").Included, "C1")
	r.Save()
	r, _ = Open(path)
	if r.Workspace("T1").Included["C1"] {
		t.Fatal("remove not saved")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
func TestCorruptConfigIsNotOverwritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte("bad"), 0600)
	if _, e := Open(p); e == nil {
		t.Fatal("corrupt config ignored")
	}
}
