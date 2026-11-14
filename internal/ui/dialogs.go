package ui

import (
	"fmt"
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

// Single-column copy wraps at the current modal width; every line is scrollable.
func helpRows(w int) []string {
	rows := []string{
		"NAVIGATE",
		keyLabel(keyFocus) + "  switch sidebar / messages",
		keyLabel(keyNext) + "/" + keyLabel(keyPrevious) + " or " + keyLabel(keyUp) + "/" + keyLabel(keyDown) + "  move / select",
		keyLabel(keyPageUp) + "/" + keyLabel(keyPageDown) + "  scroll history",
		keyLabel(keyLast) + " / " + keyLabel(keyNewest) + "  newest + local read",
		keyLabel(keyWrite) + " / " + keyLabel(keyCompose) + "  compose",
		keyLabel(keyThread) + "  open selected thread",
		keyLabel(keyEscape) + "  leave thread / focus sidebar",
		keyLabel(keyOlder) + "  older history / next reply page",
		keyLabel(keyRefresh) + "  refresh / retry saved metadata",
		keyLabel(keyRead) + "  mark read locally",
		keyLabel(keyCopy) + "  copy message via OSC52",
		"",
		"COMPOSE AND RECOVER",
		keyLabel(keySend) + "  send; " + keyLabel(keyCompose) + "  newline",
		keyLabel(keyEscape) + "  navigate, keep draft",
		keyLabel(keyRetry) + "  retry failed send from navigation",
		"Uncertain delivery: inspect Slack before retrying; confirm with a second " + keyLabel(keyRetry) + ". No automatic send retries.",
		keyLabel(keyEditFailed) + "  restore failed text for editing",
		keyLabel(keyQuitQuiet) + "  quit from navigation (warns on drafts)",
		keyLabel(keyQuit) + " twice  exit and discard unsent text",
		"",
		"SEARCH AND INCLUDE",
		keyLabel(keySearch) + "  search from navigation",
		keyLabel(keySwitch) + " / Cmd+K if delivered  search, keep draft",
		keyLabel(keyInclude) + "  choose included conversations",
		"@ people / # channels; matches loaded metadata",
		keyLabel(keyUp) + "/" + keyLabel(keyDown) + " or " + keyLabel(keyPickerUp) + "/" + keyLabel(keyPickerDown) + "  move",
		keyLabel(keyFocus) + "  complete in search; include/remove in chooser",
		keyLabel(keyCompose) + "  explicitly include & open",
		keyLabel(keyDirectoryNext) + "  next directory page",
		keyLabel(keyDirectoryReload) + "  reload / retry directory",
		keyLabel(keyEscape) + "  close, restore focus and draft",
		"",
		"LOCAL ACTIVITY",
		"● unread here; ✎ retained conversation draft",
		"Unread means locally observed activity. Slack read positions are never changed.",
		"Drafts and messages stay in memory; drafts do not survive exit.",
		"",
		"HELP AND APPEARANCE",
		keyLabel(keyHelp) + "  help from navigation; " + keyLabel(keyHelpGlobal) + "  help anywhere",
		"Solid panes/composer/modals; dashed separators. [Title] and ▸ show focus without color.",
		"Full shortcuts and limits: docs/USAGE.md",
	}
	wrapped := []string{}
	for _, row := range rows {
		for _, line := range strings.Split(ansi.Wrap(row, w-helpInsetColumns, ""), "\n") {
			wrapped = append(wrapped, " "+line)
		}
	}
	return wrapped
}

func (m *Model) helpView(w, h int) string {
	rows := helpRows(w)
	available := h - frameRows - helpFooterRows
	start := min(m.helpScroll, max(0, len(rows)-available))
	visible := append([]string{}, rows[start:min(len(rows), start+available)]...)
	visible = padRows(visible, w-frameColumns, available)
	visible = append(visible, muted.Render(fmt.Sprintf(" %d–%d/%d %s/%s · %s close", start+1, min(len(rows), start+available), len(rows), keyLabel(keyUp), keyLabel(keyDown), keyLabel(keyEscape))))
	return dialogFrame("Keyboard guide", visible, w, h, lipgloss.RoundedBorder())
}

func (m *Model) helpKey(k string) {
	w, h := m.pickerSize()
	available := h - frameRows - helpFooterRows
	end := max(0, len(helpRows(w))-available)
	// Normalize after resizing before applying another scroll operation.
	m.helpScroll = min(m.helpScroll, end)
	switch k {
	case keyDown, keyNext:
		m.helpScroll = min(end, m.helpScroll+1)
	case keyUp, keyPrevious:
		m.helpScroll = max(0, m.helpScroll-1)
	case keyPageDown:
		m.helpScroll = min(end, m.helpScroll+available)
	case keyPageUp:
		m.helpScroll = max(0, m.helpScroll-available)
	case keyNewest, keyLast:
		m.helpScroll = end
	default:
		// Consume the close key: it must never act on the obscured conversation.
		m.help = false
	}
}
