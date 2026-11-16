package store

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	messageVersion    = 1
	messageBytes      = 4 * 1024 * 1024 // Total file bytes per workspace/account.
	messageEntryBytes = 64 * 1024       // Skip oversized messages; this is a display aid.
	messageTimelines  = 16
	messageItems      = 100 // Recent messages per conversation, excluding thread replies.
	messageMaxAge     = 24 * time.Hour
	messageFileKind   = "messages"
)

// MessageHistory is a disposable fetched conversation window, never an outbox.
// Saved records successful live fetching, not the time an old entry was rewritten.
type MessageHistory struct {
	Saved    time.Time      `json:"saved"`
	Messages []core.Message `json:"messages"`
	More     bool           `json:"more"`
}

type messageFile struct {
	Version   int                       `json:"version"`
	Histories map[string]MessageHistory `json:"histories"`
}

// Messages restores only opted-in, Kept conversation content for this account.
// Any invalid file falls back to live reads without touching durable state.
func (s *Store) Messages(id core.Identity, kept map[string]bool) map[string]MessageHistory {
	out := map[string]MessageHistory{}
	if s.Path == "" {
		return out
	}
	f, err := os.Open(s.cachePath(id, messageFileKind))
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }() // Disposable read; no state depends on close.
	b, err := io.ReadAll(io.LimitReader(f, messageBytes+1))
	if err != nil || len(b) > messageBytes {
		return out
	}
	var file messageFile
	if json.Unmarshal(b, &file) != nil || file.Version != messageVersion || len(file.Histories) > messageTimelines {
		return out
	}
	for cid, h := range file.Histories {
		if !kept[cid] || !validAttentionID(cid) || !validMessageHistory(h) {
			continue
		}
		for i := range h.Messages {
			h.Messages[i].Link = "slack://channel?id=" + url.QueryEscape(cid) + "&message=" + url.QueryEscape(h.Messages[i].TS)
		}
		out[cid] = h
	}
	return out
}

func validMessageHistory(h MessageHistory) bool {
	age := time.Since(h.Saved)
	if age < 0 || age > messageMaxAge || len(h.Messages) > messageItems {
		return false
	}
	seen := map[string]bool{}
	for _, msg := range h.Messages {
		if _, err := core.MessageTime(msg.TS); err != nil || seen[msg.TS] {
			return false
		}
		seen[msg.TS] = true
	}
	return true
}

// SaveMessages writes a bounded account-local file, filtering scope again at the
// storage boundary. It does not renew old entries or download additional history.
func (s *Store) SaveMessages(id core.Identity, kept map[string]bool, histories map[string]MessageHistory) error {
	if s.Path == "" {
		return nil
	}
	file := messageFile{Version: messageVersion, Histories: map[string]MessageHistory{}}
	ids := make([]string, 0, len(histories))
	for cid := range histories {
		if kept[cid] {
			ids = append(ids, cid)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := histories[ids[i]].Saved, histories[ids[j]].Saved
		if a.Equal(b) {
			return ids[i] < ids[j]
		}
		return a.After(b)
	})
	for _, cid := range ids {
		if len(file.Histories) >= messageTimelines {
			break
		}
		h := histories[cid]
		h.Messages = core.Merge(nil, h.Messages, len(h.Messages))
		if len(h.Messages) > messageItems {
			h.Messages = h.Messages[len(h.Messages)-messageItems:]
			h.More = true
		}
		count := len(h.Messages)
		h.Messages = displayMessages(h.Messages)
		if len(h.Messages) < count {
			h.More = true
		}
		if count > 0 && len(h.Messages) == 0 {
			continue
		}
		if !validAttentionID(cid) || !validMessageHistory(h) {
			continue
		}
		file.Histories[cid] = h
		b, err := json.Marshal(file)
		if err != nil {
			return err
		}
		if len(b) > messageBytes {
			delete(file.Histories, cid)
		}
	}
	b, err := json.Marshal(file)
	if err != nil {
		return err
	}
	return writePrivate(s.cachePath(id, messageFileKind), b)
}

func displayMessages(messages []core.Message) []core.Message {
	out := make([]core.Message, 0, len(messages))
	for _, msg := range messages {
		msg.Author, msg.ClientID = "", ""
		b, err := json.Marshal(msg)
		if err == nil && len(b) <= messageEntryBytes {
			out = append(out, msg)
		}
	}
	return out
}

// ClearMessages removes this account's disposable content, independently of
// preferences, attention, drafts and credentials. Missing files are harmless.
func (s *Store) ClearMessages(id core.Identity) error {
	if s.Path == "" {
		return nil
	}
	err := os.Remove(s.cachePath(id, messageFileKind))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
