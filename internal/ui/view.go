package ui

import (
	"fmt"
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
	if w < MinWidth || h < MinHeight {
		v := tea.NewView(fit(fmt.Sprintf("Siftpane · resize to at least %d × %d", MinWidth, MinHeight), max(1, w)))
		v.AltScreen = true
		return v
	}
	left := accent.Render(" siftpane")
	workspace := strings.Join(strings.Fields(slack.Safe(m.identity.WorkspaceName)), " ")
	available := w - ansi.StringWidth(left) - 3
	username := strings.TrimPrefix(strings.Join(strings.Fields(slack.Safe(m.identity.Username)), ""), "@")
	account := ""
	if username != "" {
		account = ansi.Truncate(" · @"+username, max(0, available-accountPrefixColumns), "…")
	}
	workspace = ansi.Truncate(workspace, available-ansi.StringWidth(account), "…") + account
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(workspace) - 1
	header := left + strings.Repeat(" ", max(0, gap)) + muted.Render(workspace) + " "
	body := ""
	switch {
	case m.help:
		body = m.helpView(w, h-chromeRows)
	case m.mode != pickerClosed:
		body = m.pickerView(w, h-chromeRows)
	default:
		body = m.mainView()
	}
	statusText := m.status
	if m.quitConfirm {
		statusText = "Unsent text is in memory. " + keyLabel(keyQuit) + " again exits and discards it; any other key cancels."
	}
	if m.persistErr != "" && !m.quitConfirm {
		statusText = "Preferences not saved: " + m.persistErr
	}
	status := fit(muted.Render(" "+slack.Safe(statusText)), w)
	hint := " " + keyLabel(keyFocus) + " focus · " + keyLabel(keySearch) + " switch · " + keyLabel(keyWrite) + " write · " + keyLabel(keyThread) + " thread · " + keyLabel(keyHelp) + " help"
	if m.focus == focusCompose {
		hint = " " + keyLabel(keySend) + " send · " + keyLabel(keyCompose) + " newline · " + keyLabel(keyEscape) + " navigate · " + keyLabel(keySwitch) + " switch"
	}
	if m.mode != pickerClosed {
		hint = " " + keyLabel(keyUp) + "/" + keyLabel(keyDown) + " move · " + keyLabel(keyFocus) + " toggle · " + keyLabel(keyCompose) + " open · " + keyLabel(keyEscape) + " close"
	}
	if w < sidebarBreakpoint && m.focus != focusCompose && m.mode == pickerClosed {
		hint = " " + keyLabel(keyFocus) + " focus · " + keyLabel(keySearch) + " switch · " + keyLabel(keyWrite) + " write · " + keyLabel(keyHelp) + " help"
	}
	v := tea.NewView(strings.Join([]string{header, strings.Repeat(" ", w), body, status, fit(accent.Render(hint), w)}, "\n"))
	v.AltScreen = true
	v.WindowTitle = "Siftpane"
	return v
}

func (m *Model) mainView() string {
	side, right, hist, comp, inside := m.dimensions()
	showList := side == 0 && m.focus == focusChannels
	title := m.title()
	if m.focus == focusMessages {
		title = "[" + title + "]"
	}
	leftTitle := "Messages"
	if m.focus == focusChannels {
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

const accountPrefixColumns = 4 // Space, separator, space and @ before username.
