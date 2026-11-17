package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func TestReviewRecoverySurvivesOrdinaryKeys(t *testing.T) {
	for _, state := range []string{"credential stopped", "delivery uncertain", "delivery failure"} {
		t.Run(state, func(t *testing.T) {
			m := fixture(t)
			want := ""
			switch state {
			case "credential stopped":
				m.stopReads()
				want = m.credentialRecovery()
			case "delivery uncertain":
				m.deliveries[m.key()] = &core.Delivery{State: core.DeliveryFailed, Uncertain: true}
				m.setStatus("Delivery unknown", statusAction)
				want = "Delivery unknown"
			case "delivery failure":
				m.deliveries[m.key()] = &core.Delivery{State: core.DeliveryFailed, Err: errors.New("offline failure")}
				m.setStatus("Transient send feedback", statusAction)
				want = "offline failure"
			}
			for _, k := range []tea.KeyPressMsg{{Code: tea.KeyTab}, key(keyEscape)} {
				m.Update(k)
				if !strings.Contains(m.activityStatus(), want) {
					t.Fatalf("%s recovery disappeared: %q", state, m.activityStatus())
				}
			}
		})
	}
}

func TestReviewUppercaseNamesRemainSearches(t *testing.T) {
	for _, query := range []string{"DESIGN", "GENERAL", "DISCUSSION", "CLOUD2025"} {
		t.Run(query, func(t *testing.T) {
			m, svc := directFixture(t)
			m.composer.Reset()
			c := core.Conversation{ID: "Cname", Name: strings.ToLower(query), Kind: core.ConversationChannel}
			m.directory = []core.Conversation{c}
			m.picker.SetValue(query)
			m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.directBusy || len(svc.calls) != 0 || m.active != c.ID || m.mode != pickerClosed {
				t.Fatal("name query was handled as a direct ID", query)
			}
		})
	}
}

func TestReviewDirectoryPagesKeepKnownDMNames(t *testing.T) {
	m, svc := profileFixture(t)
	m.mode = pickerSwitch
	known := core.Conversation{ID: "Dknown", Name: "Known Person", Handle: "known.handle", User: "Uknown", Kind: core.ConversationDM}
	m.directory = []core.Conversation{known}
	m.directoryCached[known.ID] = true
	m.picker.SetValue("@known.handle")
	m.directoryProfileAttempted = make(map[string]bool)
	for i := range profileLookupLimit {
		m.directoryProfileAttempted[strings.Repeat("x", i+1)] = true
	}
	fresh := known
	fresh.Name = known.User
	fresh.Handle = ""
	m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{fresh}}})
	if len(m.choices()) != 1 || m.directory[0].Name != known.Name || m.directory[0].Handle != known.Handle || len(svc.calls) != 0 {
		t.Fatal("unresolved live page erased known identity", m.directory)
	}
	fresh.Name, fresh.Handle = "Renamed Person", "renamed.handle"
	m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{fresh}}})
	if m.directory[0].Name != fresh.Name || m.directory[0].Handle != fresh.Handle {
		t.Fatal("known names prevented an actual metadata update")
	}
}

func TestReviewCachedKeptRemovalNeedsNoLookup(t *testing.T) {
	m, svc := directFixture(t)
	cid := m.active
	m.composer.Reset()
	m.mode = pickerInclude
	m.directoryCached[cid] = true
	m.picker.SetValue("")
	for i, c := range m.choices() {
		if c.ID == cid {
			m.pickIndex = i
			break
		}
	}
	svc.err = errors.New("network offline")
	cmd := m.pickerKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd != nil || m.directBusy || len(svc.calls) != 0 || m.attention.Kept[cid] || m.included[cid] {
		t.Fatal("local unkeep required remote validation")
	}
}

func TestReviewKnownDMNamesStillEnrichIndependently(t *testing.T) {
	m, _ := profileFixture(t)
	m.mode = pickerSwitch
	known := core.Conversation{ID: "Drefresh", User: "Urefresh", Name: "Existing Name", Kind: core.ConversationDM}
	m.directory = []core.Conversation{known}
	fresh := known
	fresh.Name = known.User
	_, cmd := m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{fresh}}})
	if cmd == nil || m.directory[0].Name != known.Name || !m.directoryNamesPending() {
		t.Fatal("preserved names disabled asynchronous enrichment")
	}
	m.Update(cmd())
	if m.directory[0].Name == known.Name || m.directoryNamesPending() {
		t.Fatal("name enrichment did not replace preserved hint")
	}
}

func TestReviewPersistentRecoveryFitsMinimumFooter(t *testing.T) {
	m := fixture(t)
	m.Update(tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
	m.multipleAccounts = true
	m.stopReads()
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assertStatusBar(t, m, m.credentialRecovery())
	m.stopped = false
	m.deliveries[m.key()] = &core.Delivery{State: core.DeliveryFailed, Uncertain: true}
	m.status = ""
	assertStatusBar(t, m, "Delivery unknown; check Slack; r retry")
}
