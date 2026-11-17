package ui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

const (
	threadScenarioExpired    = "expired"
	threadScenarioRepeating  = "repeating"
	threadScenarioMissing    = "missing"
	threadScenarioComplete   = "complete"
	threadScenarioFailure    = "recovery-failure"
	threadInvalidCursorError = "invalid_cursor"
)

type threadFrontierService struct {
	core.Service
	queries  []core.Query
	scenario string
}

func (s *threadFrontierService) History(_ context.Context, q core.Query) (core.Page, error) {
	s.queries = append(s.queries, q)
	if q.Cursor != "" {
		if s.scenario == threadScenarioExpired || s.scenario == threadScenarioFailure {
			return core.Page{}, &slack.APIError{Code: threadInvalidCursorError}
		}
		next := ""
		if s.scenario == threadScenarioRepeating {
			next = q.Cursor
		}
		return core.Page{Messages: readingMessages(5, 2), More: true, Next: next}, nil
	}
	if s.scenario == threadScenarioFailure {
		return core.Page{}, errors.New("offline recovery failure")
	}
	// The send at 100 is visible locally before the intervening replies are read.
	all := append(readingMessages(5, 4), readingMessages(100, 1)...)
	var messages []core.Message
	for _, msg := range all {
		if msg.TS > q.After {
			messages = append(messages, msg)
		}
	}
	return core.Page{Messages: messages}, nil
}

func TestThreadContinuationDoesNotSkipRepliesAfterLocalSend(t *testing.T) {
	for _, scenario := range []string{threadScenarioExpired, threadScenarioRepeating, threadScenarioMissing, threadScenarioComplete, threadScenarioFailure} {
		for _, size := range [][2]int{{100, 30}, {60, 20}} {
			t.Run(fmt.Sprintf("%s/%dx%d", scenario, size[0], size[1]), func(t *testing.T) {
				m, selected, sent := threadWithAcknowledgedSend(t, size, scenario != threadScenarioComplete)
				line := m.line(m.key())
				svc := &threadFrontierService{Service: m.service, scenario: scenario}
				m.service = svc
				cmd := m.fetch(m.key(), scenario != threadScenarioComplete)
				if cmd == nil {
					t.Fatal("missing thread continuation")
				}
				m.Update(cmd())
				wantAfter := readingMessages(4, 1)[0].TS
				if scenario == threadScenarioRepeating || scenario == threadScenarioMissing {
					wantAfter = readingMessages(6, 1)[0].TS
					cmd = m.fetch(m.key(), true)
					if cmd == nil {
						t.Fatal("missing timestamp continuation")
					}
					m.Update(cmd())
				}
				if got := svc.queries[len(svc.queries)-1]; got.Cursor != "" || got.After != wantAfter {
					t.Fatalf("continuation used cached send instead of fetched page: %+v; want After=%s", got, wantAfter)
				}
				assertThreadContinuationResult(t, line, svc)
				assertThreadContains(t, line, sent.TS)
				assertVisibleSelection(t, m, selected)
				if m.composer.Value() != mainDraft || m.drafts[m.key()] != mainDraft || m.deliveries[m.key()].State != core.DeliverySent {
					t.Fatal("continuation changed draft or acknowledged delivery")
				}
			})
		}
	}
}

func assertThreadContains(t *testing.T, line *timeline, ts string) {
	t.Helper()
	for _, msg := range line.Messages {
		if msg.TS == ts {
			return
		}
	}
	t.Fatal("thread lost message", ts)
}

func threadWithAcknowledgedSend(t *testing.T, size [2]int, more bool) (*Model, core.Message, core.Message) {
	t.Helper()
	m := fixture(t)
	m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
	m.thread = readingMessages(1, 1)[0].TS
	line := m.line(m.key())
	cursor := ""
	if more {
		cursor = "page-1"
	}
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: readingMessages(1, 4), More: more, Next: cursor}})
	m.index, m.scroll, m.focus = 2, -1, focusMessages
	selected := line.Messages[m.index]
	m.composer.SetValue(mainDraft)
	m.rememberDraft()
	sent := readingMessages(100, 1)[0]
	m.deliveries[m.key()] = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Thread: m.thread, Text: sent.Text}, State: core.DeliveryPending}
	m.Update(sendResult{Key: m.key(), Message: sent})
	return m, selected, sent
}

func assertThreadContinuationResult(t *testing.T, line *timeline, svc *threadFrontierService) {
	t.Helper()
	if svc.scenario == threadScenarioFailure {
		if !line.More || line.PageErr == nil || len(svc.queries) != 2 {
			t.Fatal("failed recovery lost incomplete state or exceeded one retry")
		}
	} else {
		for _, msg := range readingMessages(5, 4) {
			assertThreadContains(t, line, msg.TS)
		}
		if line.More {
			t.Fatal("terminal fetched page remained incomplete")
		}
	}
}

func TestThreadFetchedBoundarySurvivesFailureAndResetsWithContext(t *testing.T) {
	m, _, sent := threadWithAcknowledgedSend(t, [2]int{100, 30}, true)
	line := m.line(m.key())
	boundary := readingMessages(4, 1)[0].TS
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Err: errors.New("offline page failure")})
	if line.ReplyThrough != boundary || !line.More {
		t.Fatal("failed read advanced or discarded fetched progress")
	}
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Page: core.Page{More: true, Next: "empty-page-next"}})
	if line.ReplyThrough != boundary {
		t.Fatal("empty fetched page advanced past unread replies")
	}
	assertThreadContains(t, line, sent.TS)
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Older: true, Page: core.Page{Messages: readingMessages(5, 2)}})
	if line.ReplyThrough != readingMessages(6, 1)[0].TS {
		t.Fatal("accepted page did not advance fetched progress independently of send")
	}
	target := readingMessages(2, 1)[0].TS
	m.openTargets[m.key()] = target
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Target: target, QueryAfter: target, Page: core.Page{Messages: readingMessages(2, 2)}})
	if line.ReplyThrough != readingMessages(3, 1)[0].TS {
		t.Fatal("anchored replacement retained a later window's boundary")
	}
	m.purgeConversationCache(m.active, &slack.APIError{Code: "access_denied"})
	if line.ReplyThrough != "" {
		t.Fatal("access purge retained a discarded window's boundary")
	}
}
