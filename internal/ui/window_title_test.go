package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func assertWindowTitle(t *testing.T, m *Model, want string) {
	t.Helper()
	if got := m.View().WindowTitle; got != want {
		t.Fatalf("window title = %q, want %q", got, want)
	}
}

func TestWindowTitleLocalActivityTransitions(t *testing.T) {
	m := fixture(t)
	clear(m.unread)
	assertWindowTitle(t, m, normalWindowTitle)

	// Exercise the normal asynchronous history path, with a known local read
	// watermark so this is new activity rather than a first-observation baseline.
	m.prefs.Read[demoDM] = "0.0"
	m.line(demoDM)
	m.Update(historyResult{Key: demoDM, Generation: m.generations[demoDM], Page: core.Page{
		Messages: []core.Message{{TS: futureTimestamp, Text: "new"}},
	}})
	assertWindowTitle(t, m, unreadWindowTitle)

	m.focus = focusCompose
	m.observeActivity(demoChannel, futureTimestamp)
	assertWindowTitle(t, m, unreadWindowTitle)

	m.selectConversation(demoDM)
	assertWindowTitle(t, m, unreadWindowTitle)
	m.selectConversation(demoChannel)
	assertWindowTitle(t, m, normalWindowTitle)

	m.focus = focusCompose
	m.observeActivity(demoChannel, futureTimestamp+"1")
	assertWindowTitle(t, m, unreadWindowTitle)
	m.Update(key(keyEscape))
	m.Update(key(keyRead))
	assertWindowTitle(t, m, normalWindowTitle)
}

func TestWindowTitleRemovalAndExcludedActivity(t *testing.T) {
	m := fixture(t)
	assertWindowTitle(t, m, unreadWindowTitle) // Demo seeds two unread conversations.
	m.toggle(demoDM)
	assertWindowTitle(t, m, unreadWindowTitle)
	m.toggle("Cshipping")
	assertWindowTitle(t, m, normalWindowTitle)

	// Excluded metadata (including explicit false entries) cannot contribute,
	// even if an unread entry survives in memory.
	m.unread[demoDM] = true
	m.unread["Cshipping"] = true
	m.included["Cshipping"] = false
	m.unread[demoChannel] = false
	assertWindowTitle(t, m, normalWindowTitle)
}

func TestWindowTitleAcrossViewsAndResize(t *testing.T) {
	for _, mode := range []SnapshotMode{SnapshotMessages, SnapshotCompose, SnapshotThread, SnapshotPicker, SnapshotSearch, SnapshotHelp} {
		for _, size := range [][2]int{{100, 30}, {60, 20}, {MinWidth - 1, MinHeight - 1}} {
			t.Run(fmt.Sprint(mode, size), func(t *testing.T) {
				m := fixture(t)
				Snapshot(m, size[0], size[1], mode)
				assertWindowTitle(t, m, unreadWindowTitle)
				for cid := range m.unread {
					m.read(cid)
				}
				assertWindowTitle(t, m, normalWindowTitle)
				m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
				assertWindowTitle(t, m, normalWindowTitle)
			})
		}
	}
}

const (
	normalWindowTitle = "Siftpane"
	unreadWindowTitle = "[unread] Siftpane"
)
