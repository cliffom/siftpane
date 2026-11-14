package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/slack"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AC7BA"))

// Use the same foreground on every adjoining border segment so rounded
// corner glyphs do not change contrast where they meet the straight strokes.
var (
	muted        = lipgloss.NewStyle().Foreground(lipgloss.Color("#8693A5"))
	focusedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true)
	selected     = lipgloss.NewStyle().Foreground(lipgloss.Color("#BFE4DA")).Bold(true)
)

// View renders state without performing I/O or mutating the model.
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
	switch {
	case m.help:
		body = m.helpView(w, h-4)
	case m.mode != "":
		body = m.pickerView(w, h-4)
	default:
		body = m.mainView()
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

func (m *Model) mainView() string {
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
	return strings.Join(rows, "\n")
}
