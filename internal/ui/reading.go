package ui

// readingAnchor holds a message identity and its row within the visible window.
// Selection-following mode (-1) already derives its position from the selection.
type readingAnchor struct {
	ts     string
	offset int
}

func (m *Model) captureReadingAnchor() readingAnchor {
	t := m.cache[m.key()]
	if m.scroll <= 0 || t == nil || len(t.Messages) == 0 {
		return readingAnchor{}
	}
	_, w, h, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(t, w)
	start, _ := historyStart(rows, starts, first, last, w, h, m.scroll)
	for i := len(starts) - 1; i >= 0; i-- {
		if starts[i] <= start {
			return readingAnchor{ts: t.Messages[i].TS, offset: start - starts[i]}
		}
	}
	return readingAnchor{}
}

func (m *Model) restoreReadingAnchor(anchor readingAnchor) {
	if anchor.ts == "" {
		return
	}
	t := m.cache[m.key()]
	if t == nil || len(t.Messages) == 0 {
		return
	}
	_, w, h, _, _ := m.dimensions()
	rows, starts, _, _ := m.historyRows(t, w)
	for i, msg := range t.Messages {
		if msg.TS >= anchor.ts {
			offset := 0
			if msg.TS == anchor.ts {
				end := len(rows)
				if i+1 < len(starts) {
					end = starts[i+1]
				}
				offset = min(anchor.offset, max(0, end-starts[i]-1))
			}
			// Positive scroll retains browsing even when the viewport is underfull.
			m.scroll = max(1, len(rows)-h-starts[i]-offset)
			return
		}
	}
	// The entire anchored range was evicted. Show the nearest retained selection.
	m.scroll = -1
}

// Scrolling can move the old action target out of view. Keep actions attached
// to a message actually visible in the new viewport.
func (m *Model) keepSelectionVisible() {
	t := m.cache[m.key()]
	if t == nil || len(t.Messages) == 0 || m.scroll <= 0 {
		return
	}
	_, w, h, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(t, w)
	start, continuation := historyStart(rows, starts, first, last, w, h, m.scroll)
	end := start + h
	if continuation != "" {
		end--
	}
	if first >= start && first < end {
		return
	}
	for i := len(starts) - 1; i >= 0; i-- {
		if starts[i] <= start {
			m.index = i
			return
		}
	}
}

// Automatic layout/content changes preserve the selected action identity. If
// expansion separates it from the old top anchor, reveal the selection instead.
func (m *Model) preserveVisibleSelection() {
	index := m.index
	m.keepSelectionVisible()
	if m.index != index {
		m.index, m.scroll = index, -1
	}
}
