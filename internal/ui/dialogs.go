package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/slack"
)

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
