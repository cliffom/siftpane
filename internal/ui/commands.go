package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) directoryCmd(cursor string) tea.Cmd {
	if m.directoryBusy || m.stopped {
		return nil
	}
	m.directoryBusy = true
	m.directoryErr = nil
	if m.mode != pickerClosed && m.directoryPagesLeft > 0 {
		m.directoryPagesLeft--
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.directoryCancel = cancel
	svc, generation := m.service, m.directoryGeneration
	return func() tea.Msg {
		defer cancel()
		if err := ctx.Err(); err != nil {
			return directoryResult{Cursor: cursor, Generation: generation, Err: err}
		}
		p, err := svc.Directory(ctx, cursor)
		return directoryResult{Page: p, Cursor: cursor, Generation: generation, Err: err}
	}
}

func (m *Model) fetch(key string, older bool) tea.Cmd {
	return m.fetchHistory(key, older, false)
}

func (m *Model) fetchHistory(key string, older, fill bool) tea.Cmd {
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
	if !older && thread != "" && t.Loaded && t.More && m.openTargets[key] == "" {
		return nil // Incomplete reply pages advance only on explicit o.
	}
	t.Busy = true
	t.Err = nil
	if !older {
		t.CachedRefreshErr = nil
	}
	q := core.Query{Conversation: cid, Thread: thread}
	if !older && thread != "" && t.Loaded && !t.More {
		q.After = t.ReplyThrough
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
	resumeAfter := t.ReplyThrough
	epoch := m.cacheEpoch
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	if key == m.key() {
		ctx = slack.ForegroundRead(ctx)
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return historyResult{Key: key, Generation: gen, Err: err}
		}
		p, err := svc.History(ctx, q)
		if q.Thread != "" && q.Cursor != "" && slack.IsInvalidCursor(err) && ctx.Err() == nil {
			q.Cursor, q.After = "", resumeAfter
			p, err = svc.History(ctx, q) // One read recovery; transport pacing still applies.
		}
		after := q.After
		if q.Target != "" && q.Thread != "" {
			after = q.Target
		}
		return historyResult{CacheEpoch: epoch, Key: key, Generation: gen, Page: p, Older: older, Fill: fill, Err: err, QueryCursor: q.Cursor, QueryAfter: after, Target: q.Target}
	}
}

func (m *Model) conversationCmd(cid string) tea.Cmd {
	if m.metadataBusy[cid] || !m.included[cid] || m.stopped {
		return nil
	}
	m.metadataState[cid] = metadataUnvalidated
	m.metadataBusy[cid] = true
	delete(m.metadataFailed, cid)
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	if cid == m.active {
		ctx = slack.ForegroundRead(ctx)
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return conversationResult{ID: cid, Generation: gen, Err: err}
		}
		c, err := svc.Conversation(ctx, cid)
		return conversationResult{Conversation: c, ID: cid, Generation: gen, Err: err}
	}
}

func (m *Model) refresh() tea.Cmd {
	m.resetHistoryFill()
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
