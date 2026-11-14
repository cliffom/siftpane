package ui

import (
	"crypto/rand"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) send(retry bool) tea.Cmd {
	if m.active == "" {
		return nil
	}
	key := m.key()
	d := m.deliveries[key]
	if retry {
		if d == nil || d.State != "failed" {
			return nil
		}
		if d.Uncertain && !m.retryConfirm {
			m.retryConfirm = true
			m.status = "Delivery unknown. Check Slack before retrying. Press r again to risk a duplicate."
			return nil
		}
	}
	if !retry {
		if d != nil && (d.State == "pending" || d.State == "failed") {
			m.status = "Resolve the current delivery first: Esc then r retries; x restores it for editing."
			return nil
		}
		body := m.composer.Value()
		if strings.TrimSpace(body) == "" {
			return nil
		}
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			m.status = "Cannot create send ID; draft kept"
			return nil
		}
		id[6] = (id[6] & 15) | 64
		id[8] = (id[8] & 63) | 128
		d = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Thread: m.thread, Text: body, ID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])}}
		m.deliveries[key] = d
	}
	if !d.Begin() {
		return nil
	}
	m.retryConfirm = false
	m.status = "Sending… draft retained until acknowledged"
	svc, ctx, o := m.service, m.ctx, d.Outgoing
	return func() tea.Msg { msg, err := svc.Send(ctx, o); return sendResult{key, msg, err} }
}

func (m *Model) updateSend(v sendResult) (tea.Model, tea.Cmd) {
	d := m.deliveries[v.Key]
	if d == nil {
		return m, nil
	}
	uncertain := core.UncertainDelivery(v.Err)
	d.Finish(v.Err, uncertain)
	if v.Err != nil {
		m.status = "Send failed: " + v.Err.Error() + " · text kept · Esc then r retry"
		if uncertain {
			m.status = "Delivery unknown; text kept. Check Slack before r retry."
		}
		if slack.IsAuth(v.Err) {
			m.stopped = true
		}
		return m, nil
	}
	cid, _ := split(v.Key)
	if !m.included[cid] {
		m.status = "Message sent to removed conversation"
		delete(m.drafts, v.Key)
		return m, nil
	}
	t := m.line(v.Key)
	v.Message.Author = "You"
	t.Messages = core.Merge(t.Messages, []core.Message{v.Message}, cacheLimit)
	if m.drafts[v.Key] == d.Text {
		delete(m.drafts, v.Key)
	}
	if m.key() == v.Key && m.composer.Value() == d.Text {
		m.composer.Reset()
		delete(m.drafts, v.Key)
	}
	m.status = "Message sent"
	return m, nil
}
