package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
)

const (
	opaqueTestID            = "CABCDEFGHIJ"
	directTestID            = "C0123456789"
	directTestQuery         = "#cross-pod"
	directDraft             = "draft before direct opening"
	missingConversationCode = "channel_not_found"
)

type directTestService struct {
	core.Service
	result core.Conversation
	err    error
	calls  []string
}

func (s *directTestService) Conversation(ctx context.Context, id string) (core.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return core.Conversation{}, err
	}
	s.calls = append(s.calls, id)
	return s.result, s.err
}

func TestDirectTargetValidation(t *testing.T) {
	for _, tc := range []struct {
		query, id        string
		recognized, fail bool
	}{
		{directTestQuery, "", false, false},
		{"cross-pod", "", false, false},
		{directTestID, directTestID, true, false},
		{opaqueTestID, "", false, false},
		{"id:" + opaqueTestID, opaqueTestID, true, false},
		{"https://sample.slack.com/archives/" + opaqueTestID, opaqueTestID, true, false},
		{"https://sample.slack.com/archives/" + directTestID, directTestID, true, false},
		{"https://sample.slack.com/archives/" + directTestID + "/p1234567890000001", directTestID, true, false},
		{"https://app.slack.com/client/TTEST/" + directTestID, directTestID, true, false},
		{"slack://channel?team=TTEST&id=" + directTestID, directTestID, true, false},
		{"slack://channel?team=TOTHER&id=" + directTestID, "", true, true},
		{"https://app.slack.com/client/TOTHER/" + directTestID, "", true, true},
		{"https://sample.slack.com.evil.test/archives/" + directTestID, "", true, true},
		{"https://user:secret@sample.slack.com/archives/" + directTestID, "", true, true},
		{"https://sample.slack.com/archives/not-an-id", "", true, true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			id, recognized, err := directTarget(tc.query, "TTEST")
			if id != tc.id || recognized != tc.recognized || (err != nil) != tc.fail {
				t.Fatal(id, recognized, err)
			}
		})
	}
}

func directFixture(t *testing.T) (*Model, *directTestService) {
	t.Helper()
	m := fixture(t)
	svc := &directTestService{Service: m.service, result: core.Conversation{ID: directTestID, Name: "cross-pod", Kind: core.ConversationChannel}}
	m.service = svc
	m.composer.SetValue(directDraft)
	m.openPicker(pickerSwitch)
	return m, svc
}

func TestDirectOpeningValidatesBeforeInclusionAndKeepsDraft(t *testing.T) {
	m, svc := directFixture(t)
	old := m.key()
	m.picker.SetValue("https://sample.slack.com/archives/" + directTestID)
	cmd := m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.included[directTestID] || len(svc.calls) != 0 || !m.directBusy {
		t.Fatal("typing/starting lookup included a conversation")
	}
	m.Update(cmd())
	if m.mode != pickerClosed || m.active != directTestID || !m.included[directTestID] || m.drafts[old] != directDraft || len(svc.calls) != 1 {
		t.Fatal("direct open lost explicit intent or draft")
	}
}

func TestDirectLookupFailureCancellationAndMismatch(t *testing.T) {
	for _, scenario := range []string{"denied", "queued cancel", "delayed cancel", "wrong conversation", "wrong workspace", "changed input"} {
		t.Run(scenario, func(t *testing.T) {
			m, svc := directFixture(t)
			m.picker.SetValue(directTestID)
			if scenario == "wrong workspace" {
				m.picker.SetValue("https://app.slack.com/client/TOTHER/" + directTestID)
				if cmd := m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || m.directErr == nil || len(svc.calls) != 0 {
					t.Fatal("wrong workspace started lookup")
				}
				return
			}
			if scenario == "denied" {
				svc.err = &slack.APIError{Code: missingConversationCode}
			}
			if scenario == "wrong conversation" {
				svc.result.ID = "Canother"
			}
			cmd := m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			switch scenario {
			case "queued cancel":
				m.closePicker()
				m.Update(cmd())
				if len(svc.calls) != 0 {
					t.Fatal("canceled queued lookup called service")
				}
			case "changed input":
				late := cmd()
				m.picker.SetValue("")
				m.Update(tea.PasteMsg{Content: "#another-query"})
				m.Update(late)
			case "delayed cancel":
				late := cmd()
				m.closePicker()
				m.openPicker(pickerSwitch)
				m.Update(late)
			default:
				m.Update(cmd())
			}
			if m.included[directTestID] || m.active == directTestID || m.composer.Value() != directDraft {
				t.Fatal("failed/obsolete lookup changed target or draft")
			}
		})
	}
}

func TestCachedDirectoryRestoresHintsAndRevalidates(t *testing.T) {
	m, svc := directFixture(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "directory-prefs.json"))
	if err != nil {
		t.Fatal(err)
	}
	id := m.identity
	if err = st.SaveDirectory(id, []core.Conversation{svc.result}); err != nil {
		t.Fatal(err)
	}
	fresh := New(m.ctx, svc, id, st, false)
	fresh.openPicker(pickerSwitch)
	fresh.picker.SetValue(directTestQuery)
	if len(fresh.choices()) != 1 || fresh.directoryLoaded || !fresh.directoryCached[directTestID] || fresh.included[directTestID] {
		t.Fatal("cache did not restore isolated incomplete suggestions")
	}
	cmd := fresh.openChoice(fresh.choices())
	if cmd == nil || fresh.included[directTestID] {
		t.Fatal("cached result bypassed validation")
	}
	svc.err = &slack.APIError{Code: missingConversationCode}
	fresh.Update(cmd())
	if fresh.included[directTestID] || len(st.Directory(id)) != 0 || len(fresh.choices()) != 0 {
		t.Fatal("denied cache hint survived or became included")
	}
	if fresh.directErr == nil || !strings.Contains(fresh.directoryState(), "Error") {
		t.Fatal("lookup error was hidden")
	}
}

func TestCachedLookupMismatchDoesNotTrustHint(t *testing.T) {
	m, svc := directFixture(t)
	m.directoryCached[directTestID] = true
	m.replaceConversation(svc.result)
	svc.err = errors.New("lookup unavailable")
	m.picker.SetValue(directTestQuery)
	cmd := m.temporaryChoice(m.choices())
	m.Update(cmd())
	if m.included[directTestID] || m.mode == pickerClosed {
		t.Fatal("failed cached temporary lookup changed inclusion")
	}
}

func TestBareIDActionCancelsDiscoveryOnlyOnEnter(t *testing.T) {
	m, svc := directFixture(t)
	m.cancelDirectory()
	m.directoryLoaded = false
	m.directoryNext = ""
	m.continueDirectory()
	m.Update(tea.PasteMsg{Content: directTestID})
	if !m.directoryBusy || m.directBusy || len(svc.calls) != 0 {
		t.Fatal("bare shorthand stopped discovery before explicit action")
	}
	cmd := m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.directBusy || m.directoryBusy {
		t.Fatal("Enter did not switch from discovery to lookup")
	}
	m.Update(cmd())
	if m.active != directTestID || len(svc.calls) != 1 {
		t.Fatal("bare ID lookup failed")
	}
}
