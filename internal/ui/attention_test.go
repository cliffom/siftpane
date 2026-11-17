package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	testConfigName       = "config.json"
	demoLeoDM            = "Dleo"
	attentionTestOutside = "Coutside"
)

const attentionTestTS = "1794672000.000001"

type attentionTestService struct {
	core.Service
	queries  []core.Query
	mentions []core.MentionQuery
	ctx      context.Context
}

func (s *attentionTestService) History(ctx context.Context, q core.Query) (core.Page, error) {
	s.queries = append(s.queries, q)
	if q.Target != "" {
		return core.Page{Messages: []core.Message{{TS: q.Target, Text: "offline trigger", Thread: q.Thread}}}, ctx.Err()
	}
	return s.Service.History(ctx, q)
}

func (s *attentionTestService) Mentions(ctx context.Context, q core.MentionQuery) (core.MentionPage, error) {
	s.mentions = append(s.mentions, q)
	s.ctx = ctx
	return core.MentionPage{}, ctx.Err()
}

func attentionTestMatch(t *testing.T, cid string) core.MentionMatch {
	t.Helper()
	at, err := core.MessageTime(attentionTestTS)
	if err != nil {
		t.Fatal(err)
	}
	return core.MentionMatch{Conversation: core.Conversation{ID: cid, Name: "unkept test", Kind: core.ConversationPrivate}, Trigger: core.AttentionTrigger{ID: "mention:" + attentionTestTS, TS: attentionTestTS, Thread: "1794671000.000001", Reason: core.AttentionMention, At: at}}
}

func attentionTestCheck(t *testing.T, m *Model) core.MentionQuery {
	t.Helper()
	m.attention.Enabled = true
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	if m.checkMentions(now) == nil {
		t.Fatal("no discovery command")
	}
	from := now.Add(-core.AttentionLookback)
	if overlap := m.attention.Checkpoint.Add(-discoveryOverlap); !m.attention.Checkpoint.IsZero() && overlap.After(from) {
		from = overlap
	}
	if from.Before(m.attention.Floor) {
		from = m.attention.Floor
	}
	return core.MentionQuery{User: m.identity.User, From: from, Until: now, Page: 1}
}

func attentionTestResult(m *Model, q core.MentionQuery, match core.MentionMatch, more bool) mentionResult {
	return mentionResult{Query: q, Generation: m.discoveryGeneration, Page: core.MentionPage{Matches: []core.MentionMatch{match}, More: more}}
}

func TestTemporaryKeepDismissAndDraftProtection(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerSwitch)
	m.picker.SetValue("@leo")
	cmd := m.temporaryChoice(m.choices())
	if cmd == nil || m.active != demoLeoDM || m.attention.Kept[m.active] || len(m.attention.Pending[m.active]) != 1 {
		t.Fatal("temporary add became permanent or did not open")
	}
	m.composer.SetValue("unsent draft")
	m.dismissActive()
	if len(m.attention.Pending[m.active]) != 1 || !strings.Contains(m.status, "recover") {
		t.Fatal("dismissal hid composed text")
	}
	m.composer.Reset()
	m.rememberDraft()
	m.drafts[m.active+"/thread"] = "thread draft"
	m.dismissActive()
	if len(m.attention.Pending[m.active]) != 1 {
		t.Fatal("dismissal hid thread draft")
	}
	delete(m.drafts, m.active+"/thread")
	for _, state := range []core.DeliveryState{core.DeliveryPending, core.DeliveryFailed} {
		m.deliveries[m.active+"/thread"] = &core.Delivery{State: state}
		m.dismissActive()
		if len(m.attention.Pending[m.active]) != 1 {
			t.Fatal("dismissal hid unresolved send")
		}
	}
	delete(m.deliveries, m.active+"/thread")
	cid := m.active
	m.toggleActiveKept()
	m.dismissActive()
	if !m.attention.Kept[cid] || !m.included[cid] || len(m.attention.Pending[cid]) != 0 || m.active != cid {
		t.Fatal("kept dismissal removed conversation")
	}
	m.toggle(cid)
	if m.included[cid] {
		t.Fatal("unkeep failed after dismissal")
	}
}

func TestMentionDiscoveryDoesNotReadUnopenedHistoryOrChangeSelection(t *testing.T) {
	m := fixture(t)
	svc := &attentionTestService{Service: m.service}
	m.service = svc
	m.composer.SetValue("retained current draft")
	m.focus = focusCompose
	active := m.active
	q := attentionTestCheck(t, m)
	match := attentionTestMatch(t, attentionTestOutside)
	m.Update(attentionTestResult(m, q, match, false))
	if m.active != active || m.composer.Value() != "retained current draft" || m.focus != focusCompose {
		t.Fatal("background discovery changed context")
	}
	if m.fetch(match.Conversation.ID, false) != nil || len(svc.queries) != 0 {
		t.Fatal("discovery synchronized unopened history")
	}
	if m.attention.Kept[match.Conversation.ID] {
		t.Fatal("mention automatically kept conversation")
	}
	cmd := m.selectConversation(match.Conversation.ID)
	if cmd == nil || m.thread != match.Trigger.Thread {
		t.Fatal("trigger did not open its thread")
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

	if len(svc.queries) != 1 || svc.queries[0].Target != attentionTestTS || m.index != 0 || m.scroll != -1 {
		t.Fatal("exact trigger target was lost", svc.queries)
	}
	if len(m.attention.Pending[match.Conversation.ID]) != 1 {
		t.Fatal("reading dismissed attention")
	}
}

func TestDiscoveryPartialFailureReplayAndCheckpoint(t *testing.T) {
	m := fixture(t)
	q := attentionTestCheck(t, m)
	match := attentionTestMatch(t, attentionTestOutside)
	m.Update(attentionTestResult(m, q, match, true))
	if !m.attention.Checkpoint.IsZero() || len(m.attention.Pending[match.Conversation.ID]) != 1 {
		t.Fatal("partial page advanced checkpoint or lost results")
	}
	q.Page = 2
	failed := mentionResult{Query: q, Generation: m.discoveryGeneration, Err: errors.New("offline read failure")}
	m.Update(failed)
	if m.discoveryBusy || m.discoveryPhase != discoveryFailed || !m.attention.Checkpoint.IsZero() || len(m.attention.Pending) == 0 {
		t.Fatal("failure discarded partial results or claimed success")
	}
	m.discoveryNext = time.Time{}
	q = attentionTestCheck(t, m)
	m.Update(attentionTestResult(m, q, match, false))
	if len(m.attention.Pending[match.Conversation.ID]) != 1 || !m.attention.Checkpoint.Equal(q.Until) {
		t.Fatal("replay duplicated activity or failed to commit")
	}
	m.active = match.Conversation.ID
	m.dismissActive()
	m.discoveryNext = time.Time{}
	q = attentionTestCheck(t, m)
	m.Update(attentionTestResult(m, q, match, false))
	if len(m.attention.Pending) != 0 {
		t.Fatal("dismissed mention resurrected")
	}
	match.Trigger.ID = "new-mention"
	match.Trigger.TS = "1794672000.000002"
	match.Trigger.At = match.Trigger.At.Add(time.Microsecond)
	m.discoveryNext = time.Time{}
	q = attentionTestCheck(t, m)
	m.Update(attentionTestResult(m, q, match, false))
	if len(m.attention.Pending[match.Conversation.ID]) != 1 {
		t.Fatal("genuinely new activity did not return")
	}
}

func TestDiscoveryDisableCancelsAndRejectsLateResult(t *testing.T) {
	m := fixture(t)
	svc := &attentionTestService{Service: m.service}
	m.service = svc
	m.attention.Enabled = true
	cmd := m.checkMentions(time.Now())
	if cmd == nil {
		t.Fatal("no check")
	}
	result, ok := cmd().(mentionResult)
	if !ok {
		t.Fatal("wrong discovery result")
	}
	match := attentionTestMatch(t, "Clate")
	result.Page.Matches = []core.MentionMatch{match}
	m.toggleMentions()
	if svc.ctx == nil || svc.ctx.Err() == nil {
		t.Fatal("disable did not cancel read")
	}
	m.Update(result)
	if len(m.attention.Pending) != 0 || m.discoveryBusy || m.attention.Enabled {
		t.Fatal("late result reenabled discovery")
	}
}

func TestAttentionRestartAndWriteFailure(t *testing.T) {
	m := fixture(t)
	dir := t.TempDir()
	m.store.Path = filepath.Join(dir, testConfigName)
	q := attentionTestCheck(t, m)
	match := attentionTestMatch(t, attentionTestOutside)
	m.Update(attentionTestResult(m, q, match, false))
	m.active = match.Conversation.ID
	m.dismissActive()
	restored := New(m.ctx, m.service, m.identity, m.store, false)
	if len(restored.attention.Pending) != 0 || len(restored.attention.Handled) != 1 || !restored.attention.Enabled {
		t.Fatal("restart lost durable state")
	}
	q = attentionTestCheck(t, restored)
	restored.Update(attentionTestResult(restored, q, match, false))
	if len(restored.attention.Pending) != 0 {
		t.Fatal("restart resurrected handled activity")
	}
	block := filepath.Join(dir, "block")
	if err := os.WriteFile(block, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.store.Path = filepath.Join(block, testConfigName)
	m.attention.Pending[match.Conversation.ID] = []core.AttentionTrigger{match.Trigger}
	m.included[match.Conversation.ID] = true
	m.active = match.Conversation.ID
	m.dismissActive()
	if len(m.attention.Pending[match.Conversation.ID]) != 1 || !m.included[match.Conversation.ID] || !strings.Contains(m.status, "Cannot save") {
		t.Fatal("failed write silently dismissed attention")
	}
}

func TestAttentionNavigationAndViewportLayouts(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m := fixture(t)
		q := attentionTestCheck(t, m)
		match := attentionTestMatch(t, attentionTestOutside)
		m.Update(attentionTestResult(m, q, match, false))
		m.width, m.height = size[0], size[1]
		m.resize()
		m.focus = focusChannels
		rendered := ansi.Strip(m.View().Content)
		if !strings.Contains(rendered, "NEEDS ATTENTION") || !strings.Contains(rendered, "KEPT") {
			t.Fatal("attention groups hidden", rendered)
		}
		rows := strings.Split(rendered, "\n")
		if len(rows) != size[1] {
			t.Fatal("wrong height")
		}
		for _, row := range rows {
			if ansi.StringWidth(row) != size[0] {
				t.Fatal("wrong width", row)
			}
		}
		m.nextAttention()
		if m.active != match.Conversation.ID {
			t.Fatal("next attention picked wrong conversation")
		}
	}
}

func TestAttentionEnableRequiresExplicitConfirmation(t *testing.T) {
	m := fixture(t)
	m.Update(key(keyMentions))
	if m.attention.Enabled || !strings.Contains(m.status, "search:read") {
		t.Fatal("discovery enabled without scope disclosure")
	}
	m.Update(key(keyEscape))
	m.Update(key(keyMentions))
	if m.attention.Enabled {
		t.Fatal("confirmation survived intervening key")
	}
	m.Update(key(keyMentions))
	if !m.attention.Enabled || !m.discoveryBusy {
		t.Fatal("explicit enable failed")
	}
}

func TestDiscoveryPaginationBudgetAndChangingResults(t *testing.T) {
	t.Run("budget", func(t *testing.T) {
		m := fixture(t)
		q := attentionTestCheck(t, m)
		match := attentionTestMatch(t, attentionTestOutside)
		for page := 1; page <= discoveryPageLimit; page++ {
			q.Page = page
			m.Update(attentionTestResult(m, q, match, true))
		}
		if m.discoveryBusy || m.discoveryPhase != discoveryIncomplete || !m.attention.Checkpoint.IsZero() || len(m.attention.Pending[match.Conversation.ID]) != 1 {
			t.Fatal("budget completion skipped interval or lost results")
		}
	})
	t.Run("changing total", func(t *testing.T) {
		m := fixture(t)
		q := attentionTestCheck(t, m)
		match := attentionTestMatch(t, attentionTestOutside)
		first := attentionTestResult(m, q, match, true)
		first.Page.Total = 101
		m.Update(first)
		q.Page = 2
		last := attentionTestResult(m, q, match, false)
		last.Page.Total = 102
		m.Update(last)
		if m.discoveryPhase != discoveryIncomplete || !m.attention.Checkpoint.IsZero() {
			t.Fatal("changing pagination claimed completed interval")
		}
	})
}

func TestOpeningTriggerWhileHistoryIsAlreadyBusy(t *testing.T) {
	m := fixture(t)
	svc := &attentionTestService{Service: m.service}
	m.service = svc
	match := attentionTestMatch(t, m.active)
	match.Trigger.Thread = ""
	if err := m.attention.Add(m.active, match.Trigger); err != nil {
		t.Fatal(err)
	}
	line := m.line(m.active)
	line.Busy = true
	if cmd := m.fetchTrigger(match.Trigger); cmd != nil {
		t.Fatal("duplicate request started")
	}
	_, cmd := m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: core.Page{Messages: []core.Message{{TS: "1794671999.000000"}}}})
	if cmd == nil {
		t.Fatal("in-flight recent page swallowed exact target request")
	}
	m.Update(cmd())
	if len(svc.queries) != 1 || svc.queries[0].Target != attentionTestTS || m.index != len(line.Messages)-1 {
		t.Fatal("did not recover exact opening target")
	}
}

func TestUnavailableTriggerKeepsRecoveryAndManualTargetRetry(t *testing.T) {
	m := fixture(t)
	m.statusKind = statusRoutine
	m.openTargets[m.key()] = attentionTestTS
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Target: attentionTestTS, Page: core.Page{}})
	if !strings.Contains(m.activityStatus(), "Trigger unavailable") || m.openTargets[m.key()] != attentionTestTS || m.scroll != -1 {
		t.Fatal("deleted target silently fell back to newest")
	}
	svc := &attentionTestService{Service: m.service}
	m.service = svc
	cmd := m.fetch(m.key(), false)
	if cmd == nil {
		t.Fatal("manual retry unavailable")
	}
	m.Update(cmd())
	if len(svc.queries) != 1 || svc.queries[0].Target != attentionTestTS {
		t.Fatal("retry lost exact target")
	}
	m.openTargets[m.key()] = attentionTestTS
	m.Update(key(keyLast))
	if m.openTargets[m.key()] != "" {
		t.Fatal("explicit newest retained an obsolete anchor")
	}
}

func TestDismissalReleasesDiscoveredMetadataAndReadGeneration(t *testing.T) {
	m := fixture(t)
	q := attentionTestCheck(t, m)
	match := attentionTestMatch(t, attentionTestOutside)
	m.Update(attentionTestResult(m, q, match, false))
	oldGeneration := m.generations[match.Conversation.ID]
	m.active = match.Conversation.ID
	m.dismissActive()
	for _, c := range m.directory {
		if c.ID == match.Conversation.ID {
			t.Fatal("passive discovery metadata survived dismissal")
		}
	}
	if m.generations[match.Conversation.ID] != 0 || m.contexts[match.Conversation.ID] != nil {
		t.Fatal("dismissal leaked read state")
	}
	m.discoveryNext = time.Time{}
	q = attentionTestCheck(t, m)
	match.Trigger.ID = "later-activity"
	m.Update(attentionTestResult(m, q, match, false))
	if m.generations[match.Conversation.ID] <= oldGeneration {
		t.Fatal("read generation reused after reappearance")
	}
}
