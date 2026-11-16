package core

// QuickReaction names a small, stable set of participation actions.
type QuickReaction string

const (
	// ReactionAcknowledged says the request has been seen.
	ReactionAcknowledged QuickReaction = "+1"
	// ReactionLooking says the user is investigating.
	ReactionLooking QuickReaction = "eyes"
	// ReactionResolved says the user considers the request resolved.
	ReactionResolved QuickReaction = "white_check_mark"
)

// Label describes the action independently of emoji rendering support.
func (r QuickReaction) Label() string {
	switch r {
	case ReactionAcknowledged:
		return "Acknowledged"
	case ReactionLooking:
		return "Looking"
	case ReactionResolved:
		return "Resolved"
	}
	return ""
}

// MessageTarget identifies an exact message in the authenticated service.
type MessageTarget struct{ Conversation, TS string }
