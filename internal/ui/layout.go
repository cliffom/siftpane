package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

// DefaultWidth and DefaultHeight are the initial viewport and CLI snapshot defaults.
const (
	DefaultWidth  = 100
	DefaultHeight = 30
	// MinWidth and MinHeight define the supported viewport in display cells.
	MinWidth  = 40
	MinHeight = 14
)

// Layout measurements are terminal columns/rows, including shared edges.
const (
	sidebarBreakpoint      = 76
	sidebarColumns         = 23
	compactHeight          = 22
	composerRows           = 3
	compactComposerRows    = 2
	chromeRows             = 4 // Header, blank separator, status, shortcuts.
	frameRows              = 2 // Top and bottom shared frame.
	frameColumns           = 2
	composerBorderRows     = 2
	contextRows            = 1
	composerInsetColumns   = 4 // Outer padding plus inset border.
	composerPaddingColumns = 2
	minInputColumns        = 5
	helpInsetColumns       = 4 // Transient frame and text padding.
	focusedTitleColumns    = 4 // Two brackets and surrounding spaces.
	scrollPageDivisor      = 3
	minScrollRows          = 3
	modalColumns           = 72
	modalRows              = 16
	modalMarginColumns     = 4
	modalMarginRows        = 3
	modalMinRows           = 10
	modalReservedRows      = 8 // Frame, input, scope hint, action, coverage, state, directory keys.
	sidebarRestorationRows = 1 // Keep loading/recovery visible even when the list fills.
	sidebarActivityRows    = 3
	sidebarSelectionRows   = 8
)

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

// Style each segment independently: wrapping an amber title inside a border
// style can carry the title's foreground into the remaining border strokes.
func labelLine(title string, w int, stroke string, border lipgloss.Style) string {
	focused := strings.HasPrefix(title, "[") && strings.HasSuffix(title, "]")
	if focused && w >= focusedTitleColumns {
		title = "[" + ansi.Truncate(title[1:len(title)-1], w-focusedTitleColumns, "…") + "]"
	}
	text := ansi.Truncate(" "+title+" ", w, "")
	rest := strings.Repeat(stroke, max(0, w-ansi.StringWidth(text)))
	if focused {
		return focusedTitle.Render(text) + border.Render(rest)
	}
	return border.Render(text + rest)
}

func (m *Model) dimensions() (side, right, history, compose, inside int) {
	inside = max(1, m.height-chromeRows-frameRows)
	compose = composerRows
	if m.height < compactHeight {
		compose = compactComposerRows
	}
	side = sidebarColumns
	if m.width < sidebarBreakpoint {
		side = 0
	}
	right = m.width - frameColumns
	if side > 0 {
		right -= side + 1
	}
	history = max(1, inside-compose-composerBorderRows-contextRows)
	return
}

func (m *Model) resize() {
	_, right, _, compose, _ := m.dimensions()
	m.composer.SetWidth(max(minInputColumns, right-composerInsetColumns-composerPaddingColumns))
	m.composer.SetHeight(compose)
	pickerWidth, _ := m.pickerSize()
	// Bubbles reserves an extra cursor cell beyond the text field width.
	m.picker.SetWidth(max(minInputColumns, pickerWidth-frameColumns-pickerPromptColumns-1))
	m.picker.SetCursor(m.picker.Position())
}

func (m *Model) title() string {
	name := "Choose conversations · " + keyLabel(keyInclude)
	for _, c := range m.directory {
		if c.ID == m.active {
			name = c.Label()
			break
		}
	}
	if m.thread != "" {
		name += " / Thread"
	}
	return slack.Safe(name)
}

func (m *Model) conversationContext() string {
	for _, c := range m.directory {
		if c.ID == m.active {
			kind := "Channel"
			switch c.Kind {
			case core.ConversationPrivate:
				kind = "Private channel"
			case core.ConversationDM:
				kind = "Direct message"
			case core.ConversationGroup:
				kind = "Group DM"
			}
			if c.Kind == core.ConversationGroup && c.Members > 0 {
				kind += fmt.Sprintf(" · %d people", c.Members)
			}
			detail := strings.TrimSpace(c.Topic)
			if detail == "" {
				detail = strings.TrimSpace(c.Purpose)
			}
			if detail != "" {
				kind += " · " + strings.Join(strings.Fields(slack.Readable(detail)), " ")
			}
			if m.thread != "" {
				kind += " · Thread replies"
			}
			return kind
		}
	}
	return "Choose what belongs here · " + keyLabel(keyInclude)
}

func padRows(rows []string, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		if i < len(rows) {
			out[i] = fit(rows[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}
