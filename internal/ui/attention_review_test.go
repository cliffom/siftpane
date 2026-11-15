package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

const reviewThreadPageCursor = "target-next"

func TestUnopenedAttentionDoesNotDelayEligiblePolling(t *testing.T) {
	for _, thread := range []bool{false, true} {
		t.Run(fmt.Sprint(thread), func(t *testing.T) {
			m, svc := pollingFixture(t, 1)
			for i := range core.AttentionConversations {
				cid := fmt.Sprintf("unopened-%03d", i)
				match := attentionTestMatch(t, cid)
				if err := m.attention.Add(cid, match.Trigger); err != nil {
					t.Fatal(err)
				}
				m.mergeAttentionConversation(match.Conversation)
			}
			m.syncAttentionInclusion()
			if thread {
				m.thread = pollThread
				m.line(m.key()).Loaded = true
				m.line(m.key()).Messages = []core.Message{{TS: pollThread}}
			}
			for tick := range 6 {
				want := m.active
				if thread && tick%threadPollEvery == threadPollEvery-1 {
					want = m.key()
				}
				assertPoll(t, m, svc, want)
			}
		})
	}
}

func TestOlderAttentionTargetReplacesFullThreadWindow(t *testing.T) {
	m := fixture(t)
	svc := &attentionTestService{Service: m.service}
	m.service = svc
	match := attentionTestMatch(t, m.active)
	if err := m.attention.Add(m.active, match.Trigger); err != nil {
		t.Fatal(err)
	}
	key := m.active + "/" + match.Trigger.Thread
	line := m.line(key)
	line.Loaded = true
	line.More = true
	line.Next = "old-page"
	line.After = "1794672001.000000"
	for i := range cacheLimit {
		line.Messages = append(line.Messages, core.Message{TS: fmt.Sprintf("1794672001.%06d", i)})
	}
	m.composer.SetValue(mainDraft)
	m.drafts[key] = "retained thread draft"
	runAttentionReviewCommand(t, m, m.selectConversation(m.active))
	if len(line.Messages) != 1 || line.Messages[m.index].TS != match.Trigger.TS || m.scroll != -1 {
		t.Fatal("returned opening target was evicted by newer cached replies", m.activityStatus())
	}
	if line.More || line.Next != "" || line.After != match.Trigger.TS {
		t.Fatal("opening retained pagination from the old window")
	}
	if m.composer.Value() != "retained thread draft" || m.drafts[demoChannel] != mainDraft {
		t.Fatal("rebasing timeline changed drafts")
	}
}

func runAttentionReviewCommand(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				m.Update(c())
			}
		}
	} else {
		m.Update(msg)
	}
}

func TestDiscoveryAuthFailureShowsRecovery(t *testing.T) {
	m := fixture(t)
	q := attentionTestCheck(t, m)
	m.statusKind = statusRoutine
	m.status = "Refreshed"
	m.composer.SetValue(mainDraft)
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Err: &slack.APIError{Code: "token_revoked"}})
	text := strings.ToLower(m.activityStatus())
	if !m.stopped || !strings.Contains(text, "credential") || !strings.Contains(text, "restart") {
		t.Fatal("discovery stopped polling without credential recovery", m.activityStatus())
	}
	if m.composer.Value() != mainDraft {
		t.Fatal("authentication failure discarded draft")
	}
}

func TestCatchupGapSurvivesSuccessfulChecks(t *testing.T) {
	m := fixture(t)
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	m.attention.Enabled = true
	m.attention.Checkpoint = now.Add(-core.AttentionLookback - 24*time.Hour)
	if m.checkMentions(now) == nil {
		t.Fatal("no initial check")
	}
	if m.attention.Gap == nil {
		t.Fatal("missed initial gap")
	}
	q := core.MentionQuery{User: m.identity.User, From: now.Add(-core.AttentionLookback), Until: now, Page: 1}
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{}})
	if m.checkMentions(now.Add(discoveryInterval+time.Second)) == nil {
		t.Fatal("no subsequent check")
	}
	if m.attention.Gap == nil {
		t.Fatal("uncovered interval was forgotten after a successful limited scan")
	}
}

func TestExplicitlyOpenedAttentionKeepsItsPollingTurn(t *testing.T) {
	m, svc := pollingFixture(t, 1)
	for _, cid := range []string{"opened", "unopened-a", "unopened-b"} {
		match := attentionTestMatch(t, cid)
		if err := m.attention.Add(cid, match.Trigger); err != nil {
			t.Fatal(err)
		}
		m.mergeAttentionConversation(match.Conversation)
	}
	m.syncAttentionInclusion()
	m.opened["opened"] = true
	for range 2 {
		assertPoll(t, m, svc, "opened")
		assertPoll(t, m, svc, m.active)
	}
}

func TestCatchupGapSurvivesRestartAndRequiresAcknowledgement(t *testing.T) {
	m := fixture(t)
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	m.attention.Enabled = true
	m.attention.Checkpoint = now.Add(-core.AttentionLookback - 24*time.Hour)
	if m.checkMentions(now) == nil {
		t.Fatal("no initial check")
	}
	from := m.attention.Gap.From
	m.Update(mentionResult{Query: core.MentionQuery{Page: 1}, Generation: m.discoveryGeneration, Err: errors.New("interrupted discovery")})
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	if restored.attention.Gap == nil || !restored.attention.Gap.From.Equal(from) {
		t.Fatal("failed scan/restart erased gap")
	}
	m = restored
	q := attentionTestCheck(t, m)
	m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{}})
	if m.attention.Gap == nil || !strings.Contains(m.discoveryStatus(), "gap") {
		t.Fatal("completed check hid existing gap")
	}
	before := m.attention.Checkpoint
	press := tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	m.Update(press)
	m.Update(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl, IsRepeat: true})
	if m.attention.Gap == nil {
		t.Fatal("key repeat acknowledged gap")
	}
	m.Update(key(keyEscape))
	m.Update(press)
	if m.attention.Gap == nil {
		t.Fatal("confirmation survived intervening key")
	}
	m.Update(press)
	if m.attention.Gap != nil || !m.attention.Checkpoint.Equal(before) {
		t.Fatal("acknowledgement failed or falsely advanced coverage")
	}
	again := New(m.ctx, m.service, m.identity, m.store, false)
	if again.attention.Gap != nil {
		t.Fatal("acknowledgement did not persist")
	}
}

func TestGapAcknowledgementRetainsNewGapsAndFailedWrites(t *testing.T) {
	m := fixture(t)
	now := time.Now().UTC()
	m.attention.RecordGap(now.Add(-core.AttentionLookback-time.Hour), now.Add(-core.AttentionLookback))
	press := tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	m.Update(press)
	m.attention.RecordGap(now.Add(-time.Hour), now)
	m.Update(press)
	if m.attention.Gap == nil {
		t.Fatal("acknowledged gap that changed after confirmation")
	}
	block := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(block, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.store.Path = filepath.Join(block, testConfigName)
	m.Update(press)
	if m.attention.Gap == nil || !strings.Contains(m.status, "Cannot save") {
		t.Fatal("failed write lost known gaps")
	}
}

func TestStoppedDiscoveryCannotFetchAttentionContext(t *testing.T) {
	m := fixture(t)
	match := attentionTestMatch(t, m.active)
	if err := m.attention.Add(m.active, match.Trigger); err != nil {
		t.Fatal(err)
	}
	m.stopped = true
	if cmd := m.selectConversation(m.active); cmd != nil {
		t.Fatal("opening a trigger bypassed stopped authentication")
	}
}

func TestAnchoredThreadRetryPreservesFollowingPageBounds(t *testing.T) {
	m := fixture(t)
	svc := &reviewThreadService{Service: m.service}
	m.service = svc
	m.thread = "1794671000.000001"
	line := m.line(m.key())
	line.Loaded = true
	line.More = false
	line.Messages = []core.Message{{TS: "1794672001.000000"}}
	m.openTargets[m.key()] = attentionTestTS
	svc.page = core.Page{Messages: []core.Message{{TS: attentionTestTS}}, More: true, Next: reviewThreadPageCursor}
	runAttentionReviewCommand(t, m, m.fetch(m.key(), false))
	if line.After != attentionTestTS || line.Next != reviewThreadPageCursor || !line.More {
		t.Fatal("anchor retry lost page bounds")
	}
	runAttentionReviewCommand(t, m, m.fetch(m.key(), true))
	if len(svc.queries) != 2 || svc.queries[1].After != attentionTestTS || svc.queries[1].Cursor != reviewThreadPageCursor {
		t.Fatal("following page used stale bounds", svc.queries)
	}
}

type reviewThreadService struct {
	core.Service
	page    core.Page
	queries []core.Query
}

func (s *reviewThreadService) History(ctx context.Context, q core.Query) (core.Page, error) {
	s.queries = append(s.queries, q)
	return s.page, ctx.Err()
}

func TestDiscoveryCredentialRecoveryFitsSupportedWidths(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m := fixture(t)
		q := attentionTestCheck(t, m)
		m.width, m.height = size[0], size[1]
		m.resize()
		m.Update(mentionResult{Query: q, Generation: m.discoveryGeneration, Err: &slack.APIError{Code: "token_expired"}})
		rows := strings.Split(ansi.Strip(m.View().Content), "\n")
		if !strings.Contains(rows[len(rows)-1], "login & restart") || strings.Contains(rows[len(rows)-1], "…") {
			t.Fatal("credential recovery clipped", rows[len(rows)-1])
		}
	}
}

func TestAcknowledgedGapDoesNotResurrectDuringFailedScanReplay(t *testing.T) {
	m := fixture(t)
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	m.attention.Enabled = true
	m.attention.Checkpoint = now.Add(-core.AttentionLookback - 24*time.Hour)
	if m.checkMentions(now) == nil {
		t.Fatal("no check")
	}
	missedUntil := m.attention.Gap.Until
	press := tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	m.Update(press)
	m.Update(press)
	m.Update(mentionResult{Query: core.MentionQuery{Page: 1}, Generation: m.discoveryGeneration, Err: errors.New("scan interrupted after acknowledgement")})
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	if !restored.attention.GapAcknowledgedUntil.Equal(missedUntil) {
		t.Fatal("acknowledgement watermark was not durable")
	}
	if restored.checkMentions(now) == nil {
		t.Fatal("no retry")
	}
	if restored.attention.Gap != nil {
		t.Fatal("same failed interval resurrected acknowledged warning")
	}
	restored.finishDiscovery()
	restored.discoveryNext = time.Time{}
	if restored.checkMentions(now.Add(time.Hour)) == nil {
		t.Fatal("no later retry")
	}
	if restored.attention.Gap == nil || !restored.attention.Gap.From.Equal(missedUntil) {
		t.Fatal("newly uncovered time did not warn separately")
	}
}
