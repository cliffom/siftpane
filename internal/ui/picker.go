package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func (m *Model) choices() []core.Conversation {
	return core.Filter(m.directory, m.picker.Value(), m.included, m.mode == "switch")
}

func (m *Model) openPicker(mode string) tea.Cmd {
	m.rememberDraft()
	m.mode = mode
	m.pickIndex = 0
	m.picker.SetValue("")
	m.composer.Blur()
	return m.picker.Focus()
}

func (m *Model) pickerKey(v tea.KeyPressMsg) tea.Cmd {
	cs := m.choices()
	switch v.String() {
	case "esc":
		m.mode = ""
		m.picker.Blur()
		return nil
	case "down", "ctrl+n":
		m.pickIndex = min(len(cs)-1, m.pickIndex+1)
		return nil
	case "up", "ctrl+p":
		m.pickIndex = max(0, m.pickIndex-1)
		return nil
	case "ctrl+l":
		if m.directoryNext != "" {
			return m.directoryCmd(m.directoryNext)
		}
		return nil
	case "ctrl+r":
		m.directoryNext = ""
		return m.directoryCmd("")
	case "tab":
		if m.mode == "pick" && len(cs) > 0 {
			m.toggle(cs[max(0, min(m.pickIndex, len(cs)-1))].ID)
		}
		return nil
	case "enter":
		if len(cs) > 0 {
			c := cs[max(0, min(m.pickIndex, len(cs)-1))]
			if !m.included[c.ID] {
				m.toggle(c.ID)
			}
			if m.included[c.ID] {
				m.mode = ""
				m.picker.Blur()
				return m.selectConversation(c.ID)
			}
		}
		return nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(v)
	m.pickIndex = 0
	return cmd
}
