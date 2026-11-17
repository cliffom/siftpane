package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	ambiguousSearchQuery          = "CLOUD2025"
	ambiguousSearchConversationID = "Cname"
)

func directorySearchFixture(t *testing.T) (*Model, *searchService, func() directoryResult) {
	t.Helper()
	m := fixture(t)
	s := &searchService{Service: m.service}
	m.service = s
	m.directory = nil
	m.directoryLoaded = false
	m.directoryNext = ""
	cmd := m.directoryCmd("") // Startup may still be pending when the picker opens.
	m.openPicker(pickerInclude)
	return m, s, func() directoryResult {
		t.Helper()
		result, ok := cmd().(directoryResult)
		if !ok {
			t.Fatal("directory command returned wrong result type")
		}
		return result
	}
}

func TestDirectorySearchLoadsUnseenMatch(t *testing.T) {
	m, s, first := directorySearchFixture(t)
	m.picker.SetValue("#cross-team")
	m.composer.SetValue("retained draft")
	s.page = core.DirectoryPage{Next: searchPageCursor}
	_, next := m.Update(first())
	if next == nil {
		t.Fatal("empty advancing page stopped search")
	}
	s.page = core.DirectoryPage{Next: "third"}
	_, next = m.Update(next())
	if next == nil {
		t.Fatal("second page stopped search")
	}
	s.page = core.DirectoryPage{Conversations: []core.Conversation{{ID: "Ccross", Name: "cross-team", Kind: core.ConversationChannel}}}
	_, next = m.Update(next())
	if next != nil || m.directoryBusy || m.directoryNext != "" || len(m.choices()) != 1 {
		t.Fatal("unloaded match was not discovered to completion")
	}
	if m.included["Ccross"] || m.contexts["Ccross"] != nil || m.composer.Value() != "retained draft" {
		t.Fatal("directory search changed inclusion or draft")
	}
	if got := strings.Join(s.cursors, ","); got != ",next,third" {
		t.Fatal("unexpected requests", got)
	}
}

func TestDirectorySearchCancellationAndLateResults(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		m, s, first := directorySearchFixture(t)
		m.closePicker()
		result := first()
		if !errors.Is(result.Err, context.Canceled) || len(s.cursors) != 0 {
			t.Fatal("closed picker started a queued read")
		}
	})
	t.Run("delayed page", func(t *testing.T) {
		m, s, first := directorySearchFixture(t)
		s.page = core.DirectoryPage{Next: "obsolete", Conversations: []core.Conversation{{ID: "Cold", Name: "old"}}}
		late := first()
		m.closePicker()
		m.openPicker(pickerInclude)
		_, cmd := m.Update(late)
		if cmd != nil || !m.directoryBusy || len(m.directory) != 0 || m.directoryLoaded || m.directoryNext != "" {
			t.Fatal("old result changed reopened picker")
		}
	})
	t.Run("open partial", func(t *testing.T) {
		m := fixture(t)
		m.directoryNext = "unvisited-page"
		m.openPicker(pickerSwitch)
		if !m.directoryBusy {
			t.Fatal("opening picker did not resume directory")
		}
		m.closePicker()
		if m.directoryBusy {
			t.Fatal("closing picker left loading active")
		}
	})
}

func TestDirectorySearchBoundsAndCursorCycle(t *testing.T) {
	t.Run("budget", func(t *testing.T) {
		m, s, first := directorySearchFixture(t)
		s.page = core.DirectoryPage{Next: "page-0"}
		_, next := m.Update(first())
		for i := range directoryAutoPages {
			if next == nil {
				t.Fatal("search stopped before budget", i)
			}
			s.page = core.DirectoryPage{Next: fmt.Sprintf("page-%d", i+1)}
			_, next = m.Update(next())
		}
		if next != nil || m.directoryBusy || !strings.Contains(m.directoryState(), "paused") {
			t.Fatal("search exceeded budget or hid incomplete coverage")
		}
		if len(m.directorySeen) > directoryAutoPages+1 {
			t.Fatal("cursor history exceeded scan bound")
		}
		next = m.pickerKey(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
		if next == nil {
			t.Fatal("explicit continuation did not resume")
		}
		s.page = core.DirectoryPage{}
		_, next = m.Update(next())
		if next != nil || m.directoryNext != "" {
			t.Fatal("continuation did not finish")
		}
	})
	t.Run("cycle", func(t *testing.T) {
		m, s, first := directorySearchFixture(t)
		s.page = core.DirectoryPage{Next: searchPageCursor}
		_, next := m.Update(first())
		s.page = core.DirectoryPage{Next: searchPageCursor, Conversations: []core.Conversation{{ID: "Cuseful", Name: "useful"}}}
		_, next = m.Update(next())
		if next != nil || m.directoryErr == nil || m.directoryNext == "" || len(m.directory) != 1 {
			t.Fatal("cursor cycle looped or lost useful partial results")
		}
	})
}

func TestDirectorySearchFindsColdUppercaseName(t *testing.T) {
	m, s, first := directorySearchFixture(t)
	m.Update(tea.PasteMsg{Content: ambiguousSearchQuery})
	if !m.directoryBusy {
		t.Fatal("ambiguous name canceled pending directory page")
	}
	s.page = core.DirectoryPage{Next: searchPageCursor}
	_, next := m.Update(first())
	if next == nil {
		t.Fatal("ambiguous name stopped automatic paging")
	}
	s.page = core.DirectoryPage{Conversations: []core.Conversation{{ID: ambiguousSearchConversationID, Name: "cloud2025", Kind: core.ConversationChannel}}}
	m.Update(next())
	if len(m.choices()) != 1 {
		t.Fatal("cold uppercase name was not discovered")
	}
	m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.directBusy || m.active != ambiguousSearchConversationID || m.mode != pickerClosed {
		t.Fatal("discovered name did not take precedence over ID shorthand")
	}
}

func TestAmbiguousSearchAllowsExplicitContinuation(t *testing.T) {
	for _, code := range []rune{'l', 'r'} {
		t.Run(string(code), func(t *testing.T) {
			m, _, _ := directorySearchFixture(t)
			m.cancelDirectory()
			m.directoryLoaded = true
			m.directoryNext = searchPageCursor
			m.directoryPagesLeft = 0
			m.picker.SetValue(ambiguousSearchQuery)
			if cmd := m.pickerKey(tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}); cmd == nil || !m.directoryBusy {
				t.Fatal("ambiguous query blocked directory continuation/reload")
			}
		})
	}
}
