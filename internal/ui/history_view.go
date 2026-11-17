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
	return time.Unix(n, 0).Local().Format(messageTimeFormat)
}

func (m *Model) history(w, h int) []string {
	if m.active == "" {
		if len(m.metadataBusy) > 0 && !m.stopped {
			return padHistoryRows(nil, w, h)
		}
		if text := m.restorationState(); text != "" {
			rows := []string{"", " " + text}
			if !m.stopped {
				rows = append(rows, " "+keyLabel(keyRefresh)+" retry saved conversations")
			}
			return padHistoryRows(rows, w, h)
		}
		return padHistoryRows([]string{"", " Your channels. Your people.", " Your terminal.", "", " p  choose what belongs here", "", " Only included conversations refresh."}, w, h)
	}
	t := m.cache[m.key()]
	if t != nil && !t.Loaded && t.Err != nil {
		if m.stopped {
			return padHistoryRows([]string{"", " Credentials expired/revoked.", " " + m.credentialRecovery()}, w, h)
		}
		return padHistoryRows([]string{"", " Could not load messages.", " " + slack.Safe(t.Err.Error()), " " + keyLabel(keyRefresh) + " retry / refresh"}, w, h)
	}
	if t == nil || !t.Loaded {
		return padHistoryRows(nil, w, h)
	}
	rows, starts, selectedStart, selectedEnd := m.historyRows(t, w)
	if len(rows) == 0 {
		return padHistoryRows([]string{" No messages yet. i to write the first one."}, w, h)
	}
	return renderHistoryWindow(rows, m.historyViewport(rows, starts, selectedStart, selectedEnd, w, h), w, h)
}

func (m *Model) historyRows(t *timeline, w int) ([]string, []int, int, int) {
	rows := []string{}
	starts := []int{}
	selectedStart, selectedEnd := 0, 0
	for i, msg := range t.Messages {
		starts = append(starts, len(rows))
		if i == m.index {
			selectedStart = len(rows)
		}
		rows = append(rows, messageRows(slack.WithProfiles(msg, m.profiles), w, i == m.index)...)
		if i == m.index {
			selectedEnd = len(rows)
		}
	}
	return rows, starts, selectedStart, selectedEnd
}

// historyViewportPosition separates source rows from leading blank display rows.
// Keeping both is necessary to freeze a sparse bottom-aligned viewport.
type historyViewportPosition struct {
	start, padding, height int
}

type historyViewport struct {
	historyViewportPosition
	continuation string
}

func (v historyViewport) end(rowCount, height int) int {
	available := max(0, height-v.padding)
	if v.continuation != "" {
		available = max(0, available-1)
	}
	return min(rowCount, v.start+available)
}

func defaultHistoryViewport(rows []string, starts []int, selectedStart, selectedEnd, width, height, scroll int) historyViewport {
	start, continuation := historyStart(rows, starts, selectedStart, selectedEnd, width, height, scroll)
	view := historyViewport{historyViewportPosition: historyViewportPosition{start: start, height: height}, continuation: continuation}
	if scroll == 0 || scroll == scrollPinnedNewest {
		used := view.end(len(rows), height) - start
		if continuation != "" {
			used++
		}
		view.padding = max(0, height-used)
	}
	return view
}

func (m *Model) historyViewport(rows []string, starts []int, selectedStart, selectedEnd, width, height int) historyViewport {
	if m.scroll != scrollPinnedNewest || m.pinnedHistory == nil {
		return defaultHistoryViewport(rows, starts, selectedStart, selectedEnd, width, height, m.scroll)
	}
	position := *m.pinnedHistory
	position.start = historySourceStart(rows, max(0, min(position.start, len(rows))), height)
	position.padding = max(0, min(position.padding+height-position.height, height-1))
	position.height = height
	return historyViewport{historyViewportPosition: position, continuation: historyContinuation(rows, starts, position.start, width)}
}

func historyWindow(rows []string, starts []int, selectedStart, selectedEnd, w, h, scroll int) []string {
	return renderHistoryWindow(rows, defaultHistoryViewport(rows, starts, selectedStart, selectedEnd, w, h, scroll), w, h)
}

func renderHistoryWindow(rows []string, view historyViewport, width, height int) []string {
	visible := rows[view.start:view.end(len(rows), height)]
	if view.continuation != "" {
		visible = append([]string{view.continuation}, visible...)
	}
	visible = append(make([]string, view.padding), visible...)
	return padHistoryRows(visible, width, height)
}

func historyStart(rows []string, starts []int, selectedStart, selectedEnd, w, h, scroll int) (int, string) {
	start := max(0, len(rows)-h-max(0, scroll))
	if scroll == -1 {
		start = max(0, selectedEnd-h)
		if selectedEnd-selectedStart > h {
			start = selectedStart
		}
	}
	// Reserve a context header without retaining a separator-only fragment.
	if historyContinuation(rows, starts, start, w) != "" && scroll <= 0 {
		start++
	}
	start = historySourceStart(rows, start, h)
	return start, historyContinuation(rows, starts, start, w)
}

func historySourceStart(rows []string, start, height int) int {
	for start < len(rows) && rows[start] == "" {
		start++
	}
	if start >= len(rows) {
		return max(0, len(rows)-height)
	}
	return start
}

// Derive ownership after adjusting the start: it may now be the next message.
func historyContinuation(rows []string, starts []int, start, width int) string {
	if start >= len(rows) || rows[start] == "" {
		return ""
	}
	for i, boundary := range starts {
		next := len(rows)
		if i+1 < len(starts) {
			next = starts[i+1]
		}
		if start > boundary && start < next {
			return ansi.Truncate(rows[boundary]+" · continued", historyContentWidth(width), "…")
		}
	}
	return ""
}

func messageRows(msg core.Message, w int, focused bool) []string {
	w = max(0, historyContentWidth(w)-historyRailColumns)
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
	rows = append(rows, formattedBodyRows(slack.Body(msg), w)...)
	rail := strings.Repeat(" ", historyRailColumns)
	if focused {
		rail = strings.Repeat(" ", historyRailColumns-1) + accent.Render("│")
	}
	for i := range rows {
		rows[i] = rail + rows[i]
	}
	rows = append(rows, "")
	return rows
}

const codeFence = "```"

// Visual rails distinguish quoted and fenced text; clipboard content stays plain.
// Slack permits content directly after opening fences and before closing fences.
func formattedBodyRows(body string, width int) []string {
	rows := []string{}
	for {
		prose, rest, fenced := strings.Cut(body, codeFence)
		if fenced {
			prose = strings.TrimSuffix(prose, "\n")
		}
		rows = append(rows, proseBodyRows(prose, width)...)
		if !fenced {
			return rows
		}
		literal, after, closed := strings.Cut(rest, codeFence)
		literal = strings.TrimPrefix(literal, "\n")
		if closed {
			literal = strings.TrimSuffix(literal, "\n")
		}
		rows = append(rows, "  "+muted.Render("┌ code"))
		rows = append(rows, wrappedBodyRows(literal, "│ ", width)...)
		if !closed {
			return rows
		}
		rows = append(rows, "  "+muted.Render("└"))
		body = strings.TrimPrefix(after, "\n")
	}
}

func proseBodyRows(body string, width int) []string {
	if body == "" {
		return nil
	}
	rows := []string{}
	for _, line := range strings.Split(body, "\n") {
		trimmed, prefix := strings.TrimSpace(line), ""
		switch {
		case strings.HasPrefix(trimmed, ">"):
			prefix, line = "│ ", strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			prefix, line = "• ", trimmed[2:]
		}
		rows = append(rows, wrappedBodyRows(line, prefix, width)...)
	}
	return rows
}

func wrappedBodyRows(text, prefix string, width int) []string {
	available := max(1, width-messagePaddingColumns-ansi.StringWidth(prefix))
	rows := []string{}
	for i, part := range strings.Split(ansi.Wrap(text, available, ""), "\n") {
		marker := prefix
		if prefix == "• " && i > 0 {
			marker = "  "
		}
		rows = append(rows, "  "+marker+part)
	}
	return rows
}

// Use the same content width for wrapping, continuation headers and padding.
func historyContentWidth(width int) int {
	return max(0, width-historyRightPaddingColumns)
}

func padHistoryRows(rows []string, width, height int) []string {
	out := padRows(rows, historyContentWidth(width), height)
	for i := range out {
		out[i] = fit(out[i]+strings.Repeat(" ", historyRightPaddingColumns), width)
	}
	return out
}
