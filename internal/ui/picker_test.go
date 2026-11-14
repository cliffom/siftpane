package ui

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func TestSearchCompletionRequiresExplicitInclusion(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerSwitch)
	m.Update(tea.PasteMsg{Content: "@LEO.P"})
	cs := m.choices()
	if len(cs) != 1 || cs[0].ID != "Dleo" || m.included["Dleo"] {
		t.Fatal("search must find excluded DM without including it", cs)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.picker.Value() != "@leo.park" || m.included["Dleo"] || m.contexts["Dleo"] != nil {
		t.Fatal("completion changed inclusion or lost handle")
	}
	if !strings.Contains(m.choiceAction(m.choices(), m.pickIndex), "include & open") {
		t.Fatal("new inclusion must be explicit")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.included["Dleo"] || m.active != "Dleo" || m.mode != pickerClosed {
		t.Fatal("explicit Enter did not include and open")
	}
}

func TestSearchDismissalRestoresThreadComposer(t *testing.T) {
	for _, mod := range []tea.KeyMod{tea.ModCtrl, tea.ModSuper} {
		m := fixture(t)
		m.openThread()
		target := m.key()
		m.focus = focusCompose
		m.composer.Focus()
		m.composer.SetValue("thread draft retained")
		m.Update(tea.KeyPressMsg{Code: 'k', Mod: mod})
		m.Update(tea.PasteMsg{Content: searchPrivateQuery})
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m.Update(key(keySend))
		m.Update(key(keyEscape))
		if m.mode != pickerClosed || m.key() != target || m.focus != focusCompose || !m.composer.Focused() {
			t.Fatal("dismissal lost thread or composing focus")
		}
		if m.composer.Value() != "thread draft retained" || m.drafts[target] != m.composer.Value() || m.included["Cquiet"] || len(m.deliveries) != 0 {
			t.Fatal("search completion/cancellation modified inclusion, draft, or outbox")
		}
		m.Update(tea.PasteMsg{Content: " and editable"})
		if m.composer.Value() != "thread draft retained and editable" {
			t.Fatal("composer did not resume accepting input")
		}
	}
}

func TestPendingSendDoesNotBlockSearchPaste(t *testing.T) {
	m := fixture(t)
	m.focus = focusCompose
	m.composer.SetValue("pending body")
	m.send(false)
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	m.Update(tea.PasteMsg{Content: "#DES"})
	if len(m.choices()) != 1 || m.composer.Value() != "pending body" || m.deliveries[m.key()].State != core.DeliveryPending {
		t.Fatal("pending send blocked search input or changed pending body")
	}
}

func TestIncludePickerTabStillToggles(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerInclude)
	m.Update(tea.PasteMsg{Content: searchPrivateQuery})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.included["Cquiet"] || m.picker.Value() != searchPrivateQuery {
		t.Fatal("include picker no longer toggles")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.included["Cquiet"] {
		t.Fatal("include picker did not remove")
	}
}

// searchService records only explicit metadata page reads; other calls use the offline demo.
type searchService struct {
	core.Service
	cursors []string
	page    core.DirectoryPage
	err     error
}

func (s *searchService) Directory(_ context.Context, cursor string) (core.DirectoryPage, error) {
	s.cursors = append(s.cursors, cursor)
	return s.page, s.err
}

func TestSearchDirectoryPaginationAndFailure(t *testing.T) {
	m := fixture(t)
	s := &searchService{Service: m.service, page: core.DirectoryPage{Next: searchPageCursor}}
	m.service = s
	m.directoryLoaded = false
	m.directoryNext = ""
	m.directory = nil
	m.openPicker(pickerSwitch)
	cmd := m.directoryCmd("")
	if !strings.Contains(m.directoryState(), "Loading") || m.directoryCmd("") != nil {
		t.Fatal("loading state or duplicate directory guard failed")
	}
	m.Update(cmd())
	if m.directoryNext != searchPageCursor || !m.directoryLoaded {
		t.Fatal("first page cursor not retained")
	}
	m.Update(tea.PasteMsg{Content: "@maya"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if len(s.cursors) != 1 {
		t.Fatal("search or completion fetched directory")
	}
	s.err = &slack.APIError{Code: "ratelimited"}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(cmd())
	if m.directoryNext != searchPageCursor || m.directoryErr == nil || m.directoryBusy || !strings.Contains(m.directoryState(), "Error:") {
		t.Fatal("failed next page lost cursor/error state")
	}
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Err: errors.New("unrelated refresh")})
	if !strings.Contains(m.status, "unrelated refresh") || !strings.Contains(m.directoryState(), "ratelimited") {
		t.Fatal("background update hid directory error")
	}
	// A failed first-page reload must not erase the next-page retry cursor either.
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m.Update(cmd())
	if m.directoryNext != searchPageCursor {
		t.Fatal("failed reload discarded prior coverage")
	}
	s.err = nil
	s.page = core.DirectoryPage{Conversations: []core.Conversation{{ID: "Dnew", Name: "Maya", Handle: "maya", Kind: core.ConversationDM}}}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(cmd())
	if len(m.choices()) != 1 || m.directoryNext != "" || m.directoryErr != nil || len(m.contexts) != len(m.included) || m.included["Dnew"] {
		t.Fatal("metadata pagination changed inclusion or failed to refresh suggestions")
	}
	if got := strings.Join(s.cursors, ","); got != ",next,,next" {
		t.Fatal("unexpected page requests", got)
	}
}

func TestDirectoryResultsKeepHighlightedIdentity(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerSwitch)
	m.Update(tea.PasteMsg{Content: "#"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	id := m.pickedID()
	m.Update(directoryResult{Page: core.DirectoryPage{Conversations: []core.Conversation{{ID: "Cfirst", Name: "aaa", Kind: core.ConversationChannel}}}})
	if m.pickedID() != id {
		t.Fatal("new directory metadata moved keyboard selection to another conversation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.active != id {
		t.Fatal("Enter opened a different result after directory update")
	}
}

func TestSearchEmptyAndInclusionLimit(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerSwitch)
	m.Update(tea.PasteMsg{Content: "#not-a-loaded-channel"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.pickIndex != 0 || m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter}) != nil {
		t.Fatal("empty search selected a conversation")
	}
	m.picker.SetValue("@leo")
	for len(m.included) < selectionLimit {
		m.included[strings.Repeat("c", len(m.included))] = true
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != pickerSwitch || m.included["Dleo"] || !strings.Contains(m.choiceAction(m.choices(), m.pickIndex), "Limit:") {
		t.Fatal("search bypassed inclusion limit or hid recovery")
	}
}

const (
	searchPrivateQuery = "#quiet"
	searchPageCursor   = "next"
)

func TestSearchModalResizeAndColorIsolation(t *testing.T) {
	m := fixture(t)
	m.openPicker(pickerSwitch)
	m.Update(tea.PasteMsg{Content: strings.Repeat("界", 70) + "END"})
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		mw, mh := m.pickerSize()
		if input := ansi.Strip(m.picker.View()); ansi.StringWidth(input) > mw-frameColumns || !strings.Contains(input, "END") {
			t.Fatal("resize clipped cursor/end of wide query", input)
		}
		canvas := lipgloss.NewCanvas(size[0], size[1]).Compose(lipgloss.NewLayer(m.View().Content))
		x, y := (size[0]-mw)/2, (size[1]-mh)/2
		for _, sample := range []struct {
			x, y int
			want color.Color
		}{
			{1, 0, backdrop.GetForeground()},
			{x, y, accent.GetForeground()},
			{x + 2, y, focusedTitle.GetForeground()},
		} {
			cell := canvas.CellAt(sample.x, sample.y)
			if cell == nil || cell.Style.Fg == nil || color.RGBAModel.Convert(cell.Style.Fg) != color.RGBAModel.Convert(sample.want) {
				t.Fatalf("background/modal color leaked at %d,%d: %v", sample.x, sample.y, cell)
			}
		}
	}
}

func TestSearchModalHasSolidOutline(t *testing.T) {
	m := fixture(t)
	for _, mode := range []pickerMode{pickerSwitch, pickerInclude} {
		m.openPicker(mode)
		w, h := m.pickerSize()
		lines := strings.Split(ansi.Strip(m.pickerView(w, h)), "\n")
		if !strings.Contains(lines[0], "─") || strings.ContainsAny(strings.Join(lines, "\n"), "╌╎") || !strings.HasPrefix(lines[1], "│") || !strings.HasSuffix(lines[1], "│") {
			t.Fatal("modal must have solid horizontal and vertical edges", lines)
		}
	}
}

func TestSearchModalDirectoryStatesFitViewport(t *testing.T) {
	for _, state := range []string{"loading", "empty", "error", "unloaded"} {
		for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
			m := fixture(t)
			m.openPicker(pickerSwitch)
			m.directory = nil
			m.directoryNext = ""
			m.directoryLoaded = state != "unloaded"
			m.directoryBusy = state == "loading"
			if state == "error" {
				m.directoryErr = errors.New("directory unavailable")
			}
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			view := ansi.Strip(m.View().Content)
			rows := strings.Split(view, "\n")
			if len(rows) != size[1] {
				t.Fatalf("%s: wrong viewport height", state)
			}
			for _, row := range rows {
				if ansi.StringWidth(row) != size[0] {
					t.Fatalf("%s: wrong viewport width: %q", state, row)
				}
			}
			if !strings.Contains(view, "[Search conversations]") || !strings.Contains(view, "No matches") {
				t.Fatalf("%s: modal lost focus title or empty suggestions", state)
			}
		}
	}
}
