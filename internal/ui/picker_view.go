package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

var backdrop = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).Faint(true)

func (m *Model) pickerSize() (w, h int) {
	return min(modalColumns, m.width-modalMarginColumns*2), min(modalRows, max(modalMinRows, m.height-modalMarginRows*2))
}

func (m *Model) pickerOverlay(content string, w, h int) string {
	mw, mh := m.pickerSize()
	return m.modalOverlay(content, m.pickerView(mw, mh), w, h)
}

func (m *Model) modalOverlay(content, modal string, w, h int) string {
	mw, mh := m.pickerSize()
	x, y := (w-mw)/2, (h-mh)/2
	// Strip highlights before dimming; the modal paints every cell opaquely.
	background := backdrop.Render(ansi.Strip(content))
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(background),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	))
	rows := padRows(strings.Split(canvas.Render(), "\n"), w, h)
	// Keep status and recovery readable while a modal has keyboard focus.
	rows[h-1] = strings.Split(content, "\n")[h-1]
	return strings.Join(rows, "\n")
}

func (m *Model) pickerView(w, h int) string {
	title := "Search conversations"
	if m.mode == pickerInclude {
		title = "Choose conversations"
	}
	rows := []string{m.picker.View(), muted.Render(" @ people · # channels")}
	cs := m.choices()
	available := max(1, h-modalReservedRows)
	index := max(0, min(m.pickIndex, len(cs)-1))
	start := max(0, index-available+1)
	suggestions := []string{}
	for i := start; i < min(len(cs), start+available); i++ {
		suggestions = append(suggestions, m.suggestionRow(cs[i], i == index))
	}
	if len(cs) == 0 {
		empty := " No matches in loaded conversations."
		id, direct, err := m.explicitPickerTarget()
		switch {
		case direct && err == nil:
			empty = " Conversation " + id
		case direct:
			empty = " Paste a Slack channel link or ID."
		case m.directoryBusy:
			empty = " No matches yet · loading directory…"
		case m.directoryNamesPending():
			empty = " No matches in resolved names yet."
		case m.directoryLoaded && m.directoryNext == "":
			empty = " No matching conversations."
		}
		suggestions = append(suggestions, empty)
	}
	rows = append(rows, padRows(suggestions, w-frameColumns, available)...)
	rows = append(rows, accent.Render(m.choiceAction(cs, index)))
	coverage := fmt.Sprintf(" %d loaded · more pages available", len(m.directory))
	_, direct, _ := m.explicitPickerTarget()
	switch {
	case direct:
		coverage = " Open in the current workspace"
	case m.directoryBusy:
		coverage = fmt.Sprintf(" %d loaded · searching directory…", len(m.directory))
	case !m.directoryLoaded:
		coverage = fmt.Sprintf(" %d loaded · coverage incomplete", len(m.directory))
		if len(m.directoryCached) > 0 {
			coverage = fmt.Sprintf(" %d loaded · cached suggestions", len(m.directory))
		}
	case m.directoryNext == "" && m.directoryNamesPending():
		coverage = fmt.Sprintf(" %d loaded · names incomplete", len(m.directory))
	case m.directoryNext == "":
		coverage = fmt.Sprintf(" %d loaded · directory complete", len(m.directory))
	}
	rows = append(rows, muted.Render(coverage), muted.Render(m.directoryState()))
	controls := " " + keyLabel(keyDirectoryNext) + " continue · " + keyLabel(keyDirectoryReload) + " reload"
	if direct {
		controls = " " + keyLabel(keyEscape) + " close · " + keyLabel(keyTemporary) + " temporary"
	}
	rows = append(rows, muted.Render(controls))
	return dialogFrame(title, rows, w, h, lipgloss.RoundedBorder())
}

func (m *Model) suggestionRow(c core.Conversation, focused bool) string {
	marker := "[+]"
	if len(m.attention.Pending[c.ID]) > 0 {
		marker = "[!]"
	}
	if m.attention.Kept[c.ID] {
		marker = "[✓]"
	}
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	label := slack.Safe(c.Label())
	if m.directoryCached[c.ID] {
		label = "cached " + label
	}
	text := prefix + marker + " " + label + " · " + string(c.Kind)
	if c.Handle != "" {
		text += " @" + slack.Safe(c.Handle)
	}
	if focused {
		return selected.Render(text)
	}
	return text
}

func (m *Model) choiceAction(cs []core.Conversation, index int) string {
	if m.directBusy {
		return " " + keyLabel(keyEscape) + " cancel lookup"
	}
	if _, direct, _ := m.pickerTarget(); direct {
		return " " + keyLabel(keyCompose) + " verify & open conversation"
	}
	if len(cs) == 0 {
		return " " + keyLabel(keyEscape) + " close · try another name"
	}
	c := cs[index]
	action := "open "
	if !m.included[c.ID] {
		action = "include & open "
		if len(m.attention.Kept) >= selectionLimit {
			return fmt.Sprintf(" Limit: %d included · remove via %s", selectionLimit, keyLabel(keyInclude))
		}
	}
	return " " + keyLabel(keyCompose) + " " + action + slack.Safe(c.Label())
}

func (m *Model) directoryState() string {
	_, direct, _ := m.explicitPickerTarget()
	switch {
	case m.directBusy:
		return " Checking conversation access…"
	case m.directErr != nil:
		return " Error: " + slack.Safe(m.directErr.Error())
	case direct:
		return " Access checked before opening"
	case m.directoryBusy:
		return " Loading directory…"
	case m.directoryErr != nil:
		return " Error: " + slack.Safe(m.directoryErr.Error())
	case m.directoryProfileBusy:
		return " Resolving DM names…"
	case !m.directoryLoaded:
		return " Directory not loaded · " + keyLabel(keyDirectoryReload) + " retry"
	case m.directoryNext != "" && m.directoryPagesLeft == 0:
		return " Search paused · " + keyLabel(keyDirectoryNext) + " continue"
	case m.directoryNamesPending():
		return " Names incomplete · reopen to retry"
	case m.directoryCacheErr != nil:
		return " Cache unavailable · live search works"
	case m.mode == pickerInclude:
		return " " + keyLabel(keyFocus) + " keep/unkeep · [✓] kept"
	default:
		return " " + keyLabel(keyTemporary) + " temporary · " + keyLabel(keyFocus) + " complete"
	}
}
