package ui

import (
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) updateConversation(v conversationResult) (tea.Model, tea.Cmd) {
	pickedID := m.pickedID()
	if !m.included[v.ID] || m.generations[v.ID] != v.Generation {
		return m, nil
	}
	delete(m.metadataBusy, v.ID)
	if len(m.metadataBusy) == 0 {
		m.startupRestoring = false
		m.startupSidebar = nil
	}
	if v.Err != nil {
		m.metadataFailed[v.ID] = true
		m.setStatus("Saved conversation "+v.ID+" unavailable: "+v.Err.Error(), statusRoutine)
		if slack.IsAuth(v.Err) {
			m.stopped = true
		}
		return m, nil
	}
	delete(m.metadataFailed, v.ID)
	for i, c := range m.directory {
		if c.ID == v.ID {
			m.directory[i] = v.Conversation
			m.reselectChoice(pickedID)
			m.saveMetadata()
			return m, nil
		}
	}
	m.directory = append(m.directory, v.Conversation)
	m.saveMetadata()
	m.reselectChoice(pickedID)
	if m.active == "" && m.attention.Kept[v.ID] {
		return m, m.selectConversation(v.ID)
	}
	return m, nil
}

func (m *Model) updateDirectory(v directoryResult) (tea.Model, tea.Cmd) {
	pickedID := m.pickedID()
	m.directoryBusy = false
	m.directoryErr = v.Err
	if v.Err != nil {
		m.setStatus(v.Err.Error(), statusRoutine)
		if slack.IsAuth(v.Err) {
			m.stopped = true
		}
		return m, nil
	}
	m.directoryLoaded = true
	m.directoryNext = v.Page.Next
	for _, c := range v.Page.Conversations {
		delete(m.discoveryMetadata, c.ID)
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
		return string(m.directory[i].Kind)+strings.ToLower(m.directory[i].Name) < string(m.directory[j].Kind)+strings.ToLower(m.directory[j].Name)
	})
	m.reselectChoice(pickedID)
	m.saveMetadata()
	if m.active == "" {
		cs := m.selected()
		for _, c := range cs {
			if m.attention.Kept[c.ID] {
				return m, m.selectConversation(c.ID)
			}
		}
	}
	return m, nil
}

func (m *Model) updateTick(_ tickMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tick(), m.checkMentions(time.Now())}
	cs := m.selected()
	if !m.stopped && len(cs) > 0 {
		// Keep the first visible page ahead of background timeline reads.
		if line := m.cache[m.key()]; m.active != "" && m.thread == "" && (line == nil || (!line.Loaded && line.Err == nil)) {
			if m.scroll == 0 {
				cmds = append(cmds, m.fetch(m.key(), false))
			}
			return m, tea.Batch(cmds...)
		}
		m.pollCount++
		var key string
		if m.thread != "" && m.pollCount%threadPollEvery == 0 {
			key = m.key()
		} else {
			key = cs[m.pollIndex%len(cs)].ID
			// Thread ticks leave the next conversation's turn intact.
			m.pollIndex++
		}
		if key != m.key() || m.scroll == 0 {
			cmds = append(cmds, m.fetch(key, false))
		}
	}
	return m, tea.Batch(cmds...)
}
