package ui

import (
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) updateConversation(v conversationResult) (tea.Model, tea.Cmd) {
	pickedID := m.pickedID()
	if m.stopped || !m.included[v.ID] || m.generations[v.ID] != v.Generation {
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
		if slack.IsAccess(v.Err) {
			m.purgeConversationCache(v.ID, v.Err)
		}
		if slack.IsAuth(v.Err) {
			m.stopReads()
		}
		return m, nil
	}
	delete(m.metadataFailed, v.ID)
	m.metadataState[v.ID] = metadataValidated
	observed := m.mergeDirectoryConversation(v.Conversation)
	m.saveDirectory([]core.Conversation{observed})
	m.reselectChoice(pickedID)
	m.saveMetadata()
	if m.active == "" && m.attention.Kept[v.ID] {
		cmd := m.selectConversation(v.ID)
		m.publishStartupConversation(v.ID)
		return m, cmd
	}
	return m, m.profileCmd()
}

func (m *Model) updateDirectory(v directoryResult) (tea.Model, tea.Cmd) {
	if m.stopped || v.Generation != m.directoryGeneration {
		return m, nil
	}
	pickedID := m.pickedID()
	m.directoryBusy = false
	m.directoryCancel = nil
	m.directoryErr = v.Err
	if v.Err != nil {
		m.setStatus(v.Err.Error(), statusRoutine)
		if slack.IsAuth(v.Err) {
			m.stopReads()
		}
		return m, nil
	}
	m.directoryLoaded = true
	if m.directorySeen != nil {
		m.directorySeen[v.Cursor] = true
	}
	m.directoryNext = v.Page.Next
	observed := make([]core.Conversation, 0, len(v.Page.Conversations))
	for _, c := range v.Page.Conversations {
		observed = append(observed, m.mergeDirectoryConversation(c))
	}
	m.sortDirectory()
	m.reselectChoice(pickedID)
	m.saveMetadata()
	m.saveDirectory(observed)
	if m.active == "" {
		cs := m.selected()
		for _, c := range cs {
			if m.attention.Kept[c.ID] {
				return m, tea.Batch(m.selectConversation(c.ID), m.continueDirectory(), m.directoryProfileCmd())
			}
		}
	}
	return m, tea.Batch(m.continueDirectory(), m.directoryProfileCmd())
}

func (m *Model) mergeDirectoryConversation(c core.Conversation) core.Conversation {
	delete(m.directoryCached, c.ID)
	delete(m.discoveryMetadata, c.ID)
	if c.User != "" {
		if unresolvedDirectoryName(c) {
			if m.directoryProfilePending == nil {
				m.directoryProfilePending = make(map[string]bool)
			}
			m.directoryProfilePending[c.User] = true
		} else {
			delete(m.directoryProfilePending, c.User)
		}
	}
	for i, old := range m.directory {
		if old.ID != c.ID {
			continue
		}
		if unresolvedDirectoryName(c) && old.Kind == core.ConversationDM && (old.User == "" || old.User == c.User) && old.Name != "" && old.Name != old.ID && old.Name != old.User && old.Name != c.User {
			c.Name, c.Handle = old.Name, old.Handle
		}
		m.directory[i] = c
		return c
	}
	m.directory = append(m.directory, c)
	return c
}

func (m *Model) sortDirectory() {
	sort.SliceStable(m.directory, func(i, j int) bool {
		return string(m.directory[i].Kind)+strings.ToLower(m.directory[i].Name) < string(m.directory[j].Kind)+strings.ToLower(m.directory[j].Name)
	})
}

func (m *Model) updateTick(v tickMsg) (tea.Model, tea.Cmd) {
	m.statusNow = time.Time(v)
	cmds := []tea.Cmd{m.nextTick(), m.checkMentions(time.Now()), m.checkFollowed(time.Now())}
	cs := m.pollingConversations()
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

// Only conversations eligible for history consume a turn. Sidebar-only search
// discoveries remain visible without slowing Kept or explicitly opened entries.
func (m *Model) pollingConversations() []core.Conversation {
	cs := m.selected()
	out := cs[:0]
	for _, c := range cs {
		if m.canRead(c.ID) {
			out = append(out, c)
		}
	}
	return out
}
