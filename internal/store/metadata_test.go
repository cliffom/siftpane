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

func TestMetadataPersistencePrivacyAndIsolation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "prefs", testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T1", User: "U1"}
	c := core.Conversation{ID: "C1", Name: "general", Kind: core.ConversationChannel, Topic: "private topic", Purpose: "private purpose"}
	if err := s.SaveConversations(id, []core.Conversation{c}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.metadataPath(id))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "private") {
		t.Fatal("non-summary text persisted")
	}
	info, err := os.Stat(s.metadataPath(id))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatal(info.Mode())
	}
	r, err := Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Conversations(id); len(got) != 1 || got[0].Name != c.Name || got[0].Topic != "" {
		t.Fatal(got)
	}
	for _, other := range []core.Identity{{Workspace: "T2", User: "U1"}, {Workspace: "T1", User: "U2"}} {
		if len(r.Conversations(other)) != 0 {
			t.Fatal("cross-account metadata")
		}
	}
	if len(r.Workspace(id.Workspace).Included) != 0 {
		t.Fatal("cache changed inclusion")
	}
}

func TestMetadataInvalidDataFallsBackWithoutChangingPreferences(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T1"}
	s.Workspace(id.Workspace).Included["C1"] = true
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"corrupt", []byte("broken")},
		{"oversized", []byte(strings.Repeat(" ", metadataBytes+1))},
		{"old schema", metadataWithEntry(t, metadataVersion+1, time.Now())},
		{"expired", metadataWithEntry(t, metadataVersion, time.Now().Add(-metadataMaxAge-time.Hour))},
		{"future", metadataWithEntry(t, metadataVersion, time.Now().Add(time.Hour))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(s.metadataPath(id), tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got := s.Conversations(id); len(got) != 0 {
				t.Fatal(got)
			}
			r, err := Open(s.Path)
			if err != nil || !r.Workspace(id.Workspace).Included["C1"] {
				t.Fatal("cache broke preferences", err)
			}
		})
	}
}

func TestMetadataBoundsAndValidation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T1"}
	cs := []core.Conversation{{ID: "invalid-kind", Name: "Unknown kind"}}
	for i := 0; i < metadataEntries+1; i++ {
		cs = append(cs, core.Conversation{ID: fmt.Sprint(i), Name: "channel", Kind: core.ConversationChannel})
	}
	if err := s.SaveConversations(id, cs); err != nil {
		t.Fatal(err)
	}
	if got := s.Conversations(id); len(got) != metadataEntries {
		t.Fatal(len(got))
	}
	cache := metadataFile{Version: metadataVersion, Saved: time.Now(), Conversations: []conversationSummary{
		{ID: "C1", Name: "one", Kind: core.ConversationChannel},
		{ID: "C1", Name: "duplicate", Kind: core.ConversationDM},
		{ID: "", Name: "missing", Kind: core.ConversationChannel},
		{ID: "C2", Name: "unknown"},
	}}
	if err := os.WriteFile(s.metadataPath(id), metadataJSON(t, cache), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.Conversations(id); len(got) != 1 || got[0].Name != "one" {
		t.Fatal(got)
	}
	cache.Conversations = make([]conversationSummary, metadataEntries+1)
	if err := os.WriteFile(s.metadataPath(id), metadataJSON(t, cache), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(s.Conversations(id)) != 0 {
		t.Fatal("oversized entry count accepted")
	}
	if err := (&Store{}).SaveConversations(id, cs); err != nil {
		t.Fatal(err)
	}
}

func metadataJSON(t *testing.T, c metadataFile) []byte {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func metadataWithEntry(t *testing.T, version int, saved time.Time) []byte {
	t.Helper()
	return metadataJSON(t, metadataFile{Version: version, Saved: saved, Conversations: []conversationSummary{{ID: "C1", Name: "general", Kind: core.ConversationChannel}}})
}
