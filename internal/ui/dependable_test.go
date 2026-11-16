package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func readingMessages(first, count int) []core.Message {
	var messages []core.Message
	for i := first; i < first+count; i++ {
		messages = append(messages, core.Message{TS: fmt.Sprintf("1794672000.%06d", i), Author: fmt.Sprintf("Reader %d", i), Text: fmt.Sprintf("Message %d", i)})
	}
	return messages
}

func TestReadingAnchorAcrossOlderPagesEditsAndResize(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := fixture(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			line := m.line(m.key())
			line.Messages, line.Loaded, line.More = readingMessages(20, 40), true, true
			m.index, m.scroll = 25, 40
			m.composer.SetValue(mainDraft)
			before := m.captureReadingAnchor()
			selectedTS := line.Messages[m.index].TS
			m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Page: core.Page{Messages: readingMessages(1, 19), More: true}})
			if after := m.captureReadingAnchor(); after != before || line.Messages[m.index].TS != selectedTS {
				t.Fatalf("older page moved reading/selection: %+v -> %+v", before, after)
			}
			page := readingMessages(1, 59)
			page[0].Text = strings.Repeat("edited wrapped text ", 30)
			m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: page}})
			if after := m.captureReadingAnchor(); after != before {
				t.Fatalf("edit moved reading: %+v -> %+v", before, after)
			}
			m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
			if after := m.captureReadingAnchor(); (after.ts != before.ts && m.scroll != -1) || m.composer.Value() != mainDraft || line.Messages[m.index].TS != selectedTS {
				t.Fatalf("resize lost anchor/draft: %+v -> %+v", before, after)
			}
			assertVisibleSelection(t, m, line.Messages[m.index])
		})
	}
}

func TestDeletedSelectionChoosesVisibleSuccessor(t *testing.T) {
	m := fixture(t)
	line := m.line(m.key())
	line.Messages, line.Loaded = readingMessages(1, 8), true
	m.index, m.scroll = 3, -1
	want := line.Messages[4]
	page := append(append([]core.Message{}, line.Messages[:3]...), line.Messages[4:]...)
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: page}})
	assertVisibleSelection(t, m, want)
}

func TestEvictedSelectionFallsBackToLastRetainedMessage(t *testing.T) {
	m := fixture(t)
	line := m.line(m.key())
	line.Messages, line.Loaded = readingMessages(1000, 20), true
	m.index, m.scroll = 5, -1
	page := readingMessages(1, cacheLimit)
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Page: core.Page{Messages: page, More: true}})
	want := page[len(page)-1]
	if line.Messages[m.index].TS != want.TS {
		t.Fatal("eviction did not choose nearest retained boundary", line.Messages[m.index].TS, want.TS)
	}
	assertVisibleSelection(t, m, want)
}

type cursorRecoveryService struct {
	core.Service
	queries []core.Query
	pages   []core.Page
	errors  []error
}

func (s *cursorRecoveryService) History(_ context.Context, q core.Query) (core.Page, error) {
	i := len(s.queries)
	s.queries = append(s.queries, q)
	return s.pages[i], s.errors[i]
}

func TestExpiredThreadCursorRecoversOnceAfterRetainedReply(t *testing.T) {
	for _, failRecovery := range []bool{false, true} {
		t.Run(fmt.Sprint(failRecovery), func(t *testing.T) {
			m := fixture(t)
			m.thread = attentionTestTS
			line := m.line(m.key())
			line.Messages, line.Loaded, line.More, line.Next = readingMessages(1, 3), true, true, "expired"
			last := line.Messages[2].TS
			fresh := readingMessages(4, 2)
			svc := &cursorRecoveryService{Service: m.service, pages: []core.Page{{}, {Messages: fresh}}, errors: []error{&slack.APIError{Code: "invalid_cursor"}, nil}}
			if failRecovery {
				svc.errors[1] = &slack.APIError{Code: "invalid_cursor"}
			}
			m.service = svc
			cmd := m.fetch(m.key(), true)
			m.Update(cmd())
			if len(svc.queries) != 2 || svc.queries[1].Cursor != "" || svc.queries[1].After != last {
				t.Fatal("recovery did not use retained timestamp", svc.queries)
			}
			if failRecovery {
				if len(line.Messages) != 3 || line.PageErr == nil || !line.More {
					t.Fatal("failed recovery lost partial thread")
				}
			} else if len(line.Messages) != 5 || line.More || line.PageErr != nil {
				t.Fatal("recovered page not accepted", line)
			}
		})
	}
}

func TestLargeThreadRetainsRootAndRejectsStalledPagination(t *testing.T) {
	m := fixture(t)
	m.thread = readingMessages(1, 1)[0].TS
	line := m.line(m.key())
	line.Messages, line.Loaded, line.More = readingMessages(1, cacheLimit), true, true
	m.index, m.scroll = cacheLimit-2, -1
	want := line.Messages[m.index].TS
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, QueryCursor: "same", Page: core.Page{Messages: readingMessages(cacheLimit+1, 50), More: true, Next: "same"}})
	if len(line.Messages) != cacheLimit || line.Messages[0].TS != m.thread || line.Messages[m.index].TS != want || line.Next != "" {
		t.Fatal("large thread lost root/selection/bounds", line.Next, m.index)
	}
	last := line.Messages[len(line.Messages)-1].TS
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, QueryAfter: last, Page: core.Page{More: true}})
	if !line.More || line.PageErr == nil || line.After != last || m.fetch(m.key(), false) != nil {
		t.Fatal("stalled pages were marked complete or automatically restarted")
	}
}

func TestReplyCursorCycleFallsBackToTimestamp(t *testing.T) {
	m := fixture(t)
	m.thread = attentionTestTS
	line := m.line(m.key())
	line.Messages, line.Loaded, line.More = readingMessages(1, 2), true, true
	const firstCursor = "page-a"
	for i, cursor := range []string{firstCursor, "page-b", firstCursor} {
		next := "page-b"
		if cursor == next {
			next = firstCursor
		}
		m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, QueryCursor: cursor, Page: core.Page{Messages: readingMessages(i+3, 1), More: true, Next: next}})
		if i == 1 && line.Next != "" {
			t.Fatal("cursor cycle was retained instead of timestamp continuation")
		}
	}
}

func TestScrollingKeepsActionTargetVisible(t *testing.T) {
	m := fixture(t)
	m.focus = focusMessages
	line := m.line(m.key())
	line.Messages, line.Loaded = readingMessages(1, 50), true
	m.index, m.scroll = len(line.Messages)-1, 0
	m.navigateKey(keyPageUp)
	assertVisibleSelection(t, m, line.Messages[m.index])
	m.navigateKey(keyPageDown)
	assertVisibleSelection(t, m, line.Messages[m.index])
}

func TestRefreshDetailsRetainFailureAfterOtherActionsAndOlderSuccess(t *testing.T) {
	m := fixture(t)
	err := errors.New("offline outage")
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Err: err})
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Page: core.Page{Messages: readingMessages(1, 2)}})
	m.setStatus("Marked read", statusAction)
	if !strings.Contains(strings.Join(m.refreshRows(70), "\n"), err.Error()) {
		t.Fatal("older load/action erased refresh failure")
	}
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.refreshDetails = true
		m.composer.SetValue(mainDraft)
		m.Update(tea.PasteMsg{Content: "must not enter draft"})
		rows := strings.Split(ansi.Strip(m.View().Content), "\n")
		if len(rows) != size[1] || m.composer.Value() != mainDraft {
			t.Fatal("details changed layout or draft")
		}
		for _, row := range rows {
			if ansi.StringWidth(row) != size[0] {
				t.Fatal("details overflow", row)
			}
		}
	}
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: readingMessages(1, 2)}})
	if line := m.line(m.key()); line.RecentErr != nil || time.Since(line.Checked) > time.Second {
		t.Fatal("recent success failed to clear error/establish freshness")
	}
}
