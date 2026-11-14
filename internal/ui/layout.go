package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/slack"
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
	if focused && w >= 4 {
		title = "[" + ansi.Truncate(title[1:len(title)-1], w-4, "…") + "]"
	}
	text := ansi.Truncate(" "+title+" ", w, "")
	rest := strings.Repeat(stroke, max(0, w-ansi.StringWidth(text)))
	if focused {
		return focusedTitle.Render(text) + border.Render(rest)
	}
	return border.Render(text + rest)
}

func (m *Model) dimensions() (side, right, history, compose, inside int) {
	inside = max(1, m.height-6)
	compose = 3
	if m.height < 22 {
		compose = 2
	}
	side = 23
	if m.width < 76 {
		side = 0
	}
	right = m.width - 2
	if side > 0 {
		right -= side + 1
	}
	history = max(1, inside-compose-3)
	return
}

func (m *Model) resize() {
	_, right, _, compose, _ := m.dimensions()
	m.composer.SetWidth(max(5, right-6))
	m.composer.SetHeight(compose)
	m.picker.SetWidth(max(5, m.width-10))
}

func (m *Model) title() string {
	name := "Choose conversations · p"
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
			case "private":
				kind = "Private channel"
			case "dm":
				kind = "Direct message"
			case "group":
				kind = "Group DM"
			}
			if c.Kind == "group" && c.Members > 0 {
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
	return "Choose what belongs here · p"
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
