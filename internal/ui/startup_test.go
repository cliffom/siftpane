package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/store"
)

func TestStartupMetadataOrderAndEarlyHistory(t *testing.T) {
	cs := []core.Conversation{
		{ID: demoDM, Name: demoAuthor, Kind: core.ConversationDM},
		{ID: demoChannel, Name: "studio", Kind: core.ConversationChannel},
		{ID: demoGroup, Name: "crew", Kind: core.ConversationGroup},
		{ID: "Cprivate", Name: "private", Kind: core.ConversationPrivate},
	}
	want := []string{"Cprivate", demoChannel, demoGroup, demoDM}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}} {
		st := &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}
		for _, c := range cs {
			st.Workspace(startupWorkspace).Included[c.ID] = true
		}
		m := New(context.Background(), demo.New(), core.Identity{Workspace: startupWorkspace}, st, false)
		m.Init() // Mark asynchronous work pending; do not run the timer or directory.
		for step, index := range order {
			c := cs[index]
			_, cmd := m.Update(conversationResult{ID: c.ID, Conversation: c, Generation: m.generations[c.ID]})
			if step == 0 {
				if cmd == nil || m.active != c.ID || !m.cache[c.ID].Busy {
					t.Fatal("first available history waited for metadata batch")
				}
				m.composer.SetValue(mainDraft)
			}
			if m.active != cs[order[0]].ID || m.composer.Value() != mainDraft {
				t.Fatal("metadata changed selection or draft")
			}
			sidebar := ansi.Strip(strings.Join(m.sidebar(60, 30), "\n"))
			if step < len(order)-1 && len(m.sidebarConversations()) != 1 {
				t.Fatal("uncached sidebar did not retain only its usable active conversation")
			}
			if strings.Count(sidebar, "KEPT") > 1 || strings.Count(sidebar, "NEEDS ATTENTION") > 1 {
				t.Fatal(sidebar)
			}
		}
		got := []string{}
		for _, c := range m.selected() {
			got = append(got, c.ID)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatal(want, got)
		}
	}
}

func TestCachedStartupLoadsHistoryBeforeMetadataAndKeepsIdentity(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), testConfigName))
	if err != nil {
		t.Fatal(err)
	}
	svc := demo.New()
	id, err := svc.Identity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st.Workspace(id.Workspace).Included[demoChannel] = true
	c, err := svc.Conversation(context.Background(), demoChannel)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveConversations(id, []core.Conversation{c, {ID: "excluded", Name: "Not included", Kind: core.ConversationDM}}); err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), svc, id, st, false)
	if len(m.selected()) != 1 || m.active != demoChannel || m.directoryLoaded {
		t.Fatal("cache unavailable or claims complete directory")
	}
	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("missing startup batch")
	}
	// History is scheduled alongside metadata; no metadata result is needed.
	m.Update(batch[len(batch)-1]())
	if !m.cache[demoChannel].Loaded || len(m.metadataBusy) != 1 {
		t.Fatal("history gated on metadata")
	}
	m.focus = focusCompose
	m.composer.SetValue(mainDraft)
	c.Name = "renamed"
	m.Update(conversationResult{ID: c.ID, Conversation: c, Generation: m.generations[c.ID]})
	if m.active != c.ID || m.composer.Value() != mainDraft || m.focus != focusCompose || m.selected()[0].Name != "renamed" {
		t.Fatal("refresh lost identity/draft")
	}
	m.toggle(c.ID)
	if len(st.Conversations(id)) != 0 {
		t.Fatal("removed conversation retained in metadata cache")
	}
}

func TestStartupFailuresRemainVisibleAndRetry(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Busy: true}
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Err: errors.New("offline history")})
	rows := strings.Join(m.history(60, 12), "\n")
	if !strings.Contains(rows, "Could not load messages") || strings.Contains(rows, "Loading messages") {
		t.Fatal(rows)
	}
	// Unrelated success must not erase this timeline's recovery state.
	m.Update(historyResult{Key: demoDM, Generation: m.generations[demoDM], Page: core.Page{}})
	if !strings.Contains(strings.Join(m.history(60, 12), "\n"), "offline history") {
		t.Fatal("error hidden")
	}
	m.Update(key(keyRefresh))
	if !m.cache[m.key()].Busy || m.cache[m.key()].Err != nil {
		t.Fatal("retry did not restart loading")
	}
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{}})
	if !strings.Contains(strings.Join(m.history(60, 12), "\n"), "No messages yet") {
		t.Fatal("empty result shown as loading")
	}
	m.Update(conversationResult{ID: demoDM, Generation: m.generations[demoDM], Err: errors.New("offline metadata")})
	if !strings.Contains(m.restorationState(), "Unavailable") {
		t.Fatal("metadata failure hidden")
	}
	m.Update(key(keyRefresh))
	if !m.metadataBusy[demoDM] || m.metadataFailed[demoDM] {
		t.Fatal("metadata retry not scheduled")
	}
	old := m.generations[demoDM]
	m.toggle(demoDM)
	m.toggle(demoDM)
	m.conversationCmd(demoDM)
	m.Update(conversationResult{ID: demoDM, Generation: old, Err: errors.New("late")})
	if !m.metadataBusy[demoDM] || m.metadataFailed[demoDM] {
		t.Fatal("late result changed current restoration")
	}
}

func TestColdStartupAllMetadataFailsAndRecovers(t *testing.T) {
	st := &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}
	st.Workspace(startupWorkspace).Included[demoChannel] = true
	m := New(context.Background(), demo.New(), core.Identity{Workspace: startupWorkspace}, st, false)
	m.Init()
	m.Update(conversationResult{ID: demoChannel, Generation: m.generations[demoChannel], Err: errors.New("metadata unavailable")})
	if m.startupRestoring || !strings.Contains(strings.Join(m.history(60, 12), "\n"), "Unavailable") {
		t.Fatal("cold failure stayed loading")
	}
	_, cmd := m.Update(key(keyRefresh))
	if cmd == nil {
		t.Fatal("missing metadata retry")
	}
	_, historyCmd := m.Update(cmd())
	if historyCmd == nil || m.active != demoChannel {
		t.Fatal("recovery did not select available history")
	}
	m.Update(historyCmd())
	if !m.cache[demoChannel].Loaded || len(m.metadataFailed) != 0 {
		t.Fatal("recovery failed")
	}
}

func TestCachedSidebarStaysStableWhileMetadataChanges(t *testing.T) {
	m := fixture(t)
	m.Init()
	before := m.sidebarConversations()
	c := before[0]
	c.Name = "renamed-last"
	m.Update(conversationResult{ID: c.ID, Conversation: c, Generation: m.generations[c.ID]})
	if !reflect.DeepEqual(before, m.sidebarConversations()) {
		t.Fatal("partial metadata reshuffled sidebar")
	}
	// Removing outstanding conversations must allow restoration to settle, even
	// though their canceled results may never arrive.
	for cid := range m.metadataBusy {
		m.toggle(cid)
	}
	if m.startupRestoring || len(m.sidebarConversations()) != 1 || m.sidebarConversations()[0].Name != c.Name {
		t.Fatal("removal stranded startup publication")
	}
}

const startupWorkspace = "startup-workspace"

func TestStoppedRestorationShowsRestartInsteadOfRetry(t *testing.T) {
	m := fixture(t)
	m.active = ""
	m.stopped = true
	m.metadataFailed[demoChannel] = true
	rows := strings.Join(m.history(60, 12), "\n")
	if !strings.Contains(rows, "restart") || strings.Contains(rows, "R retry") {
		t.Fatal(rows)
	}
}

func TestLargeCachedStartupDefersInactiveMetadata(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), testConfigName))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: startupWorkspace, User: "Ucached"}
	cs := make([]core.Conversation, selectionLimit)
	for i := range cs {
		cid := fmt.Sprintf("C%03d", i)
		cs[i] = core.Conversation{ID: cid, Name: cid, Kind: core.ConversationChannel}
		st.AccountWorkspace(id).Included[cid] = true
	}
	if err := st.SaveConversations(id, cs); err != nil {
		t.Fatal(err)
	}
	uncached := New(context.Background(), demo.New(), id, st, false)
	uncached.Init()
	if len(uncached.metadataBusy) != selectionLimit {
		t.Fatal("old summaries without fresh observations skipped validation")
	}
	if err := st.SaveDirectory(id, cs); err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), demo.New(), id, st, false)
	m.Init()
	if len(m.metadataBusy) != 1 || !m.metadataBusy[m.active] || len(m.metadataState) != selectionLimit {
		t.Fatal("cached startup launched inactive metadata reads", m.metadataBusy)
	}
	for _, c := range cs {
		if c.ID != m.active && m.metadataState[c.ID] != metadataDeferred {
			t.Fatal("inactive cached metadata was not deferred", c.ID)
		}
	}
	if len(m.sidebarConversations()) != selectionLimit || !m.cache[m.key()].Busy {
		t.Fatal("cached sidebar or immediate message request unavailable")
	}
	m.composer.SetValue(mainDraft)
	old := m.active
	target := cs[len(cs)-1].ID
	m.selectConversation(target)
	if m.drafts[old] != mainDraft || !m.metadataBusy[target] || m.metadataState[target] == metadataDeferred || !m.cache[target].Busy {
		t.Fatal("opening deferred metadata blocked history or lost the draft")
	}
}
