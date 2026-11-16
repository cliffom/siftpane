package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

func TestMessagesPrivacyIsolationScopeExpiryAndClear(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "cache-config")}
	id := core.Identity{Workspace: "cache-workspace", User: "person"}
	now := time.Now().Add(-time.Minute)
	histories := map[string]MessageHistory{
		"kept":         {Saved: now, Messages: []core.Message{{TS: cacheMessageTS, Text: "offline body", ClientID: "not-an-outbox", Author: "not-durable"}}},
		"excluded":     {Saved: now, Messages: []core.Message{{TS: "1794672000.000002", Text: "excluded body"}}},
		cacheExpiredID: {Saved: now.Add(-messageMaxAge)},
		cacheFutureID:  {Saved: now.Add(time.Hour)},
	}
	kept := map[string]bool{"kept": true, cacheExpiredID: true, cacheFutureID: true}
	if err := s.SaveMessages(id, kept, histories); err != nil {
		t.Fatal(err)
	}
	loaded := s.Messages(id, kept)
	if len(loaded) != 1 || loaded["kept"].Messages[0].Text != "offline body" || !loaded["kept"].Saved.Equal(now) {
		t.Fatal("scope/expiry or timestamp renewal", loaded)
	}
	b, err := os.ReadFile(s.cachePath(id, messageFileKind))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "not-an-outbox") || strings.Contains(string(b), "excluded body") || strings.Contains(string(b), "not-durable") {
		t.Fatal("persisted excluded fields")
	}
	info, err := os.Stat(s.cachePath(id, messageFileKind))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatal("nonprivate file", info.Mode())
	}
	for _, other := range []core.Identity{{Workspace: id.Workspace, User: "cache-other"}, {Workspace: "cache-other", User: id.User}} {
		if len(s.Messages(other, kept)) != 0 {
			t.Fatal("account leak")
		}
	}
	if len(s.Messages(id, map[string]bool{})) != 0 {
		t.Fatal("removed kept restored")
	}
	if err = s.ClearMessages(id); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearMessages(id); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages(id, kept)) != 0 {
		t.Fatal("clear failed")
	}
}

func TestMessagesBoundsAndInvalidFiles(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "config")}
	id := core.Identity{Workspace: "w", User: "u"}
	kept := map[string]bool{}
	histories := map[string]MessageHistory{}
	for i := range messageTimelines + 1 {
		cid := fmt.Sprintf("channel-%02d", i)
		kept[cid] = true
		h := MessageHistory{Saved: time.Now().Add(-time.Duration(i) * time.Minute)}
		for j := range messageItems + 1 {
			h.Messages = append(h.Messages, core.Message{TS: fmt.Sprintf("1794672000.%06d", j), Text: strings.Repeat("a", messageEntryBytes/4)})
		}
		histories[cid] = h
	}
	if err := s.SaveMessages(id, kept, histories); err != nil {
		t.Fatal(err)
	}
	loaded := s.Messages(id, kept)
	if len(loaded) == 0 || len(loaded) > messageTimelines {
		t.Fatal("timeline bounds", len(loaded))
	}
	for _, h := range loaded {
		if len(h.Messages) > messageItems || !h.More {
			t.Fatal("message bounds/paging")
		}
	}
	info, err := os.Stat(s.cachePath(id, messageFileKind))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > messageBytes {
		t.Fatal("byte limit")
	}
	valid := messageFile{Version: messageVersion, Histories: map[string]MessageHistory{"channel-00": {Saved: time.Now().Add(-time.Minute), Messages: []core.Message{{TS: "invalid"}}}}}
	invalidJSON, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{[]byte("invalid"), []byte(`{"version":999}`), invalidJSON, []byte(strings.Repeat(" ", messageBytes+1))} {
		if err = os.WriteFile(s.cachePath(id, messageFileKind), b, 0o600); err != nil {
			t.Fatal(err)
		}
		if len(s.Messages(id, kept)) != 0 {
			t.Fatal("invalid cache restored")
		}
	}
}

func TestMessagesOversizeBodiesAndMemoryOnly(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "config")}
	id := core.Identity{Workspace: "w", User: "u"}
	kept := map[string]bool{cacheChannelID: true}
	h := map[string]MessageHistory{cacheChannelID: {Saved: time.Now().Add(-time.Minute), Messages: []core.Message{{TS: cacheMessageTS, Text: strings.Repeat("x", messageEntryBytes)}}}}
	if err := s.SaveMessages(id, kept, h); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages(id, kept)) != 0 {
		t.Fatal("oversize message kept")
	}
	s.Path = ""
	if err := s.SaveMessages(id, kept, h); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages(id, kept)) != 0 {
		t.Fatal("memory-only store restored disk data")
	}
}

const (
	cacheExpiredID = "cache-expired"
	cacheFutureID  = "cache-future"
	cacheChannelID = "cache-channel"
	cacheMessageTS = "1794672000.000001"
)

func TestMessagesRejectsExpiredWindowsAlreadyOnDisk(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "cache-expiry-config")}
	id := core.Identity{Workspace: "expiry-workspace", User: "expiry-person"}
	for _, saved := range []time.Time{time.Now().Add(-messageMaxAge - time.Minute), time.Now().Add(time.Hour)} {
		file := messageFile{Version: messageVersion, Histories: map[string]MessageHistory{cacheChannelID: {Saved: saved, Messages: []core.Message{{TS: cacheMessageTS, Text: "expired body"}}}}}
		b, err := json.Marshal(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(s.cachePath(id, messageFileKind), b, 0o600); err != nil {
			t.Fatal(err)
		}
		if len(s.Messages(id, map[string]bool{cacheChannelID: true})) != 0 {
			t.Fatal("already-stored expiry/future data restored")
		}
	}
}
