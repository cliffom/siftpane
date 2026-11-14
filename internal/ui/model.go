// Package ui owns Bubble Tea application state and terminal rendering.
package ui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

const (
	cacheLimit     = 300
	selectionLimit = 32
)

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

// Model owns application state; only the Bubble Tea event loop may mutate it.
type Model struct {
	persistErr string
	service    core.Service
	identity   core.Identity
	store      *store.Store
	prefs      *store.Workspace
	ctx        context.Context

	directory     []core.Conversation
	directoryNext string
	directoryBusy bool
	included      map[string]bool
	contexts      map[string]context.Context
	cancels       map[string]context.CancelFunc
	generations   map[string]uint64
	cache         map[string]*timeline
	clock         uint64

	drafts     map[string]string
	deliveries map[string]*core.Delivery
	unread     map[string]bool
	latest     map[string]string

	active, thread, focus, mode string

	picker    textinput.Model
	pickIndex int

	composer                     textarea.Model
	width, height, index, scroll int

	status    string
	stopped   bool
	pollIndex int
	pollCount int

	help, quitConfirm, retryConfirm bool
}

// New creates a UI using an authenticated service and workspace preferences.
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
	m := &Model{
		service:     s,
		identity:    id,
		store:       st,
		prefs:       p,
		ctx:         ctx,
		included:    p.Included,
		contexts:    map[string]context.Context{},
		cancels:     map[string]context.CancelFunc{},
		generations: map[string]uint64{},
		cache:       map[string]*timeline{},
		drafts:      map[string]string{},
		deliveries:  map[string]*core.Delivery{},
		unread:      map[string]bool{},
		latest:      map[string]string{},
		composer:    t,
		picker:      search,
		width:       100,
		height:      30,
		focus:       "channels",
		status:      "Choose your conversations with p · local read state",
	}
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

// Init starts metadata discovery and the bounded refresh scheduler.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.directoryCmd(""), tick()}
	for cid, included := range m.included {
		if included {
			id, ctx, svc, gen := cid, m.contexts[cid], m.service, m.generations[cid]
			cmds = append(cmds, func() tea.Msg {
				c, err := svc.Conversation(ctx, id)
				return conversationResult{Conversation: c, ID: id, Generation: gen, Err: err}
			})
		}
	}
	if m.active != "" {
		cmds = append(cmds, m.fetch(m.key(), false))
	}
	return tea.Batch(cmds...)
}

// Update serializes input and asynchronous service results into model changes.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		m.resize()
		return m, nil
	case conversationResult:
		return m.updateConversation(v)
	case directoryResult:
		return m.updateDirectory(v)
	case historyResult:
		return m.updateHistory(v)
	case sendResult:
		return m.updateSend(v)
	case tickMsg:
		return m.updateTick(v)
	case tea.KeyPressMsg:
		return m.updateKey(v)
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
