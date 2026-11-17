package ui

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/store"
)

func TestStartupRestoresKnownDMLabelsFromSummary(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), testConfigName))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T_LABEL_RESTORE", User: "U_LABEL_RESTORE"}
	channel := core.Conversation{ID: demoChannel, Name: "alpha", Kind: core.ConversationChannel}
	hint := core.Conversation{ID: demoDM, Name: "U_LABEL_TARGET", User: "U_LABEL_TARGET", Kind: core.ConversationDM, Members: 2}
	summary := hint
	summary.Name, summary.Handle, summary.Members = "Known colleague", "colleague", 1
	for _, c := range []core.Conversation{channel, hint} {
		st.AccountWorkspace(id).Included[c.ID] = true
	}
	if err := st.SaveDirectory(id, []core.Conversation{channel, hint}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveConversations(id, []core.Conversation{channel, summary}); err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), demo.New(), id, st, false)
	m.Init() // Schedule work without running commands or contacting a service.
	want := hint
	want.Name, want.Handle = summary.Name, summary.Handle
	got := core.Filter(m.directory, "@colleague", nil, false)
	if !reflect.DeepEqual(got, []core.Conversation{want}) {
		t.Fatalf("saved label is not searchable with fresh metadata: %+v", got)
	}
	if !m.directoryCached[hint.ID] || m.metadataState[hint.ID] != metadataDeferred || m.metadataBusy[hint.ID] || len(m.metadataBusy) != 1 || m.active != channel.ID {
		t.Fatal("display fallback changed cached provenance or startup scheduling")
	}
	if !reflect.DeepEqual(st.Directory(id), []core.Conversation{channel, hint}) {
		t.Fatal("restoration rewrote the directory observation")
	}
	other := id
	other.User = "U_OTHER_LABEL_RESTORE"
	st.AccountWorkspace(other).Included[hint.ID] = true
	if err := st.SaveDirectory(other, []core.Conversation{hint}); err != nil {
		t.Fatal(err)
	}
	otherModel := New(context.Background(), demo.New(), other, st, false)
	if len(core.Filter(otherModel.directory, "@colleague", nil, false)) != 0 {
		t.Fatal("restoration borrowed another account's summary")
	}
}

func TestKnownDMNameOnlyFillsUnresolvedMatchingParticipant(t *testing.T) {
	known := core.Conversation{ID: demoDM, Name: "Saved person", Handle: "saved-person", User: "U_SAVED_PERSON", Kind: core.ConversationDM}
	hint := known
	hint.Name, hint.Handle = hint.User, ""
	for _, tc := range []struct {
		name string
		edit func(*core.Conversation, *core.Conversation)
	}{
		{"resolved name wins", func(c, _ *core.Conversation) { c.Name, c.Handle = "Current person", "current-person" }},
		{"different participant", func(_, old *core.Conversation) { old.User = "U_DIFFERENT_PERSON" }},
		{"conversation ID placeholder", func(_, old *core.Conversation) { old.Name = old.ID }},
		{"participant ID placeholder", func(_, old *core.Conversation) { old.Name = old.User }},
		{"different kind", func(_, old *core.Conversation) { old.Kind = core.ConversationGroup }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, old := hint, known
			tc.edit(&current, &old)
			if got := withKnownDMName(current, old); got != current {
				t.Fatalf("ineligible fallback replaced current metadata: %+v", got)
			}
		})
	}
}
