package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/store"
)

func fixture(t *testing.T) *Model {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := demo.New()
	id, _ := s.Identity(ctx)
	m := New(ctx, s, id, &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}, true)
	if e := LoadSnapshot(ctx, m, s); e != nil {
		t.Fatal(e)
	}
	return m
}
func key(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	if s == "ctrl+s" {
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}
func TestLayoutDimensionsAndBorderJunctions(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {100, 30}, {80, 24}, {60, 20}, {40, 14}} {
		for _, mode := range []string{"messages", "compose", "picker", "thread", "help"} {
			t.Run(fmt.Sprint(size)+mode, func(t *testing.T) {
				m := fixture(t)
				s := Snapshot(m, size[0], size[1], mode)
				rows := strings.Split(s, "\n")
				if len(rows) != size[1] {
					t.Fatalf("rows=%d want=%d", len(rows), size[1])
				}
				for i, r := range rows {
					if n := ansi.StringWidth(r); n != size[0] {
						t.Fatalf("row %d has width %d want %d: %q", i, n, size[0], r)
					}
				}
				if strings.Contains(s, "││") || strings.Contains(s, "cell-border;") {
					t.Fatal("doubled border or experimental protocol")
				}
				focusedHeader := "[" + m.title() + "]"
				if mode == "compose" {
					focusedHeader = "[Message]"
				}
				if mode == "picker" {
					focusedHeader = "[Choose conversations]"
				}
				if mode != "help" && !strings.Contains(s, focusedHeader) {
					t.Fatal("missing non-color focus cue")
				}
			})
		}
	}
}
func TestDraftSurvivesSwitchThreadAndFailure(t *testing.T) {
	m := fixture(t)
	m.composer.SetValue("main draft")
	m.selectConversation("Dmaya")
	m.composer.SetValue("DM draft")
	m.selectConversation("Cstudio")
	if m.composer.Value() != "main draft" {
		t.Fatal("draft lost")
	}
	m.index = 0
	m.Update(key("t"))
	m.composer.SetValue("thread draft")
	m.Update(key("esc"))
	if m.composer.Value() != "main draft" {
		t.Fatalf("thread destroyed parent draft: active=%q thread=%q focus=%q value=%q drafts=%v", m.active, m.thread, m.focus, m.composer.Value(), m.drafts)
	}
	m.composer.SetValue("/fail hello")
	cmd := m.send(false)
	if cmd == nil {
		t.Fatal("no send")
	}
	id := m.deliveries[m.key()].ID
	if m.send(false) != nil {
		t.Fatal("duplicate queued")
	}
	m.Update(cmd())
	if m.composer.Value() != "/fail hello" || m.deliveries[m.key()].State != "failed" {
		t.Fatal("failed draft discarded")
	}
	cmd = m.send(true)
	if cmd == nil {
		t.Fatal("retry unavailable")
	}
	m.Update(cmd())
	if m.composer.Value() != "" || m.deliveries[m.key()].ID != id {
		t.Fatal("retry changed identity or did not clear acknowledged draft")
	}
}
func TestPendingPasteLockedAndSuccessDoesNotClearAnotherDraft(t *testing.T) {
	m := fixture(t)
	m.focus = "compose"
	m.composer.Focus()
	m.composer.SetValue("original")
	m.rememberDraft()
	cmd := m.send(false)
	m.Update(tea.PasteMsg{Content: "PASTED"})
	if m.composer.Value() != "original" {
		t.Fatal("pending composer changed")
	}
	m.selectConversation("Dmaya")
	m.composer.SetValue("other draft")
	m.Update(cmd())
	if m.composer.Value() != "other draft" || m.drafts["Cstudio"] != "" {
		t.Fatal("success erased wrong draft")
	}
}
func TestRemovalCancelsAndRejectsLateResponses(t *testing.T) {
	m := fixture(t)
	cid := "Cstudio"
	ctx := m.contexts[cid]
	generation := m.generations[cid]
	m.composer.SetValue("keep after removal")
	m.toggle(cid)
	if ctx.Err() != context.Canceled {
		t.Fatal("removal did not cancel fetch context")
	}
	if m.fetch(cid, false) != nil {
		t.Fatal("removed conversation still fetched")
	}
	m.Update(historyResult{Key: cid, Generation: generation, Page: core.Page{Messages: []core.Message{{TS: "9999999999.1", Text: "late"}}}})
	if m.cache[cid] != nil || m.unread[cid] {
		t.Fatal("removed results applied")
	}
	m.toggle(cid)
	m.Update(historyResult{Key: cid, Generation: generation, Page: core.Page{Messages: []core.Message{{TS: "9999999999.1"}}}})
	if m.cache[cid] != nil {
		t.Fatal("old generation resurrected after re-inclusion")
	}
	m.selectConversation(cid)
	if m.composer.Value() != "keep after removal" {
		t.Fatal("removal discarded unsent text")
	}
}
func TestUnknownSendRequiresExplicitRetryAndKeepsOriginal(t *testing.T) {
	m := fixture(t)
	m.composer.SetValue("uncertain text")
	m.send(false)
	m.Update(sendResult{Key: m.key(), Err: errors.New("connection lost")})
	if m.send(true) != nil || !m.retryConfirm {
		t.Fatal("uncertain send retried without notice")
	}
	cmd := m.send(true)
	if cmd == nil || m.deliveries[m.key()].Text != "uncertain text" {
		t.Fatal("explicit retry lost original")
	}
}
func TestBoundedCacheAndOlderWindow(t *testing.T) {
	m := fixture(t)
	for i := 0; i < 100; i++ {
		m.line(fmt.Sprint(i))
	}
	if len(m.cache) > 16 {
		t.Fatal("unbounded timelines", len(m.cache))
	}
	tline := m.line(m.active)
	tline.Loaded = true
	tline.More = true
	for i := 1000; i < 1300; i++ {
		tline.Messages = append(tline.Messages, core.Message{TS: fmt.Sprintf("%010d.000001", i)})
	}
	fresh := []core.Message{}
	for i := 950; i < 1000; i++ {
		fresh = append(fresh, core.Message{TS: fmt.Sprintf("%010d.000001", i)})
	}
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Older: true, Page: core.Page{Messages: fresh, More: true, Next: "older"}})
	if len(tline.Messages) != 300 || tline.Messages[0].TS != "0000000950.000001" {
		t.Fatal("older window not retained", len(tline.Messages), tline.Messages[0].TS)
	}
	if tline.Next != "older" {
		t.Fatal("older cursor lost")
	}
}
func TestOnlyIncludedPollingAndLocalRead(t *testing.T) {
	m := fixture(t)
	delete(m.unread, m.active)
	m.Update(tickMsg(time.Now()))
	for k := range m.cache {
		cid, _ := split(k)
		if !m.included[cid] {
			t.Fatal("fetched excluded conversation")
		}
	}
	m.fetch("Dmaya", false)
	m.prefs.Read["Dmaya"] = "1.0"
	m.Update(historyResult{Key: "Dmaya", Generation: m.generations["Dmaya"], Page: core.Page{Messages: []core.Message{{TS: "9999999999.1", Text: "new"}}}})
	if !m.unread["Dmaya"] {
		t.Fatal("missing local activity")
	}
	m.selectConversation("Dmaya")
	if m.unread["Dmaya"] || m.prefs.Read["Dmaya"] != "9999999999.1" {
		t.Fatalf("local read not advanced: unread=%v read=%q latest=%q", m.unread, m.prefs.Read["Dmaya"], m.latest["Dmaya"])
	}
}

func TestOlderPageBecomesVisibleAndRefreshDoesNotStrandPagination(t *testing.T) {
	m := fixture(t)
	m.scroll = 0
	cmd := m.fetch(m.active, true)
	if cmd == nil {
		t.Fatal("older unavailable")
	}
	m.Update(cmd())
	if m.scroll != -1 || m.index != 0 {
		t.Fatal("older page not brought into view")
	}
	oldest := m.cache[m.active].Messages[0].TS
	cmd = m.fetch(m.active, true)
	m.Update(cmd())
	if m.cache[m.active].Messages[0].TS >= oldest {
		t.Fatal("older history did not advance")
	}
	m.scroll = 0
	cmd = m.fetch(m.active, false)
	m.Update(cmd())
	if !m.cache[m.active].More {
		t.Fatal("refresh stranded history paging")
	}
}
func TestSavedConversationLookupIndependentOfDirectory(t *testing.T) {
	m := fixture(t)
	m.directory = nil
	m.active = ""
	c, _ := m.service.Conversation(m.ctx, "Dmaya")
	_, cmd := m.Update(conversationResult{Conversation: c, ID: c.ID, Generation: m.generations[c.ID]})
	if m.active != "Dmaya" || cmd == nil {
		t.Fatal("saved selection needs directory scan")
	}
	m.toggle("Dmaya")
	m.Update(conversationResult{Conversation: c, ID: c.ID, Generation: m.generations[c.ID] - 1})
	if m.included[c.ID] {
		t.Fatal("late metadata restored excluded conversation")
	}
}
func TestCombiningFailedTextNeverTruncatesNewDraft(t *testing.T) {
	m := fixture(t)
	m.deliveries[m.key()] = &core.Delivery{Outgoing: core.Outgoing{Text: strings.Repeat("a", 7000)}, State: "failed"}
	body := strings.Repeat("b", 7000)
	m.composer.SetValue(body)
	m.Update(key("x"))
	if m.composer.Value() != body || m.deliveries[m.key()] == nil {
		t.Fatal("restoring failure truncated text")
	}
}

func TestReadWatermarkNeverMovesBackward(t *testing.T) {
	m := fixture(t)
	m.prefs.Read[m.active] = "9999999999.1"
	m.latest[m.active] = "1.0"
	m.read(m.active)
	if m.prefs.Read[m.active] != "9999999999.1" {
		t.Fatal("older cached window moved read watermark backward")
	}
}

func TestHistoryViewportStartsWithAuthor(t *testing.T) {
	m := fixture(t)
	for _, w := range []int{36, 56, 74, 100} {
		for _, h := range []int{3, 8, 17, 23} {
			for _, scroll := range []int{0, 2, 5, -1} {
				m.scroll = scroll
				rows := m.history(w, h)
				first := ansi.Strip(rows[0])
				if !strings.Contains(first, "Nov 13") {
					t.Fatalf("viewport %dx%d scroll %d starts without author: %q", w, h, scroll, first)
				}
			}
		}
	}
}
func TestOversizedMessageKeepsAuthorWhileScrolling(t *testing.T) {
	m := fixture(t)
	m.cache[m.active].Messages = []core.Message{{TS: "1794589200.000001", Author: "Maya Chen", Text: strings.Repeat("A longer message line.\n", 25)}}
	m.index = 0
	m.scroll = 5
	rows := m.history(50, 8)
	if !strings.Contains(ansi.Strip(rows[0]), "Maya Chen") || !strings.Contains(ansi.Strip(rows[0]), "continued") {
		t.Fatal("missing continuation context", rows[0])
	}
	if !strings.Contains(strings.Join(rows[1:], "\n"), "A longer message line.") {
		t.Fatal("long-message body became inaccessible")
	}
}
func TestConversationContextUsesKnownMetadata(t *testing.T) {
	m := fixture(t)
	m.selectConversation("Gcrew")
	if got := m.conversationContext(); !strings.Contains(got, "Group DM · 3 people") {
		t.Fatal(got)
	}
	m.selectConversation("Cstudio")
	if !strings.Contains(m.conversationContext(), "Daily collaboration") {
		t.Fatal("missing topic")
	}
	m.selectConversation("Dmaya")
	if got := m.conversationContext(); got != "Direct message" {
		t.Fatal(got)
	}
	// Rich metadata can arrive after the directory entry.
	m.Update(conversationResult{Conversation: core.Conversation{ID: "Dmaya", Name: "Maya Chen", Kind: "dm", Topic: "Updated context"}, ID: "Dmaya", Generation: m.generations["Dmaya"]})
	if !strings.Contains(m.conversationContext(), "Updated context") {
		t.Fatal("late metadata discarded")
	}
}

func TestUnsentExitWarningSurvivesBackgroundRefresh(t *testing.T) {
	m := fixture(t)
	m.composer.SetValue("Unsent draft")
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: core.Page{Messages: m.cache[m.active].Messages}})
	if !strings.Contains(ansi.Strip(m.View().Content), "Unsent text is in memory") {
		t.Fatal("background refresh hid discard warning")
	}
}
