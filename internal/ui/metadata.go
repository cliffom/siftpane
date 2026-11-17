package ui

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
	for _, c := range m.store.Directory(m.identity) {
		m.replaceConversation(c)
		m.directoryCached[c.ID] = true
		if m.included[c.ID] {
			m.metadataState[c.ID] = metadataDeferred
		}
	}
	for _, c := range m.store.Conversations(m.identity) {
		if m.included[c.ID] && m.metadataState[c.ID] != metadataDeferred {
			m.replaceConversation(c)
		}
	}
}
