package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	sparseEarlierTS   = "100.01"
	sparseLaterTS     = "100.02"
	sparseEarlierBody = "visible earlier"
	sparseLaterBody   = "visible later"
)

func historyTextRow(m *Model, text string) int {
	_, w, h, _, _ := m.dimensions()
	for i, row := range m.history(w, h) {
		if strings.Contains(ansi.Strip(row), text) {
			return i
		}
	}
	return -1
}

func TestUnderfullResizeDoesNotMoveBrowsing(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{{TS: sparseEarlierTS, Author: "Sparse earlier author", Text: sparseEarlierBody}, {TS: sparseLaterTS, Author: "Sparse later author", Text: sparseLaterBody}}}
	m.index, m.scroll, m.focus = 0, 1, focusMessages
	before := historyTextRow(m, sparseEarlierBody)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 55})
	after := historyTextRow(m, sparseEarlierBody)
	if before != after {
		t.Fatalf("same-size resize moved existing text from row %d to %d; scroll=%d", before, after, m.scroll)
	}
}

func TestPinnedSparsePendingReadKeepsRows(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	msgs := []core.Message{{TS: sparseEarlierTS, Author: "Sparse earlier author", Text: sparseEarlierBody}, {TS: sparseLaterTS, Author: "Sparse later author", Text: sparseLaterBody}}
	m.cache[m.key()] = &timeline{Loaded: true, Messages: msgs}
	m.index, m.scroll, m.focus = 1, 0, focusMessages
	m.Update(key(keyPrevious))
	before := historyTextRow(m, sparseEarlierBody)
	msgs = append(msgs, core.Message{TS: "100.03", Author: "Incoming", Text: "new incoming"})
	m.Update(historyResult{Key: m.key(), Generation: m.generations[m.active], Page: core.Page{Messages: msgs}})
	after := historyTextRow(m, sparseEarlierBody)
	if before != after {
		t.Fatalf("pending read moved pinned text from row %d to %d; scroll=%d", before, after, m.scroll)
	}
}

func TestContinuationDoesNotShowHeaderForMessageOutsideWindow(t *testing.T) {
	rows := []string{"old author", "old body", "", "middle author", "middle body", "", "ending author", testNewestBody, ""}
	got := historyWindow(rows, []int{0, 3, 6}, 6, 9, 60, 5, 0)
	if strings.Contains(ansi.Strip(got[0]), "middle author") && !strings.Contains(strings.Join(got, "\n"), "middle body") {
		t.Fatalf("continuation header without any continued message content: %q", got)
	}
}

func TestCachedDMNameSurvivesDirectVerificationWithoutProfile(t *testing.T) {
	m, svc := profileFixture(t)
	svc.err = errors.New("profile unavailable")
	m.store.Path = filepath.Join(t.TempDir(), testConfigName)
	old := core.Conversation{ID: demoDM, Name: "Cached colleague", Handle: "colleague", User: "U_REVIEW", Kind: core.ConversationDM}
	m.replaceConversation(old)
	m.directoryCached[old.ID] = true
	m.openPicker(pickerSwitch)
	_, cmd := m.Update(directConversationResult{ID: old.ID, Generation: m.directoryGeneration, Action: directOpen, Conversation: core.Conversation{ID: old.ID, Name: old.User, User: old.User, Kind: core.ConversationDM}})
	runAttentionReviewCommand(t, m, cmd)
	for _, c := range m.directory {
		if c.ID == old.ID && c.Name != old.Name {
			t.Fatalf("direct verification discarded cached DM name %q and left %q after optional profile failure", old.Name, c.Name)
		}
	}
	stored := false
	for _, c := range m.store.Directory(m.identity) {
		if c.ID == old.ID {
			stored = true
			if c.Name != old.Name || c.Handle != old.Handle {
				t.Fatal("verified cached DM lost its persisted name or handle", c)
			}
		}
	}
	if !stored {
		t.Fatal("verified cached DM hint was not persisted")
	}
}

func TestPageUpStartsFromPinnedViewport(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Loaded: true, Messages: readingMessages(1, 40)}
	m.index, m.scroll, m.focus = 20, -1, focusMessages
	m.Update(key(keyPrevious))
	before, _ := selectionViewport(t, m)
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	after, _ := selectionViewport(t, m)
	if after >= before {
		t.Fatal("page up did not move upward from the pinned source row", before, after)
	}
}

func TestSparsePinResizeKeepsSelectedBodyVisible(t *testing.T) {
	m := fixture(t)
	m.width, m.height = 100, 55
	m.resize()
	m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{
		{TS: sparseEarlierTS, Author: "Sparse first", Text: sparseEarlierBody},
		{TS: sparseLaterTS, Author: "Sparse next", Text: sparseLaterBody},
	}}
	m.index, m.scroll, m.focus = 1, 0, focusMessages
	m.Update(key(keyPrevious))
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if historyTextRow(m, sparseEarlierBody) < 0 || m.cache[m.key()].Messages[m.index].TS != sparseEarlierTS {
		t.Fatal("shrinking a sparse pinned view hid the selected body")
	}
}
