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
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return directoryResult{Err: err}
		}
		p, err := svc.Directory(ctx, cursor)
		return directoryResult{Page: p, Err: err}
	}
}

func (m *Model) fetch(key string, older bool) tea.Cmd {
	cid, thread := split(key)
	if !m.canRead(cid) || m.stopped {
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
	if !older {
		t.CachedRefreshErr = nil
	}
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
	if !older && m.openTargets[key] != "" {
		q.Target = m.openTargets[key]
		q.After = ""
	}
	epoch := m.cacheEpoch
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return historyResult{Key: key, Generation: gen, Err: err}
		}
		p, err := svc.History(ctx, q)
		after := q.After
		if q.Target != "" && q.Thread != "" {
			after = q.Target
		}
		return historyResult{CacheEpoch: epoch, Key: key, Generation: gen, Page: p, Older: older, Err: err, QueryAfter: after, Target: q.Target}
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
		if err := ctx.Err(); err != nil {
			return conversationResult{ID: cid, Generation: gen, Err: err}
		}
		c, err := svc.Conversation(ctx, cid)
		return conversationResult{Conversation: c, ID: cid, Generation: gen, Err: err}
	}
}

func (m *Model) refresh() tea.Cmd {
	m.profileAttempted = map[string]bool{}
	m.discoveryNext = time.Time{}
	cmds := []tea.Cmd{m.fetch(m.key(), false), m.profileCmd(), m.checkMentions(time.Now()), m.retryFollowed()}
	for cid := range m.metadataFailed {
		cmds = append(cmds, m.conversationCmd(cid))
	}
	return tea.Batch(cmds...)
}

func (m *Model) nextTick() tea.Cmd {
	if m.multipleAccounts {
		return nil
	}
	return tick()
}

// Explicit terminal commands bypass widget-result routing. All other completions
// belong to their originating account, including private Bubbles paste messages.
type terminalResult struct{ message tea.Msg }

func (m *Model) terminalCmd(cmd tea.Cmd) tea.Cmd {
	if !m.multipleAccounts {
		return cmd
	}
	return func() tea.Msg { return terminalResult{message: cmd()} }
}
