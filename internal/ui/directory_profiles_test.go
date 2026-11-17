package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func TestDirectoryNamesNeverHoldLaterPages(t *testing.T) {
	m, svc := profileFixture(t)
	m.mode = pickerSwitch
	m.directory = []core.Conversation{{ID: "Dwaiting", Name: "Uwaiting", User: "Uwaiting", Kind: core.ConversationDM}}
	m.directoryNext = searchPageCursor
	m.directoryPagesLeft = directoryAutoPages
	m.directorySeen = make(map[string]bool)
	m.picker.SetValue("@Name Uwaiting")
	name := m.directoryProfileCmd() // Hold this command as if users.info is slow.
	if name == nil || m.directoryProfileCmd() != nil {
		t.Fatal("name lookup is not single flight")
	}
	_, page := m.Update(directoryResult{Cursor: "first", Page: core.DirectoryPage{Next: "last", Conversations: []core.Conversation{{ID: "Ccross", Name: "cross-team", Kind: core.ConversationChannel}}}})
	if page == nil || !m.directoryBusy || !m.directoryProfileBusy || len(svc.calls) != 0 {
		t.Fatal("pending name blocked directory progress")
	}
	m.picker.SetValue("#cross")
	if len(m.choices()) != 1 {
		t.Fatal("channel was hidden behind name resolution")
	}
	m.picker.SetValue("@Name Uwaiting")
	if len(m.choices()) != 0 {
		t.Fatal("unresolved name already matches")
	}
	m.Update(name())
	if len(m.choices()) != 1 || m.choices()[0].ID != "Dwaiting" || m.included["Dwaiting"] {
		t.Fatal("async name did not update search without inclusion")
	}
}

func TestDirectoryNamesCancelAndIgnoreOldResults(t *testing.T) {
	const delayedUser = "Udelayed"
	m, svc := profileFixture(t)
	m.mode = pickerInclude
	m.directory = []core.Conversation{{ID: "Ddelayed", Name: delayedUser, User: delayedUser, Kind: core.ConversationDM}}
	cmd := m.directoryProfileCmd()
	old := cmd()
	m.closePicker()
	m.openPicker(pickerInclude)
	m.Update(old)
	if !m.directoryProfileBusy || m.directory[0].Name != delayedUser {
		t.Fatal("old profile changed reopened picker")
	}
	m.closePicker()
	m.mode = pickerInclude
	m.directoryProfileAttempted = make(map[string]bool)
	queued := m.directoryProfileCmd()
	before := len(svc.calls)
	m.closePicker()
	m.Update(queued())
	if len(svc.calls) != before || m.directoryProfileBusy {
		t.Fatal("closed picker started a queued lookup")
	}
}

func TestDirectoryNamesFailuresAndBudgetStayPartial(t *testing.T) {
	m, svc := profileFixture(t)
	m.mode = pickerSwitch
	m.directory = nil
	for i := range profileLookupLimit + 1 {
		user := fmt.Sprintf("Uname%d", i)
		m.directory = append(m.directory, core.Conversation{ID: fmt.Sprintf("Dname%d", i), User: user, Name: user, Kind: core.ConversationDM})
	}
	cmd := m.directoryProfileCmd()
	for range profileLookupLimit {
		if cmd == nil {
			t.Fatal("name stream stopped before budget")
		}
		_, cmd = m.Update(cmd())
	}
	if cmd != nil || !m.directoryNamesPending() || !strings.Contains(m.directoryState(), "incomplete") {
		t.Fatal("name budget looped or claimed complete coverage")
	}
	m.directoryProfileAttempted = make(map[string]bool)
	svc.err = &slack.APIError{Code: "missing_scope"}
	m.Update(m.directoryProfileCmd()())
	if m.stopped || !m.directoryNamesPending() {
		t.Fatal("optional name failure stopped account or hid missing name")
	}
}
