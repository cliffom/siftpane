package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func dialogFrame(title string, rows []string, w, h int, border lipgloss.Border) string {
	inner := w - frameColumns
	out := []string{accent.Render(border.TopLeft) + labelLine("["+title+"]", inner, border.Top, accent) + accent.Render(border.TopRight)}
	for _, row := range padRows(rows, inner, h-frameRows) {
		out = append(out, accent.Render(border.Left)+row+accent.Render(border.Right))
	}
	out = append(out, accent.Render(border.BottomLeft+strings.Repeat(border.Bottom, inner)+border.BottomRight))
	return strings.Join(out, "\n")
}

func (m *Model) helpView(w, h int) string {
	rows := []string{
		" NAVIGATE                           COMPOSE",
		" " + keyLabel(keyFocus) + "        channels / messages     " + keyLabel(keyWrite) + " / " + keyLabel(keyCompose) + "  start composing",
		" " + keyLabel(keyNext) + "/" + keyLabel(keyPrevious) + " or " + keyLabel(keyUp) + "/" + keyLabel(keyDown) + " move/select              " + keyLabel(keySend) + "     send",
		" " + keyLabel(keyPageUp) + "/" + keyLabel(keyPageDown) + "  scroll history           " + keyLabel(keyCompose) + "      newline",
		" " + keyLabel(keyLast) + " / " + keyLabel(keyNewest) + "    newest + local read      " + keyLabel(keyEscape) + "        keep draft, navigate",
		" " + keyLabel(keyThread) + "          open selected thread    " + keyLabel(keySwitch) + "     search, keep draft",
		" " + keyLabel(keyEscape) + "        leave thread",
		" " + keyLabel(keyOlder) + "          load older / next thread page",
		" " + keyLabel(keyInclude) + "          choose included conversations (" + keyLabel(keyFocus) + " toggles)",
		" " + keyLabel(keySearch) + ", " + keyLabel(keySwitch) + "  search @people / #channels",
		" " + keyLabel(keyRefresh) + "          refresh conversation    " + keyLabel(keyRead) + "  mark read locally",
		" " + keyLabel(keyRetry) + "          retry failed send       " + keyLabel(keyEditFailed) + "  restore failed text for editing",
		" " + keyLabel(keyCopy) + "          copy selected message with OSC52",
		" " + keyLabel(keyQuitQuiet) + "          quit (warns on drafts)  " + keyLabel(keyQuit) + " twice discards unsent text",
		"",
		" ··· BORDERS ···",
		" Solid = panes, composer, search; dashed = help / separators.",
		" Dotted = subtle groups. [Header] and ▸ work without color.",
		" Unread is local observed activity; Slack read state is never changed.",
		" Demo: send /fail to inspect a failed delivery, then " + keyLabel(keyEscape) + " and " + keyLabel(keyRetry) + ".",
		" Any key closes help. Full shortcuts and limits are in README.md.",
	}
	wrapped := []string{}
	for _, r := range rows {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(r, w-helpInsetColumns, ""), "\n")...)
	}
	border := lipgloss.RoundedBorder()
	border.Top, border.Bottom = "╌", "╌"
	border.Left, border.Right = "╎", "╎"
	return dialogFrame("Keyboard guide · any key closes", wrapped, w, h, border)
}
