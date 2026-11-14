package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/slack"
)

func (m *Model) transient(title string, rows []string, w, h int) string {
	inner := w - frameColumns
	out := []string{accent.Render("╭") + labelLine("["+title+"]", inner, "╌", accent) + accent.Render("╮")}
	for _, row := range padRows(rows, inner, h-frameRows) {
		out = append(out, accent.Render("╎")+row+accent.Render("╎"))
	}
	out = append(out, accent.Render("╰"+strings.Repeat("╌", inner)+"╯"))
	return strings.Join(out, "\n")
}

func (m *Model) pickerView(w, h int) string {
	title := "Choose conversations"
	if m.mode == pickerSwitch {
		title = "Switch included conversations"
	}
	rows := []string{m.picker.View(), muted.Render(" " + keyLabel(keyFocus) + " toggles locally · " + keyLabel(keyCompose) + " opens · " + keyLabel(keyEscape) + " closes"), muted.Render(" ··· CHANNELS / DIRECT / GROUP MESSAGES ···")}
	cs := m.choices()
	available := max(1, h-pickerReservedRows)
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
		s := prefix + check + " " + slack.Safe(c.Label()) + "  " + string(c.Kind)
		if i == m.pickIndex {
			s = selected.Render(s)
		}
		rows = append(rows, s)
	}
	if len(cs) == 0 {
		rows = append(rows, " No matches in loaded directory pages.")
	}
	more := " Directory complete · " + keyLabel(keyDirectoryReload) + " reload first page"
	if m.directoryNext != "" {
		more = " More available · " + keyLabel(keyDirectoryNext) + " load next directory page"
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
		" " + keyLabel(keyFocus) + "        channels / messages     " + keyLabel(keyWrite) + " / " + keyLabel(keyCompose) + "  start composing",
		" " + keyLabel(keyNext) + "/" + keyLabel(keyPrevious) + " or " + keyLabel(keyUp) + "/" + keyLabel(keyDown) + " move/select              " + keyLabel(keySend) + "     send",
		" " + keyLabel(keyPageUp) + "/" + keyLabel(keyPageDown) + "  scroll history           " + keyLabel(keyCompose) + "      newline",
		" " + keyLabel(keyLast) + " / " + keyLabel(keyNewest) + "    newest + local read      " + keyLabel(keyEscape) + "        keep draft, navigate",
		" " + keyLabel(keyThread) + "          open selected thread    " + keyLabel(keySwitch) + "     switch, keep draft",
		" " + keyLabel(keyEscape) + "        leave thread",
		" " + keyLabel(keyOlder) + "          load older / next thread page",
		" " + keyLabel(keyInclude) + "          choose included conversations (" + keyLabel(keyFocus) + " toggles)",
		" " + keyLabel(keySearch) + ", " + keyLabel(keySwitch) + "  search included conversations",
		" " + keyLabel(keyRefresh) + "          refresh conversation    " + keyLabel(keyRead) + "  mark read locally",
		" " + keyLabel(keyRetry) + "          retry failed send       " + keyLabel(keyEditFailed) + "  restore failed text for editing",
		" " + keyLabel(keyCopy) + "          copy selected message with OSC52",
		" " + keyLabel(keyQuitQuiet) + "          quit (warns on drafts)  " + keyLabel(keyQuit) + " twice discards unsent text",
		"",
		" ··· BORDERS ···",
		" Solid = panes and composer; dashed = transient regions / separators.",
		" Dotted = subtle groups. [Header] and ▸ work without color.",
		" Unread is local observed activity; Slack read state is never changed.",
		" Demo: send /fail to inspect a failed delivery, then " + keyLabel(keyEscape) + " and " + keyLabel(keyRetry) + ".",
		" Any key closes help. Full shortcuts and limits are in README.md.",
	}
	wrapped := []string{}
	for _, r := range rows {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(r, w-helpInsetColumns, ""), "\n")...)
	}
	return m.transient("Keyboard guide · any key closes", wrapped, w, h)
}
