package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func (m *Model) choices() []core.Conversation {
	return core.Filter(m.directory, m.picker.Value(), m.included, false)
}

func (m *Model) openPicker(mode pickerMode) tea.Cmd {
	m.rememberDraft()
	m.mode = mode
	m.pickIndex = 0
	m.picker.SetValue("")
	m.composer.Blur()
	m.resize()
	return m.picker.Focus()
}

func (m *Model) closePicker() tea.Cmd {
	m.mode = pickerClosed
	m.picker.Blur()
	m.resize()
	if m.focus == focusCompose {
		return m.composer.Focus()
	}
	return nil
}

func (m *Model) pickerKey(v tea.KeyPressMsg) tea.Cmd {
	cs := m.choices()
	switch v.String() {
	case keyEscape:
		return m.closePicker()
	case keySwitch, keyCommandSwitch:
		return nil
	case keyDown, keyPickerDown:
		m.pickIndex = max(0, min(len(cs)-1, m.pickIndex+1))
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
		return m.directoryCmd("")
	case keyFocus:
		if len(cs) > 0 {
			c := cs[max(0, min(m.pickIndex, len(cs)-1))]
			if m.mode == pickerInclude {
				m.toggle(c.ID)
			} else {
				m.completeChoice(c)
			}
		}
		return nil
	case keyCompose:
		return m.openChoice(cs)
	}
	return m.updatePickerInput(v)
}

func (m *Model) openChoice(cs []core.Conversation) tea.Cmd {
	if len(cs) == 0 {
		return nil
	}
	c := cs[max(0, min(m.pickIndex, len(cs)-1))]
	if !m.included[c.ID] {
		m.toggle(c.ID)
	}
	if !m.included[c.ID] {
		return nil
	}
	m.mode = pickerClosed
	m.picker.Blur()
	m.resize()
	return m.selectConversation(c.ID)
}

func (m *Model) completeChoice(c core.Conversation) {
	m.picker.SetValue(c.Completion())
	m.picker.CursorEnd()
	m.reselectChoice(c.ID)
}

func (m *Model) pickedID() string {
	if m.mode == pickerClosed {
		return ""
	}
	cs := m.choices()
	if len(cs) == 0 {
		return ""
	}
	return cs[max(0, min(m.pickIndex, len(cs)-1))].ID
}

// Keep keyboard selection attached to identity when asynchronous metadata reorders suggestions.
func (m *Model) reselectChoice(id string) {
	if m.mode == pickerClosed || id == "" {
		return
	}
	m.pickIndex = 0
	for i, c := range m.choices() {
		if c.ID == id {
			m.pickIndex = i
			break
		}
	}
}

func (m *Model) updatePickerInput(msg tea.Msg) tea.Cmd {
	previous := m.picker.Value()
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if m.picker.Value() != previous {
		m.pickIndex = 0
	}
	return cmd
}
