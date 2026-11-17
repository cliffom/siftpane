package ui

import (
	"context"
	"errors"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

type directAction uint8

const (
	directOpen directAction = iota
	directTemporary
	directKeep
)

type directConversationResult struct {
	Conversation core.Conversation
	ID           string
	Generation   uint64
	Action       directAction
	Err          error
}

func conversationID(s string) bool {
	if len(s) <= 1 || !strings.ContainsRune("CGD", rune(s[0])) {
		return false
	}
	for _, r := range s[1:] {
		if r >= '0' && r <= '9' {
			continue
		}
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

const bareIDMinChars = 9 // Conservative shorthand detection; id: accepts opaque all-letter IDs.

func bareConversationID(s string) bool {
	return len(s) >= bareIDMinChars && strings.ContainsAny(s, "0123456789") && conversationID(s)
}

// Only extract IDs from Slack links. Never follow a pasted URL or use its host
// for API requests; the active account's service remains the authority.
func directTarget(input, workspace string) (string, bool, error) {
	q := strings.TrimSpace(input)
	invalid := errors.New("paste a Slack channel link or conversation ID")
	if strings.HasPrefix(strings.ToLower(q), "id:") {
		id := strings.TrimSpace(q[len("id:"):])
		if !conversationID(id) {
			return "", true, invalid
		}
		return id, true, nil
	}
	if bareConversationID(q) {
		return q, true, nil
	}
	if !strings.Contains(q, "://") {
		return "", false, nil
	}
	u, err := url.Parse(q)
	if err != nil || u.User != nil || u.Port() != "" {
		return "", true, invalid
	}
	id, team := "", ""
	switch {
	case u.Scheme == "slack" && u.Host == "channel":
		id, team = u.Query().Get("id"), u.Query().Get("team")
	case u.Scheme == "https" && (u.Host == "app.slack.com" || strings.HasSuffix(u.Host, ".slack.com")):
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		switch {
		case len(parts) >= 3 && parts[0] == "client" && u.Host == "app.slack.com":
			team, id = parts[1], parts[2]
		case len(parts) >= 2 && parts[0] == "archives":
			id = parts[1]
		default:
			return "", true, invalid
		}
	default:
		return "", true, invalid
	}
	if team != "" && team != workspace {
		return "", true, errors.New("link belongs to another workspace; switch with Ctrl+W")
	}
	if !conversationID(id) {
		return "", true, invalid
	}
	return id, true, nil
}

func (m *Model) resolveConversation(id string, action directAction) tea.Cmd {
	if m.directBusy || m.stopped {
		return nil
	}
	m.cancelDirectory()
	m.directBusy = true
	m.directErr = nil
	ctx, cancel := context.WithCancel(m.ctx)
	m.directCancel = cancel
	svc, generation := m.service, m.directoryGeneration
	return func() tea.Msg {
		defer cancel()
		if err := ctx.Err(); err != nil {
			return directConversationResult{ID: id, Generation: generation, Action: action, Err: err}
		}
		c, err := svc.Conversation(ctx, id)
		return directConversationResult{ID: id, Generation: generation, Action: action, Conversation: c, Err: err}
	}
}

func (m *Model) updateDirectConversation(v directConversationResult) (tea.Model, tea.Cmd) {
	if m.stopped || m.mode == pickerClosed || v.Generation != m.directoryGeneration {
		return m, nil
	}
	m.directBusy = false
	m.directCancel = nil
	m.directErr = v.Err
	if v.Err != nil {
		if slack.IsAccess(v.Err) {
			m.purgeConversationCache(v.ID, v.Err)
		}
		if slack.IsAuth(v.Err) {
			m.stopReads()
		}
		return m, nil
	}
	if v.Conversation.ID != v.ID {
		m.directErr = errors.New("conversation lookup returned another identity")
		return m, nil
	}
	m.replaceConversation(v.Conversation)
	delete(m.directoryCached, v.ID)
	m.saveDirectory([]core.Conversation{v.Conversation})
	switch v.Action {
	case directTemporary:
		return m, m.temporaryChoice([]core.Conversation{v.Conversation})
	case directKeep:
		m.toggle(v.ID)
		return m, tea.Batch(m.continueDirectory(), m.directoryProfileCmd())
	default:
		return m, m.openChoice([]core.Conversation{v.Conversation})
	}
}

func (m *Model) saveDirectory(cs []core.Conversation) {
	m.directoryCacheErr = m.store.SaveDirectory(m.identity, cs)
}

func (m *Model) removeDirectory(cid string) {
	m.directoryCacheErr = m.store.RemoveDirectory(m.identity, cid)
	delete(m.directoryCached, cid)
	if m.included[cid] {
		return
	} // Keep selected/draft context for its established recovery path.
	out := m.directory[:0]
	for _, c := range m.directory {
		if c.ID != cid {
			out = append(out, c)
		}
	}
	m.directory = out
}

func (m *Model) directPickerAction(action directAction) (tea.Cmd, bool) {
	id, recognized, err := m.pickerTarget()
	if !recognized {
		return nil, false
	}
	if err != nil {
		m.directErr = err
		return nil, true
	}
	if action == directKeep && m.attention.Kept[id] {
		m.toggle(id)
		return nil, true
	}
	return m.resolveConversation(id, action), true
}

func (m *Model) pickerTarget() (string, bool, error) {
	q := strings.TrimSpace(m.picker.Value())
	if bareConversationID(q) && len(m.choices()) > 0 {
		return "", false, nil // A visible name match takes precedence over shorthand IDs.
	}
	return directTarget(q, m.identity.Workspace)
}
