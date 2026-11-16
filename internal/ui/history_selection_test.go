package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const selectionNewestText = "Newest visible selection"

type selectionHistoryService struct {
	core.Service
	page core.Page
}

func (s *selectionHistoryService) History(ctx context.Context, _ core.Query) (core.Page, error) {
	return s.page, ctx.Err()
}

func selectionPage(first core.Message) core.Page {
	messages := []core.Message{first}
	for i := range 12 {
		messages = append(messages, core.Message{TS: fmt.Sprintf("1794672100.%06d", i+1), Author: "New author", Text: fmt.Sprintf("incoming message %d", i)})
	}
	messages[len(messages)-1].Text = selectionNewestText
	return core.Page{Messages: messages}
}

func assertVisibleSelection(t *testing.T, m *Model, want core.Message) {
	t.Helper()
	target, ok := m.messageTarget()
	if !ok || target.TS != want.TS {
		t.Fatal("action target differs from selected message", target, want.TS)
	}
	_, width, height, _, _ := m.dimensions()
	visible := ansi.Strip(strings.Join(m.history(width, height), "\n"))
	if !strings.Contains(visible, "▸ "+want.Author) || !strings.Contains(visible, want.Text) {
		t.Fatal("selected action target is offscreen", want.TS, visible)
	}
}

func TestNewestRefreshKeepsVisibleActionTarget(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}} {
		for _, accounts := range []int{1, 2} {
			t.Run(fmt.Sprint(size, accounts), func(t *testing.T) {
				a, _ := accountFixture(t, accounts)
				m := a.current()
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m.focus, m.scroll = focusMessages, 0
				page := selectionPage(m.line(m.key()).Messages[0])
				m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: page})
				newest := page.Messages[len(page.Messages)-1]
				assertVisibleSelection(t, m, newest)
				svc := &participationTestService{Service: m.service}
				m.service = svc
				cmd := m.toggleReaction(core.ReactionAcknowledged)
				if cmd == nil {
					t.Fatal("missing reaction command")
				}
				m.Update(cmd())
				if svc.target.TS != newest.TS {
					t.Fatal("reaction targeted hidden older message", svc.target)
				}
				cmd = m.openSlack()
				if cmd == nil {
					t.Fatal("missing permalink command")
				}
				m.Update(cmd()) // The fake returns an error so no browser can launch.
				if svc.target.TS != newest.TS {
					t.Fatal("open in Slack targeted hidden older message", svc.target)
				}
				m.openThread()
				if m.thread != newest.TS {
					t.Fatal("thread targeted hidden older message", m.thread)
				}
			})
		}
	}
}

func TestExplicitSelectionSurvivesRefresh(t *testing.T) {
	a, _ := accountFixture(t, 1)
	m := a.current()
	m.focus, m.scroll = focusMessages, -1
	m.composer.SetValue(mainDraft)
	old := m.line(m.key()).Messages[0]
	old.Author = "Selected author"
	old.Text = "Selected older message"
	m.line(m.key()).Messages[0] = old
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: selectionPage(old)})
	assertVisibleSelection(t, m, old)
	if m.scroll != -1 || m.focus != focusMessages || m.composer.Value() != mainDraft {
		t.Fatal("refresh changed explicit browsing context")
	}
	m.navigateKey(keyLast)
	if m.scroll != 0 {
		t.Fatal("newest action did not resume following")
	}
	page := selectionPage(old)
	newest := page.Messages[len(page.Messages)-1]
	assertVisibleSelection(t, m, newest)
}

func TestAccountActivationRespectsNewestAndExplicitSelection(t *testing.T) {
	for _, scroll := range []int{0, -1} {
		t.Run(fmt.Sprint(scroll), func(t *testing.T) {
			a, _ := accountFixture(t, 2)
			m := a.current()
			a.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
			m.focus, m.scroll = focusMessages, scroll
			m.composer.SetValue(mainDraft)
			old := m.line(m.key()).Messages[0]
			old.Author, old.Text = "Retained author", "Retained selection"
			m.line(m.key()).Messages[0] = old
			page := selectionPage(old)
			m.service = &selectionHistoryService{Service: m.service, page: page}
			switchAccount(t, a, 1)
			switchAccount(t, a, 0)
			want := old
			if scroll == 0 {
				want = page.Messages[len(page.Messages)-1]
			}
			assertVisibleSelection(t, m, want)
			if m.scroll != scroll || m.focus != focusMessages || m.composer.Value() != mainDraft {
				t.Fatal("activation changed restored context")
			}
		})
	}
}
