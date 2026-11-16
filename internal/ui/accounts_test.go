package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
)

const obsoleteAccountResult = "obsolete"

type accountTestService struct {
	core.Service
	reads    int
	sends    int
	query    core.Query
	outgoing core.Outgoing
}

func (s *accountTestService) History(ctx context.Context, q core.Query) (core.Page, error) {
	if err := ctx.Err(); err != nil {
		return core.Page{}, err
	}
	s.reads++
	s.query = q
	return core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: "live"}}}, nil
}

func (s *accountTestService) Send(ctx context.Context, out core.Outgoing) (core.Message, error) {
	if err := ctx.Err(); err != nil {
		return core.Message{}, err
	}
	s.sends++
	s.outgoing = out
	return core.Message{TS: attentionTestTS, Text: out.Text}, nil
}

func accountFixture(t *testing.T, count int) (*Accounts, []*accountTestService) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}
	accounts := []Account{}
	services := []*accountTestService{}
	for i := range count {
		id := core.Identity{Workspace: fmt.Sprintf("T%d", i/2), User: fmt.Sprintf("U%d", i), WorkspaceName: fmt.Sprintf("Workspace %d", i/2), Username: fmt.Sprintf("user%d", i)}
		st.AccountWorkspace(id).Included[demoChannel] = true
		svc := &accountTestService{Service: demo.New()}
		services = append(services, svc)
		accounts = append(accounts, Account{Service: svc, Identity: id})
	}
	a := NewAccounts(ctx, accounts, st)
	// The scheduler itself is tested with explicit ticks, avoiding wall-clock waits.
	runAccountCommand(t, a, a.activate(0))
	return a, services
}

func runAccountCommand(t *testing.T, a *Accounts, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runAccountCommand(t, a, c)
		}
		return
	}
	if _, ok := msg.(tickMsg); ok {
		return
	}
	if msg != nil {
		_, next := a.Update(msg)
		runAccountCommand(t, a, next)
	}
}

func switchAccount(t *testing.T, a *Accounts, index int) {
	t.Helper()
	a.deactivate()
	runAccountCommand(t, a, a.activate(index))
}

func TestAccountIsolationAndRestoredView(t *testing.T) {
	a, _ := accountFixture(t, 3)
	first := a.current()
	first.thread, first.index, first.scroll, first.focus = attentionTestTS, 0, 4, focusCompose
	first.composer.SetValue(mainDraft)
	first.line(first.key()).Messages = []core.Message{{TS: attentionTestTS}}
	first.line(first.key()).Loaded = true
	first.prefs.Read[demoChannel] = "first-account-read"
	for _, index := range []int{1, 2, 0, 1, 0} {
		switchAccount(t, a, index)
		if index != 0 && (a.current().composer.Value() != "" || a.current().prefs.Read[demoChannel] == "first-account-read") {
			t.Fatal("cross-account state")
		}
	}
	if first.composer.Value() != mainDraft || first.thread != attentionTestTS || first.scroll != 4 || first.focus != focusCompose {
		t.Fatal("view/draft lost on return")
	}
	if a.sessions[0].model == a.sessions[1].model {
		t.Fatal("users share session")
	}
}

func TestAccountLateReadsRejectedAfterSwitchBack(t *testing.T) {
	a, _ := accountFixture(t, 2)
	first := a.current()
	epoch := a.sessions[0].epoch
	oldCtx := first.ctx
	gen := first.generations[demoChannel]
	switchAccount(t, a, 1)
	if oldCtx.Err() != context.Canceled {
		t.Fatal("read context not canceled")
	}
	switchAccount(t, a, 0)
	before := first.line(demoChannel).Messages[0].Text
	for _, msg := range []tea.Msg{
		historyResult{Key: demoChannel, Generation: gen, Page: core.Page{Messages: []core.Message{{TS: attentionTestTS, Text: obsoleteAccountResult}}}},
		directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{{ID: obsoleteAccountResult}}}},
		mentionResult{Generation: first.discoveryGeneration, Err: errors.New(obsoleteAccountResult)},
	} {
		a.Update(accountEvent{index: 0, epoch: epoch, message: msg})
	}
	if first.line(demoChannel).Messages[0].Text != before || strings.Contains(first.status, obsoleteAccountResult) {
		t.Fatal("late result accepted")
	}
	for _, c := range first.directory {
		if c.ID == obsoleteAccountResult {
			t.Fatal("late directory accepted")
		}
	}
}

func TestAccountSendFinishesForOriginalUser(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			a, services := accountFixture(t, 2)
			first := a.current()
			first.composer.SetValue(mainDraft)
			first.rememberDraft()
			cmd := a.wrap(0, a.sessions[0].epoch, first.send(false))
			switchAccount(t, a, 1)
			a.current().composer.SetValue("second-account draft")
			msg, ok := cmd().(accountEvent)
			if !ok {
				t.Fatal("send result not scoped to account")
			}
			if fail {
				msg.message = sendResult{Key: demoChannel, Err: errors.New("unknown outcome")}
			}
			a.Update(msg)
			if services[0].sends != 1 || services[1].sends != 0 || services[0].outgoing.Text != mainDraft {
				t.Fatal("send account changed/canceled")
			}
			if a.current().composer.Value() != "second-account draft" {
				t.Fatal("send changed active draft")
			}
			d := first.deliveries[demoChannel]
			if fail && (d.State != core.DeliveryFailed || !d.Uncertain || first.drafts[demoChannel] != mainDraft) {
				t.Fatal("failed recovery lost")
			}
			if !fail && (d.State != core.DeliverySent || first.composer.Value() != "") {
				t.Fatal("acknowledgement not routed")
			}
			switchAccount(t, a, 0)
			if services[0].sends != 1 {
				t.Fatal("activation retried send")
			}
		})
	}
}

func TestInactiveAccountsDoNotConsumeRefreshTurns(t *testing.T) {
	for _, count := range []int{1, 2, 20} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a, services := accountFixture(t, count)
			initial := services[0].reads
			for range 6 {
				_, cmd := a.current().updateTick(tickMsg(time.Now()))
				runAccountCommand(t, a, a.wrap(0, a.sessions[0].epoch, cmd))
			}
			if services[0].reads-initial != 6 {
				t.Fatal("active cadence diluted")
			}
			for _, s := range services[1:] {
				if s.reads != 0 {
					t.Fatal("inactive polling")
				}
			}
		})
	}
}

func TestAccountExitChecksInactiveTextAndHelpConsumesKeys(t *testing.T) {
	a, _ := accountFixture(t, 2)
	a.current().composer.SetValue(mainDraft)
	switchAccount(t, a, 1)
	_, cmd := a.Update(key(keyQuitQuiet))
	if cmd != nil || !strings.Contains(a.current().status, "Unsent") {
		t.Fatal("inactive draft bypassed quit protection")
	}
	ctrlC := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	_, cmd = a.Update(ctrlC)
	if cmd != nil || !a.quitConfirm {
		t.Fatal("discard did not require confirmation")
	}
	a.Update(tea.WindowSizeMsg{Width: 40, Height: 14})
	if !strings.Contains(ansi.Strip(a.View().Content), "Ctrl+C again discards/exits") {
		t.Fatal("minimum layout clipped discard instruction")
	}
	a.current().help = true
	a.Update(ctrlC)
	if a.current().help || a.quitConfirm {
		t.Fatal("help did not consume quit key")
	}
	a.Update(ctrlC)
	_, cmd = a.Update(ctrlC)
	if cmd == nil {
		t.Fatal("explicit confirmed quit blocked")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("not quit")
	}
}

func TestAccountSelectorLayouts(t *testing.T) {
	a, _ := accountFixture(t, 3)
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		a.choosing = true
		a.choice = 2
		content := ansi.Strip(a.View().Content)
		rows := strings.Split(content, "\n")
		if len(rows) != size[1] {
			t.Fatal("selector height", size, len(rows))
		}
		for _, row := range rows {
			if ansi.StringWidth(row) != size[0] {
				t.Fatal("selector width", size, row)
			}
		}
		if !strings.Contains(content, "[Workspaces]") || !strings.Contains(content, "@user2") {
			t.Fatal("missing selected identity", content)
		}
	}
}

func TestAccountCanceledQueuedReadNeverStartsService(t *testing.T) {
	a, services := accountFixture(t, 2)
	m := a.current()
	cmd := a.wrap(0, a.sessions[0].epoch, m.fetch(m.key(), false))
	before := services[0].reads
	switchAccount(t, a, 1)
	runAccountCommand(t, a, cmd)
	if services[0].reads != before {
		t.Fatal("queued inactive read started service")
	}
}

func TestSwitchRetainsPartialDiscoveryAndRejectsObsoleteCompletion(t *testing.T) {
	a, _ := accountFixture(t, 2)
	a.store.Path = filepath.Join(t.TempDir(), "config.json")
	m := a.current()
	m.service = &attentionTestService{Service: m.service}
	m.attention.Enabled = true
	checkpoint := time.Now().Add(-time.Hour)
	m.attention.Checkpoint = checkpoint
	m.discoveryNext = time.Time{}
	if m.checkMentions(time.Now()) == nil {
		t.Fatal("no discovery")
	}
	q := core.MentionQuery{Reason: core.AttentionMention, User: m.identity.User, From: checkpoint.Add(-discoveryOverlap), Until: time.Now(), Page: 1}
	epoch, gen := a.sessions[0].epoch, m.discoveryGeneration
	match := attentionTestMatch(t, attentionTestOutside)
	trigger, err := core.MessageTrigger(messageBound(checkpoint), "", core.AttentionMention)
	if err != nil {
		t.Fatal(err)
	}
	match.Trigger = trigger
	partial := mentionResult{Query: q, Generation: gen, Page: core.MentionPage{Matches: []core.MentionMatch{match}, More: true, Total: 2}}
	// Accept the first page but hold the second command across deactivation.
	_, next := a.Update(accountEvent{index: 0, epoch: epoch, message: partial})
	if next == nil || len(m.attention.Pending[attentionTestOutside]) != 1 || !m.attention.Checkpoint.Equal(checkpoint) {
		t.Fatal("partial acceptance advanced coverage")
	}
	saved, err := a.store.Attention(m.identity)
	if err != nil || len(saved.Pending[attentionTestOutside]) != 1 || !saved.Checkpoint.Equal(checkpoint) {
		t.Fatal("partial activity not durable", err)
	}
	switchAccount(t, a, 1)
	runAccountCommand(t, a, next)
	if !m.attention.Checkpoint.Equal(checkpoint) {
		t.Fatal("inactive completion advanced checkpoint")
	}
	a.deactivate()
	activation := a.activate(0)
	if activation == nil || !m.discoveryBusy || !m.attention.Enabled || len(m.attention.Pending[attentionTestOutside]) != 1 {
		t.Fatal("activation lost partial activity or did not catch up")
	}
	q.Page = 2
	a.Update(accountEvent{index: 0, epoch: epoch, message: mentionResult{Query: q, Generation: gen, Page: core.MentionPage{Total: 2}}})
	if !m.attention.Checkpoint.Equal(checkpoint) {
		t.Fatal("obsolete terminal page advanced checkpoint")
	}
}

func TestLongAbsenceRecordsGapAndAuthFailureIsAccountLocal(t *testing.T) {
	a, _ := accountFixture(t, 2)
	first := a.current()
	first.service = &attentionTestService{Service: first.service}
	first.attention.Enabled = true
	first.attention.Checkpoint = time.Now().Add(-2 * core.AttentionLookback)
	first.attention.Floor = first.attention.Checkpoint
	switchAccount(t, a, 1)
	a.deactivate()
	cmd := a.activate(0)
	if cmd == nil || first.attention.Gap == nil || !first.discoveryBusy {
		t.Fatal("long absence claimed complete coverage")
	}
	a.Update(accountEvent{index: 0, epoch: a.sessions[0].epoch, message: mentionResult{Generation: first.discoveryGeneration, Query: core.MentionQuery{Page: first.discoveryPage, Reason: core.AttentionMention}, Err: &slack.APIError{Code: "account_inactive"}}})
	if !first.stopped {
		t.Fatal("expired account not stopped")
	}
	switchAccount(t, a, 1)
	if a.current().stopped || a.current().attention.Enabled {
		t.Fatal("auth failure/consent crossed accounts")
	}
	reads := a.current().pollCount
	_, cmd = a.current().updateTick(tickMsg(time.Now()))
	runAccountCommand(t, a, a.wrap(1, a.sessions[1].epoch, cmd))
	if a.current().pollCount != reads+1 {
		t.Fatal("healthy account stopped polling")
	}
}

func TestAccountDisplayNamesCannotInjectRows(t *testing.T) {
	a, _ := accountFixture(t, 2)
	id := a.current().identity
	id.WorkspaceName = "Remote\nWorkspace\x1b[31m"
	id.Username = "remote\nuser"
	a.current().identity = id
	a.sessions[0].account.Identity = id
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, choosing := range []bool{false, true} {
			a.choosing = choosing
			rows := strings.Split(ansi.Strip(a.View().Content), "\n")
			if len(rows) != size[1] {
				t.Fatal("remote account name injected a row", size, choosing)
			}
			for _, row := range rows {
				if ansi.StringWidth(row) != size[0] {
					t.Fatal("remote account name broke width", row)
				}
			}
		}
	}
}
