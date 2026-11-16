package ui

import (
	"strings"

	"github.com/cliffom/siftpane/internal/core"
)

// The inset frame keeps the editing surface separate from both history and
// application chrome. Its two border rows are reserved by dimensions().
func (m *Model) composerRows(w, h int) []string {
	stroke := "─"
	border := muted
	title := "Message"
	hint := ""
	if m.thread != "" {
		title = "Thread reply"
	}
	if m.focus == focusCompose {
		border = accent
	}
	if d := m.deliveries[m.key()]; d != nil {
		switch d.State {
		case core.DeliveryPending:
			title += " · sending…"
			hint = "Text kept until acknowledged"
		case core.DeliveryFailed:
			title += " · failed"
			hint = keyLabel(keyRetry) + " retry · " + keyLabel(keyEditFailed) + " edit · text kept"
		}
	}
	if m.multipleAccounts {
		title += " · @" + m.identity.Username + " · " + m.identity.WorkspaceName
	}
	if m.focus == focusCompose {
		title = "[" + title + "]"
	}
	inner := w - composerInsetColumns
	rows := []string{" " + border.Render("╭") + labelLine(title, inner, stroke, border) + border.Render("╮") + " "}
	for _, line := range padRows(strings.Split(m.composer.View(), "\n"), w-composerInsetColumns-composerPaddingColumns, h) {
		rows = append(rows, " "+border.Render("│")+" "+line+" "+border.Render("│")+" ")
	}
	bottom := border.Render(strings.Repeat(stroke, inner))
	if hint != "" {
		bottom = labelLine(hint, inner, stroke, border)
	}
	rows = append(rows, " "+border.Render("╰")+bottom+border.Render("╯")+" ")
	return rows
}
