package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

func TestProfilePersistenceIsolationExpiryAndPermissions(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T1", User: "U1"}
	now := time.Now()
	ps := []core.Profile{{ID: "U2", Name: "Maya", Handle: "maya", Updated: now}, {ID: "U3", Name: "Expired", Updated: now.Add(-profileMaxAge - time.Hour)}, {ID: "U4", Name: "Future", Updated: now.Add(time.Hour)}}
	if err := st.SaveProfiles(id, ps); err != nil {
		t.Fatal(err)
	}
	got := st.Profiles(id)
	if len(got) != 1 || got[0].Name != "Maya" || !got[0].Updated.Equal(now) {
		t.Fatal(got)
	}
	info, err := os.Stat(st.cachePath(id, profileCacheKind))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatal(info.Mode())
	}
	for _, other := range []core.Identity{{Workspace: "T2", User: "U1"}, {Workspace: "T1", User: "other"}} {
		if len(st.Profiles(other)) != 0 {
			t.Fatal("cross-account profiles")
		}
	}
	// Saving an old identity again must not renew its freshness.
	if err := st.SaveProfiles(id, ps[1:2]); err != nil {
		t.Fatal(err)
	}
	if len(st.Profiles(id)) != 0 {
		t.Fatal("expired name renewed by save")
	}
}

func TestProfileCacheInvalidInputAndBounds(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "profiles"}
	for _, b := range [][]byte{[]byte("invalid names"), []byte(strings.Repeat(" ", profileBytes+1)), []byte(`{"version":2}`)} {
		if err := os.WriteFile(st.cachePath(id, profileCacheKind), b, 0o600); err != nil {
			t.Fatal(err)
		}
		if len(st.Profiles(id)) != 0 {
			t.Fatal("invalid names accepted")
		}
	}
	ps := []core.Profile{}
	for i := 0; i < profileEntries+1; i++ {
		ps = append(ps, core.Profile{ID: fmt.Sprint(i), Name: "Person", Updated: time.Now()})
	}
	if err := st.SaveProfiles(id, ps); err != nil {
		t.Fatal(err)
	}
	if len(st.Profiles(id)) != profileEntries {
		t.Fatal("names not bounded")
	}
	b, err := json.Marshal(profileFile{Version: profileVersion, Profiles: make([]profileSummary, profileEntries+1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.cachePath(id, profileCacheKind), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if len(st.Profiles(id)) != 0 {
		t.Fatal("oversized cache accepted")
	}
	if err := (&Store{}).SaveProfiles(id, ps); err != nil {
		t.Fatal(err)
	}
}
