package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const expectedHistoryLoading = "Loading messages…"

func TestHistoryLoadingUsesStatusBar(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m := fixture(t)
		m.focus = focusMessages
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.cache[m.key()] = &timeline{Busy: true}
		assertStatusBar(t, m, expectedHistoryLoading)
		if strings.TrimSpace(strings.Join(m.history(60, 12), "")) != "" {
			t.Fatal("loading text remains inside messages")
		}
		// A different timeline completing must not replace selected loading progress.
		m.line(demoDM)
		m.Update(historyResult{Key: demoDM, Generation: m.generations[demoDM], Page: core.Page{}})
		assertStatusBar(t, m, expectedHistoryLoading)
		m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Err: errors.New("history unavailable")})
		assertStatusBar(t, m, "history unavailable")
		if !strings.Contains(strings.Join(m.history(60, 12), "\n"), "Could not load messages") {
			t.Fatal("error recovery disappeared")
		}
		m.fetch(m.key(), false)
		assertStatusBar(t, m, expectedHistoryLoading)
		m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{}})
		assertStatusBar(t, m, "Refreshed")
		if !strings.Contains(strings.Join(m.history(60, 12), "\n"), "No messages yet") {
			t.Fatal("empty result indistinguishable from loading")
		}
		m.fetch(m.key(), false)
		assertStatusBar(t, m, "Refreshing messages…")
	}
}

func TestStatusProgressFollowsSelectionAndThread(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Busy: true}
	m.thread = "123.456"
	m.cache[m.key()] = &timeline{Loaded: true}
	m.status = "Ready"
	assertStatusBar(t, m, "Ready")
	m.cache[m.key()].Busy = true
	assertStatusBar(t, m, "Refreshing messages…")
	m.thread = ""
	assertStatusBar(t, m, expectedHistoryLoading)
	m.active = ""
	m.metadataBusy[demoChannel] = true
	assertStatusBar(t, m, "Loading 1 saved conversations…")
	if strings.TrimSpace(strings.Join(m.history(60, 12), "")) != "" {
		t.Fatal("startup loading duplicated in message pane")
	}
	if strings.Contains(strings.Join(m.sidebar(23, 20), ""), "Loading") {
		t.Fatal("startup progress still duplicated in sidebar")
	}
}

func TestProgressDoesNotHideRecovery(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Busy: true}
	for _, state := range []core.DeliveryState{core.DeliveryPending, core.DeliveryFailed} {
		m.deliveries[m.key()] = &core.Delivery{State: state}
		m.status = "Delivery needs attention"
		assertStatusBar(t, m, m.status)
	}
	delete(m.deliveries, m.key())
	m.persistErr = "disk unavailable"
	assertStatusBar(t, m, "Preferences not saved")
	m.quitConfirm = true
	assertStatusBar(t, m, "Unsent text")
	m.quitConfirm = false
	m.persistErr = ""
	m.stopped = true
	m.status = "Credentials expired"
	assertStatusBar(t, m, m.status)
}

func assertStatusBar(t *testing.T, m *Model, want string) {
	t.Helper()
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	if !strings.HasPrefix(rows[len(rows)-1], " "+want) {
		t.Fatalf("status bar %q; want %q", rows[len(rows)-1], want)
	}
}

func TestActionWarningsOverrideTimelineProgress(t *testing.T) {
	for _, state := range []string{"initial-page-pending", "refreshing", "failed"} {
		t.Run(state, func(t *testing.T) {
			m := fixture(t)
			m.focus = focusMessages
			m.composer.SetValue(mainDraft)
			switch state {
			case "initial-page-pending":
				m.cache[m.key()] = &timeline{Busy: true}
			case "refreshing":
				m.cache[m.key()].Busy = true
			case "failed":
				m.cache[m.key()].Err = errors.New("network unavailable")
			}
			_, cmd := m.Update(key(keyQuitQuiet))
			if cmd != nil {
				t.Fatal("quit discarded unsent text")
			}
			assertStatusBar(t, m, "Unsent drafts")
			m.Update(key(keyRead))
			assertStatusBar(t, m, "Marked read locally")
			// Another navigation key restores timeline progress/error priority.
			m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if m.statusKind != statusRoutine || m.activityStatus() == m.status {
				t.Fatal("action priority outlived the next explicit key")
			}
		})
	}
}

func TestQuitRecoveryInstructionFitsMinimumViewport(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		for _, unsent := range []string{"draft", "delivery"} {
			m := fixture(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.focus = focusMessages
			if unsent == "draft" {
				m.composer.SetValue(mainDraft)
			} else {
				m.deliveries[m.key()] = &core.Delivery{State: core.DeliveryFailed}
			}
			_, cmd := m.Update(key(keyQuitQuiet))
			if cmd != nil {
				t.Fatal("quit discarded unsent work")
			}
			rows := strings.Split(ansi.Strip(m.View().Content), "\n")
			footer := rows[len(rows)-1]
			if !strings.Contains(footer, keyLabel(keyQuit)+" twice") || !strings.Contains(footer, "exit") || strings.Contains(footer, "…") {
				t.Fatalf("%s at %v: recovery instruction clipped: %q", unsent, size, footer)
			}
			if ansi.StringWidth(footer) != size[0] {
				t.Fatal("status bar changed viewport width")
			}
		}
	}
}
