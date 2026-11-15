package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) updateHistory(v historyResult) (tea.Model, tea.Cmd) {
	cid, thread := split(v.Key)
	if !m.included[cid] || m.generations[cid] != v.Generation {
		return m, nil
	}
	t := m.cache[v.Key]
	if t == nil {
		return m, nil
	}
	t.Busy = false
	t.Err = v.Err
	if v.Err != nil {
		if !errors.Is(v.Err, context.Canceled) {
			m.setStatus(v.Err.Error()+" · R refresh", statusRoutine)
		}
		if slack.IsAuth(v.Err) {
			m.stopped = true
			m.setStatus("Credentials expired/revoked. Drafts kept; reinstall or replace user token and restart.", statusRoutine)
		}
		return m, nil
	}
	previous := t.Loaded
	oldLen := len(t.Messages)
	t.applyPage(v, thread)
	if len(t.Messages) > 0 && thread == "" {
		m.observeActivity(cid, t.Messages[len(t.Messages)-1].TS)
	}
	if v.Key == m.key() {
		if v.Older {
			if thread == "" {
				m.index = 0
			} else {
				m.index = max(0, oldLen-1)
			}
			m.scroll = -1
		} else if !previous || m.scroll == 0 {
			m.index = max(0, len(t.Messages)-1)
		}
	}
	if cmd := m.selectTriggerResult(v, t); cmd != nil {
		return m, cmd
	}

	if d := m.deliveries[m.key()]; !m.stopped && m.statusKind != statusAction && (d == nil || d.State == core.DeliverySent) {
		m.setStatus("Refreshed", statusRoutine)
	}
	return m, m.profileCmd()
}

func (m *Model) observeActivity(cid, ts string) {
	before := m.latest[cid]
	if ts > before {
		m.latest[cid] = ts
	}
	if m.prefs.Read[cid] == "" && before == "" {
		m.prefs.Read[cid] = ts
		m.save()
	} else if ts > m.prefs.Read[cid] {
		m.unread[cid] = true
	}
	if cid == m.active && m.thread == "" && m.scroll == 0 && m.focus != focusCompose && m.mode == pickerClosed {
		m.read(cid)
	}
}

func (t *timeline) applyPage(v historyResult, thread string) {
	// An explicit anchor starts a new bounded window. Merging it into a full
	// newer thread cache would immediately evict the requested older message.
	if v.Target != "" {
		t.Messages = nil
		t.Loaded = false
	}
	previous := t.Loaded
	// Refresh replaces the observed range so edits/deletions within it are reflected.
	if !v.Older && thread == "" && !v.Page.More {
		t.Messages = nil
	}
	if !v.Older && thread == "" && len(v.Page.Messages) > 0 {
		minTS := v.Page.Messages[0].TS
		for _, x := range v.Page.Messages {
			if x.TS < minTS {
				minTS = x.TS
			}
		}
		keep := []core.Message{}
		for _, x := range t.Messages {
			if x.TS < minTS {
				keep = append(keep, x)
			}
		}
		t.Messages = keep
	}
	if v.Older && thread == "" {
		t.Messages = core.Merge(t.Messages, v.Page.Messages, len(t.Messages)+len(v.Page.Messages))
		if len(t.Messages) > cacheLimit {
			t.Messages = t.Messages[:cacheLimit]
		}
	} else {
		t.Messages = core.Merge(t.Messages, v.Page.Messages, cacheLimit)
	}
	if !previous || v.Older || thread == "" {
		t.Next = v.Page.Next
		t.More = v.Page.More
		t.After = v.QueryAfter
	} else if thread != "" && !t.More && v.Page.More {
		t.Next = v.Page.Next
		t.More = true
		t.After = v.QueryAfter
	}
	t.Loaded = true
}

func (m *Model) selectTriggerResult(v historyResult, t *timeline) tea.Cmd {
	target := m.openTargets[v.Key]
	if target == "" || v.Key != m.key() {
		return nil
	}
	for i, msg := range t.Messages {
		if msg.TS == target {
			m.index = i
			m.scroll = -1
			delete(m.openTargets, v.Key)
			return nil
		}
	}
	if v.Target != target {
		return m.fetchTrigger(core.AttentionTrigger{TS: target})
	}
	m.scroll = -1
	m.setStatus("Trigger unavailable in returned context · inspect Slack or R retry", statusAction)
	return nil
}
