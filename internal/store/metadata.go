package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	metadataEntries = 64        // Included conversation summaries per authenticated workspace/user.
	metadataBytes   = 64 * 1024 // Maximum disposable cache file size.
	metadataMaxAge  = 7 * 24 * time.Hour
	metadataVersion = 1
)

// A whitelist keeps topics, purposes, message bodies and credentials off disk.
type conversationSummary struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Handle  string                `json:"handle,omitempty"`
	Kind    core.ConversationKind `json:"kind"`
	Members int                   `json:"members,omitempty"`
}

type metadataFile struct {
	Version       int                   `json:"version"`
	Saved         time.Time             `json:"saved"`
	Conversations []conversationSummary `json:"conversations"`
}

func (s *Store) metadataPath(id core.Identity) string {
	// Separate accounts in the same workspace as well as separate config paths.
	key := sha256.Sum256([]byte(id.Workspace + "\x00" + id.User))
	return fmt.Sprintf("%s.%x.metadata.json", s.Path, key)
}

// Conversations returns disposable cached summaries, or no entries for missing,
// expired or invalid data. Preferences remain independently readable.
func (s *Store) Conversations(id core.Identity) []core.Conversation {
	if s.Path == "" {
		return nil
	}
	f, err := os.Open(s.metadataPath(id))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() // Read-only cache; close errors cannot affect preferences.
	b, err := io.ReadAll(io.LimitReader(f, metadataBytes+1))
	if err != nil || len(b) > metadataBytes {
		return nil
	}
	var cache metadataFile
	if json.Unmarshal(b, &cache) != nil || cache.Version != metadataVersion || len(cache.Conversations) > metadataEntries {
		return nil
	}
	age := time.Since(cache.Saved)
	if age < 0 || age > metadataMaxAge {
		return nil
	}
	out := []core.Conversation{}
	seen := map[string]bool{}
	for _, c := range cache.Conversations {
		if c.ID == "" || c.Name == "" || !validKind(c.Kind) || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out = append(out, core.Conversation{ID: c.ID, Name: c.Name, Handle: c.Handle, Kind: c.Kind, Members: c.Members})
	}
	return out
}

// SaveConversations saves bounded display summaries independently of preferences.
// Callers supply only explicitly included conversations. Empty paths disable I/O.
func (s *Store) SaveConversations(id core.Identity, cs []core.Conversation) error {
	if s.Path == "" {
		return nil
	}
	cache := metadataFile{Version: metadataVersion, Saved: time.Now()}
	for _, c := range cs {
		if len(cache.Conversations) >= metadataEntries {
			break
		}
		if c.ID == "" || c.Name == "" || !validKind(c.Kind) {
			continue
		}
		cache.Conversations = append(cache.Conversations, conversationSummary{ID: c.ID, Name: c.Name, Handle: c.Handle, Kind: c.Kind, Members: c.Members})
	}
	b, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if len(b) > metadataBytes {
		return fmt.Errorf("metadata exceeds %d bytes", metadataBytes)
	}
	return writePrivate(s.metadataPath(id), b)
}

func validKind(k core.ConversationKind) bool {
	switch k {
	case core.ConversationChannel, core.ConversationPrivate, core.ConversationDM, core.ConversationGroup:
		return true
	default:
		return false
	}
}
