package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
)

func cacheFixture(t *testing.T) *Model {
	t.Helper()
	m := fixture(t)
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	m.saveMetadata()
	m.toggleMessageCache()
	m.toggleMessageCache()
	if !m.attention.MessageCache {
		t.Fatal("cache opt-in failed")
	}
	return m
}

func TestCachedStartupDoesNotReadOrSendAndLiveReconciles(t *testing.T) {
	m := cacheFixture(t)
	cid := m.active
	old := []core.Message{{TS: "1794672000.000001", Text: "before edit"}, {TS: resumeSelectedTS, Text: "deleted"}}
	if err := m.store.SaveMessages(m.identity, m.attention.Kept, map[string]store.MessageHistory{cid: {Saved: time.Now().Add(-time.Minute), Messages: old}}); err != nil {
		t.Fatal(err)
	}
	beforeRead := m.prefs.Read[cid]
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	line := restored.cache[cid]
	if line == nil || !line.Cached || !line.Loaded || len(line.Messages) != 2 {
		t.Fatal("startup cache missing")
	}
	if len(restored.unread) != 0 || len(restored.deliveries) != 0 || restored.prefs.Read[cid] != beforeRead || len(restored.drafts) != 0 {
		t.Fatal("cache created work or read state")
	}
	restored.active = cid
	restored.statusKind = statusRoutine
	if !strings.Contains(restored.activityStatus(), "Cached") {
		t.Fatal("cache freshness hidden")
	}
	restored.composer.SetValue(mainDraft)
	restored.Update(historyResult{Key: cid, Generation: restored.generations[cid], CacheEpoch: restored.cacheEpoch, Err: errors.New("offline")})
	if !line.Cached || len(line.Messages) != 2 || !strings.Contains(restored.activityStatus(), "refresh failed") || restored.composer.Value() != mainDraft {
		t.Fatal("offline cache/draft lost")
	}
	restored.Update(historyResult{Key: cid, Generation: restored.generations[cid], CacheEpoch: restored.cacheEpoch, Page: core.Page{Messages: []core.Message{{TS: old[0].TS, Text: "edited"}, {TS: "1794672000.000003", Text: "arriving"}}}})
	if line.Cached || len(line.Messages) != 2 || line.Messages[0].Text != "edited" || line.Messages[1].Text != "arriving" {
		t.Fatal("live reconciliation failed", line.Messages)
	}
}

func TestClearDisableRemovalAndAuthRejectLateCacheWrites(t *testing.T) {
	for _, action := range []string{"clear", "disable", "remove", "auth", "access"} {
		t.Run(action, func(t *testing.T) {
			m := cacheFixture(t)
			cid := m.active
			pending := m.fetch(cid, false)
			if pending == nil {
				t.Fatal("missing history command")
			}
			switch action {
			case "clear":
				m.clearMessages()
			case "disable":
				m.toggleMessageCache()
			case "remove":
				m.toggle(cid)
			case "auth":
				m.stopReads()
			case "access":
				m.purgeConversationCache(cid, &slack.APIError{Code: missingConversationCode})
			}
			m.Update(pending())
			if len(m.store.Messages(m.identity, m.attention.Kept)) != 0 {
				t.Fatal("late result recreated cleared content")
			}
		})
	}
}

func TestCacheWriteFailureAndAccountOptInIsolation(t *testing.T) {
	m := cacheFixture(t)
	cid := m.active
	block := filepath.Join(t.TempDir(), "cache-block")
	if err := os.WriteFile(block, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.store.Path = filepath.Join(block, testConfigName)
	m.composer.SetValue(mainDraft)
	m.Update(historyResult{Key: cid, Generation: m.generations[cid], CacheEpoch: m.cacheEpoch, Page: core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: "visible"}}}})
	if m.messageCacheErr == "" || !m.cache[cid].Loaded || m.composer.Value() != mainDraft {
		t.Fatal("cache failure blocked messaging")
	}
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	if err := m.store.SaveAttention(m.identity, m.attention); err != nil {
		t.Fatal(err)
	}
	other := m.identity
	other.User = "other-person"
	a, err := m.store.Attention(other)
	if err != nil {
		t.Fatal(err)
	}
	if a.MessageCache {
		t.Fatal("opt-in leaked accounts")
	}
}

func TestBroadcastControlsIndependentAndConfirmationsSafe(t *testing.T) {
	m := fixture(t)
	m.attention.Enabled = true
	for _, k := range []string{keyChannel, keyHere} {
		m.Update(key(k))
		m.Update(tea.KeyPressMsg{Code: []rune(k)[0], Text: k, IsRepeat: true})
		if k == keyChannel && m.attention.ChannelEnabled || k == keyHere && m.attention.HereEnabled {
			t.Fatal("repeat enabled broadcasts")
		}
		m.Update(key(keyEscape))
		m.Update(key(k))
		if m.broadcastConfirm != broadcastReason(k) {
			t.Fatal("intervening key did not cancel confirmation")
		}
		m.Update(key(k))
	}
	if !m.attention.Enabled || !m.attention.ChannelEnabled || !m.attention.HereEnabled {
		t.Fatal("sources coupled")
	}
	m.Update(key(keyChannel))
	if m.attention.ChannelEnabled || !m.attention.Enabled || !m.attention.HereEnabled {
		t.Fatal("disable suppressed direct requests")
	}
	m.Update(key(keyMentions))
	if m.attention.Enabled || !m.attention.HereEnabled || !m.discoveryBusy {
		t.Fatal("broadcast-only scan unavailable")
	}
}

func TestMultiSourceCompletionAndLateSourceResults(t *testing.T) {
	m := fixture(t)
	m.attention.Enabled, m.attention.ChannelEnabled, m.attention.HereEnabled = true, true, true
	now := time.Now()
	cmd := m.checkMentions(now)
	if cmd == nil {
		t.Fatal("no scan")
	}
	q := core.MentionQuery{User: m.identity.User, From: now.Add(-core.AttentionLookback), Until: now, Page: 1}
	match := attentionTestMatch(t, m.active)
	_, cmd = m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{Matches: []core.MentionMatch{match}, Total: 1}})
	if cmd == nil || !m.attention.Checkpoint.IsZero() || len(m.attention.Pending[m.active]) != 1 {
		t.Fatal("first source prematurely completed")
	}
	// Duplicated terminal page from the old source cannot complete @channel.
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{}})
	if m.discoverySources[0] != core.AttentionChannel {
		t.Fatal("stale source advanced queue")
	}
	q.Reason = core.AttentionChannel
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{}})
	if !m.attention.Checkpoint.IsZero() {
		t.Fatal("second source prematurely completed")
	}
	q.Reason = core.AttentionHere
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Err: errors.New("offline")})
	if !m.attention.Checkpoint.IsZero() || len(m.attention.Pending[m.active]) != 1 || m.discoveryPhase != discoveryFailed {
		t.Fatal("failure lost partial state or advanced completion")
	}
}

func followFixture(t *testing.T) (*Model, string) {
	t.Helper()
	m := fixture(t)
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	m.thread = followTestRoot
	cmd := m.toggleFollow()
	if cmd == nil {
		t.Fatal("follow did not scan")
	}
	key := m.key()
	return m, key
}

func followResult(m *Model, key string, more bool) followedResult {
	f := m.attention.Followed[key]
	until := f.Checkpoint.Add(time.Minute)
	result := followedResult{Key: key, Generation: m.followGeneration, From: f.Checkpoint, Until: until, Number: 1, Query: core.Query{Conversation: f.Conversation, Thread: f.Root, After: messageBound(f.Checkpoint), Before: messageBound(until)}, Page: core.Page{More: more, Messages: []core.Message{{TS: messageBound(f.Checkpoint.Add(time.Second)), User: "another-person", Thread: f.Root}}}}
	if more {
		result.Page.Next = "next-replies"
	}
	return result
}

func TestFollowedPartialRestartReplayAndUnfollowCancellation(t *testing.T) {
	m, key := followFixture(t)
	m.composer.SetValue(mainDraft)
	original := m.attention.Followed[key].Checkpoint
	v := followResult(m, key, true)
	_, cmd := m.Update(v)
	if cmd == nil || !m.attention.Followed[key].Checkpoint.Equal(original) || len(m.attention.Pending[m.active]) != 1 {
		t.Fatal("partial reply page lost or advanced completion")
	}
	// Failure retains accepted identities and prior coverage on disk.
	v.Number++
	v.Err = errors.New("interrupted")
	m.Update(v)
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	if len(restored.attention.Pending[m.active]) != 1 || !restored.attention.Followed[key].Checkpoint.Equal(original) {
		t.Fatal("restart lost partial state")
	}
	if restored.checkFollowed(time.Now().Add(time.Minute)) == nil {
		t.Fatal("no reconnect catchup")
	}
	v = followResult(restored, key, false)
	restored.Update(v)
	if len(restored.attention.Pending[m.active]) != 1 || !restored.attention.Followed[key].Checkpoint.Equal(v.Until) {
		t.Fatal("replay duplicated or terminal page not committed")
	}
	// Original model unfollows without destroying thread draft, ignores old responses.
	m.followNext = map[string]time.Time{}
	m.checkFollowed(time.Now().Add(time.Minute))
	late := followResult(m, key, false)
	ctx := m.followCtx
	m.toggleFollow()
	if ctx.Err() == nil {
		t.Fatal("unfollow did not cancel")
	}
	m.Update(late)
	if len(m.attention.Followed) != 0 || m.composer.Value() != mainDraft {
		t.Fatal("late unfollow mutated state")
	}
}

func TestFollowedSelfRootBudgetGapAndAuth(t *testing.T) {
	m, key := followFixture(t)
	v := followResult(m, key, true)
	f := m.attention.Followed[key]
	v.Number = discoveryPageLimit
	m.followPage = discoveryPageLimit
	v.Page.Messages = []core.Message{{TS: f.Root, User: "another-person"}, {TS: messageBound(f.Checkpoint.Add(time.Second)), User: m.identity.User}}
	m.Update(v)
	if len(m.attention.Pending) != 0 || m.followErrors[key] == "" || !m.attention.Followed[key].Checkpoint.Equal(f.Checkpoint) {
		t.Fatal("self/root activity or false coverage")
	}
	m.followNext = map[string]time.Time{}
	if m.checkFollowed(time.Now().Add(core.AttentionLookback+time.Hour)) == nil || m.attention.Gap == nil {
		t.Fatal("long absence did not warn")
	}
	v = followResult(m, key, false)
	v.Err = &slack.APIError{Code: "token_expired"}
	m.Update(v)
	if !m.stopped || !strings.Contains(m.status, "login & restart") {
		t.Fatal("follow auth recovery hidden")
	}
}

func TestCacheAndFollowKeysAreOrdinaryComposerInput(t *testing.T) {
	m := fixture(t)
	m.focus = focusCompose
	m.composer.Focus()
	for _, k := range []string{keyChannel, keyHere, keyFollow} {
		m.Update(key(k))
	}
	if m.composer.Value() != keyChannel+keyHere+keyFollow || m.attention.SearchEnabled() || len(m.attention.Followed) != 0 {
		t.Fatal("navigation shortcuts escaped composing")
	}
}

const followTestRoot = "1794671000.000001"

func TestCachedSelectionSurvivesRefreshAndMissingMetadata(t *testing.T) {
	m := cacheFixture(t)
	cid := m.active
	h := store.MessageHistory{Saved: time.Now().Add(-time.Minute), Messages: []core.Message{{TS: "1794672000.000001", Text: "first cached"}, {TS: resumeSelectedTS, Text: "selected cached"}}}
	if err := m.store.SaveMessages(m.identity, m.attention.Kept, map[string]store.MessageHistory{cid: h}); err != nil {
		t.Fatal(err)
	}
	if err := m.store.SaveConversations(m.identity, nil); err != nil {
		t.Fatal(err)
	}
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	if restored.active != cid || restored.cache[cid] == nil || restored.index != 1 {
		t.Fatal("cache depends on metadata or newest selection lost")
	}
	restored.scroll = -1
	restored.Update(historyResult{Key: cid, Generation: restored.generations[cid], CacheEpoch: restored.cacheEpoch, Page: core.Page{Messages: []core.Message{{TS: h.Messages[1].TS, Text: "updated selected"}, {TS: "1794672000.000003", Text: "arrived later"}}}})
	if restored.index != 0 || restored.cache[cid].Messages[restored.index].TS != h.Messages[1].TS {
		t.Fatal("refresh moved explicit cached message selection")
	}
}

func TestMultiSourceTerminalCompletionAndMixedReplayDismissal(t *testing.T) {
	m := fixture(t)
	m.attention.Enabled, m.attention.ChannelEnabled = true, true
	now := time.Now()
	m.checkMentions(now)
	q := core.MentionQuery{User: m.identity.User, From: now.Add(-core.AttentionLookback), Until: now, Page: 1}
	match := attentionTestMatch(t, m.active)
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{Matches: []core.MentionMatch{match}, Total: 1}})
	q.Reason = core.AttentionChannel
	match.Trigger.Reason = core.AttentionChannel
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{Matches: []core.MentionMatch{match}, Total: 1}})
	if !m.attention.Checkpoint.Equal(now) || m.discoveryPhase != discoveryChecked || len(m.attention.Pending[m.active]) != 1 {
		t.Fatal("combined completion or deduplication failed")
	}
	m.attention.Dismiss(m.active)
	m.discoveryNext = time.Time{}
	m.checkMentions(now.Add(time.Minute))
	q.Until = now.Add(time.Minute)
	q.Reason = core.AttentionMention
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{Matches: []core.MentionMatch{match}, Total: 1}})
	if len(m.attention.Pending) != 0 {
		t.Fatal("replay resurrected mixed-source dismissal")
	}
}

const resumeSelectedTS = "1794672000.000002"

func TestFollowedBoundaryAndStalePage(t *testing.T) {
	m, key := followFixture(t)
	v := followResult(m, key, true)
	f := m.attention.Followed[key]
	// The lower API bound is padded so a message exactly at the previous upper
	// checkpoint is accepted by this window, rather than lost between windows.
	if q := m.checkFollowed(time.Now()); q != nil {
		t.Fatal("parallel followed request")
	}
	v.Page.Messages = []core.Message{{TS: messageBound(f.Checkpoint), User: "boundary-author"}, {TS: messageBound(v.Until), User: "future-window"}}
	m.Update(v)
	if len(m.attention.Pending[f.Conversation]) != 1 || m.attention.Pending[f.Conversation][0].TS != messageBound(f.Checkpoint) {
		t.Fatal("window boundary gap/overlap")
	}
	if _, cmd := m.Update(v); cmd != nil || m.followPage != 2 {
		t.Fatal("old page scheduled duplicate request")
	}
	v.Number = 2
	v.Page = core.Page{}
	m.Update(v)
	if !m.attention.Followed[key].Checkpoint.Equal(v.Until) {
		t.Fatal("terminal window not completed")
	}
}
