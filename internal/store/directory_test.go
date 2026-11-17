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

func TestDirectoryCachePrivacyIsolationAndAge(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "Tcache", User: "Ucache"}
	c := core.Conversation{ID: "Ccache", Name: "cross-pod", Kind: core.ConversationChannel, Topic: "never persist topic", Purpose: "never persist purpose"}
	if err = s.SaveDirectory(id, []core.Conversation{c}); err != nil {
		t.Fatal(err)
	}
	path := s.cachePath(id, directoryCacheKind)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "never persist") {
		t.Fatal("cache leaked non-summary content")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("cache permissions", info, err)
	}
	var saved directoryFile
	if err = json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-directoryMaxAge + time.Hour).UTC()
	saved.Entries[0].Checked = old
	writeDirectoryFixture(t, path, saved)
	if err = s.SaveDirectory(id, []core.Conversation{{ID: "Dcache", Name: "Other", Kind: core.ConversationDM}}); err != nil {
		t.Fatal(err)
	}
	entries := s.directoryEntries(id)
	if len(entries) != 2 || !entries[1].Checked.Equal(old) {
		t.Fatal("saving another page renewed old entries", entries)
	}
	fresh, err := Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Directory(id)) != 2 || len(fresh.Workspace(id.Workspace).Included) != 0 {
		t.Fatal("cache lost unkept suggestions or restored inclusion")
	}
	if len(fresh.Directory(core.Identity{Workspace: id.Workspace, User: "another"})) != 0 || len(fresh.Directory(core.Identity{Workspace: "another", User: id.User})) != 0 {
		t.Fatal("cache crossed accounts")
	}
	for _, age := range []time.Duration{directoryMaxAge + time.Second, -time.Hour} {
		saved.Entries[0].Checked = time.Now().Add(-age)
		writeDirectoryFixture(t, path, saved)
		if len(s.Directory(id)) != 0 {
			t.Fatal("expired/future entry accepted")
		}
	}
}

func TestDirectoryCacheBoundsAndRemoval(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "Tbounds", User: "Ubounds"}
	cs := []core.Conversation{}
	for i := range directoryEntries + 1 {
		cs = append(cs, core.Conversation{ID: fmt.Sprintf("C%05d", i), Name: strings.Repeat("界", directoryTextRunes), Handle: strings.Repeat("界", directoryTextRunes), Kind: core.ConversationChannel})
	}
	if err = s.SaveDirectory(id, cs); err != nil {
		t.Fatal(err)
	}
	got := s.Directory(id)
	b, err := os.ReadFile(s.cachePath(id, directoryCacheKind))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || len(got) > directoryEntries || len(b) > directoryBytes {
		t.Fatal("cache resource bounds", len(got), len(b))
	}
	before := s.directoryEntries(id)
	if err = s.RemoveDirectory(id, got[0].ID); err != nil {
		t.Fatal(err)
	}
	after := s.directoryEntries(id)
	if len(after) != len(before)-1 || !after[0].Checked.Equal(before[1].Checked) {
		t.Fatal("removal changed other entries")
	}
	for _, bad := range []string{"not json", `{"version":99}`, strings.Repeat("x", directoryBytes+1)} {
		if err = os.WriteFile(s.cachePath(id, directoryCacheKind), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if len(s.Directory(id)) != 0 {
			t.Fatal("invalid cache accepted")
		}
	}
}

func writeDirectoryFixture(t *testing.T, path string, v directoryFile) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}
