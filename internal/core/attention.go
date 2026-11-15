package core

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"time"
)

// AttentionReason describes why a conversation needs explicit handling.
type AttentionReason string

const (
	// AttentionManual is a conversation explicitly brought in temporarily.
	AttentionManual AttentionReason = "temporary"
	// AttentionMention is a message directly mentioning the authenticated person.
	AttentionMention AttentionReason = "mention"
	// AttentionChannel identifies an explicit @channel broadcast.
	AttentionChannel AttentionReason = "@channel"
	// AttentionHere identifies an explicit @here broadcast.
	AttentionHere AttentionReason = "@here"
	// AttentionReply identifies another person's reply to a locally followed thread.
	AttentionReply AttentionReason = "reply"
	// FollowedThreads bounds explicit local subscriptions per account.
	FollowedThreads = 32
	// AttentionConversations bounds active attention independently of Kept.
	AttentionConversations = 128
	// AttentionTriggers bounds active and handled identities together per account.
	AttentionTriggers = 1000
	// AttentionLookback is the supported search/replay window, not an archive guarantee.
	AttentionLookback = 7 * 24 * time.Hour
)

// AttentionTrigger identifies one relevant message or explicit local addition.
// Message timestamps retain their original precision; Thread identifies its root.
type AttentionTrigger struct {
	ID     string          `json:"id"`
	TS     string          `json:"ts,omitempty"`
	Thread string          `json:"thread,omitempty"`
	Reason AttentionReason `json:"reason"`
	At     time.Time       `json:"at"`
}

// AttentionGap bounds known uncovered discovery intervals. Checked periods may
// exist between these bounds when multiple gaps are summarized together.
type AttentionGap struct {
	From  time.Time `json:"from"`
	Until time.Time `json:"until"`
}

// AttentionState contains durable account-local preferences and minimal identities.
// Bodies and display names never belong in this state.
type AttentionState struct {
	ChannelEnabled       bool                          `json:"channel_enabled,omitempty"`
	HereEnabled          bool                          `json:"here_enabled,omitempty"`
	Followed             map[string]FollowedThread     `json:"followed,omitempty"`
	MessageCache         bool                          `json:"message_cache,omitempty"`
	GapAcknowledgedUntil time.Time                     `json:"gap_acknowledged_until"`
	Gap                  *AttentionGap                 `json:"gap,omitempty"`
	Kept                 map[string]bool               `json:"kept"`
	Pending              map[string][]AttentionTrigger `json:"pending"`
	Handled              map[string]AttentionTrigger   `json:"handled"`
	Enabled              bool                          `json:"mentions_enabled"`
	Checkpoint           time.Time                     `json:"checkpoint"`
	LastCheck            time.Time                     `json:"last_check"`
	Floor                time.Time                     `json:"replay_floor"`
}

// NewAttention migrates existing inclusion to Kept without creating attention.
func NewAttention(included map[string]bool) AttentionState {
	kept := map[string]bool{}
	for cid, v := range included {
		if v {
			kept[cid] = true
		}
	}
	return AttentionState{Kept: kept, Pending: map[string][]AttentionTrigger{}, Handled: map[string]AttentionTrigger{}}
}

// Clone isolates a candidate change until its durable write succeeds.
func (a AttentionState) Clone() AttentionState {
	b := a
	if a.Gap != nil {
		gap := *a.Gap
		b.Gap = &gap
	}
	b.Followed = maps.Clone(a.Followed)
	b.Kept = maps.Clone(a.Kept)
	b.Handled = maps.Clone(a.Handled)
	b.Pending = make(map[string][]AttentionTrigger, len(a.Pending))
	for cid, ts := range a.Pending {
		b.Pending[cid] = slices.Clone(ts)
	}
	return b
}

// Add deduplicates replay and groups attention by conversation; it never dismisses.
func (a *AttentionState) Add(cid string, t AttentionTrigger) error {
	// Concurrent source commands may have captured an older replay window before
	// another completed result pruned identities. Never accept replay behind it.
	if t.Reason != AttentionManual && t.At.Before(a.Floor) {
		return nil
	}
	if _, ok := a.Handled[cid+"/"+t.ID]; ok {
		return nil
	}
	for i, old := range a.Pending[cid] {
		if old.ID == t.ID {
			if old.Thread == "" && t.Thread != "" {
				a.Pending[cid][i].Thread = t.Thread
			}
			if attentionPriority(t.Reason) < attentionPriority(old.Reason) {
				a.Pending[cid][i].Reason = t.Reason
			}
			return nil
		}
	}
	if len(a.Pending[cid]) == 0 && len(a.Pending) >= AttentionConversations {
		return errors.New("attention full; dismiss an entry and retry")
	}
	count := len(a.Handled)
	for _, ts := range a.Pending {
		count += len(ts)
	}
	if count >= AttentionTriggers {
		return errors.New("attention identities full; coverage incomplete")
	}
	a.Pending[cid] = append(a.Pending[cid], t)
	slices.SortFunc(a.Pending[cid], func(a, b AttentionTrigger) int {
		if order := a.At.Compare(b.At); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return nil
}

// Dismiss handles only the currently observed trigger set for a conversation.
func (a *AttentionState) Dismiss(cid string) {
	for _, t := range a.Pending[cid] {
		a.Handled[cid+"/"+t.ID] = t
	}
	delete(a.Pending, cid)
}

// Prune expires handled identities only behind a monotonic replay floor. Active
// entries survive indefinitely until explicit dismissal, even beyond the window.
func (a *AttentionState) Prune(floor time.Time) {
	if floor.Before(a.Floor) {
		floor = a.Floor
	}
	a.Floor = floor
	for k, t := range a.Handled {
		if t.At.Before(floor) {
			delete(a.Handled, k)
		}
	}
}

// MentionQuery is an immutable bounded discovery request, independent of Kept.
type MentionQuery struct {
	Reason      AttentionReason
	User        string
	From, Until time.Time
	Page        int
}

// MentionMatch carries only discovery metadata and the exact opening target.
type MentionMatch struct {
	Conversation Conversation
	Trigger      AttentionTrigger
}

// MentionPage records pagination explicitly; incomplete pages never advance a checkpoint.
type MentionPage struct {
	Matches []MentionMatch
	More    bool
	Total   int
}

// RecordGap preserves the bounds of all known missed intervals until the user
// acknowledges them. Successful scans never implicitly erase this record.
func (a *AttentionState) RecordGap(from, until time.Time) {
	if from.Before(a.GapAcknowledgedUntil) {
		from = a.GapAcknowledgedUntil
	}
	if !until.After(from) {
		return
	}
	if a.Gap == nil {
		a.Gap = &AttentionGap{From: from, Until: until}
		return
	}
	if from.Before(a.Gap.From) {
		a.Gap.From = from
	}
	if until.After(a.Gap.Until) {
		a.Gap.Until = until
	}
}

// FollowedThread retains only explicit local scope and completed coverage.
// It does not mirror Slack's followed-thread list or notification preferences.
type FollowedThread struct {
	Conversation string    `json:"conversation"`
	Root         string    `json:"root"`
	Checkpoint   time.Time `json:"checkpoint"`
	LastCheck    time.Time `json:"last_check"`
}

// SearchEnabled reports whether any independent search source is opted in.
func (a AttentionState) SearchEnabled() bool { return a.Enabled || a.ChannelEnabled || a.HereEnabled }

// MessageTrigger uses the existing mention identity for all automatic sources,
// preserving 0.5 handled identities when a message matches several sources.
func MessageTrigger(ts, root string, reason AttentionReason) (AttentionTrigger, error) {
	at, err := MessageTime(ts)
	if err != nil {
		return AttentionTrigger{}, err
	}
	return AttentionTrigger{ID: "mention:" + ts, TS: ts, Thread: root, Reason: reason, At: at}, nil
}

func attentionPriority(reason AttentionReason) int {
	order := [...]AttentionReason{AttentionMention, AttentionChannel, AttentionHere, AttentionReply, AttentionManual}
	for i, candidate := range order {
		if candidate == reason {
			return i
		}
	}
	return len(order)
}

// Reason returns the strongest pending reason for the grouped conversation.
// Opening order still uses the oldest trigger, independently of its display label.
func (a AttentionState) Reason(cid string) AttentionReason {
	var best AttentionReason
	for _, t := range a.Pending[cid] {
		if best == "" || attentionPriority(t.Reason) < attentionPriority(best) {
			best = t.Reason
		}
	}
	return best
}
