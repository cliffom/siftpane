package core

import "strings"

// Completion returns a name suitable for a scoped conversation search.
// Private channels use # too; the directory label still indicates privacy.
func (c Conversation) Completion() string {
	if c.Kind == ConversationChannel || c.Kind == ConversationPrivate {
		return "#" + c.Name
	}
	if c.Handle != "" {
		return "@" + c.Handle
	}
	return "@" + c.Name
}

func matchesConversation(c Conversation, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if strings.HasPrefix(q, "#") {
		if c.Kind != ConversationChannel && c.Kind != ConversationPrivate {
			return false
		}
		q = strings.TrimSpace(strings.TrimPrefix(q, "#"))
	} else if strings.HasPrefix(q, "@") {
		if c.Kind != ConversationDM && c.Kind != ConversationGroup {
			return false
		}
		q = strings.TrimSpace(strings.TrimPrefix(q, "@"))
	}
	return strings.Contains(strings.ToLower(c.Name+" "+c.Handle+" "+string(c.Kind)), q)
}
