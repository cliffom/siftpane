package ui

// scrollPinnedNewest keeps the newest viewport while pausing automatic following.
// Cursor selection can move inside that fixed window until an edge is reached.
const scrollPinnedNewest = -2

// readingAnchor holds a message identity and its row within the visible window.
// Selection-following mode (-1) already derives its position from the selection.
type readingAnchor struct {
	ts      string
	offset  int
	padding int
	height  int
}

func (m *Model) captureReadingAnchor() readingAnchor {
	t := m.cache[m.key()]
	if (m.scroll <= 0 && m.scroll != scrollPinnedNewest) || t == nil || len(t.Messages) == 0 {
		return readingAnchor{}
	}
	_, w, h, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(t, w)
	view := m.historyViewport(rows, starts, first, last, w, h)
	start := view.start
	for i := len(starts) - 1; i >= 0; i-- {
		if starts[i] <= start {
			return readingAnchor{ts: t.Messages[i].TS, offset: start - starts[i], padding: view.padding, height: h}
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
			// Preserve blank screen rows as well as the first visible source row.
			if anchor.padding > 0 {
				m.pinHistoryViewport(historyViewportPosition{start: starts[i] + offset, padding: max(0, anchor.padding+h-anchor.height), height: h})
			} else {
				m.pinHistoryStart(len(rows), h, starts[i]+offset)
			}
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
	if t == nil || len(t.Messages) == 0 || (m.scroll <= 0 && m.scroll != scrollPinnedNewest) {
		return
	}
	_, w, h, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(t, w)
	view := m.historyViewport(rows, starts, first, last, w, h)
	start, end := view.start, view.end(len(rows), h)
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

// Move actions between visible messages without moving their viewport. A target
// crossing an edge is revealed; oversized targets start at their own header.
func (m *Model) moveMessageSelection(delta int) {
	t := m.line(m.key())
	if len(t.Messages) == 0 {
		return
	}
	nextIndex := max(0, min(len(t.Messages)-1, m.index+delta))
	if nextIndex == m.index {
		return
	}
	_, width, height, _, _ := m.dimensions()
	rows, starts, first, last := m.historyRows(t, width)
	view := m.historyViewport(rows, starts, first, last, width, height)
	start, end := view.start, view.end(len(rows), height)
	targetStart, targetEnd := starts[nextIndex], len(rows)
	if nextIndex+1 < len(starts) {
		targetEnd = starts[nextIndex+1]
	}
	m.index = nextIndex
	switch {
	case targetEnd-targetStart > height:
		m.scroll = -1
	case targetStart >= start && targetEnd-1 <= end:
		switch m.scroll {
		case 0, -1:
			m.pinHistoryViewport(view.historyViewportPosition)
		}
	case targetStart < start:
		m.pinHistoryStart(len(rows), height, targetStart)
	default:
		m.scroll = -1
	}
}

func (m *Model) pinHistoryViewport(position historyViewportPosition) {
	m.scroll = scrollPinnedNewest
	m.pinnedHistory = &position
}

func (m *Model) pinHistoryStart(rows, height, start int) {
	m.scroll = rows - height - start
	if m.scroll <= 0 {
		m.pinHistoryViewport(historyViewportPosition{start: start, height: height})
	}
}

// Page movement starts at the current pinned source row, not the newest page.
func (m *Model) historyScrollOffset() int {
	if m.scroll != scrollPinnedNewest || m.pinnedHistory == nil {
		return max(0, m.scroll)
	}
	if t := m.cache[m.key()]; t != nil {
		_, width, height, _, _ := m.dimensions()
		rows, _, _, _ := m.historyRows(t, width)
		return max(0, len(rows)-height-m.pinnedHistory.start)
	}
	return 0
}
