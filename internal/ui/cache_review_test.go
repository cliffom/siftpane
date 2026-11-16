package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func TestOlderPagePreservesCachedRecentFailure(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := cacheFixture(t)
			cid := m.active
			line := m.line(cid)
			saved := time.Now().Add(-time.Hour)
			line.Messages = []core.Message{{TS: attentionTestTS, Text: "unrefreshed cached message"}}
			line.Loaded, line.Cached, line.More, line.Saved = true, true, true, saved
			m.statusKind = statusRoutine
			m.focus = focusMessages
			m.scroll = -1
			m.composer.SetValue(mainDraft)
			read, latest := m.prefs.Read[cid], m.latest[cid]
			delete(m.unread, cid)
			m.Update(historyResult{Key: cid, Generation: m.generations[cid], Err: errors.New("recent window unavailable")})
			svc := &reviewThreadService{Service: m.service, page: core.Page{Messages: []core.Message{{TS: followTestRoot, Text: "live older page"}}, More: true}}
			m.service = svc
			runAttentionReviewCommand(t, m, m.fetch(cid, true))
			if !line.Cached || !line.Saved.Equal(saved) || len(line.Messages) != 2 || !strings.Contains(m.activityStatus(), "refresh failed") {
				t.Fatalf("older page falsely refreshed cached recent content: %s", m.activityStatus())
			}
			if m.prefs.Read[cid] != read || m.latest[cid] != latest || m.unread[cid] || m.composer.Value() != mainDraft {
				t.Fatal("older page observed stale cache as fresh activity or changed draft")
			}
			m.width, m.height = size[0], size[1]
			m.resize()
			rows := strings.Split(ansi.Strip(m.View().Content), "\n")
			if !strings.Contains(rows[len(rows)-1], keyLabel(keyRefresh)+" retry") {
				t.Fatal("cached recovery clipped", rows[len(rows)-1])
			}
			svc.page = core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: "live replacement"}}}
			cmd := m.fetch(cid, false)
			if !strings.Contains(m.activityStatus(), "· refreshing / R refresh") {
				t.Fatal("explicit retry still shows old failure")
			}
			runAttentionReviewCommand(t, m, cmd)
			if line.Cached || !line.Saved.After(saved) || len(line.Messages) != 1 || line.Messages[0].Text != "live replacement" {
				t.Fatal("successful recent refresh did not replace cache provenance")
			}
		})
	}
}

func TestAccessPurgeRejectsLateConversationAndThreadReads(t *testing.T) {
	for _, source := range []string{"metadata", "history", "followed"} {
		t.Run(source, func(t *testing.T) {
			m := cacheFixture(t)
			cid := m.active
			generation, epoch := m.generations[cid], m.cacheEpoch
			ctx := m.contexts[cid]
			threadKey := cid + "/" + followTestRoot
			m.line(threadKey).Messages = []core.Message{{TS: attentionTestTS, Text: "cached reply"}}
			m.line(threadKey).Loaded, m.line(threadKey).Busy = true, true
			m.metadataBusy[cid] = true
			m.startupRestoring = true
			m.startupSidebar = m.selected()
			m.composer.SetValue(mainDraft)
			m.drafts[threadKey] = "retained reply draft"
			m.deliveries[threadKey] = &core.Delivery{State: core.DeliveryPending}
			accessErr := &slack.APIError{Code: "channel_not_found"}
			switch source {
			case "metadata":
				m.Update(conversationResult{ID: cid, Generation: generation, Err: accessErr})
			case "history":
				m.Update(historyResult{Key: cid, Generation: generation, Err: accessErr})
			case "followed":
				m.thread = followTestRoot
				m.toggleFollow()
				v := followResult(m, threadKey, false)
				v.Err = accessErr
				m.Update(v)
				m.thread = ""
			}
			m.Update(historyResult{Key: cid, Generation: generation, CacheEpoch: epoch, Page: core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: "late conversation response"}}}})
			m.Update(historyResult{Key: threadKey, Generation: generation, CacheEpoch: epoch, Page: core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: "late reply response"}}}})
			m.Update(conversationResult{ID: cid, Generation: generation, Conversation: core.Conversation{ID: cid, Name: "late metadata"}})
			if ctx.Err() == nil || m.generations[cid] == generation || len(m.cache[cid].Messages) != 0 || len(m.cache[threadKey].Messages) != 0 {
				t.Fatal("access denial did not cancel/invalidate old reads")
			}
			if m.cache[cid].Busy || m.cache[threadKey].Busy || m.metadataBusy[cid] || m.startupRestoring || !m.metadataFailed[cid] {
				t.Fatal("invalidated reads left busy or restoration state stuck")
			}
			if m.composer.Value() != mainDraft || m.drafts[threadKey] != "retained reply draft" || m.deliveries[threadKey].State != core.DeliveryPending {
				t.Fatal("access purge damaged draft/send recovery")
			}
			if len(m.store.Messages(m.identity, m.attention.Kept)) != 0 {
				t.Fatal("late result restored disk content")
			}
			runAttentionReviewCommand(t, m, m.fetch(cid, false))
			if !m.cache[cid].Loaded || len(m.cache[cid].Messages) == 0 {
				t.Fatal("fresh generation could not recover access")
			}
		})
	}
}

func TestAccessPurgeCancelsEnrichmentAndFollowWithoutStoppingOtherConversations(t *testing.T) {
	m := cacheFixture(t)
	cid := m.active
	m.thread = followTestRoot
	if m.toggleFollow() == nil {
		t.Fatal("follow did not start")
	}
	lateFollow := followResult(m, m.key(), false)
	followCtx := m.followCtx
	profileCtx, cancel := context.WithCancel(m.contexts[cid])
	m.profileCancel, m.profileBusy, m.profileKey = cancel, true, m.key()
	lateProfile := profileResult{ID: "late-author", Key: m.key(), Generation: m.generations[cid], LookupGeneration: m.profileLookupGeneration, Profile: core.Profile{ID: "late-author", Name: "late profile"}}
	healthyCtx, healthyGeneration := m.contexts[demoDM], m.generations[demoDM]
	m.Update(conversationResult{ID: cid, Generation: m.generations[cid], Err: &slack.APIError{Code: "not_in_channel"}})
	m.Update(lateFollow)
	m.Update(lateProfile)
	if profileCtx.Err() == nil || followCtx.Err() == nil || m.followBusy || m.profileBusy || len(m.attention.Pending[cid]) != 0 {
		t.Fatal("purged conversation retained/enacted stale follow or profile reads")
	}
	if healthyCtx.Err() != nil || m.generations[demoDM] != healthyGeneration || m.stopped {
		t.Fatal("conversation revocation stopped unrelated conversations")
	}
}
