package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func (m *Model) choices() []core.Conversation {
	return core.Filter(m.directory, m.picker.Value(), m.included, m.mode == pickerSwitch)
}

func (m *Model) openPicker(mode pickerMode) tea.Cmd {
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
	case keyEscape:
		m.mode = pickerClosed
		m.picker.Blur()
		return nil
	case keyDown, keyPickerDown:
		m.pickIndex = min(len(cs)-1, m.pickIndex+1)
		return nil
	case keyUp, keyPickerUp:
		m.pickIndex = max(0, m.pickIndex-1)
		return nil
	case keyDirectoryNext:
		if m.directoryNext != "" {
			return m.directoryCmd(m.directoryNext)
		}
		return nil
	case keyDirectoryReload:
		m.directoryNext = ""
		return m.directoryCmd("")
	case keyFocus:
		if m.mode == pickerInclude && len(cs) > 0 {
			m.toggle(cs[max(0, min(m.pickIndex, len(cs)-1))].ID)
		}
		return nil
	case keyCompose:
		if len(cs) > 0 {
			c := cs[max(0, min(m.pickIndex, len(cs)-1))]
			if !m.included[c.ID] {
				m.toggle(c.ID)
			}
			if m.included[c.ID] {
				m.mode = pickerClosed
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
