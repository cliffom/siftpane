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
		if d == nil || d.State != core.DeliveryFailed {
			return nil
		}
		if d.Uncertain && !m.retryConfirm {
			m.retryConfirm = true
			m.setStatus("Delivery unknown. Check Slack before retrying. Press "+keyLabel(keyRetry)+" again to risk a duplicate.", statusAction)
			return nil
		}
	}
	if !retry {
		if d != nil && (d.State == core.DeliveryPending || d.State == core.DeliveryFailed) {
			m.setStatus("Resolve the current delivery first: "+keyLabel(keyEscape)+" then "+keyLabel(keyRetry)+" retries; "+keyLabel(keyEditFailed)+" restores it for editing.", statusAction)
			return nil
		}
		body := m.composer.Value()
		if strings.TrimSpace(body) == "" {
			return nil
		}
		// UUIDv4: 16 bytes; version nibble 0100 and variant bits 10 (RFC 9562).
		id := make([]byte, sendIDBytes)
		if _, err := rand.Read(id); err != nil {
			m.setStatus("Cannot create send ID; draft kept", statusAction)
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
	m.setStatus("Sending… draft retained until acknowledged", statusAction)
	svc, ctx, o, generation := m.service, m.sendCtx, d.Outgoing, m.serviceGeneration
	return func() tea.Msg {
		msg, err := svc.Send(ctx, o)
		return sendResult{Key: key, Message: msg, Err: err, ServiceGeneration: generation}
	}
}

func (m *Model) updateSend(v sendResult) (tea.Model, tea.Cmd) {
	d := m.deliveries[v.Key]
	if d == nil {
		return m, nil
	}
	uncertain := core.UncertainDelivery(v.Err)
	d.Finish(v.Err, uncertain)
	if v.Err != nil {
		m.setStatus("Send failed: "+v.Err.Error()+" · text kept · "+keyLabel(keyEscape)+" then "+keyLabel(keyRetry)+" retry", statusAction)
		if uncertain {
			m.setStatus("Delivery unknown; text kept. Check Slack before "+keyLabel(keyRetry)+" retry.", statusAction)
		}
		if slack.IsAuth(v.Err) && v.ServiceGeneration == m.serviceGeneration {
			m.stopReads()
		}
		return m, nil
	}
	cid, _ := split(v.Key)
	if !m.included[cid] {
		m.setStatus("Message sent to removed conversation", statusAction)
		delete(m.drafts, v.Key)
		return m, nil
	}
	t := m.line(v.Key)
	anchor := m.captureReadingAnchor()
	selectedTS := ""
	if v.Key == m.key() && m.scroll != 0 && m.index >= 0 && m.index < len(t.Messages) {
		selectedTS = t.Messages[m.index].TS
	}
	v.Message.Author = "You"
	if _, thread := split(v.Key); thread != "" {
		t.Messages = mergeReplies(t.Messages, []core.Message{v.Message}, thread)
	} else {
		t.Messages = core.Merge(t.Messages, []core.Message{v.Message}, cacheLimit)
	}
	if v.Key == m.key() {
		if m.scroll == 0 {
			m.index = max(0, len(t.Messages)-1)
		}
		m.restoreMessageSelection(historyResult{}, t, selectedTS)
		m.restoreReadingAnchor(anchor)
		m.preserveVisibleSelection()
	}
	if m.drafts[v.Key] == d.Text {
		delete(m.drafts, v.Key)
	}
	if m.key() == v.Key && m.composer.Value() == d.Text {
		m.composer.Reset()
		delete(m.drafts, v.Key)
	}
	m.setStatus("Message sent", statusAction)
	return m, nil
}

const sendIDBytes = 16 // UUIDv4 size in bytes.
