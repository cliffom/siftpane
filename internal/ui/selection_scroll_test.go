package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

func selectionViewport(t *testing.T, m *Model) (int, bool) {
	t.Helper()
	_, width, height, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(m.cache[m.key()], width)
	view := m.historyViewport(rows, starts, first, last, width, height)
	return view.start, view.continuation != ""
}

func TestCursorMovesWithinVisibleMessagesBeforeScrolling(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	_, _, height, _, _ := m.dimensions()
	messages := []core.Message{{TS: "100.000002", Author: "Earlier", Text: strings.Repeat("Earlier context.\n", height-12)}}
	for i := range 4 {
		messages = append(messages, core.Message{TS: fmt.Sprintf("200.%06d", i), Author: fmt.Sprintf("Recent %d", i), Text: "A visible update."})
	}
	m.cache[m.key()] = &timeline{Loaded: true, Messages: messages}
	m.index, m.scroll, m.focus = len(messages)-1, 0, focusMessages
	m.composer.SetValue(mainDraft)
	start, continued := selectionViewport(t, m)
	for i := range 3 {
		m.Update(key(keyPrevious))
		after, continuation := selectionViewport(t, m)
		if after != start || continuation != continued || m.index != len(messages)-2-i {
			t.Fatal("visible cursor movement scrolled the viewport", i, start, after)
		}
	}
	if m.scroll != scrollPinnedNewest || m.composer.Value() != mainDraft {
		t.Fatal("visible navigation did not pause following or retain the draft")
	}
	m.Update(key(keyPrevious))
	if after, _ := selectionViewport(t, m); after >= start || m.index != 0 {
		t.Fatal("offscreen message header was not revealed", start, after)
	}
	pinnedStart, _ := selectionViewport(t, m)
	for range 3 {
		m.Update(key(keyNext))
		if after, _ := selectionViewport(t, m); after != pinnedStart {
			t.Fatal("visible downward navigation scrolled the viewport", pinnedStart, after)
		}
	}
	m.Update(key(keyNext))
	if after, _ := selectionViewport(t, m); after <= pinnedStart || m.index != len(messages)-1 {
		t.Fatal("downward edge did not reveal the newest message", pinnedStart, after)
	}
}

func TestPinnedSelectionRetainsViewportAcrossIncomingHistory(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 30
	m.resize()
	messages := readingMessages(1, 20)
	m.cache[m.key()] = &timeline{Loaded: true, Messages: messages}
	m.index, m.scroll, m.focus = len(messages)-1, 0, focusMessages
	m.Update(key(keyPrevious))
	before := m.captureReadingAnchor()
	selected := m.cache[m.key()].Messages[m.index].TS
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: readingMessages(1, 21)}})
	if after := m.captureReadingAnchor(); after != before || m.cache[m.key()].Messages[m.index].TS != selected {
		t.Fatal("incoming history moved the pinned viewport or selection", before, after)
	}
}

func TestCursorRevealsOversizedMessageAtItsHeader(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{
		{TS: "100.000002", Author: "Long author", Text: strings.Repeat("Long message.\n", 60)},
		{TS: "200.000001", Author: "Newest author", Text: "Recent context."},
	}}
	m.index, m.scroll, m.focus = 1, 0, focusMessages
	m.Update(key(keyPrevious))
	if start, continued := selectionViewport(t, m); start != 0 || continued || m.index != 0 {
		t.Fatal("oversized selection did not reveal its own header", start, continued)
	}
}
