package ui

import (
	"errors"

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
	m.directErr = nil
	m.composer.Blur()
	m.resize()
	m.directoryPagesLeft = directoryAutoPages
	m.directorySeen = make(map[string]bool)
	m.directoryProfileAttempted = make(map[string]bool)
	return tea.Batch(m.picker.Focus(), m.continueDirectory(), m.directoryProfileCmd())
}

func (m *Model) closePicker() tea.Cmd {
	m.mode = pickerClosed
	m.cancelDirectory()
	m.picker.Blur()
	m.resize()
	if m.focus == focusCompose {
		return m.composer.Focus()
	}
	return nil
}

func (m *Model) pickerKey(v tea.KeyPressMsg) tea.Cmd {
	if m.directBusy && v.String() != keyEscape {
		return nil
	}
	cs := m.choices()
	switch v.String() {
	case keyTemporary:
		if cmd, handled := m.directPickerAction(directTemporary); handled {
			return cmd
		}
		return m.temporaryChoice(cs)
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
	case keyDirectoryNext, keyDirectoryReload:
		return m.pickerDirectoryKey(v.String())
	case keyFocus:
		if m.mode == pickerInclude {
			if cmd, handled := m.directPickerAction(directKeep); handled {
				return cmd
			}
		}
		if len(cs) > 0 {
			c := cs[max(0, min(m.pickIndex, len(cs)-1))]
			if m.mode == pickerInclude {
				if m.directoryCached[c.ID] && !m.attention.Kept[c.ID] {
					return m.resolveConversation(c.ID, directKeep)
				}
				m.toggle(c.ID)
			} else {
				m.completeChoice(c)
			}
		}
		return nil
	case keyCompose:
		if cmd, handled := m.directPickerAction(directOpen); handled {
			return cmd
		}
		return m.openChoice(cs)
	}
	return m.updatePickerInput(v)
}

func (m *Model) pickerDirectoryKey(k string) tea.Cmd {
	if _, direct, _ := m.explicitPickerTarget(); direct || m.directoryBusy {
		return nil
	}
	if k == keyDirectoryReload {
		m.cancelDirectory()
		m.directoryPagesLeft = directoryAutoPages
		m.directorySeen = make(map[string]bool)
		m.directoryProfileAttempted = make(map[string]bool)
		return tea.Batch(m.directoryCmd(""), m.directoryProfileCmd())
	}
	if m.directoryPagesLeft == 0 {
		m.directorySeen = make(map[string]bool)
	}
	m.directoryPagesLeft = directoryAutoPages
	m.directoryErr = nil
	return m.continueDirectory()
}

func (m *Model) openChoice(cs []core.Conversation) tea.Cmd {
	if len(cs) == 0 {
		return nil
	}
	c := cs[max(0, min(m.pickIndex, len(cs)-1))]
	if m.directoryCached[c.ID] {
		return m.resolveConversation(c.ID, directOpen)
	}
	if !m.included[c.ID] {
		m.toggle(c.ID)
	}
	if !m.included[c.ID] {
		return nil
	}
	m.publishStartupConversation(c.ID)
	m.mode = pickerClosed
	m.cancelDirectory()
	m.picker.Blur()
	m.resize()
	return m.selectConversation(c.ID)
}

// continueDirectory expands search coverage only while its picker is open.
// A page budget and cursor history bound automatic work even for bad pagination.
func (m *Model) continueDirectory() tea.Cmd {
	if _, recognized, _ := m.explicitPickerTarget(); recognized {
		return nil
	}
	if m.mode == pickerClosed || m.directoryBusy || m.stopped || m.directoryErr != nil || m.directoryPagesLeft == 0 {
		return nil
	}
	if m.directoryLoaded && m.directoryNext == "" {
		return nil
	}
	cursor := m.directoryNext
	if m.directorySeen[cursor] {
		m.directoryErr = errors.New("directory cursor repeated; Ctrl+R reload")
		return nil
	}
	return m.directoryCmd(cursor)
}

func (m *Model) cancelDirectory() {
	if m.directCancel != nil {
		m.directCancel()
		m.directCancel = nil
	}
	m.directBusy = false
	if m.directoryCancel != nil {
		m.directoryCancel()
		m.directoryCancel = nil
	}
	if m.directoryProfileCancel != nil {
		m.directoryProfileCancel()
		m.directoryProfileCancel = nil
	}
	m.directoryProfileBusy = false
	delete(m.directoryProfileAttempted, m.directoryProfileUser)
	m.directoryProfileUser = ""
	m.directoryGeneration++
	m.directoryBusy = false
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
		m.directErr = nil
		if m.directBusy {
			m.cancelDirectory()
		}
		if _, recognized, _ := m.explicitPickerTarget(); recognized {
			m.cancelDirectory()
			return cmd
		}
		return tea.Batch(cmd, m.continueDirectory(), m.directoryProfileCmd())
	}
	return cmd
}
