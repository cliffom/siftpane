package ui

import (
	"fmt"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) sidebar(w, h int) []string {
	rows := []string{}
	cs := m.selected()
	activeIndex := 0
	for i, c := range cs {
		if c.ID == m.active {
			activeIndex = i
		}
	}
	start := max(0, activeIndex-max(1, h-8))
	includedChannels, totalChannels := m.channelCounts()
	lastKind := ""
	for i := start; i < len(cs); i++ {
		c := cs[i]
		kind := "CHANNELS"
		if c.Kind == "dm" || c.Kind == "group" {
			kind = "PEOPLE"
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
	if len(cs) == 0 {
		rows = append(rows, " Nothing included yet.", " p  choose conversations")
	}
	for len(rows) < h-3 {
		rows = append(rows, "")
	}
	if len(rows) <= h-3 {
		rows = append(rows, muted.Render(" ··· UNREAD HERE ···"), muted.Render(" ● unread here  ✎ draft"), muted.Render(" p choose   / switch"))
	}
	return padRows(rows, w, h)
}

func (m *Model) channelCounts() (int, int) {
	includedChannels, totalChannels := 0, 0
	for _, c := range m.directory {
		if c.Kind == "channel" || c.Kind == "private" {
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
	text := prefix + slack.Safe(c.Label())
	if m.unread[c.ID] {
		text += " ●"
	}
	if m.drafts[c.ID] != "" {
		text += " ✎"
	}
	if c.ID == m.active {
		return selected.Render(fit(text, w))
	}
	return fit(text, w)
}
