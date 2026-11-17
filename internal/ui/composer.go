package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"

	"github.com/cliffom/siftpane/internal/core"
)

func composerStyles() textarea.Styles {
	styles := textarea.DefaultDarkStyles()
	styles.Focused.Placeholder = muted
	styles.Blurred.Placeholder = muted
	// Keep the editing row on the terminal background instead of relying on
	// the theme's ANSI black, which can wash out placeholder text.
	styles.Focused.CursorLine = styles.Focused.Text
	return styles
}

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
		title += " · @" + accountText(m.identity.Username) + " · " + accountText(m.identity.WorkspaceName)
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
