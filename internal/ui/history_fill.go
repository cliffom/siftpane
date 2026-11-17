package ui

import tea "charm.land/bubbletea/v2"

// Fill only the selected channel's visible window, retaining newest-following.
// A repeated oldest boundary, failure, browsing or page budget stops automatic work.
func (m *Model) fillHistory() tea.Cmd {
	if m.stopped || m.thread != "" || m.scroll != 0 || m.openTargets[m.key()] != "" {
		return nil
	}
	t := m.cache[m.key()]
	if t == nil || !t.Loaded || t.Busy || t.Err != nil || t.Cached || !t.More || len(t.Messages) == 0 || len(t.Messages) >= cacheLimit || t.FillPages >= historyFillPages {
		return nil
	}
	before := t.Messages[0].TS
	if before == "" || before == t.FillBefore {
		return nil
	}
	_, width, height, _, _ := m.dimensions()
	rows, _, _, _ := m.historyRows(t, width)
	if len(rows) >= height {
		return nil
	}
	cmd := m.fetchHistory(m.key(), true, true)
	if cmd != nil {
		t.FillPages++
		t.FillBefore = before
	}
	return cmd
}

func (m *Model) resetHistoryFill() {
	if t := m.cache[m.key()]; t != nil {
		t.FillPages = 0
		t.FillBefore = ""
	}
}
