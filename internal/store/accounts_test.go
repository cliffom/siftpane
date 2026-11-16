package store

import (
	"path/filepath"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

func TestAccountPreferencesClaimLegacyOnce(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	first := core.Identity{Workspace: "T1", User: "U1"}
	second := core.Identity{Workspace: "T1", User: "U2"}
	other := core.Identity{Workspace: "T2", User: "U1"}
	st.Workspace(first.Workspace).Included["C1"] = true
	st.Workspace(first.Workspace).Read["C1"] = "42.000001"
	a := st.AccountWorkspace(first)
	if !a.Included["C1"] || a.Read["C1"] != "42.000001" {
		t.Fatal("legacy state lost")
	}
	for _, id := range []core.Identity{second, other} {
		b := st.AccountWorkspace(id)
		if len(b.Included) != 0 || len(b.Read) != 0 {
			t.Fatal("legacy state shared across accounts")
		}
		b.Included["C2"] = true
	}
	if a.Included["C2"] {
		t.Fatal("shared preference maps")
	}
	if err = st.Save(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.AccountWorkspace(first).Included["C1"] || restored.AccountWorkspace(second).Included["C1"] {
		t.Fatal("isolation lost on restart")
	}
}
