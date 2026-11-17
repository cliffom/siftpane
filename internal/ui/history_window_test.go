package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	testContinuationLabel = "continued"
	testNewestBody        = "newest body"
)

func TestNewestHistoryUsesBottomOfPane(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {100, 55}} {
		m := fixture(t)
		m.width, m.height = size[0], size[1]
		m.resize()
		m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{{TS: futureTimestamp, User: "reviewer", Text: "newest message"}}}
		m.index, m.scroll = 0, 0
		_, w, h, _, _ := m.dimensions()
		rows := m.history(w, h)
		if len(rows) != h || strings.TrimSpace(ansi.Strip(rows[h-2])) != "│  newest message" || strings.TrimSpace(rows[0]) != "" {
			t.Fatalf("newest message not above the composer at %v:\n%s", size, strings.Join(rows, "\n"))
		}
		// Explicit selection-following keeps the existing browsing position.
		m.scroll = -1
		rows = m.history(w, h)
		if !strings.Contains(ansi.Strip(rows[0]), "reviewer") {
			t.Fatal("browsing was bottom-aligned")
		}
	}
}

func TestNewestWindowDropsSeparatorOnlyContinuation(t *testing.T) {
	rows := []string{"older author", "older body", "", "newest author", testNewestBody, ""}
	got := historyWindow(rows, []int{0, 3}, 3, 6, 40, 5, 0)
	if strings.Contains(strings.Join(got, "\n"), "older author") || strings.TrimSpace(got[3]) != testNewestBody {
		t.Fatal("separator-only continuation remained or newest content was clipped", got)
	}
}

func TestNewestWindowDoesNotDiscardLongMessageThatFitsPane(t *testing.T) {
	rows := []string{"long author"}
	for range 10 {
		rows = append(rows, "long body")
	}
	rows = append(rows, "", "recent author", "recent body", "", "newest author", testNewestBody, "")
	for _, scroll := range []int{0, -1, 2} {
		got := historyWindow(rows, []int{0, 12, 15}, 15, 18, 60, 14, scroll)
		if !strings.Contains(got[0], "long author") || !strings.Contains(got[0], testContinuationLabel) {
			t.Fatal("long preceding message was replaced with blank space", scroll, got)
		}
		if strings.TrimSpace(got[1]) != "long body" {
			t.Fatal("viewport discarded preceding content", scroll, got)
		}
		if scroll <= 0 && strings.TrimSpace(got[12]) != testNewestBody {
			t.Fatal("viewport clipped the selected newest tail", scroll, got)
		}
	}
}

func TestNavigatingToNewestFillsTallViewportWithLoadedContext(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	_, width, height, _, _ := m.dimensions()
	m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{
		{TS: "1794589100.000001", Author: "Earlier author", Text: strings.Repeat("Loaded earlier context.\n", height-4)},
		{TS: "1794589200.000001", Author: "Recent author", Text: "Recent update."},
		{TS: "1794589300.000001", Author: "Newest author", Text: "Newest update."},
	}}
	m.index, m.scroll = 1, 0
	m.focus = focusMessages
	m.composer.SetValue(mainDraft)
	m.Update(key(keyNext))
	rows := m.history(width, height)
	if !strings.Contains(ansi.Strip(rows[0]), testContinuationLabel) || !strings.Contains(ansi.Strip(rows[1]), "Loaded earlier context.") || !strings.Contains(ansi.Strip(rows[height-2]), "Newest update.") {
		t.Fatal("selection-following hid loaded history or clipped newest", strings.Join(rows, "\n"))
	}
	if m.index != 2 || m.scroll == 0 || m.composer.Value() != mainDraft {
		t.Fatal("rendering changed selection, browsing mode or draft")
	}
}
