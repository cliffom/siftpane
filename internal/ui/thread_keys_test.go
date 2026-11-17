package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func TestArrowAndEnterThreadNavigationRetainsDrafts(t *testing.T) {
	const savedArrowThreadDraft = "reply text retained across arrow navigation"
	for _, open := range []rune{tea.KeyRight, tea.KeyEnter} {
		m := fixture(t)
		m.focus = focusMessages
		m.cache[m.key()].Messages = []core.Message{{TS: futureTimestamp, Replies: 2}}
		m.index = 0
		channel := m.key()
		thread := channel + "/" + futureTimestamp
		m.composer.SetValue(mainDraft)
		m.drafts[thread] = savedArrowThreadDraft
		_, cmd := m.Update(tea.KeyPressMsg{Code: open})
		if cmd == nil || m.thread != futureTimestamp || m.composer.Value() != savedArrowThreadDraft || m.drafts[channel] != mainDraft {
			t.Fatal("thread shortcut lost context or draft", open)
		}
		m.composer.SetValue("updated thread draft")
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		if m.thread != "" || m.active != channel || m.composer.Value() != mainDraft || m.drafts[thread] != "updated thread draft" {
			t.Fatal("left arrow failed to return with drafts intact")
		}
	}
}

func TestThreadArrowsAreInertWithoutRepliesOrInSidebar(t *testing.T) {
	m := fixture(t)
	m.focus = focusMessages
	m.cache[m.key()].Messages = []core.Message{{TS: futureTimestamp}}
	m.index = 0
	for _, arrow := range []rune{tea.KeyRight, tea.KeyLeft} {
		_, cmd := m.Update(tea.KeyPressMsg{Code: arrow})
		if cmd != nil || m.thread != "" || m.focus != focusMessages {
			t.Fatal("arrow changed a message without a thread")
		}
	}
	m.cache[m.key()].Messages[0].Replies = 2
	m.focus = focusChannels
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if cmd != nil || m.thread != "" || m.focus != focusChannels {
		t.Fatal("sidebar arrow opened an unrelated selected message")
	}
	m.focus = focusMessages
	m.cache[m.key()].Messages[0].Replies = 0
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focus != focusCompose || m.thread != "" {
		t.Fatal("Enter without replies did not compose")
	}
}

func TestThreadArrowsKeepComposerEditingAndHelpIsolation(t *testing.T) {
	m := fixture(t)
	m.focus = focusCompose
	m.composer.Focus()
	m.composer.SetValue("ab")
	m.composer.MoveToEnd()
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m.Update(key("X"))
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(key("Y"))
	if m.composer.Value() != "aXbY" || m.focus != focusCompose || m.thread != "" {
		t.Fatal("thread shortcuts intercepted text editing", m.composer.Value())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(m.composer.Value(), "\n") {
		t.Fatal("composer Enter lost newline behavior")
	}
	m.focus, m.help = focusMessages, true
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.thread != "" || m.help {
		t.Fatal("help did not consume the thread shortcut")
	}
}
