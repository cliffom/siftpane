package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) updateConversation(v conversationResult) (tea.Model, tea.Cmd) {
	if !m.included[v.ID] || m.generations[v.ID] != v.Generation {
		return m, nil
	}
	if v.Err != nil {
		m.status = "Saved conversation " + v.ID + " unavailable: " + v.Err.Error()
		if slack.IsAuth(v.Err) {
			m.stopped = true
		}
		return m, nil
	}
	for i, c := range m.directory {
		if c.ID == v.ID {
			m.directory[i] = v.Conversation
			return m, nil
		}
	}
	m.directory = append(m.directory, v.Conversation)
	if m.active == "" {
		return m, m.selectConversation(v.ID)
	}
	return m, nil
}

func (m *Model) updateDirectory(v directoryResult) (tea.Model, tea.Cmd) {
	m.directoryBusy = false
	if v.Err != nil {
		m.status = v.Err.Error()
		if slack.IsAuth(v.Err) {
			m.stopped = true
		}
		return m, nil
	}
	m.directoryNext = v.Page.Next
	for _, c := range v.Page.Conversations {
		found := false
		for i, old := range m.directory {
			if old.ID == c.ID {
				m.directory[i] = c
				found = true
				break
			}
		}
		if !found {
			m.directory = append(m.directory, c)
		}
	}
	sort.SliceStable(m.directory, func(i, j int) bool {
		return m.directory[i].Kind+strings.ToLower(m.directory[i].Name) < m.directory[j].Kind+strings.ToLower(m.directory[j].Name)
	})
	if m.active == "" {
		cs := m.selected()
		if len(cs) > 0 {
			return m, m.selectConversation(cs[0].ID)
		}
	}
	return m, nil
}

func (m *Model) updateTick(_ tickMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tick()}
	cs := m.selected()
	if !m.stopped && len(cs) > 0 {
		m.pollCount++
		key := cs[m.pollIndex%len(cs)].ID
		m.pollIndex++
		if m.thread != "" && m.pollCount%2 == 0 {
			key = m.key()
		}
		if key != m.key() || m.scroll == 0 {
			cmds = append(cmds, m.fetch(key, false))
		}
	}
	return m, tea.Batch(cmds...)
}
