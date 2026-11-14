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
	x, y := (w-mw)/2, (h-mh)/2
	// Strip prior highlights before applying one quiet backdrop style. The modal
	// owns its colors and paints every cell, including padding over message text.
	background := backdrop.Render(ansi.Strip(content))
	modal := m.pickerView(mw, mh)
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(background),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	))
	rows := padRows(strings.Split(canvas.Render(), "\n"), w, h)
	// Keep the unsent-exit warning and save failures readable above background updates.
	if m.quitConfirm || m.persistErr != "" || m.metadataErr != "" {
		rows[h-2] = strings.Split(content, "\n")[h-2]
	}
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
		suggestions = append(suggestions, " No matches in loaded conversations.")
	}
	rows = append(rows, padRows(suggestions, w-frameColumns, available)...)
	rows = append(rows, accent.Render(m.choiceAction(cs, index)))
	coverage := fmt.Sprintf(" %d loaded · more pages available", len(m.directory))
	if !m.directoryLoaded {
		coverage = fmt.Sprintf(" %d loaded · coverage incomplete", len(m.directory))
	} else if m.directoryNext == "" {
		coverage = fmt.Sprintf(" %d loaded · directory complete", len(m.directory))
	}
	rows = append(rows, muted.Render(coverage), muted.Render(m.directoryState()))
	rows = append(rows, muted.Render(" "+keyLabel(keyDirectoryNext)+" more · "+keyLabel(keyDirectoryReload)+" reload"))
	return dialogFrame(title, rows, w, h, lipgloss.RoundedBorder())
}

func (m *Model) suggestionRow(c core.Conversation, focused bool) string {
	marker := "[+]"
	if m.included[c.ID] {
		marker = "[✓]"
	}
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	text := prefix + marker + " " + slack.Safe(c.Label()) + " · " + string(c.Kind)
	if c.Handle != "" {
		text += " @" + slack.Safe(c.Handle)
	}
	if focused {
		return selected.Render(text)
	}
	return text
}

func (m *Model) choiceAction(cs []core.Conversation, index int) string {
	if len(cs) == 0 {
		return " " + keyLabel(keyEscape) + " close · try another name"
	}
	c := cs[index]
	action := "open "
	if !m.included[c.ID] {
		action = "include & open "
		if len(m.included) >= selectionLimit {
			return fmt.Sprintf(" Limit: %d included · remove via %s", selectionLimit, keyLabel(keyInclude))
		}
	}
	return " " + keyLabel(keyCompose) + " " + action + slack.Safe(c.Label())
}

func (m *Model) directoryState() string {
	switch {
	case m.directoryBusy:
		return " Loading directory…"
	case m.directoryErr != nil:
		return " Error: " + slack.Safe(m.directoryErr.Error())
	case !m.directoryLoaded:
		return " Directory not loaded · " + keyLabel(keyDirectoryReload) + " retry"
	case m.mode == pickerInclude:
		return " " + keyLabel(keyFocus) + " include/remove · [✓] included"
	default:
		return " " + keyLabel(keyFocus) + " complete · " + keyLabel(keyEscape) + " close"
	}
}
