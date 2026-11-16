package ui

import (
	"sort"
	"time"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

func (m *Model) restoreMessages() {
	if !m.attention.MessageCache {
		if err := m.store.ClearMessages(m.identity); err != nil {
			m.messageCacheErr = "Cannot remove message cache · retry clear"
		}
		return
	}
	m.persisted = m.store.Messages(m.identity, m.attention.Kept)
	m.saveMessages() // Physically trim expired/removed windows even when live reads fail.
	for cid, h := range m.persisted {
		found := false
		for _, c := range m.directory {
			if c.ID == cid {
				found = true
				break
			}
		}
		if !found {
			m.directory = append(m.directory, core.Conversation{ID: cid, Name: cid, Kind: core.ConversationChannel})
		}
		line := m.line(cid)
		line.Messages, line.Loaded, line.More, line.Cached, line.Saved = h.Messages, true, h.More, true, h.Saved
	}
}

func (m *Model) saveMessages() {
	m.messageCacheErr = ""
	if !m.attention.MessageCache {
		return
	}
	if err := m.store.SaveMessages(m.identity, m.attention.Kept, m.persisted); err != nil {
		m.messageCacheErr = "Message cache write failed · messaging available"
	}
}

func (m *Model) cacheHistory(v historyResult, cid, thread string) {
	if !m.attention.MessageCache || !m.attention.Kept[cid] || thread != "" || v.CacheEpoch != m.cacheEpoch {
		return
	}
	// Anchored/older windows are not the recent startup window.
	if v.Target != "" || v.Older {
		return
	}
	m.persisted[cid] = store.MessageHistory{Saved: time.Now(), Messages: core.Merge(nil, v.Page.Messages, cacheLimit), More: v.Page.More}
	m.trimPersisted()
	m.saveMessages()
}

func (m *Model) clearMessages() {
	m.cacheEpoch++ // Already-running reads may display, but cannot repopulate disk.
	m.persisted = map[string]store.MessageHistory{}
	m.messageCacheErr = ""
	if err := m.store.ClearMessages(m.identity); err != nil {
		m.messageCacheErr = "Cannot remove message cache · retry clear"
		m.setStatus(m.messageCacheErr, statusAction)
		return
	}
	m.setStatus("Disk message cache cleared · drafts retained", statusAction)
}

func (m *Model) toggleMessageCache() {
	if !m.attention.MessageCache && !m.cacheConfirm {
		m.cacheConfirm = true
		m.setStatus(keyLabel(keyMessageCache)+" again: store Slack content on disk", statusAction)
		return
	}
	m.cacheConfirm = false
	a := m.attention.Clone()
	a.MessageCache = !a.MessageCache
	if !m.persistAttention(a) {
		return
	}
	m.cacheEpoch++
	if !a.MessageCache {
		m.clearMessages()
		return
	}
	// Do not persist current buffers: only subsequent successful live reads qualify.
	m.setStatus("Message cache enabled · fetched Kept history only", statusAction)
}

func (m *Model) trimPersisted() {
	ids := make([]string, 0, len(m.persisted))
	for cid := range m.persisted {
		ids = append(ids, cid)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := m.persisted[ids[i]].Saved, m.persisted[ids[j]].Saved
		if a.Equal(b) {
			return ids[i] < ids[j]
		}
		return a.After(b)
	})
	for i, cid := range ids {
		if i >= timelineLimit {
			delete(m.persisted, cid)
		}
	}
}

func (m *Model) purgeConversationCache(cid string, err error) {
	m.invalidateConversationReads(cid)
	m.cacheEpoch++
	delete(m.persisted, cid)
	m.saveMessages()
	for key, t := range m.cache {
		c, _ := split(key)
		if c == cid {
			t.Messages = nil
			t.Loaded = false
			t.Cached = false
			t.CachedRefreshErr = nil
			t.Saved = time.Time{}
			t.Busy = false
			t.Err = err
		}
	}
}

func (m *Model) stopReads() {
	m.stopped = true
	m.discoveryGeneration++
	m.finishDiscovery()
	m.cancelFollowed()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.clearMessages()
	m.setStatus(m.credentialRecovery(), statusAction)
}

func (m *Model) credentialRecovery() string {
	if m.multipleAccounts {
		return "Expired · " + keyLabel(keyRefreshDetails) + " steps · " + keyLabel(keyReloadCredential) + " reload"
	}
	return "Credentials expired · login & restart"
}
