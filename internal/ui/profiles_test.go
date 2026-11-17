package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
)

type profileTestService struct {
	core.Service
	profiles map[string]core.Profile
	calls    []string
	err      error
	capacity int
}

func (s *profileTestService) Profile(ctx context.Context, id string) (core.Profile, error) {
	if ctx.Err() != nil {
		return core.Profile{}, ctx.Err()
	}
	s.calls = append(s.calls, id)
	if s.err != nil {
		return core.Profile{}, s.err
	}
	p := core.Profile{ID: id, Name: "Name " + id, Updated: time.Now()}
	if s.capacity > 0 && len(s.profiles) >= s.capacity {
		for key := range s.profiles {
			delete(s.profiles, key)
			break
		}
	}
	s.profiles[id] = p
	return p, nil
}

func (s *profileTestService) CachedProfiles() []core.Profile {
	ps := []core.Profile{}
	for _, p := range s.profiles {
		ps = append(ps, p)
	}
	return ps
}

func (s *profileTestService) RestoreProfiles(ps []core.Profile) {
	for _, p := range ps {
		s.profiles[p.ID] = p
	}
}

func profileFixture(t *testing.T) (*Model, *profileTestService) {
	t.Helper()
	m := fixture(t)
	svc := &profileTestService{Service: m.service, profiles: map[string]core.Profile{}}
	m.service = svc
	m.cache[m.active] = &timeline{Busy: true}
	return m, svc
}

func TestHistoryRendersBeforeNamesAndEnrichesWithoutMutatingDraft(t *testing.T) {
	m, svc := profileFixture(t)
	m.focus = focusCompose
	m.composer.SetValue(mainDraft)
	_, cmd := m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: []core.Message{{TS: futureTimestamp, User: "U1", Text: "hello <@U2>"}}}})
	if !m.cache[m.key()].Loaded || len(svc.calls) != 0 || cmd == nil {
		t.Fatal("messages waited for names")
	}
	if got := strings.Join(m.history(60, 12), "\n"); !strings.Contains(got, "hello @U2") {
		t.Fatal(got)
	}
	_, next := m.Update(cmd())
	if next == nil || !strings.Contains(strings.Join(m.history(60, 12), "\n"), "Name U1") {
		t.Fatal("author did not appear incrementally")
	}
	_, last := m.Update(next())
	if last != nil || !strings.Contains(strings.Join(m.history(60, 12), "\n"), "hello @Name U2") {
		t.Fatal("mention not resolved")
	}
	if m.cache[m.key()].Messages[0].Text != "hello <@U2>" || m.composer.Value() != mainDraft || m.focus != focusCompose {
		t.Fatal("enrichment mutated cached text/draft/focus")
	}
}

func TestNameLookupSwitchCancelsAndRejectsLateResponse(t *testing.T) {
	m, _ := profileFixture(t)
	m.cache[m.active] = &timeline{Loaded: true, Messages: []core.Message{{User: "U1"}}}
	pending := m.profileCmd()
	m.selectConversation(demoDM)
	_, current := m.Update(historyResult{Key: demoDM, Generation: m.generations[demoDM], Page: core.Page{Messages: []core.Message{{TS: futureTimestamp, User: "U2"}}}})
	if pending == nil || current == nil {
		t.Fatal("missing name lookup")
	}
	m.Update(pending())
	if !m.profileBusy || m.profileKey != demoDM {
		t.Fatal("late response cleared active lookup")
	}
	m.Update(current())
	if m.profiles["U2"].Name == "" || m.profiles["U1"].Name != "" {
		t.Fatal("canceled identity applied")
	}
}

func TestNameFailuresRequireRefreshAndLookupAttemptsRemainBounded(t *testing.T) {
	m, svc := profileFixture(t)
	m.cache[m.active] = &timeline{Loaded: true, Messages: []core.Message{{User: "U1"}}}
	svc.err = errors.New("offline names")
	cmd := m.profileCmd()
	_, next := m.Update(cmd())
	if next != nil || m.profileCmd() != nil || len(svc.calls) != 1 {
		t.Fatal("name failure retried automatically")
	}
	svc.err = nil
	m.refresh()
	if !m.profileBusy {
		t.Fatal("manual refresh did not retry name")
	}
	m.cancelProfileLookup()
	for i := 0; i < profileLookupLimit; i++ {
		m.profileAttempted[fmt.Sprint(i)] = true
	}
	if m.profileCmd() != nil {
		t.Fatal("unbounded name attempts")
	}
}

func TestNameEvictionDoesNotCauseLookupLoop(t *testing.T) {
	m, svc := profileFixture(t)
	svc.capacity = 1
	m.cache[m.active] = &timeline{Loaded: true, Messages: []core.Message{{User: "U1"}, {User: "U2"}, {User: "U3"}}}
	cmd := m.profileCmd()
	for attempts := 0; cmd != nil; attempts++ {
		if attempts > 3 {
			t.Fatal("evicted identities retried endlessly")
		}
		_, cmd = m.Update(cmd())
	}
	if len(svc.calls) != 3 || svc.calls[0] != "U3" {
		t.Fatal("names not bounded or newest-first", svc.calls)
	}
}

func TestNamesSurviveRestartWithoutCrossAccountReuse(t *testing.T) {
	m, svc := profileFixture(t)
	st, err := store.Open(filepath.Join(t.TempDir(), testConfigName))
	if err != nil {
		t.Fatal(err)
	}
	m.store = st
	m.cache[m.active] = &timeline{Loaded: true, Messages: []core.Message{{User: "U1"}}}
	m.Update(m.profileCmd()())
	if len(st.Profiles(m.identity)) != 1 {
		t.Fatal("successful name was not saved")
	}
	fresh := &profileTestService{Service: svc.Service, profiles: map[string]core.Profile{}}
	restored := New(m.ctx, fresh, m.identity, st, false)
	if restored.profiles["U1"].Name == "" || len(fresh.calls) != 0 {
		t.Fatal("name not restored without network")
	}
	other := m.identity
	other.User += "-other"
	isolated := New(m.ctx, &profileTestService{Service: svc.Service, profiles: map[string]core.Profile{}}, other, st, false)
	if len(isolated.profiles) != 0 {
		t.Fatal("cross-account names")
	}
}

func TestInitialActiveHistoryTakesPriorityWithoutAdvancingRotation(t *testing.T) {
	m, svc := pollingFixture(t, 3)
	m.cache[m.active].Loaded = false
	m.pollIndex = 2
	assertPoll(t, m, svc, m.active)
	if m.pollIndex != 2 || m.pollCount != 0 {
		t.Fatal("initial history consumed background turn")
	}
	assertPoll(t, m, svc, m.directory[2].ID)
	m.cache[m.active].Loaded = false
	m.cache[m.active].Busy = true
	before := len(svc.queries)
	if pollCommand(t, m) != nil || len(svc.queries) != before {
		t.Fatal("background read competed with initial active request")
	}
}

func TestFailedInitialHistoryDoesNotStarveOtherConversations(t *testing.T) {
	m, svc := pollingFixture(t, 3)
	m.cache[m.active].Loaded = false
	m.cache[m.active].Err = errors.New("conversation unavailable")
	m.pollIndex = 2
	assertPoll(t, m, svc, m.directory[2].ID)
}

func TestProfileAuthenticationFailureStopsReadsWithoutHidingMessages(t *testing.T) {
	m, svc := profileFixture(t)
	m.composer.SetValue(mainDraft)
	m.cache[m.active] = &timeline{Loaded: true, Messages: []core.Message{{TS: futureTimestamp, User: "U1", Text: "visible before name resolution"}}}
	svc.err = &slack.APIError{Code: "token_revoked"}
	m.Update(m.profileCmd()())
	if !m.stopped || m.composer.Value() != mainDraft || !strings.Contains(strings.Join(m.history(60, 12), "\n"), "visible before name resolution") {
		t.Fatal("auth error lost messages/draft or kept reading")
	}
}

func TestSavedDMNameEnrichmentDoesNotBlockHistory(t *testing.T) {
	const participant = "Udm"
	m, svc := profileFixture(t)
	m.active = demoDM
	m.Init()
	c := core.Conversation{ID: demoDM, Name: participant, User: participant, Kind: core.ConversationDM}
	_, cmd := m.Update(conversationResult{ID: demoDM, Conversation: c, Generation: m.generations[demoDM]})
	if cmd == nil || !m.cache[demoDM].Busy || len(svc.calls) != 0 {
		t.Fatal("saved DM waited for name enrichment before loading messages")
	}
	m.composer.SetValue(mainDraft)
	m.Update(cmd())
	for _, item := range m.directory {
		if item.ID == demoDM && (item.Name != "Name Udm" || item.User != participant) {
			t.Fatal("DM identity not enriched", item)
		}
	}
	if m.active != demoDM || m.composer.Value() != mainDraft || !m.cache[demoDM].Busy {
		t.Fatal("name result changed the draft, selection or pending history")
	}
}
