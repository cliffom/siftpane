package ui

import "github.com/cliffom/siftpane/internal/core"

// metadataValidation records session-local validation of saved display hints.
// Deferred hints are checked on opening; none of these states grants access.
type metadataValidation uint8

const (
	metadataUnvalidated metadataValidation = iota
	metadataDeferred
	metadataValidated
)

// Restore display hints without granting history access or renewing their ages.
func (m *Model) restoreConversationMetadata() {
	m.directoryCached = make(map[string]bool)
	summaries := m.store.Conversations(m.identity)
	included := make(map[string]core.Conversation)
	for _, c := range summaries {
		if m.included[c.ID] {
			included[c.ID] = c
		}
	}
	for _, c := range m.store.Directory(m.identity) {
		c = withKnownDMName(c, included[c.ID])
		m.replaceConversation(c)
		m.directoryCached[c.ID] = true
		if m.included[c.ID] {
			m.metadataState[c.ID] = metadataDeferred
		}
	}
	for _, c := range summaries {
		if m.included[c.ID] && m.metadataState[c.ID] != metadataDeferred {
			m.replaceConversation(c)
		}
	}
}

// Retain useful display hints while participant profile enrichment is pending.
// The current observation still owns identity, kind and other metadata.
func withKnownDMName(c, known core.Conversation) core.Conversation {
	if unresolvedDirectoryName(c) && known.Kind == core.ConversationDM && (known.User == "" || known.User == c.User) && known.Name != "" && known.Name != known.ID && known.Name != known.User && known.Name != c.User {
		c.Name, c.Handle = known.Name, known.Handle
	}
	return c
}
