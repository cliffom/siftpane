package ui

import "github.com/cliffom/siftpane/internal/core"

// activityStatus derives progress from current state, so unrelated background
// completions cannot replace the selected timeline's loading indicator.
func (m *Model) activityStatus() string {
	if m.stopped || m.statusKind == statusAction {
		return m.status
	}
	if d := m.deliveries[m.key()]; d != nil && (d.State == core.DeliveryPending || d.State == core.DeliveryFailed) {
		return m.status
	}
	if m.active != "" {
		t := m.cache[m.key()]
		switch {
		case t == nil || (!t.Loaded && t.Err == nil):
			return "Loading messages…"
		case t.Err != nil:
			return t.Err.Error() + " · " + keyLabel(keyRefresh) + " retry / refresh"
		case t.Busy:
			return "Refreshing messages…"
		}
	}
	if len(m.metadataBusy) > 0 {
		return m.restorationState()
	}
	if m.attention.Enabled || m.attentionErr != "" {
		return m.discoveryStatus()
	}
	return m.status
}

func (m *Model) setStatus(text string, kind statusKind) {
	m.status, m.statusKind = text, kind
}
