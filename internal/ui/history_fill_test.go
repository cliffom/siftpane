package ui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

type fillHistoryService struct {
	core.Service
	queries []core.Query
	page    core.Page
	err     error
}

func (s *fillHistoryService) History(ctx context.Context, q core.Query) (core.Page, error) {
	s.queries = append(s.queries, q)
	if err := ctx.Err(); err != nil {
		return core.Page{}, err
	}
	return s.page, s.err
}

func fillFixture(t *testing.T) (*Model, *fillHistoryService) {
	t.Helper()
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	m.cache[m.key()] = &timeline{Loaded: true, More: true, Messages: []core.Message{{TS: futureTimestamp, Text: "newest"}}}
	m.index, m.scroll = 0, 0
	svc := &fillHistoryService{Service: m.service}
	m.service = svc
	return m, svc
}

func TestHistoryFillsTallPaneWithoutLeavingNewest(t *testing.T) {
	m, svc := fillFixture(t)
	m.composer.SetValue(mainDraft)
	_, cmd := m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{More: true, Messages: m.cache[m.key()].Messages}})
	for i := range historyFillPages {
		if cmd == nil {
			t.Fatal("stopped before page budget")
		}
		svc.page = core.Page{More: true, Messages: []core.Message{{TS: fmt.Sprintf("100.%06d", historyFillPages-i), Text: "older context"}}}
		before := m.cache[m.key()].Messages[0].TS
		_, cmd = m.Update(cmd())
		if got := svc.queries[i]; got.Before != before || got.Thread != "" || got.Conversation != m.active {
			t.Fatal("fill did not request the preceding channel page", got)
		}
		line := m.cache[m.key()]
		if m.scroll != 0 || line.Messages[m.index].TS != futureTimestamp || m.composer.Value() != mainDraft {
			t.Fatal("fill changed newest selection or draft")
		}
	}
	if cmd != nil || m.fillHistory() != nil || len(svc.queries) != historyFillPages {
		t.Fatal("automatic history exceeded its budget")
	}
}

func TestHistoryFillStopsAtFailureOrNonadvancingBoundary(t *testing.T) {
	for _, failure := range []bool{false, true} {
		m, svc := fillFixture(t)
		svc.page = core.Page{More: true, Messages: m.cache[m.key()].Messages}
		if failure {
			svc.err = errors.New("older page unavailable")
		}
		cmd := m.fillHistory()
		_, next := m.Update(cmd())
		if next != nil || m.fillHistory() != nil || len(svc.queries) != 1 {
			t.Fatal("failed or nonadvancing fill continued")
		}
		if failure && m.cache[m.key()].PageErr == nil {
			t.Fatal("automatic page failure lost explicit retry details")
		}
	}
}

func TestHistoryFillRequiresAvailableOlderHistoryAndUnfilledNewestView(t *testing.T) {
	const filledView = "full"
	for _, state := range []string{"complete", "browsing", "thread", "cached", filledView, "busy"} {
		t.Run(state, func(t *testing.T) {
			m, _ := fillFixture(t)
			line := m.cache[m.key()]
			switch state {
			case "complete":
				line.More = false
			case "browsing":
				m.scroll = -1
			case "thread":
				m.thread = futureTimestamp
			case "cached":
				line.Cached = true
			case filledView:
				m.height = 14
				m.resize()
			case "busy":
				line.Busy = true
			}
			if state == filledView {
				for range 10 {
					line.Messages = append(line.Messages, core.Message{TS: futureTimestamp, Text: "context"})
				}
			}
			if m.fillHistory() != nil {
				t.Fatal("ineligible view started an automatic page")
			}
		})
	}
}

func TestBrowsingDuringHistoryFillKeepsSelectionAndStopsContinuation(t *testing.T) {
	m, svc := fillFixture(t)
	cmd := m.fillHistory()
	m.scroll = -1
	m.focus = focusCompose
	m.composer.SetValue(mainDraft)
	svc.page = core.Page{More: true, Messages: []core.Message{{TS: "100.000001", Text: "older context"}}}
	_, next := m.Update(cmd())
	if next != nil || m.scroll != -1 || m.cache[m.key()].Messages[m.index].TS != futureTimestamp || m.focus != focusCompose || m.composer.Value() != mainDraft {
		t.Fatal("automatic page interrupted explicit browsing or composing")
	}
}
