package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AC7BA"))

// Use the same foreground on every adjoining border segment so rounded
// corner glyphs do not change contrast where they meet the straight strokes.
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#8693A5"))
var bright = lipgloss.NewStyle().Bold(true)
var focusedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true)
var selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#BFE4DA")).Bold(true)

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

// Style each segment independently: wrapping an amber title inside a border
// style can carry the title's foreground into the remaining border strokes.
func labelLine(title string, w int, stroke string, border lipgloss.Style) string {
	focused := strings.HasPrefix(title, "[") && strings.HasSuffix(title, "]")
	if focused && w >= 4 {
		title = "[" + ansi.Truncate(title[1:len(title)-1], w-4, "…") + "]"
	}
	text := ansi.Truncate(" "+title+" ", w, "")
	rest := strings.Repeat(stroke, max(0, w-ansi.StringWidth(text)))
	if focused {
		return focusedTitle.Render(text) + border.Render(rest)
	}
	return border.Render(text + rest)
}
func (m *Model) dimensions() (side, right, history, compose, inside int) {
	inside = max(1, m.height-6)
	compose = 3
	if m.height < 22 {
		compose = 2
	}
	side = 23
	if m.width < 76 {
		side = 0
	}
	right = m.width - 2
	if side > 0 {
		right -= side + 1
	}
	history = max(1, inside-compose-3)
	return
}
func (m *Model) resize() {
	_, right, _, compose, _ := m.dimensions()
	m.composer.SetWidth(max(5, right-6))
	m.composer.SetHeight(compose)
	m.picker.SetWidth(max(5, m.width-10))
}
func (m *Model) title() string {
	name := "Choose conversations · p"
	for _, c := range m.directory {
		if c.ID == m.active {
			name = c.Label()
			break
		}
	}
	if m.thread != "" {
		name += " / Thread"
	}
	return slack.Safe(name)
}
func (m *Model) conversationContext() string {
	for _, c := range m.directory {
		if c.ID == m.active {
			kind := "Channel"
			switch c.Kind {
			case "private":
				kind = "Private channel"
			case "dm":
				kind = "Direct message"
			case "group":
				kind = "Group DM"
			}
			if c.Kind == "group" && c.Members > 0 {
				kind += fmt.Sprintf(" · %d people", c.Members)
			}
			detail := strings.TrimSpace(c.Topic)
			if detail == "" {
				detail = strings.TrimSpace(c.Purpose)
			}
			if detail != "" {
				kind += " · " + strings.Join(strings.Fields(slack.Readable(detail)), " ")
			}
			if m.thread != "" {
				kind += " · Thread replies"
			}
			return kind
		}
	}
	return "Choose what belongs here · p"
}

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
	includedChannels, totalChannels := 0, 0
	for _, c := range m.directory {
		if c.Kind == "channel" || c.Kind == "private" {
			totalChannels++
			if m.included[c.ID] {
				includedChannels++
			}
		}
	}
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
			rows = append(rows, selected.Render(fit(text, w)))
		} else {
			rows = append(rows, fit(text, w))
		}
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
func padRows(rows []string, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		if i < len(rows) {
			out[i] = fit(rows[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}
func stamp(ts string) string {
	s, _, _ := strings.Cut(ts, ".")
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
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
		prefix := "  "
		if i == m.index {
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
		if i == m.index {
			head = selected.Render(head)
		} else {
			head = muted.Render(head)
		}
		rows = append(rows, ansi.Truncate(head, w, "…"))
		body := slack.Body(msg)
		wrapped := strings.Split(ansi.Wrap(body, max(1, w-2), ""), "\n")
		for _, line := range wrapped {
			rows = append(rows, "  "+line)
		}
		rows = append(rows, "")
		if i == m.index {
			selectedEnd = len(rows)
		}
	}
	if len(rows) == 0 {
		rows = []string{" No messages yet. i to write the first one."}
	}
	start := max(0, len(rows)-h-max(0, m.scroll))
	if m.scroll == -1 {
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

// The inset frame keeps the editing surface separate from both history and
// application chrome. Its two border rows are reserved by dimensions().
func (m *Model) composerRows(w, h int) []string {
	stroke := "─"
	border := muted
	title := "Message"
	hint := "i compose"
	if m.thread != "" {
		title = "Thread reply"
	}
	if m.focus == "compose" {
		border = accent
		hint = "Ctrl+S send · Esc navigate"
	}
	if d := m.deliveries[m.key()]; d != nil {
		switch d.State {
		case "pending":
			title += " · sending…"
			hint = "Text kept until acknowledged"
		case "failed":
			title += " · failed"
			hint = "r retry · x edit · text kept"
		}
	}
	if m.focus == "compose" {
		title = "[" + title + "]"
	}
	inner := w - 4
	rows := []string{" " + border.Render("╭") + labelLine(title, inner, stroke, border) + border.Render("╮") + " "}
	for _, line := range padRows(strings.Split(m.composer.View(), "\n"), w-6, h) {
		rows = append(rows, " "+border.Render("│")+" "+line+" "+border.Render("│")+" ")
	}
	rows = append(rows, " "+border.Render("╰")+labelLine(hint, inner, stroke, border)+border.Render("╯")+" ")
	return rows
}

func (m *Model) View() tea.View {
	w, h := m.width, m.height
	if w < 40 || h < 14 {
		v := tea.NewView(fit("Siftpane · resize to at least 40 × 14", max(1, w)))
		v.AltScreen = true
		return v
	}
	left := accent.Render(" siftpane")
	workspace := strings.Join(strings.Fields(slack.Safe(m.identity.WorkspaceName)), " ")
	available := w - ansi.StringWidth(left) - 3
	username := strings.TrimPrefix(strings.Join(strings.Fields(slack.Safe(m.identity.Username)), ""), "@")
	account := ""
	if username != "" {
		account = ansi.Truncate(" · @"+username, max(0, available-4), "…")
	}
	workspace = ansi.Truncate(workspace, available-ansi.StringWidth(account), "…") + account
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(workspace) - 1
	header := left + strings.Repeat(" ", max(0, gap)) + muted.Render(workspace) + " "
	body := ""
	if m.help {
		body = m.helpView(w, h-4)
	} else if m.mode != "" {
		body = m.pickerView(w, h-4)
	} else {
		side, right, hist, comp, inside := m.dimensions()
		showList := side == 0 && m.focus == "channels"
		title := m.title()
		if m.focus == "messages" {
			title = "[" + title + "]"
		}
		leftTitle := "Messages"
		if m.focus == "channels" {
			leftTitle = "[Messages]"
		}
		rows := []string{}
		var lrows []string
		if side > 0 {
			lrows = m.sidebar(side, inside)
		}
		var rrows []string
		if showList {
			title = leftTitle
			rrows = m.sidebar(right, inside)
		} else {
			rrows = []string{muted.Render(fit(" "+m.conversationContext(), right))}
			rrows = append(rrows, m.history(right, hist)...)
			rrows = append(rrows, m.composerRows(right, comp)...)
		}
		if side > 0 {
			rows = append(rows, muted.Render("╭")+labelLine(leftTitle, side, "─", muted)+muted.Render("┬")+labelLine(title, right, "─", muted)+muted.Render("╮"))
		} else {
			rows = append(rows, muted.Render("╭")+labelLine(title, right, "─", muted)+muted.Render("╮"))
		}
		for i := 0; i < inside; i++ {
			sep := "│"
			tail := "│"

			if side > 0 {
				rows = append(rows, muted.Render("│")+lrows[i]+muted.Render(sep)+fit(rrows[i], right)+muted.Render(tail))
			} else {
				rows = append(rows, muted.Render(sep)+fit(rrows[i], right)+muted.Render(tail))
			}
		}
		if side > 0 {
			rows = append(rows, muted.Render("╰"+strings.Repeat("─", side)+"┴"+strings.Repeat("─", right)+"╯"))
		} else {
			rows = append(rows, muted.Render("╰"+strings.Repeat("─", right)+"╯"))
		}
		body = strings.Join(rows, "\n")
	}
	statusText := m.status
	if m.quitConfirm {
		statusText = "Unsent text is in memory. Ctrl+C again exits and discards it; any other key cancels."
	}
	if m.persistErr != "" && !m.quitConfirm {
		statusText = "Preferences not saved: " + m.persistErr
	}
	status := fit(muted.Render(" "+slack.Safe(statusText)), w)
	hint := " Tab focus · / switch · i write · t thread · ? help"
	if m.focus == "compose" {
		hint = " Ctrl+S send · Enter newline · Esc navigate · Ctrl+K switch"
	}
	if m.mode != "" {
		hint = " ↑/↓ move · Tab toggle · Enter open · Esc close"
	}
	if w < 76 && m.focus != "compose" && m.mode == "" {
		hint = " Tab focus · / switch · i write · ? help"
	}
	v := tea.NewView(strings.Join([]string{header, strings.Repeat(" ", w), body, status, fit(accent.Render(hint), w)}, "\n"))
	v.AltScreen = true
	v.WindowTitle = "Siftpane"
	return v
}
func (m *Model) transient(title string, rows []string, w, h int) string {
	inner := w - 2
	out := []string{accent.Render("╭") + labelLine("["+title+"]", inner, "╌", accent) + accent.Render("╮")}
	for _, row := range padRows(rows, inner, h-2) {
		out = append(out, accent.Render("╎")+row+accent.Render("╎"))
	}
	out = append(out, accent.Render("╰"+strings.Repeat("╌", inner)+"╯"))
	return strings.Join(out, "\n")
}
func (m *Model) pickerView(w, h int) string {
	title := "Choose conversations"
	if m.mode == "switch" {
		title = "Switch included conversations"
	}
	rows := []string{m.picker.View(), muted.Render(" Tab toggles locally · Enter opens · Esc closes"), muted.Render(" ··· CHANNELS / DIRECT / GROUP MESSAGES ···")}
	cs := m.choices()
	available := max(1, h-7)
	start := max(0, m.pickIndex-available+1)
	for i := start; i < min(len(cs), start+available); i++ {
		c := cs[i]
		check := "[ ]"
		if m.included[c.ID] {
			check = "[✓]"
		}
		prefix := "  "
		if i == m.pickIndex {
			prefix = "▸ "
		}
		s := prefix + check + " " + slack.Safe(c.Label()) + "  " + c.Kind
		if i == m.pickIndex {
			s = selected.Render(s)
		}
		rows = append(rows, s)
	}
	if len(cs) == 0 {
		rows = append(rows, " No matches in loaded directory pages.")
	}
	more := " Directory complete · Ctrl+R reload first page"
	if m.directoryNext != "" {
		more = " More available · Ctrl+L load next directory page"
	}
	if m.directoryBusy {
		more = " Loading directory…"
	}
	rows = append(rows, muted.Render(more))
	return m.transient(title, rows, w, h)
}
func (m *Model) helpView(w, h int) string {
	rows := []string{
		" NAVIGATE                           COMPOSE",
		" Tab        channels / messages     i / Enter  start composing",
		" j/k or ↑/↓ move/select              Ctrl+S     send",
		" PgUp/PgDn  scroll history           Enter      newline",
		" G / End    newest + local read      Esc        keep draft, navigate",
		" t          open selected thread    Ctrl+K     switch, keep draft",
		" Esc        leave thread",
		" o          load older / next thread page",
		" p          choose included conversations (Tab toggles)",
		" /, Ctrl+K  search included conversations",
		" R          refresh conversation    m  mark read locally",
		" r          retry failed send       x  restore failed text for editing",
		" y          copy selected message with OSC52",
		" q          quit (warns on drafts)  Ctrl+C twice discards unsent text",
		"",
		" ··· BORDERS ···",
		" Solid = panes and composer; dashed = transient regions / separators.",
		" Dotted = subtle groups. [Header] and ▸ work without color.",
		" Unread is local observed activity; Slack read state is never changed.",
		" Demo: send /fail to inspect a failed delivery, then Esc and r.",
		" Any key closes help. Full shortcuts and limits are in README.md.",
	}
	wrapped := []string{}
	for _, r := range rows {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(r, w-4, ""), "\n")...)
	}
	return m.transient("Keyboard guide · any key closes", wrapped, w, h)
}

// Snapshot renders a deterministic review state through the normal view path.
func Snapshot(m *Model, width, height int, mode string) string {
	m.width = width
	m.height = height
	m.resize()
	m.focus = "messages"
	switch mode {
	case "picker":
		m.mode = "pick"
	case "compose":
		m.focus = "compose"
		m.composer.SetValue("Thanks, Maya. I'll review the keyboard flow this afternoon.\nThe small details are coming together.")
		m.composer.Focus()
	case "help":
		m.help = true
	case "thread":
		t := m.cache[m.active]
		if t != nil {
			for _, msg := range t.Messages {
				if msg.Replies > 0 {
					m.thread = msg.TS
					p, _ := m.service.History(m.ctx, core.Query{Conversation: m.active, Thread: m.thread})
					m.cache[m.key()] = &timeline{Messages: core.Merge(nil, p.Messages, cacheLimit), Loaded: true}
					break
				}
			}
		}
	}
	return ansi.Strip(m.View().Content)
}
