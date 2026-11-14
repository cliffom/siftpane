package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) quitKey() tea.Cmd {
	m.rememberDraft()
	for _, s := range m.drafts {
		if s != "" {
			m.status = "Unsent drafts in memory. Use Ctrl+C twice to exit."
			return nil
		}
	}
	for _, d := range m.deliveries {
		if d.State == "pending" || d.State == "failed" {
			m.status = "Unresolved send. Use Ctrl+C twice to exit."
			return nil
		}
	}
	return tea.Quit
}

func (m *Model) updateKey(v tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := v.String()
	if k != "r" {
		m.retryConfirm = false
	}
	if v.Key().IsRepeat && (k == "r" || k == "ctrl+s" || k == "ctrl+c") {
		return m, nil
	}
	if k == "ctrl+c" {
		m.rememberDraft()
		dirty := false
		for _, s := range m.drafts {
			if s != "" {
				dirty = true
			}
		}
		for _, d := range m.deliveries {
			if d.State == "pending" || d.State == "failed" {
				dirty = true
			}
		}
		if dirty && !m.quitConfirm {
			m.quitConfirm = true
			m.status = "Unsent text is in memory. Ctrl+C again exits and discards it; any other key cancels."
			return m, nil
		}
		return m, tea.Quit
	}
	m.quitConfirm = false
	if m.help {
		m.help = false
		return m, nil
	}
	if m.mode != "" {
		return m, m.pickerKey(v)
	}
	if k == "ctrl+k" {
		return m, m.openPicker("switch")
	}
	if m.focus == "compose" {
		if k == "esc" {
			m.rememberDraft()
			m.focus = "messages"
			m.composer.Blur()
			return m, nil
		}
		if k == "ctrl+s" {
			m.rememberDraft()
			return m, m.send(false)
		}
		if d := m.deliveries[m.key()]; d != nil && d.State == "pending" {
			m.status = "Sending… composer locked until acknowledgement; text kept"
			return m, nil
		}
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(v)
		m.rememberDraft()
		return m, cmd
	}
	return m.navigateKey(k)
}

func (m *Model) navigateKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "q":
		return m, m.quitKey()
	case "?":
		m.help = true
	case "p":
		return m, m.openPicker("pick")
	case "/":
		return m, m.openPicker("switch")
	case "tab":
		if m.focus == "channels" {
			m.focus = "messages"
		} else {
			m.focus = "channels"
		}
	case "i", "enter":
		if m.active != "" {
			m.focus = "compose"
			return m, m.composer.Focus()
		}
	case "esc":
		if m.thread != "" {
			m.rememberDraft()
			m.thread = ""
			m.scroll = 0
			m.restoreDraft()
			return m, m.fetch(m.key(), false)
		}
		m.focus = "channels"
	case "j", "down", "k", "up":
		return m, m.moveSelection(k)
	case "pgup":
		m.scroll = max(0, m.scroll) + max(3, m.height/3)
	case "pgdown":
		m.scroll = max(0, m.scroll-max(3, m.height/3))
		if m.scroll == 0 {
			m.read(m.active)
		}
	case "G", "end":
		m.scroll = 0
		t := m.line(m.key())
		m.index = max(0, len(t.Messages)-1)
		m.read(m.active)
	case "o":
		return m, m.fetch(m.key(), true)
	case "R":
		return m, m.fetch(m.key(), false)
	case "m":
		m.read(m.active)
		m.status = "Marked read locally; Slack read position unchanged"
	case "t":
		return m, m.openThread()
	case "y":
		return m, m.copyMessage()
	case "r":
		return m, m.send(true)
	case "x":
		m.restoreFailedDraft()
	}
	return m, nil
}

func (m *Model) moveSelection(k string) tea.Cmd {
	delta := 1
	if k == "k" || k == "up" {
		delta = -1
	}
	if m.focus == "channels" {
		cs := m.selected()
		if len(cs) > 0 {
			idx := 0
			for i, c := range cs {
				if c.ID == m.active {
					idx = i
				}
			}
			idx = (idx + delta + len(cs)) % len(cs)
			cmd := m.selectConversation(cs[idx].ID)
			m.focus = "channels"
			return cmd
		}
	} else {
		t := m.line(m.key())
		m.index = max(0, min(len(t.Messages)-1, m.index+delta))
		m.scroll = -1
	}
	return nil
}

func (m *Model) openThread() tea.Cmd {
	if m.active != "" && m.thread == "" {
		t := m.line(m.key())
		if len(t.Messages) > 0 {
			m.rememberDraft()
			m.thread = t.Messages[min(m.index, len(t.Messages)-1)].TS
			m.scroll = 0
			m.index = 0
			m.restoreDraft()
			return m.fetch(m.key(), false)
		}
	}
	return nil
}

func (m *Model) copyMessage() tea.Cmd {
	t := m.line(m.key())
	if len(t.Messages) > 0 {
		m.status = "Copied message via OSC52 (terminal support required)"
		return tea.SetClipboard(slack.Body(t.Messages[min(m.index, len(t.Messages)-1)]))
	}
	return nil
}

func (m *Model) restoreFailedDraft() {
	if d := m.deliveries[m.key()]; d != nil && d.State == "failed" {
		s := m.composer.Value()
		if s != "" && s != d.Text {
			s += "\n" + d.Text
		} else {
			s = d.Text
		}
		if len([]rune(s)) > m.composer.CharLimit || strings.Count(s, "\n") >= m.composer.MaxHeight {
			m.status = "Cannot combine drafts without truncation. Copy your current draft first; failed body retained."
			return
		}
		m.composer.SetValue(s)
		m.rememberDraft()
		delete(m.deliveries, m.key())
		m.status = "Restored failed text for editing; uncertain sends may already exist in Slack"
	}
}
