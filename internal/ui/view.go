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
		v.WindowTitle = m.windowTitle()
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
	if m.multipleAccounts {
		identity := slack.Safe(m.identity.Workspace) + "/" + slack.Safe(m.identity.User)
		account = " · " + identity
		if ansi.StringWidth(" · @"+username+account) <= available-accountPrefixColumns {
			account = " · @" + username + account
		}
		account = ansi.Truncate(account, max(0, available), "…")
	}
	workspace = ansi.Truncate(workspace, max(0, available-ansi.StringWidth(account)), "…") + account
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(workspace) - 1
	header := left + strings.Repeat(" ", max(0, gap)) + muted.Render(workspace) + " "
	body := m.mainView()
	statusText := m.activityStatus()
	if m.quitConfirm {
		statusText = "Unsent text is in memory. " + keyLabel(keyQuit) + " again exits and discards it; any other key cancels."
		if m.multipleAccounts {
			statusText = "Unsent · " + keyLabel(keyQuit) + " again discards/exits"
		}
	}
	if m.messageCacheErr != "" && !m.quitConfirm && !m.stopped {
		statusText = m.messageCacheErr
	}
	if m.profileCacheErr != "" && !m.quitConfirm {
		statusText = "Name cache not saved: " + m.profileCacheErr
	}
	if m.metadataErr != "" && !m.quitConfirm {
		statusText = "Metadata cache not saved: " + m.metadataErr
	}
	if m.persistErr != "" && !m.quitConfirm {
		statusText = "Preferences not saved: " + m.persistErr
	}
	helpKey := keyHelp
	if m.focus == focusCompose || m.mode != pickerClosed {
		helpKey = keyHelpGlobal
	}
	hint := " " + keyLabel(helpKey) + " help "
	statusColumns := w - ansi.StringWidth(hint)
	statusContent := " " + slack.Safe(statusText)
	status := fit(muted.Render(statusContent), statusColumns) + muted.Render(hint)
	// Action feedback gets the full row when the help entry would clip it.
	if m.quitConfirm || m.persistErr != "" || m.metadataErr != "" || m.profileCacheErr != "" || m.messageCacheErr != "" ||
		(m.statusKind == statusAction && ansi.StringWidth(statusContent) > statusColumns) {
		status = fit(muted.Render(statusContent), w)
	}
	content := strings.Join([]string{header, strings.Repeat(" ", w), body, status}, "\n")
	if m.mode != pickerClosed {
		content = m.pickerOverlay(content, w, h)
	}
	if m.help {
		mw, mh := m.pickerSize()
		content = m.modalOverlay(content, m.helpView(mw, mh), w, h)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()
	return v
}

// windowTitle reflects the same local activity as the sidebar, including only
// conversations selected for this workspace. It is independent of pane focus.
func (m *Model) windowTitle() string {
	for cid, unread := range m.unread {
		if unread && m.included[cid] {
			return windowTitle + " *"
		}
	}
	return windowTitle
}

const windowTitle = "Siftpane"

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
