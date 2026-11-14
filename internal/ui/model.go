package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
)

const cacheLimit = 300
const selectionLimit = 32

type timeline struct {
	After              string
	Messages           []core.Message
	Next               string
	More, Loaded, Busy bool
	Touched            uint64
}
type historyResult struct {
	QueryAfter string
	Key        string
	Generation uint64
	Page       core.Page
	Older      bool
	Err        error
}
type conversationResult struct {
	Conversation core.Conversation
	ID           string
	Generation   uint64
	Err          error
}
type directoryResult struct {
	Page core.DirectoryPage
	Err  error
}
type sendResult struct {
	Key     string
	Message core.Message
	Err     error
}
type tickMsg time.Time

type Model struct {
	persistErr                      string
	service                         core.Service
	identity                        core.Identity
	store                           *store.Store
	prefs                           *store.Workspace
	ctx                             context.Context
	directory                       []core.Conversation
	directoryNext                   string
	directoryBusy                   bool
	included                        map[string]bool
	contexts                        map[string]context.Context
	cancels                         map[string]context.CancelFunc
	generations                     map[string]uint64
	cache                           map[string]*timeline
	clock                           uint64
	drafts                          map[string]string
	deliveries                      map[string]*core.Delivery
	unread                          map[string]bool
	latest                          map[string]string
	active, thread, focus, mode     string
	picker                          textinput.Model
	pickIndex                       int
	composer                        textarea.Model
	width, height, index, scroll    int
	status                          string
	stopped                         bool
	pollIndex                       int
	pollCount                       int
	help, quitConfirm, retryConfirm bool
}

func New(ctx context.Context, s core.Service, id core.Identity, st *store.Store, isDemo bool) *Model {
	p := st.Workspace(id.Workspace)
	if isDemo && len(p.Included) == 0 {
		for _, cid := range []string{"Cstudio", "Cshipping", "Cdesign", "Dmaya", "Gcrew"} {
			p.Included[cid] = true
		}
	}
	t := textarea.New()
	t.Placeholder = "Write a message…"
	t.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "› "
		}
		return "  "
	})
	t.ShowLineNumbers = false
	t.CharLimit = 12000
	t.MaxHeight = 1000
	t.MaxWidth = 0
	t.SetHeight(3)
	t.SetWidth(60)
	t.SetVirtualCursor(true)
	search := textinput.New()
	search.Prompt = " / "
	search.Placeholder = "Search conversations"
	search.CharLimit = 160
	m := &Model{service: s, identity: id, store: st, prefs: p, ctx: ctx, included: p.Included, contexts: map[string]context.Context{}, cancels: map[string]context.CancelFunc{}, generations: map[string]uint64{}, cache: map[string]*timeline{}, drafts: map[string]string{}, deliveries: map[string]*core.Delivery{}, unread: map[string]bool{}, latest: map[string]string{}, composer: t, picker: search, width: 100, height: 30, focus: "channels", status: "Choose your conversations with p · local read state"}
	for cid, v := range m.included {
		if v {
			m.addContext(cid)
		}
	}
	if isDemo {
		m.active = "Cstudio"
		m.unread["Cshipping"] = true
		m.unread["Dmaya"] = true
		m.status = "Offline demo · p choose · / switch · i compose · ? help"
	}
	m.restoreDraft()
	return m
}
func (m *Model) addContext(cid string) {
	m.contexts[cid], m.cancels[cid] = context.WithCancel(m.ctx)
	m.generations[cid]++
}
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.directoryCmd(""), tick()}
	for cid, included := range m.included {
		if included {
			id, ctx, svc, gen := cid, m.contexts[cid], m.service, m.generations[cid]
			cmds = append(cmds, func() tea.Msg {
				c, e := svc.Conversation(ctx, id)
				return conversationResult{Conversation: c, ID: id, Generation: gen, Err: e}
			})
		}
	}
	if m.active != "" {
		cmds = append(cmds, m.fetch(m.key(), false))
	}
	return tea.Batch(cmds...)
}
func tick() tea.Cmd { return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func (m *Model) key() string {
	if m.thread != "" {
		return m.active + "/" + m.thread
	}
	return m.active
}
func split(key string) (string, string) {
	p := strings.SplitN(key, "/", 2)
	if len(p) == 2 {
		return p[0], p[1]
	}
	return key, ""
}
func (m *Model) line(key string) *timeline {
	m.clock++
	if t := m.cache[key]; t != nil {
		t.Touched = m.clock
		return t
	}
	if len(m.cache) >= 16 {
		old := ""
		oldTime := ^uint64(0)
		for k, t := range m.cache {
			if k != m.key() && t.Touched < oldTime {
				old = k
				oldTime = t.Touched
			}
		}
		if old != "" {
			delete(m.cache, old)
		}
	}
	t := &timeline{Touched: m.clock}
	m.cache[key] = t
	return t
}
func (m *Model) directoryCmd(cursor string) tea.Cmd {
	if m.directoryBusy {
		return nil
	}
	m.directoryBusy = true
	svc, ctx := m.service, m.ctx
	return func() tea.Msg { p, e := svc.Directory(ctx, cursor); return directoryResult{p, e} }
}
func (m *Model) fetch(key string, older bool) tea.Cmd {
	cid, thread := split(key)
	if !m.included[cid] || m.stopped {
		return nil
	}
	t := m.line(key)
	if t.Busy {
		return nil
	}
	if older && t.Loaded && !t.More {
		return nil
	}
	t.Busy = true
	q := core.Query{Conversation: cid, Thread: thread}
	if !older && thread != "" && t.Loaded && !t.More && len(t.Messages) > 0 {
		q.After = t.Messages[len(t.Messages)-1].TS
	}
	if older {
		q.Cursor = t.Next
		q.After = t.After
		if thread == "" && len(t.Messages) > 0 {
			q.Cursor = ""
			q.Before = t.Messages[0].TS
		}
	}
	ctx, svc, gen := m.contexts[cid], m.service, m.generations[cid]
	return func() tea.Msg {
		p, e := svc.History(ctx, q)
		return historyResult{Key: key, Generation: gen, Page: p, Older: older, Err: e, QueryAfter: q.After}
	}
}
func (m *Model) save() {
	m.persistErr = ""
	if e := m.store.Save(); e != nil {
		m.persistErr = e.Error()
		m.status = "Cannot save preferences: " + e.Error()
	}
}
func (m *Model) rememberDraft() {
	if m.active != "" {
		m.drafts[m.key()] = m.composer.Value()
	}
}
func (m *Model) restoreDraft()                 { m.composer.SetValue(m.drafts[m.key()]); m.composer.MoveToEnd() }
func (m *Model) selected() []core.Conversation { return core.Filter(m.directory, "", m.included, true) }
func (m *Model) choices() []core.Conversation {
	return core.Filter(m.directory, m.picker.Value(), m.included, m.mode == "switch")
}
func (m *Model) selectConversation(cid string) tea.Cmd {
	m.rememberDraft()
	m.active = cid
	m.thread = ""
	m.index = 0
	m.scroll = 0
	m.focus = "messages"
	m.composer.Blur()
	m.restoreDraft()
	m.read(cid)
	return m.fetch(cid, false)
}
func (m *Model) read(cid string) {
	delete(m.unread, cid)
	if ts := m.latest[cid]; ts > m.prefs.Read[cid] {
		m.prefs.Read[cid] = ts
		m.save()
	}
}
func (m *Model) toggle(cid string) {
	if m.included[cid] {
		delete(m.included, cid)
		if f := m.cancels[cid]; f != nil {
			f()
		}
		delete(m.contexts, cid)
		delete(m.cancels, cid)
		delete(m.unread, cid)
		for k := range m.cache {
			c, _ := split(k)
			if c == cid {
				delete(m.cache, k)
			}
		}
		m.generations[cid]++
		if m.active == cid {
			m.rememberDraft()
			m.active = ""
			m.thread = ""
			m.composer.Reset()
		}
		m.status = "Removed locally; Slack membership unchanged. Draft retained for this session."
	} else {
		if len(m.included) >= selectionLimit {
			m.status = "Limit: 32 included conversations. Remove one first."
			return
		}
		m.included[cid] = true
		m.addContext(cid)
		m.status = "Included. Only chosen conversations are refreshed."
	}
	m.save()
}
func (m *Model) openPicker(mode string) tea.Cmd {
	m.rememberDraft()
	m.mode = mode
	m.pickIndex = 0
	m.picker.SetValue("")
	m.composer.Blur()
	return m.picker.Focus()
}
func (m *Model) send(retry bool) tea.Cmd {
	if m.active == "" {
		return nil
	}
	key := m.key()
	d := m.deliveries[key]
	if retry {
		if d == nil || d.State != "failed" {
			return nil
		}
		if d.Uncertain && !m.retryConfirm {
			m.retryConfirm = true
			m.status = "Delivery unknown. Check Slack before retrying. Press r again to risk a duplicate."
			return nil
		}
	}
	if !retry {
		if d != nil && (d.State == "pending" || d.State == "failed") {
			m.status = "Resolve the current delivery first: Esc then r retries; x restores it for editing."
			return nil
		}
		body := m.composer.Value()
		if strings.TrimSpace(body) == "" {
			return nil
		}
		id := make([]byte, 16)
		if _, e := rand.Read(id); e != nil {
			m.status = "Cannot create send ID; draft kept"
			return nil
		}
		id[6] = (id[6] & 15) | 64
		id[8] = (id[8] & 63) | 128
		d = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Thread: m.thread, Text: body, ID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])}}
		m.deliveries[key] = d
	}
	if !d.Begin() {
		return nil
	}
	m.retryConfirm = false
	m.status = "Sending… draft retained until acknowledged"
	svc, ctx, o := m.service, m.ctx, d.Outgoing
	return func() tea.Msg { msg, e := svc.Send(ctx, o); return sendResult{key, msg, e} }
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		m.resize()
		return m, nil
	case conversationResult:
		if !m.included[v.ID] || m.generations[v.ID] != v.Generation {
			return m, nil
		}
		if v.Err != nil {
			m.status = "Saved conversation " + v.ID + " unavailable: " + v.Err.Error()
			if slack.IsAuth(v.Err) {
				m.stopped = true
			}
			return m, nil
		}
		for i, c := range m.directory {
			if c.ID == v.ID {
				m.directory[i] = v.Conversation
				return m, nil
			}
		}
		m.directory = append(m.directory, v.Conversation)
		if m.active == "" {
			return m, m.selectConversation(v.ID)
		}
		return m, nil
	case directoryResult:
		m.directoryBusy = false
		if v.Err != nil {
			m.status = v.Err.Error()
			if slack.IsAuth(v.Err) {
				m.stopped = true
			}
			return m, nil
		}
		m.directoryNext = v.Page.Next
		for _, c := range v.Page.Conversations {
			found := false
			for i, old := range m.directory {
				if old.ID == c.ID {
					m.directory[i] = c
					found = true
					break
				}
			}
			if !found {
				m.directory = append(m.directory, c)
			}
		}
		sort.SliceStable(m.directory, func(i, j int) bool {
			return m.directory[i].Kind+strings.ToLower(m.directory[i].Name) < m.directory[j].Kind+strings.ToLower(m.directory[j].Name)
		})
		if m.active == "" {
			cs := m.selected()
			if len(cs) > 0 {
				return m, m.selectConversation(cs[0].ID)
			}
		}
		return m, nil
	case historyResult:
		cid, thread := split(v.Key)
		if !m.included[cid] || m.generations[cid] != v.Generation {
			return m, nil
		}
		t := m.cache[v.Key]
		if t == nil {
			return m, nil
		}
		t.Busy = false
		if v.Err != nil {
			if v.Err != context.Canceled {
				m.status = v.Err.Error() + " · R refresh"
			}
			if slack.IsAuth(v.Err) {
				m.stopped = true
				m.status = "Credentials expired/revoked. Drafts kept; reinstall or replace user token and restart."
			}
			return m, nil
		}
		previous := t.Loaded
		oldLen := len(t.Messages)
		// Refresh replaces the observed range so edits/deletions within it are reflected.
		if !v.Older && thread == "" && !v.Page.More {
			t.Messages = nil
		}
		if !v.Older && thread == "" && len(v.Page.Messages) > 0 {
			minTS := v.Page.Messages[0].TS
			for _, x := range v.Page.Messages {
				if x.TS < minTS {
					minTS = x.TS
				}
			}
			keep := []core.Message{}
			for _, x := range t.Messages {
				if x.TS < minTS {
					keep = append(keep, x)
				}
			}
			t.Messages = keep
		}
		if v.Older && thread == "" {
			t.Messages = core.Merge(t.Messages, v.Page.Messages, len(t.Messages)+len(v.Page.Messages))
			if len(t.Messages) > cacheLimit {
				t.Messages = t.Messages[:cacheLimit]
			}
		} else {
			t.Messages = core.Merge(t.Messages, v.Page.Messages, cacheLimit)
		}
		if !previous || v.Older || thread == "" {
			t.Next = v.Page.Next
			t.More = v.Page.More
			t.After = v.QueryAfter
		} else if thread != "" && !t.More && v.Page.More {
			t.Next = v.Page.Next
			t.More = true
			t.After = v.QueryAfter
		}
		t.Loaded = true
		if len(t.Messages) > 0 && thread == "" {
			ts := t.Messages[len(t.Messages)-1].TS
			before := m.latest[cid]
			if ts > before {
				m.latest[cid] = ts
			}
			if m.prefs.Read[cid] == "" && before == "" {
				m.prefs.Read[cid] = ts
				m.save()
			} else if ts > m.prefs.Read[cid] {
				m.unread[cid] = true
			}
			if cid == m.active && m.thread == "" && m.scroll == 0 && m.focus != "compose" && m.mode == "" {
				m.read(cid)
			}
		}
		if v.Key == m.key() {
			if v.Older {
				if thread == "" {
					m.index = 0
				} else {
					m.index = max(0, oldLen-1)
				}
				m.scroll = -1
			} else if !previous || m.scroll == 0 {
				m.index = max(0, len(t.Messages)-1)
			}
		}
		if d := m.deliveries[m.key()]; !m.stopped && (d == nil || d.State == "sent") {
			m.status = "Refreshed · local unread only · polling included conversations"
		}
		return m, nil
	case sendResult:
		d := m.deliveries[v.Key]
		if d == nil {
			return m, nil
		}
		uncertain := slack.Uncertain(v.Err) // fake failures are definite
		if strings.HasPrefix(fmt.Sprint(v.Err), "demo:") {
			uncertain = false
		}
		d.Finish(v.Err, uncertain)
		if v.Err != nil {
			m.status = "Send failed: " + v.Err.Error() + " · text kept · Esc then r retry"
			if uncertain {
				m.status = "Delivery unknown; text kept. Check Slack before r retry."
			}
			if slack.IsAuth(v.Err) {
				m.stopped = true
			}
			return m, nil
		}
		cid, _ := split(v.Key)
		if !m.included[cid] {
			m.status = "Message sent to removed conversation"
			delete(m.drafts, v.Key)
			return m, nil
		}
		t := m.line(v.Key)
		v.Message.Author = "You"
		t.Messages = core.Merge(t.Messages, []core.Message{v.Message}, cacheLimit)
		if m.drafts[v.Key] == d.Text {
			delete(m.drafts, v.Key)
		}
		if m.key() == v.Key && m.composer.Value() == d.Text {
			m.composer.Reset()
			delete(m.drafts, v.Key)
		}
		m.status = "Message sent"
		return m, nil
	case tickMsg:
		cmds := []tea.Cmd{tick()}
		cs := m.selected()
		if !m.stopped && len(cs) > 0 {
			m.pollCount++
			key := cs[m.pollIndex%len(cs)].ID
			m.pollIndex++
			if m.thread != "" && m.pollCount%2 == 0 {
				key = m.key()
			}
			if key != m.key() || m.scroll == 0 {
				cmds = append(cmds, m.fetch(key, false))
			}
		}
		return m, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		k := v.String()
		if k != "r" {
			m.retryConfirm = false
		}
		if v.Key().IsRepeat && (k == "r" || k == "ctrl+s" || k == "ctrl+c") {
			return m, nil
		}
		if k == "ctrl+c" {
			m.rememberDraft()
			dirty := false
			for _, s := range m.drafts {
				if s != "" {
					dirty = true
				}
			}
			for _, d := range m.deliveries {
				if d.State == "pending" || d.State == "failed" {
					dirty = true
				}
			}
			if dirty && !m.quitConfirm {
				m.quitConfirm = true
				m.status = "Unsent text is in memory. Ctrl+C again exits and discards it; any other key cancels."
				return m, nil
			}
			return m, tea.Quit
		}
		m.quitConfirm = false
		if m.help {
			m.help = false
			return m, nil
		}
		if m.mode != "" {
			return m, m.pickerKey(v)
		}
		if k == "ctrl+k" {
			return m, m.openPicker("switch")
		}
		if m.focus == "compose" {
			if k == "esc" {
				m.rememberDraft()
				m.focus = "messages"
				m.composer.Blur()
				return m, nil
			}
			if k == "ctrl+s" {
				m.rememberDraft()
				return m, m.send(false)
			}
			if d := m.deliveries[m.key()]; d != nil && d.State == "pending" {
				m.status = "Sending… composer locked until acknowledgement; text kept"
				return m, nil
			}
			var cmd tea.Cmd
			m.composer, cmd = m.composer.Update(msg)
			m.rememberDraft()
			return m, cmd
		}
		switch k {
		case "q":
			return m, m.quitKey()
		case "?":
			m.help = true
		case "p":
			return m, m.openPicker("pick")
		case "/":
			return m, m.openPicker("switch")
		case "tab":
			if m.focus == "channels" {
				m.focus = "messages"
			} else {
				m.focus = "channels"
			}
		case "i", "enter":
			if m.active != "" {
				m.focus = "compose"
				return m, m.composer.Focus()
			}
		case "esc":
			if m.thread != "" {
				m.rememberDraft()
				m.thread = ""
				m.scroll = 0
				m.restoreDraft()
				return m, m.fetch(m.key(), false)
			}
			m.focus = "channels"
		case "j", "down", "k", "up":
			delta := 1
			if k == "k" || k == "up" {
				delta = -1
			}
			if m.focus == "channels" {
				cs := m.selected()
				if len(cs) > 0 {
					idx := 0
					for i, c := range cs {
						if c.ID == m.active {
							idx = i
						}
					}
					idx = (idx + delta + len(cs)) % len(cs)
					cmd := m.selectConversation(cs[idx].ID)
					m.focus = "channels"
					return m, cmd
				}
			} else {
				t := m.line(m.key())
				m.index = max(0, min(len(t.Messages)-1, m.index+delta))
				m.scroll = -1
			}
		case "pgup":
			m.scroll = max(0, m.scroll) + max(3, m.height/3)
		case "pgdown":
			m.scroll = max(0, m.scroll-max(3, m.height/3))
			if m.scroll == 0 {
				m.read(m.active)
			}
		case "G", "end":
			m.scroll = 0
			t := m.line(m.key())
			m.index = max(0, len(t.Messages)-1)
			m.read(m.active)
		case "o":
			return m, m.fetch(m.key(), true)
		case "R":
			return m, m.fetch(m.key(), false)
		case "m":
			m.read(m.active)
			m.status = "Marked read locally; Slack read position unchanged"
		case "t":
			if m.active != "" && m.thread == "" {
				t := m.line(m.key())
				if len(t.Messages) > 0 {
					m.rememberDraft()
					m.thread = t.Messages[min(m.index, len(t.Messages)-1)].TS
					m.scroll = 0
					m.index = 0
					m.restoreDraft()
					return m, m.fetch(m.key(), false)
				}
			}
		case "y":
			t := m.line(m.key())
			if len(t.Messages) > 0 {
				m.status = "Copied message via OSC52 (terminal support required)"
				return m, tea.SetClipboard(slack.Body(t.Messages[min(m.index, len(t.Messages)-1)]))
			}
		case "r":
			return m, m.send(true)
		case "x":
			if d := m.deliveries[m.key()]; d != nil && d.State == "failed" {
				s := m.composer.Value()
				if s != "" && s != d.Text {
					s += "\n" + d.Text
				} else {
					s = d.Text
				}
				if len([]rune(s)) > m.composer.CharLimit || strings.Count(s, "\n") >= m.composer.MaxHeight {
					m.status = "Cannot combine drafts without truncation. Copy your current draft first; failed body retained."
					return m, nil
				}
				m.composer.SetValue(s)
				m.rememberDraft()
				delete(m.deliveries, m.key())
				m.status = "Restored failed text for editing; uncertain sends may already exist in Slack"
			}
		}
		return m, nil
	}
	// Textarea cursor blink and paste messages continue through Bubble Tea commands.
	if d := m.deliveries[m.key()]; d != nil && d.State == "pending" && m.focus == "compose" {
		return m, nil
	}
	if m.mode != "" {
		var c tea.Cmd
		m.picker, c = m.picker.Update(msg)
		return m, c
	}
	if m.focus == "compose" {
		var c tea.Cmd
		m.composer, c = m.composer.Update(msg)
		m.rememberDraft()
		return m, c
	}
	return m, nil
}
func (m *Model) quitKey() tea.Cmd {
	m.rememberDraft()
	for _, s := range m.drafts {
		if s != "" {
			m.status = "Unsent drafts in memory. Use Ctrl+C twice to exit."
			return nil
		}
	}
	for _, d := range m.deliveries {
		if d.State == "pending" || d.State == "failed" {
			m.status = "Unresolved send. Use Ctrl+C twice to exit."
			return nil
		}
	}
	return tea.Quit
}
func (m *Model) pickerKey(v tea.KeyPressMsg) tea.Cmd {
	cs := m.choices()
	switch v.String() {
	case "esc":
		m.mode = ""
		m.picker.Blur()
		return nil
	case "down", "ctrl+n":
		m.pickIndex = min(len(cs)-1, m.pickIndex+1)
		return nil
	case "up", "ctrl+p":
		m.pickIndex = max(0, m.pickIndex-1)
		return nil
	case "ctrl+l":
		if m.directoryNext != "" {
			return m.directoryCmd(m.directoryNext)
		}
		return nil
	case "ctrl+r":
		m.directoryNext = ""
		return m.directoryCmd("")
	case "tab":
		if m.mode == "pick" && len(cs) > 0 {
			m.toggle(cs[max(0, min(m.pickIndex, len(cs)-1))].ID)
		}
		return nil
	case "enter":
		if len(cs) > 0 {
			c := cs[max(0, min(m.pickIndex, len(cs)-1))]
			if !m.included[c.ID] {
				m.toggle(c.ID)
			}
			if m.included[c.ID] {
				m.mode = ""
				m.picker.Blur()
				return m.selectConversation(c.ID)
			}
		}
		return nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(v)
	m.pickIndex = 0
	return cmd
}

func LoadSnapshot(ctx context.Context, m *Model, s core.Service) error {
	cursor := ""
	for {
		p, e := s.Directory(ctx, cursor)
		if e != nil {
			return e
		}
		m.Update(directoryResult{Page: p})
		cursor = p.Next
		if cursor == "" {
			break
		}
	}
	p, e := s.History(ctx, core.Query{Conversation: m.active})
	if e != nil {
		return e
	}
	m.line(m.active)
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: p})
	return nil
}
