package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	attentionFileKind      = "attention"
	attentionVersion       = 1
	attentionBytes         = 512 * 1024 // Maximum durable identities/preferences file per account.
	attentionIdentityBytes = 256
)

type attentionFile struct {
	Version int                 `json:"version"`
	State   core.AttentionState `json:"state"`
}

// Attention loads durable account-isolated state. Corrupt files are errors, never
// silently discarded as if they were disposable caches. Missing files migrate Kept.
func (s *Store) Attention(id core.Identity) (core.AttentionState, error) {
	initial := core.NewAttention(s.Workspace(id.Workspace).Included)
	if s.Path == "" {
		return initial, nil
	}
	f, err := os.Open(s.cachePath(id, attentionFileKind))
	if errors.Is(err, os.ErrNotExist) {
		return initial, nil
	}
	if err != nil {
		return initial, err
	}
	defer func() { _ = f.Close() }() // Read-only; preserve decode failures.
	b, err := io.ReadAll(io.LimitReader(f, attentionBytes+1))
	if err != nil {
		return initial, err
	}
	if len(b) > attentionBytes {
		return initial, errors.New("attention state exceeds size limit")
	}
	var af attentionFile
	if err = json.Unmarshal(b, &af); err != nil {
		return initial, errors.New("invalid attention state; preserve file and recover before changing attention")
	}
	if af.Version != attentionVersion {
		return initial, errors.New("unsupported attention state version")
	}
	if err = validateAttention(af.State); err != nil {
		return initial, err
	}
	return af.State, nil
}

// SaveAttention atomically commits minimal durable state before the UI hides an entry.
func (s *Store) SaveAttention(id core.Identity, a core.AttentionState) error {
	if err := validateAttention(a); err != nil {
		return err
	}
	if s.Path == "" {
		return nil
	}
	b, err := json.Marshal(attentionFile{Version: attentionVersion, State: a})
	if err != nil {
		return err
	}
	if len(b) > attentionBytes {
		return errors.New("attention state exceeds size limit")
	}
	return writePrivate(s.cachePath(id, attentionFileKind), b)
}

func validateAttention(a core.AttentionState) error {
	if a.Kept == nil || a.Pending == nil || a.Handled == nil || len(a.Kept) > core.AttentionConversations || len(a.Pending) > core.AttentionConversations {
		return errors.New("invalid attention maps or capacity")
	}
	count := len(a.Handled)
	for cid, ts := range a.Pending {
		if !validAttentionID(cid) || len(ts) == 0 {
			return errors.New("invalid attention conversation")
		}
		seen := map[string]bool{}
		for _, t := range ts {
			if !validTrigger(t) || seen[t.ID] {
				return errors.New("invalid attention trigger")
			}
			seen[t.ID] = true
		}
		count += len(ts)
	}
	for cid := range a.Kept {
		if !validAttentionID(cid) {
			return errors.New("invalid kept identity")
		}
	}
	for k, t := range a.Handled {
		if len(k) > attentionIdentityBytes*2 || !strings.Contains(k, "/") || !validTrigger(t) {
			return errors.New("invalid handled identity")
		}
	}
	if count > core.AttentionTriggers {
		return fmt.Errorf("attention identity limit: %d", core.AttentionTriggers)
	}
	return nil
}

func validAttentionID(s string) bool {
	return s != "" && len(s) <= attentionIdentityBytes && !strings.ContainsAny(s, "/\x00\n\r")
}

func validTrigger(t core.AttentionTrigger) bool {
	if !validAttentionID(t.ID) || t.At.IsZero() {
		return false
	}
	switch t.Reason {
	case core.AttentionManual:
		return t.TS == "" && t.Thread == ""
	case core.AttentionMention:
		at, err := core.MessageTime(t.TS)
		if err != nil || !at.Equal(t.At) {
			return false
		}
		if t.Thread != "" {
			if _, err := core.MessageTime(t.Thread); err != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}
