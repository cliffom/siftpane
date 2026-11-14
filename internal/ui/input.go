package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) quitKey() tea.Cmd {
	m.rememberDraft()
	for _, s := range m.drafts {
		if s != "" {
			m.setStatus("Unsent drafts in memory. Use "+keyLabel(keyQuit)+" twice to exit.", statusAction)
			return nil
		}
	}
	for _, d := range m.deliveries {
		if d.State == core.DeliveryPending || d.State == core.DeliveryFailed {
			m.setStatus("Unresolved send. Use "+keyLabel(keyQuit)+" twice to exit.", statusAction)
			return nil
		}
	}
	return tea.Quit
}

func (m *Model) updateKey(v tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := v.String()
	if m.help {
		return m, m.helpKey(k)
	}
	// The next explicit key dismisses transient action priority; handlers can
	// publish fresh feedback without losing the underlying delivery text.
	m.statusKind = statusRoutine
	if k == keyHelpGlobal {
		m.quitConfirm = false
		m.retryConfirm = false
		m.help, m.helpScroll = true, 0
		return m, nil
	}
	if k != keyRetry {
		m.retryConfirm = false
	}
	if v.Key().IsRepeat && (k == keyRetry || k == keySend || k == keyQuit) {
		return m, nil
	}
	if k == keyQuit {
		return m, m.confirmQuit()
	}
	m.quitConfirm = false
	if m.mode != pickerClosed {
		return m, m.pickerKey(v)
	}
	if isSearchKey(k) {
		return m, m.openPicker(pickerSwitch)
	}
	if m.focus == focusCompose {
		if k == keyEscape {
			m.rememberDraft()
			m.focus = focusMessages
			m.composer.Blur()
			return m, nil
		}
		if k == keySend {
			m.rememberDraft()
			return m, m.send(false)
		}
		if d := m.deliveries[m.key()]; d != nil && d.State == core.DeliveryPending {
			m.setStatus("Sending… composer locked until acknowledgement; text kept", statusAction)
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
	case keyQuitQuiet:
		return m, m.quitKey()
	case keyHelp:
		m.help, m.helpScroll = true, 0
	case keyInclude:
		return m, m.openPicker(pickerInclude)
	case keySearch:
		return m, m.openPicker(pickerSwitch)
	case keyFocus:
		if m.focus == focusChannels {
			m.focus = focusMessages
		} else {
			m.focus = focusChannels
		}
	case keyWrite, keyCompose:
		if m.active != "" {
			m.focus = focusCompose
			return m, m.composer.Focus()
		}
	case keyEscape:
		if m.thread != "" {
			m.rememberDraft()
			m.cancelProfileLookup()
			m.thread = ""
			m.scroll = 0
			m.restoreDraft()
			return m, tea.Batch(m.fetch(m.key(), false), m.profileCmd())
		}
		m.focus = focusChannels
	case keyNext, keyDown, keyPrevious, keyUp:
		return m, m.moveSelection(k)
	case keyPageUp:
		m.scroll = max(0, m.scroll) + max(minScrollRows, m.height/scrollPageDivisor)
	case keyPageDown:
		m.scroll = max(0, m.scroll-max(minScrollRows, m.height/scrollPageDivisor))
		if m.scroll == 0 {
			m.read(m.active)
		}
	case keyLast, keyNewest:
		m.scroll = 0
		t := m.line(m.key())
		m.index = max(0, len(t.Messages)-1)
		m.read(m.active)
	case keyOlder:
		return m, m.fetch(m.key(), true)
	case keyRefresh:
		return m, m.refresh()
	case keyRead:
		m.read(m.active)
		m.setStatus("Marked read locally; Slack read position unchanged", statusAction)
	case keyThread:
		return m, m.openThread()
	case keyCopy:
		return m, m.copyMessage()
	case keyRetry:
		return m, m.send(true)
	case keyEditFailed:
		m.restoreFailedDraft()
	}
	return m, nil
}

func (m *Model) moveSelection(k string) tea.Cmd {
	delta := 1
	if k == keyPrevious || k == keyUp {
		delta = -1
	}
	if m.focus == focusChannels {
		cs := m.sidebarConversations()
		if len(cs) > 0 {
			idx := 0
			for i, c := range cs {
				if c.ID == m.active {
					idx = i
				}
			}
			idx = (idx + delta + len(cs)) % len(cs)
			cmd := m.selectConversation(cs[idx].ID)
			m.focus = focusChannels
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
			m.cancelProfileLookup()
			m.thread = t.Messages[min(m.index, len(t.Messages)-1)].TS
			m.scroll = 0
			m.index = 0
			m.restoreDraft()
			return tea.Batch(m.fetch(m.key(), false), m.profileCmd())
		}
	}
	return nil
}

func (m *Model) copyMessage() tea.Cmd {
	t := m.line(m.key())
	if len(t.Messages) > 0 {
		m.setStatus("Copied message via OSC52 (terminal support required)", statusAction)
		return tea.SetClipboard(slack.Body(slack.WithProfiles(t.Messages[min(m.index, len(t.Messages)-1)], m.profiles)))
	}
	return nil
}

func (m *Model) restoreFailedDraft() {
	if d := m.deliveries[m.key()]; d != nil && d.State == core.DeliveryFailed {
		s := m.composer.Value()
		if s != "" && s != d.Text {
			s += "\n" + d.Text
		} else {
			s = d.Text
		}
		if len([]rune(s)) > m.composer.CharLimit || strings.Count(s, "\n") >= m.composer.MaxHeight {
			m.setStatus("Cannot combine drafts without truncation. Copy your current draft first; failed body retained.", statusAction)
			return
		}
		m.composer.SetValue(s)
		m.rememberDraft()
		delete(m.deliveries, m.key())
		m.setStatus("Restored failed text for editing; uncertain sends may already exist in Slack", statusAction)
	}
}

func (m *Model) confirmQuit() tea.Cmd {
	m.rememberDraft()
	dirty := false
	for _, s := range m.drafts {
		if s != "" {
			dirty = true
		}
	}
	for _, d := range m.deliveries {
		if d.State == core.DeliveryPending || d.State == core.DeliveryFailed {
			dirty = true
		}
	}
	if dirty && !m.quitConfirm {
		m.quitConfirm = true
		m.setStatus("Unsent text is in memory. "+keyLabel(keyQuit)+" again exits and discards it; any other key cancels.", statusAction)
		return nil
	}
	return tea.Quit
}
