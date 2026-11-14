package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) directoryCmd(cursor string) tea.Cmd {
	if m.directoryBusy {
		return nil
	}
	m.directoryBusy = true
	m.directoryErr = nil
	svc, ctx := m.service, m.ctx
	return func() tea.Msg { p, err := svc.Directory(ctx, cursor); return directoryResult{Page: p, Err: err} }
}

func (m *Model) fetch(key string, older bool) tea.Cmd {
	cid, thread := split(key)
	if !m.included[cid] || m.stopped {
		return nil
	}
	t := m.line(key)
	if t.Busy {
		return nil
	}
	if older && t.Loaded && !t.More {
		return nil
	}
	t.Busy = true
	t.Err = nil
	q := core.Query{Conversation: cid, Thread: thread}
	if !older && thread != "" && t.Loaded && !t.More && len(t.Messages) > 0 {
		q.After = t.Messages[len(t.Messages)-1].TS
	}
	if older {
		q.Cursor = t.Next
		q.After = t.After
		if thread == "" && len(t.Messages) > 0 {
			q.Cursor = ""
			q.Before = t.Messages[0].TS
		}
	}
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	return func() tea.Msg {
		p, err := svc.History(ctx, q)
		return historyResult{Key: key, Generation: gen, Page: p, Older: older, Err: err, QueryAfter: q.After}
	}
}

func (m *Model) conversationCmd(cid string) tea.Cmd {
	if m.metadataBusy[cid] || !m.included[cid] || m.stopped {
		return nil
	}
	m.metadataBusy[cid] = true
	delete(m.metadataFailed, cid)
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	return func() tea.Msg {
		c, err := svc.Conversation(ctx, cid)
		return conversationResult{Conversation: c, ID: cid, Generation: gen, Err: err}
	}
}

func (m *Model) refresh() tea.Cmd {
	m.profileAttempted = map[string]bool{}
	cmds := []tea.Cmd{m.fetch(m.key(), false), m.profileCmd()}
	for cid := range m.metadataFailed {
		cmds = append(cmds, m.conversationCmd(cid))
	}
	return tea.Batch(cmds...)
}
