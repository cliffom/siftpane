package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func stamp(ts string) string {
	s, _, _ := strings.Cut(ts, ".")
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(n, 0).Local().Format("Jan 02 15:04")
}

func (m *Model) history(w, h int) []string {
	if m.active == "" {
		return padRows([]string{"", " Your channels. Your people.", " Your terminal.", "", " p  choose what belongs here", "", " Only included conversations refresh."}, w, h)
	}
	t := m.cache[m.key()]
	if t == nil || !t.Loaded {
		return padRows([]string{"", " Loading messages…", " R retry / refresh"}, w, h)
	}
	rows := []string{}
	starts := []int{}
	selectedStart, selectedEnd := 0, 0
	for i, msg := range t.Messages {
		starts = append(starts, len(rows))
		if i == m.index {
			selectedStart = len(rows)
		}
		rows = append(rows, messageRows(msg, w, i == m.index)...)
		if i == m.index {
			selectedEnd = len(rows)
		}
	}
	if len(rows) == 0 {
		rows = []string{" No messages yet. i to write the first one."}
	}
	return historyWindow(rows, starts, selectedStart, selectedEnd, w, h, m.scroll)
}

func historyWindow(rows []string, starts []int, selectedStart, selectedEnd, w, h, scroll int) []string {
	start := max(0, len(rows)-h-max(0, scroll))
	if scroll == -1 {
		start = max(0, selectedEnd-h)
		if selectedEnd-selectedStart > h {
			start = selectedStart
		}
	}
	// Prefer the next complete message, preserving the newest messages below it.
	// For oversized messages, retain scroll access and repeat the author as context.
	continuation := ""
	for i, boundary := range starts {
		next := len(rows)
		if i+1 < len(starts) {
			next = starts[i+1]
		}
		if start > boundary && start < next {
			if next-boundary <= h {
				start = next
			} else {
				continuation = ansi.Truncate(rows[boundary]+" · continued", w, "…")
			}
			break
		}
	}
	if start >= len(rows) {
		start = max(0, len(rows)-h)
	}
	available := h
	if continuation != "" {
		available--
	}
	end := min(len(rows), start+available)
	visible := rows[start:end]
	if continuation != "" {
		visible = append([]string{continuation}, visible...)
	}
	return padRows(visible, w, h)
}

func messageRows(msg core.Message, w int, focused bool) []string {
	rows := []string{}
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	author := slack.Safe(msg.Author)
	if author == "" {
		author = slack.Safe(msg.User)
	}
	if author == "" {
		author = "Slack"
	}
	head := prefix + author + "  " + stamp(msg.TS)
	if msg.Replies > 0 {
		head += fmt.Sprintf("  ↳ %d replies", msg.Replies)
	}
	if focused {
		head = selected.Render(head)
	} else {
		head = muted.Render(head)
	}
	rows = append(rows, ansi.Truncate(head, w, "…"))
	body := slack.Body(msg)
	wrapped := strings.Split(ansi.Wrap(body, max(1, w-messagePaddingColumns), ""), "\n")
	for _, line := range wrapped {
		rows = append(rows, "  "+line)
	}
	rows = append(rows, "")
	return rows
}

const messagePaddingColumns = 2 // History text inset in display cells.
