package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func (m *Model) addContext(cid string) {
	m.contexts[cid], m.cancels[cid] = context.WithCancel(m.ctx)
	m.generations[cid]++
}

func (m *Model) key() string {
	if m.thread != "" {
		return m.active + "/" + m.thread
	}
	return m.active
}

func split(key string) (string, string) {
	conversation, thread, _ := strings.Cut(key, "/")
	return conversation, thread
}

func (m *Model) line(key string) *timeline {
	m.clock++
	if t := m.cache[key]; t != nil {
		t.Touched = m.clock
		return t
	}
	if len(m.cache) >= timelineLimit {
		old := ""
		oldTime := ^uint64(0)
		for k, t := range m.cache {
			if k != m.key() && t.Touched < oldTime {
				old = k
				oldTime = t.Touched
			}
		}
		if old != "" {
			delete(m.cache, old)
		}
	}
	t := &timeline{Touched: m.clock}
	m.cache[key] = t
	return t
}

func (m *Model) save() {
	m.persistErr = ""
	if err := m.store.Save(); err != nil {
		m.persistErr = err.Error()
		m.status = "Cannot save preferences: " + err.Error()
	}
}

func (m *Model) rememberDraft() {
	if m.active != "" {
		m.drafts[m.key()] = m.composer.Value()
	}
}

func (m *Model) restoreDraft() { m.composer.SetValue(m.drafts[m.key()]); m.composer.MoveToEnd() }

// Sidebar and keyboard navigation share one grouped, deterministic order.
func (m *Model) selected() []core.Conversation {
	cs := core.Filter(m.directory, "", m.included, true)
	sortConversations(cs)
	return cs
}

func sortConversations(cs []core.Conversation) {
	sort.Slice(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if isPerson(a) != isPerson(b) {
			return !isPerson(a)
		}
		an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
}

func isPerson(c core.Conversation) bool {
	return c.Kind == core.ConversationDM || c.Kind == core.ConversationGroup
}

func (m *Model) saveMetadata() {
	m.metadataErr = ""
	if err := m.store.SaveConversations(m.identity, m.selected()); err != nil {
		m.metadataErr = err.Error()
	}
}

func (m *Model) selectConversation(cid string) tea.Cmd {
	m.rememberDraft()
	m.active = cid
	m.thread = ""
	m.index = 0
	m.scroll = 0
	m.focus = focusMessages
	m.composer.Blur()
	m.restoreDraft()
	m.read(cid)
	return m.fetch(cid, false)
}

func (m *Model) read(cid string) {
	delete(m.unread, cid)
	if ts := m.latest[cid]; ts > m.prefs.Read[cid] {
		m.prefs.Read[cid] = ts
		m.save()
	}
}

func (m *Model) toggle(cid string) {
	if m.included[cid] {
		delete(m.included, cid)
		if f := m.cancels[cid]; f != nil {
			f()
		}
		delete(m.contexts, cid)
		delete(m.metadataBusy, cid)
		if len(m.metadataBusy) == 0 {
			m.startupRestoring = false
			m.startupSidebar = nil
		}
		delete(m.metadataFailed, cid)
		delete(m.cancels, cid)
		delete(m.unread, cid)
		for k := range m.cache {
			c, _ := split(k)
			if c == cid {
				delete(m.cache, k)
			}
		}
		m.generations[cid]++
		if m.active == cid {
			m.rememberDraft()
			m.active = ""
			m.thread = ""
			m.composer.Reset()
		}
		m.status = "Removed locally; Slack membership unchanged. Draft retained for this session."
	} else {
		if len(m.included) >= selectionLimit {
			m.status = fmt.Sprintf("Limit: %d included conversations. Remove one first.", selectionLimit)
			return
		}
		m.included[cid] = true
		m.addContext(cid)
		m.publishStartupConversation(cid)
		m.status = "Included. Only chosen conversations are refreshed."
	}
	m.save()
	m.saveMetadata()
}

// Explicit user actions publish their target without exposing unrelated pending
// metadata or thawing the rest of the startup list.
func (m *Model) publishStartupConversation(cid string) {
	if !m.startupRestoring || !m.included[cid] {
		return
	}
	for _, c := range m.directory {
		if c.ID != cid {
			continue
		}
		for i, old := range m.startupSidebar {
			if old.ID == cid {
				m.startupSidebar[i] = c
				sortConversations(m.startupSidebar)
				return
			}
		}
		m.startupSidebar = append(m.startupSidebar, c)
		sortConversations(m.startupSidebar)
		return
	}
}
