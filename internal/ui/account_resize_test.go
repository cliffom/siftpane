package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func TestAccountResizeRunsOnlyActiveHistoryFill(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprintf("accounts=%d", count), func(t *testing.T) {
			a, services, before := resizeAccountFixture(t, count)
			m := a.current()
			m.composer.SetValue(mainDraft)
			_, cmd := a.Update(tea.WindowSizeMsg{Width: 100, Height: 55})
			if cmd == nil || !m.cache[m.key()].Busy || m.cache[m.key()].FillPages != 1 {
				t.Fatal("resize did not return its scheduled active history command")
			}
			assertResizedAccountLayouts(t, a)
			_, duplicate := a.Update(tea.WindowSizeMsg{Width: 100, Height: 55})
			if duplicate != nil {
				t.Fatal("resize duplicated the pending active page")
			}
			event, ok := cmd().(accountEvent)
			if !ok || event.index != a.active || event.epoch != a.sessions[a.active].epoch {
				t.Fatal("resize command lost account/epoch ownership")
			}
			_, next := a.Update(event)
			runAccountCommand(t, a, next)
			if m.cache[m.key()].Busy || m.cache[m.key()].Messages[m.index].TS != futureTimestamp || m.composer.Value() != mainDraft {
				t.Fatal("resize completion stranded loading or changed selection/draft")
			}
			assertResizeAccountReads(t, a, services, before)
			if m.fetch(m.key(), false) == nil {
				t.Fatal("resize completion left manual refresh blocked")
			}
		})
	}
}

func TestResizeFillResultIsRejectedAfterAccountSwitch(t *testing.T) {
	a, _ := accountFixture(t, 2)
	m := a.current()
	m.cache[m.key()] = &timeline{Loaded: true, More: true, Messages: []core.Message{{TS: futureTimestamp}}}
	_, cmd := a.Update(tea.WindowSizeMsg{Width: 100, Height: 55})
	if cmd == nil {
		t.Fatal("missing resize fill command")
	}
	m.composer.SetValue(mainDraft)
	switchAccount(t, a, 1)
	active := a.current()
	active.composer.SetValue("other account draft")
	_, next := a.Update(cmd())
	if next != nil || m.cache[m.key()].Busy || m.drafts[m.key()] != mainDraft || active.composer.Value() != "other account draft" {
		t.Fatal("late resize result affected the active account or stranded its owner")
	}
}

func resizeAccountFixture(t *testing.T, count int) (*Accounts, []*accountTestService, []int) {
	t.Helper()
	a, services := accountFixture(t, count)
	if count > 1 {
		switchAccount(t, a, 1)
		switchAccount(t, a, 0)
	}
	before := make([]int, count)
	for i := range a.sessions {
		m := a.sessions[i].model
		m.cache[m.key()] = &timeline{Loaded: true, More: true, Messages: []core.Message{{TS: futureTimestamp, Text: "resize newest"}}}
		m.index, m.scroll = 0, 0
		before[i] = services[i].reads
	}
	return a, services, before
}

func assertResizedAccountLayouts(t *testing.T, a *Accounts) {
	t.Helper()
	for i := range a.sessions {
		model := a.sessions[i].model
		if model.width != 100 || model.height != 55 {
			t.Fatal("retained account layout did not resize", i)
		}
		if i != a.active && (model.cache[model.key()].Busy || model.cache[model.key()].FillPages != 0) {
			t.Fatal("inactive resize scheduled history work", i)
		}
	}
}

func assertResizeAccountReads(t *testing.T, a *Accounts, services []*accountTestService, before []int) {
	t.Helper()
	for i, svc := range services {
		want := before[i]
		if i == a.active {
			want++
		}
		if svc.reads != want {
			t.Fatal("resize read the wrong account", i, svc.reads, want)
		}
	}
}
