package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

const pollThread = "100.000001"

type pollingService struct {
	core.Service
	queries []core.Query
}

func (s *pollingService) History(ctx context.Context, q core.Query) (core.Page, error) {
	s.queries = append(s.queries, q)
	return core.Page{Messages: []core.Message{{TS: pollThread}}}, ctx.Err()
}

func pollingFixture(t *testing.T, count int) (*Model, *pollingService) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	svc := &pollingService{}
	m := New(ctx, svc, core.Identity{Workspace: "polling"}, &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}, false)
	for i := 0; i < count; i++ {
		cid := fmt.Sprintf("C%d", i)
		m.directory = append(m.directory, core.Conversation{ID: cid, Name: cid, Kind: core.ConversationChannel})
		m.included[cid] = true
		m.addContext(cid)
	}
	// Loaded metadata for an excluded conversation must never cause polling.
	m.directory = append(m.directory, core.Conversation{ID: "excluded", Kind: core.ConversationChannel})
	if count > 0 {
		m.active = m.directory[0].ID
		m.focus = focusMessages
		m.line(m.active).Loaded = true
		m.line(m.active).Messages = []core.Message{{TS: pollThread}}
	}
	return m, svc
}

// pollCommand delivers a synthetic tick and extracts only its history command.
// The timer command is deliberately never executed, so tests need no sleeps.
func pollCommand(t *testing.T, m *Model) tea.Cmd {
	t.Helper()
	busy := map[string]bool{}
	for k, line := range m.cache {
		busy[k] = line.Busy
	}
	_, cmd := m.Update(tickMsg(time.Time{}))
	started := 0
	for k, line := range m.cache {
		if line.Busy && !busy[k] {
			started++
		}
	}
	if started == 0 {
		return nil
	}
	if started != 1 {
		t.Fatalf("tick started %d timeline requests", started)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected timer and one history command, got %T", batch)
	}
	return batch[1]
}

func assertPoll(t *testing.T, m *Model, svc *pollingService, want string) {
	t.Helper()
	before := len(svc.queries)
	cmd := pollCommand(t, m)
	if want == "" {
		if cmd != nil {
			t.Fatal("unexpected timeline request")
		}
		return
	}
	if cmd == nil {
		t.Fatalf("no request for %s", want)
	}
	m.Update(cmd())
	if len(svc.queries) != before+1 {
		t.Fatal("tick did not perform exactly one history request")
	}
	cid, thread := split(want)
	if got := svc.queries[before]; got.Conversation != cid || got.Thread != thread || got.Cursor != "" || got.Before != "" {
		t.Fatalf("query = %+v, want timeline %s", got, want)
	}
}

func TestPollingRotation(t *testing.T) {
	for _, count := range []int{1, 2, 3, 4, 5} {
		for _, threadOpen := range []bool{false, true} {
			t.Run(fmt.Sprintf("conversations=%d/thread=%t", count, threadOpen), func(t *testing.T) {
				m, svc := pollingFixture(t, count)
				if threadOpen {
					m.thread = pollThread
				}
				// Two full conversation rotations, including even and odd counts.
				for i := 0; i < 2*count; i++ {
					assertPoll(t, m, svc, m.directory[i%count].ID)
					if threadOpen {
						assertPoll(t, m, svc, m.key())
					}
				}
			})
		}
	}
}

func TestPollingThreadOpenCloseRetainsNextConversation(t *testing.T) {
	for _, initialTicks := range []int{0, 1} {
		t.Run(fmt.Sprintf("initialTicks=%d", initialTicks), func(t *testing.T) {
			m, svc := pollingFixture(t, 2)
			for i := 0; i < initialTicks; i++ {
				assertPoll(t, m, svc, m.directory[i].ID)
			}
			// Exercise the actual navigation handlers, including their immediate reads.
			_, cmd := m.Update(key(keyThread))
			if cmd == nil || m.thread != pollThread {
				t.Fatal("thread did not open")
			}
			m.Update(cmd())
			if initialTicks == 1 {
				assertPoll(t, m, svc, m.key())
			}
			assertPoll(t, m, svc, m.directory[initialTicks].ID)
			assertPoll(t, m, svc, m.key())
			_, cmd = m.Update(key(keyEscape))
			if cmd == nil || m.thread != "" {
				t.Fatal("thread did not close")
			}
			m.Update(cmd())
			assertPoll(t, m, svc, m.directory[(initialTicks+1)%2].ID)
			assertPoll(t, m, svc, m.directory[initialTicks].ID)
		})
	}
}

func TestPollingSkippedTurnsDoNotIssueReplacementRequests(t *testing.T) {
	for _, threadOpen := range []bool{false, true} {
		t.Run(fmt.Sprintf("thread=%t", threadOpen), func(t *testing.T) {
			m, svc := pollingFixture(t, 2)
			if threadOpen {
				m.thread = pollThread
			}
			m.scroll = -1 // Browsing older messages pauses only the active timeline.
			if threadOpen {
				assertPoll(t, m, svc, m.active)
				assertPoll(t, m, svc, "")
			} else {
				assertPoll(t, m, svc, "")
			}
			assertPoll(t, m, svc, m.directory[1].ID)
			m.scroll = 0
			if threadOpen {
				assertPoll(t, m, svc, m.key())
			}
			// Leave the next conversation request in flight across another rotation.
			pending := pollCommand(t, m)
			if pending == nil {
				t.Fatal("missing pending request")
			}
			for i := 0; i < 2; i++ {
				if threadOpen {
					assertPoll(t, m, svc, m.key())
				}
				if i == 0 {
					assertPoll(t, m, svc, m.directory[1].ID)
				} else {
					assertPoll(t, m, svc, "")
				}
			}
			m.Update(pending())
			if m.cache[m.active].Busy {
				t.Fatal("completed request remains busy")
			}
		})
	}
}

func TestPollingRemovalCancelsPendingAndRejectsLateResult(t *testing.T) {
	m, svc := pollingFixture(t, 2)
	pending := pollCommand(t, m)
	cid := m.active
	ctx := m.contexts[cid]
	m.toggle(cid)
	if ctx.Err() != context.Canceled {
		t.Fatal("pending poll context was not canceled")
	}
	result := pending()
	m.toggle(cid)
	m.Update(result)
	if m.cache[cid] != nil || m.unread[cid] {
		t.Fatal("late poll affected re-included conversation")
	}
	assertPoll(t, m, svc, m.directory[1].ID)
}

func TestPollingBusyThreadLeavesConversationRotationIntact(t *testing.T) {
	m, svc := pollingFixture(t, 2)
	m.thread = pollThread
	assertPoll(t, m, svc, m.active)
	pending := pollCommand(t, m)
	if pending == nil {
		t.Fatal("missing pending thread request")
	}
	assertPoll(t, m, svc, m.directory[1].ID)
	assertPoll(t, m, svc, "")
	assertPoll(t, m, svc, m.active)
	m.Update(pending())
	assertPoll(t, m, svc, m.key())
	assertPoll(t, m, svc, m.directory[1].ID)
}

func TestPollingStoppedOrEmpty(t *testing.T) {
	for _, count := range []int{0, 2} {
		m, svc := pollingFixture(t, count)
		m.stopped = count > 0
		assertPoll(t, m, svc, "")
		if len(svc.queries) != 0 || m.pollCount != 0 || m.pollIndex != 0 {
			t.Fatal("disabled polling advanced scheduler or called service")
		}
	}
}
