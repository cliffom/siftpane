package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

func TestExplicitInclusionDuringStartup(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cached=%v", cached), func(t *testing.T) {
			m := fixture(t)
			m.Init()
			if !cached {
				m.startupSidebar = nil
			}
			before := len(m.sidebarConversations())
			m.composer.SetValue(mainDraft)
			c := startupChoice()
			m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{c}}})
			m.openPicker(pickerSwitch)
			m.picker.SetValue(c.Name)
			m.openChoice(m.choices())
			if !m.included[c.ID] || m.active != c.ID {
				t.Fatal("did not include and open")
			}
			assertPublishedChoice(t, m, c, before+1)
			m.composer.SetValue("new conversation draft")
			m.focus = focusChannels
			m.moveSelection(keyNext)
			m.moveSelection(keyPrevious)
			if m.active != c.ID || m.composer.Value() != "new conversation draft" || m.drafts[demoChannel] != mainDraft {
				t.Fatal("startup navigation lost selection or drafts")
			}
		})
	}
}

func TestExplicitOpenPublishesAlreadyIncludedStartupChoice(t *testing.T) {
	m := fixture(t)
	c := startupChoice()
	// Saved inclusion has no cached summary; its lookup is still outstanding.
	m.included[c.ID] = true
	m.addContext(c.ID)
	m.Init()
	before := len(m.sidebarConversations())
	m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{c}}})
	m.openPicker(pickerSwitch)
	m.picker.SetValue(c.Name)
	m.openChoice(m.choices())
	assertPublishedChoice(t, m, c, before+1)
}

func TestExplicitTogglePublishesWithoutOpeningOrDuplicating(t *testing.T) {
	m := fixture(t)
	m.Init()
	c := startupChoice()
	m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{c}}})
	before := len(m.sidebarConversations())
	active := m.active
	m.openPicker(pickerInclude)
	m.picker.SetValue(c.Name)
	m.pickerKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assertPublishedChoice(t, m, c, before+1)
	if m.active != active {
		t.Fatal("include changed active conversation")
	}
	m.pickerKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if len(m.sidebarConversations()) != before {
		t.Fatal("removal retained published row")
	}
	m.pickerKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assertPublishedChoice(t, m, c, before+1)
}

func startupChoice() core.Conversation {
	return core.Conversation{ID: "Cnew", Name: "newly-included", Kind: core.ConversationChannel}
}

func assertPublishedChoice(t *testing.T, m *Model, c core.Conversation, count int) {
	t.Helper()
	if !m.startupRestoring || len(m.metadataBusy) == 0 || len(m.sidebarConversations()) != count {
		t.Fatal("explicit action thawed unrelated startup rows or duplicated selection")
	}
	visible := ansi.Strip(strings.Join(m.sidebar(58, 14), "\n"))
	if !strings.Contains(visible, c.Name) {
		t.Fatal("explicit choice missing from sidebar", visible)
	}
}

func TestMetadataFailureVisibleWithFullSidebar(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m := fullRestoredSidebar(t)
		m.width, m.height = size[0], size[1]
		m.resize()
		m.focus = focusChannels
		rendered := ansi.Strip(m.View().Content)
		if !strings.Contains(rendered, m.restorationState()) || !strings.Contains(rendered, "R retry") {
			t.Fatalf("metadata failure has no visible recovery state at %v:\n%s", size, rendered)
		}
		if !strings.Contains(rendered, "▸ #design") {
			t.Fatal("recovery row displaced selection", rendered)
		}
		assertLayout(t, m, rendered, size, SnapshotStartup)
	}
}

func fullRestoredSidebar(t *testing.T) *Model {
	t.Helper()
	m := fixture(t)
	for i := 0; i < 20; i++ {
		c := core.Conversation{ID: fmt.Sprintf("Cextra%d", i), Name: fmt.Sprintf("extra-%02d", i), Kind: core.ConversationChannel}
		m.directory = append(m.directory, c)
		m.included[c.ID] = true
		m.addContext(c.ID)
	}
	m.active = m.selected()[0].ID
	m.Init()
	// Finish all restoration, with one metadata failure.
	for cid := range m.metadataBusy {
		if cid == demoDM {
			m.Update(conversationResult{ID: cid, Generation: m.generations[cid], Err: errors.New("metadata unavailable")})
		} else {
			var c core.Conversation
			for _, item := range m.directory {
				if item.ID == cid {
					c = item
					break
				}
			}
			m.Update(conversationResult{ID: cid, Generation: m.generations[cid], Conversation: c})
		}
	}
	// An ordinary successful history update replaces the transient status.
	m.line(m.active)
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: core.Page{}})
	return m
}
