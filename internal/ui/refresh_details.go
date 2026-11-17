package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/slack"
)

// Details read retained state, independently of transient action/status feedback.
func (m *Model) refreshRows(w int) []string {
	rows := []string{keyLabel(keyRefresh) + " retry current view/sources · " + keyLabel(keyEscape) + " close", ""}
	if m.multipleAccounts {
		rows = append(rows, "Credential recovery: --login --account "+m.identity.Workspace+"/"+m.identity.User+" in another terminal, then "+keyLabel(keyReloadCredential)+" here from navigation. Drafts stay in this session.")
	}
	if m.stopped {
		rows = append(rows, "Account reads stopped: replace/revalidate its credential.")
	}
	keys := make([]string, 0, len(m.cache))
	for key := range m.cache {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t := m.cache[key]
		state := "Not checked"
		if !t.Checked.IsZero() {
			state = "Last refresh " + t.Checked.Local().Format(messageTimeFormat)
		}
		if t.Cached {
			state = "Cached " + t.Saved.Local().Format(messageTimeFormat) + "; live freshness unverified"
		}
		if t.Busy {
			state += " · checking"
		}
		rows = append(rows, "", key+": "+state)
		if _, thread := split(key); thread != "" {
			coverage := "Loaded reply pages complete; bounded retained window"
			if t.More {
				coverage = "Partial replies · " + keyLabel(keyOlder) + " next page from this thread"
			}
			rows = append(rows, coverage)
		}
		if t.RecentErr != nil {
			rows = append(rows, "Refresh failed: "+t.RecentErr.Error())
		}
		if t.PageErr != nil {
			rows = append(rows, "Page failed: "+t.PageErr.Error()+" · "+keyLabel(keyOlder)+" retry page")
		}
		if t.Err != nil && t.RecentErr == nil && t.PageErr == nil {
			rows = append(rows, t.Err.Error())
		}
	}
	rows = append(rows, "", "Discovery: "+m.discoveryStatus())
	if text := m.followStatus(); text != "" {
		rows = append(rows, "Local follows: "+text)
	}
	for _, key := range sortedFollowErrors(m.followErrors) {
		rows = append(rows, key+": "+m.followErrors[key])
	}
	if m.directoryErr != nil {
		rows = append(rows, "Directory: "+m.directoryErr.Error()+" · p then Ctrl+R retry")
	}
	wrapped := []string{}
	for _, row := range rows {
		for _, line := range strings.Split(ansi.Wrap(slack.Safe(row), w-helpInsetColumns, ""), "\n") {
			wrapped = append(wrapped, " "+line)
		}
	}
	return wrapped
}

func sortedFollowErrors(errors map[string]string) []string {
	keys := make([]string, 0, len(errors))
	for key := range errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *Model) refreshDetailsView(w, h int) string {
	rows := m.refreshRows(w)
	available := h - frameRows - helpFooterRows
	start := min(m.detailsScroll, max(0, len(rows)-available))
	visible := append([]string{}, rows[start:min(len(rows), start+available)]...)
	visible = padRows(visible, w-frameColumns, available)
	visible = append(visible, fmt.Sprintf(" %d–%d/%d %s/%s · %s close", start+1, min(len(rows), start+available), len(rows), keyLabel(keyUp), keyLabel(keyDown), keyLabel(keyEscape)))
	return dialogFrame("Refresh and recovery", visible, w, h, lipgloss.RoundedBorder())
}

func (m *Model) refreshDetailsKey(k string) tea.Cmd {
	w, h := m.pickerSize()
	available := h - frameRows - helpFooterRows
	end := max(0, len(m.refreshRows(w))-available)
	m.detailsScroll = min(m.detailsScroll, end)
	switch k {
	case keyDown, keyNext:
		m.detailsScroll = min(end, m.detailsScroll+1)
	case keyUp, keyPrevious:
		m.detailsScroll = max(0, m.detailsScroll-1)
	case keyPageDown:
		m.detailsScroll = min(end, m.detailsScroll+available)
	case keyPageUp:
		m.detailsScroll = max(0, m.detailsScroll-available)
	case keyRefresh:
		return m.refresh()
	default:
		m.refreshDetails = false
		return m.resumeInput()
	}
	return nil
}
