package ui

import (
	"strings"
)

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
