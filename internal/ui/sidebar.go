package ui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) sidebar(w, h int) []string {
	rows := []string{}
	state := m.restorationState()
	// Loading progress belongs to the bottom status bar; keep sidebar recovery.
	if len(m.metadataBusy) > 0 && !m.stopped {
		state = ""
	}
	listHeight := h
	if state != "" {
		listHeight = max(0, h-sidebarRestorationRows)
	}
	cs := m.sidebarConversations()
	activeIndex := 0
	for i, c := range cs {
		if c.ID == m.active {
			activeIndex = i
		}
	}
	start := max(0, activeIndex-max(1, listHeight-sidebarSelectionRows))

	lastKind := ""
	for i := start; i < len(cs); i++ {
		c := cs[i]
		kind := "KEPT"
		if len(m.attention.Pending[c.ID]) > 0 {
			kind = "NEEDS ATTENTION"
		}
		needed := 1
		if lastKind != kind {
			needed++
		}
		if len(rows)+needed > listHeight {
			break
		}
		if lastKind != kind {
			label := kind

			rows = append(rows, labelLine(label, w, "╌", muted))
			lastKind = kind
		}
		rows = append(rows, m.conversationRow(c, w))
	}
	if state != "" {
		rows = append(rows, muted.Render(" "+state))
	} else if len(cs) == 0 && len(m.metadataBusy) == 0 {
		rows = append(rows, " Nothing included yet.", " "+keyLabel(keyInclude)+"  choose conversations")
	}
	return padRows(rows, w, h)
}

func (m *Model) conversationRow(c core.Conversation, w int) string {
	prefix := "  "
	if c.ID == m.active {
		prefix = "▸ "
	}
	// Fixed trailing slots keep local activity visible even for truncated names.
	markers := " "
	if m.unread[c.ID] {
		markers = "●"
	}
	markers += " "
	if m.drafts[c.ID] != "" {
		markers += "✎"
	} else {
		markers += " "
	}
	label := slack.Safe(c.Label())
	reason := ""
	if ts := m.attention.Pending[c.ID]; len(ts) > 0 {
		reason = " · " + string(m.attention.Reason(c.ID))
		if len(ts) > 1 {
			reason += fmt.Sprintf("+%d", len(ts)-1)
		}
	}
	text := fit(prefix+label, max(0, w-sidebarMarkerColumns-ansi.StringWidth(reason))) + reason + " " + markers

	if c.ID == m.active {
		return selected.Render(fit(text, w))
	}
	return fit(text, w)
}

// Restoration state stays visible independently of transient history/send status.
func (m *Model) restorationState() string {
	if m.stopped && len(m.metadataFailed) > 0 {
		return "Credentials expired · replace token and restart"
	}
	if len(m.metadataBusy) > 0 {
		return fmt.Sprintf("Loading %d saved conversations…", len(m.metadataBusy))
	}
	if len(m.metadataFailed) > 0 {
		return "Unavailable · " + keyLabel(keyRefresh) + " retry"
	}
	return ""
}

// Publish restored sidebar entries together so uncached startup cannot shuffle
// visible rows. Active history and search use arriving metadata immediately.
func (m *Model) sidebarConversations() []core.Conversation {
	if m.startupRestoring {
		cs := []core.Conversation{}
		for _, c := range m.startupSidebar {
			if m.included[c.ID] {
				cs = append(cs, c)
			}
		}
		return cs
	}
	return m.selected()
}
