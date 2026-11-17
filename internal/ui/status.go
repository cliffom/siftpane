package ui

import "github.com/cliffom/siftpane/internal/core"

const messageTimeFormat = "Jan 02 15:04" // Message and refresh timestamps.

const statusTimeFormat = "15:04" // Compact local check time shared by source status.

// activityStatus derives progress from current state, so unrelated background
// completions cannot replace the selected timeline's loading indicator.
func (m *Model) activityStatus() string {
	if m.statusKind == statusAction {
		return m.status
	}
	if m.stopped {
		return m.credentialRecovery()
	}
	if d := m.deliveries[m.key()]; d != nil && (d.State == core.DeliveryPending || d.State == core.DeliveryFailed) {
		return deliveryStatus(d)
	}
	if m.active != "" {
		t := m.cache[m.key()]
		switch {
		case t == nil || (!t.Loaded && t.Err == nil):
			return "Loading messages…"
		case t.Cached && (t.Err != nil || t.CachedRefreshErr != nil):
			return "Cached refresh failed · " + keyLabel(keyRefresh) + " retry"
		case t.Cached:
			return "Cached " + t.Saved.Local().Format(messageTimeFormat) + " · R refresh"
		case t.RecentErr != nil && !t.Loaded:
			return t.RecentErr.Error() + " · " + keyLabel(keyRefreshDetails) + " details"
		case t.RecentErr != nil:
			return "Refresh failed · " + keyLabel(keyRefreshDetails) + " details · R retry"
		case t.PageErr != nil:
			return "Page failed · " + keyLabel(keyRefreshDetails) + " details · o retry"
		case t.More && m.thread != "" && !t.Busy:
			return "Partial replies · o next page · " + keyLabel(keyRefreshDetails) + " details"
		case t.Err != nil:
			return t.Err.Error() + " · " + keyLabel(keyRefresh) + " retry / refresh"
		case !t.Checked.IsZero() && m.statusNow.Sub(t.Checked) > staleAfter:
			return "Data stale · R refresh"
		}
	}
	if len(m.metadataBusy) > 0 {
		return m.restorationState()
	}
	if m.messageCacheErr != "" {
		return m.messageCacheErr
	}
	if text := m.followStatus(); text != "" {
		return text
	}
	if m.attention.SearchEnabled() || m.attentionErr != "" {
		return m.discoveryStatus()
	}
	return m.status
}

func deliveryStatus(d *core.Delivery) string {
	if d.State == core.DeliveryPending {
		return "Sending… draft retained"
	}
	if d.Uncertain {
		return "Delivery unknown; check Slack; " + keyLabel(keyRetry) + " retry"
	}
	text := "Send failed"
	if d.Err != nil {
		text += ": " + d.Err.Error()
	}
	return text + " · " + keyLabel(keyRetry) + " retry · " + keyLabel(keyEditFailed) + " edit"
}

func (m *Model) hasPersistentRecovery() bool {
	if m.stopped {
		return true
	}
	d := m.deliveries[m.key()]
	return d != nil && (d.State == core.DeliveryPending || d.State == core.DeliveryFailed)
}

func (m *Model) setStatus(text string, kind statusKind) {
	m.status, m.statusKind = text, kind
}

func (m *Model) gapMarker() string {
	if m.attention.Gap != nil {
		return " · gap"
	}
	return ""
}
