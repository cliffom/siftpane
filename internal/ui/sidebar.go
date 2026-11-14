package ui

import (
	"fmt"

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
	includedChannels, totalChannels := m.channelCounts()
	lastKind := ""
	for i := start; i < len(cs); i++ {
		c := cs[i]
		kind := "CHANNELS"
		if isPerson(c) {
			kind = "PEOPLE"
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
			if kind == "CHANNELS" {
				label += fmt.Sprintf(" %d/%d", includedChannels, totalChannels)
			}
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

func (m *Model) channelCounts() (int, int) {
	includedChannels, totalChannels := 0, 0
	for _, c := range m.directory {
		if c.Kind == core.ConversationChannel || c.Kind == core.ConversationPrivate {
			totalChannels++
			if m.included[c.ID] {
				includedChannels++
			}
		}
	}
	return includedChannels, totalChannels
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
	text := fit(prefix+slack.Safe(c.Label()), w-sidebarMarkerColumns) + " " + markers
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
