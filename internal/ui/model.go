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

type timeline struct {
	FillPages          int
	FillBefore         string
	replyCursors       []string
	ReplyThrough       string // Highest timestamp accepted from fetched reply pages, never sends.
	After              string
	Messages           []core.Message
	Next               string
	CachedRefreshErr   error
	Cached             bool
	Saved              time.Time
	More, Loaded, Busy bool
	Err                error
	RecentErr          error
	PageErr            error
	Checked            time.Time
	Touched            uint64
}
type historyResult struct {
	CacheEpoch  uint64
	Target      string
	QueryCursor string
	QueryAfter  string
	Key         string
	Generation  uint64
	Page        core.Page
	Older       bool
	Fill        bool
	Err         error
}
type conversationResult struct {
	Conversation core.Conversation
	ID           string
	Generation   uint64
	Err          error
}
type directoryResult struct {
	Page       core.DirectoryPage
	Cursor     string
	Generation uint64
	Err        error
}
type sendResult struct {
	ServiceGeneration uint64
	Key               string
	Message           core.Message
	Err               error
}
type tickMsg time.Time

// Model owns application state; only the Bubble Tea event loop may mutate it.
type Model struct {
	serviceGeneration   uint64
	refreshDetails      bool
	detailsScroll       int
	participationBusy   bool
	broadcastConfirm    core.AttentionReason
	discoverySources    []core.AttentionReason
	followBusy          bool
	followKey           string
	followGeneration    uint64
	followCancel        context.CancelFunc
	followCtx           context.Context
	followNext          map[string]time.Time
	followErrors        map[string]string
	followPage          int
	followIndex         int
	messageCacheErr     string
	cacheConfirm        bool
	cacheEpoch          uint64
	persisted           map[string]store.MessageHistory
	attention           core.AttentionState
	attentionErr        string
	opened              map[string]bool
	discoveryMetadata   map[string]bool
	openTargets         map[string]string
	mentionConfirm      bool
	discoveryPhase      discoveryPhase
	discoveryBusy       bool
	discoveryTotal      int
	discoveryPage       int
	discoveryErr        string
	gapConfirmation     *core.AttentionGap
	discoveryNext       time.Time
	discoveryGeneration uint64
	discoveryCtx        context.Context
	discoveryCancel     context.CancelFunc

	profileCacheErr         string
	profiles                map[string]core.Profile
	profileAttempted        map[string]bool
	profileBusy             bool
	profileCancel           context.CancelFunc
	profileLookupGeneration uint64
	profileKey              string

	persistErr       string
	metadataErr      string
	metadataBusy     map[string]bool
	metadataFailed   map[string]bool
	metadataState    map[string]metadataValidation
	startupSidebar   []core.Conversation
	startupRestoring bool
	service          core.Service
	identity         core.Identity
	store            *store.Store
	prefs            *store.Workspace
	ctx              context.Context
	sendCtx          context.Context
	multipleAccounts bool

	directory                 []core.Conversation
	directoryNext             string
	directoryBusy             bool
	directoryLoaded           bool
	directoryErr              error
	directoryCancel           context.CancelFunc
	directoryGeneration       uint64
	directoryPagesLeft        int
	directorySeen             map[string]bool
	directoryCached           map[string]bool
	directoryCacheErr         error
	directBusy                bool
	directCancel              context.CancelFunc
	directErr                 error
	directoryProfileCancel    context.CancelFunc
	directoryProfileBusy      bool
	directoryProfileUser      string
	directoryProfileAttempted map[string]bool
	directoryProfilePending   map[string]bool
	included                  map[string]bool
	contexts                  map[string]context.Context
	cancels                   map[string]context.CancelFunc
	generations               map[string]uint64
	readGeneration            uint64
	cache                     map[string]*timeline
	clock                     uint64

	drafts     map[string]string
	deliveries map[string]*core.Delivery
	unread     map[string]bool
	latest     map[string]string

	active, thread string
	focus          focusPane
	mode           pickerMode

	picker    textinput.Model
	pickIndex int

	composer                     textarea.Model
	width, height, index, scroll int
	pinnedHistory                *historyViewportPosition

	status     string
	statusKind statusKind
	statusNow  time.Time
	stopped    bool
	pollIndex  int
	pollCount  int

	help, quitConfirm, retryConfirm bool
	helpScroll                      int
}

// New creates a UI using an authenticated service and workspace preferences.
func New(ctx context.Context, s core.Service, id core.Identity, st *store.Store, isDemo bool) *Model {
	p := st.AccountWorkspace(id)
	if isDemo && len(p.Included) == 0 {
		for _, cid := range []string{"Cstudio", demoShippingConversation, "Cdesign", "Dmaya", "Gcrew"} {
			p.Included[cid] = true
		}
	}
	t := textarea.New()
	t.SetStyles(composerStyles())
	t.Placeholder = "Write a message…"
	t.SetPromptFunc(composerPromptColumns, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "› "
		}
		return "  "
	})
	t.ShowLineNumbers = false
	t.CharLimit = composerRuneLimit
	t.MaxHeight = composerLineLimit
	t.MaxWidth = 0
	t.SetHeight(composerRows)
	t.SetWidth(initialComposerColumns)
	t.SetVirtualCursor(true)
	search := textinput.New()
	search.Prompt = " / "
	search.Placeholder = "Search conversations"
	search.CharLimit = pickerRuneLimit
	search.SetVirtualCursor(true)
	m := &Model{
		followNext:       map[string]time.Time{},
		followErrors:     map[string]string{},
		persisted:        map[string]store.MessageHistory{},
		profiles:         map[string]core.Profile{},
		profileAttempted: map[string]bool{},

		service:        s,
		metadataBusy:   map[string]bool{},
		metadataFailed: map[string]bool{},
		metadataState:  map[string]metadataValidation{},
		identity:       id,
		store:          st,
		prefs:          p,
		ctx:            ctx,
		sendCtx:        ctx,
		included:       p.Included,
		contexts:       map[string]context.Context{},
		cancels:        map[string]context.CancelFunc{},
		generations:    map[string]uint64{},
		cache:          map[string]*timeline{},
		drafts:         map[string]string{},
		deliveries:     map[string]*core.Delivery{},
		unread:         map[string]bool{},
		latest:         map[string]string{},
		composer:       t,
		picker:         search,
		width:          DefaultWidth,
		height:         DefaultHeight,
		focus:          focusChannels,
		status:         "Choose your conversations with " + keyLabel(keyInclude) + " · local read state",
		statusNow:      time.Now(),
	}
	m.initAttention()
	if m.attentionErr == "" {
		if err := m.store.SaveAttention(m.identity, m.attention); err != nil {
			m.attentionErr = err.Error()
			m.setStatus("Cannot save attention: "+err.Error(), statusAction)
		}
	}
	for cid := range m.attention.Pending {
		m.directory = append(m.directory, core.Conversation{ID: cid, Name: cid, Kind: core.ConversationChannel})
	}
	for cid, v := range m.included {
		if v {
			m.addContext(cid)
		}
	}
	if svc, ok := s.(profileService); ok {
		ps := st.Profiles(id)
		svc.RestoreProfiles(ps)
		for _, p := range ps {
			m.profiles[p.ID] = p
		}
	}
	m.restoreConversationMetadata()
	m.restoreMessages()
	if cs := m.selected(); len(cs) > 0 {
		for _, c := range cs {
			if m.attention.Kept[c.ID] {
				m.active = c.ID
				break
			}
		}
	}
	if isDemo {
		m.active = "Cstudio"
		m.unread[demoShippingConversation] = true
		m.unread["Dmaya"] = true
		m.setStatus("Offline demo", statusRoutine)
	}
	if line := m.cache[m.key()]; line != nil && line.Cached {
		m.index = max(0, len(line.Messages)-1)
	}
	m.restoreDraft()
	return m
}

// Init starts metadata discovery and the bounded refresh scheduler.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.directoryCmd(""), m.nextTick(), m.checkMentions(time.Now()), m.checkFollowed(time.Now())}
	for cid, included := range m.included {
		if included && (cid == m.active || m.metadataState[cid] != metadataDeferred) {
			cmds = append(cmds, m.conversationCmd(cid))
		}
	}
	m.startupSidebar = m.selected()
	m.startupRestoring = len(m.metadataBusy) > 0
	if m.active != "" {
		cmds = append(cmds, m.fetch(m.key(), false))
	}
	return tea.Batch(cmds...)
}

// Update serializes input and asynchronous service results into model changes.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		anchor := m.captureReadingAnchor()
		m.width = v.Width
		m.height = v.Height
		m.resize()
		m.restoreReadingAnchor(anchor)
		m.preserveVisibleSelection()
		return m, m.fillHistory()
	case conversationResult:
		return m.updateConversation(v)
	case directoryResult:
		return m.updateDirectory(v)
	case directoryProfileResult:
		return m.updateDirectoryProfile(v)
	case directConversationResult:
		return m.updateDirectConversation(v)
	case mentionResult:
		return m.updateMentions(v)
	case followedResult:
		return m.updateFollowed(v)
	case historyResult:
		return m.updateHistory(v)
	case profileResult:
		return m.updateProfile(v)
	case participationResult:
		return m.updateParticipation(v)
	case openSlackResult:
		if v.err != nil {
			m.setStatus("Open in Slack failed: "+v.err.Error(), statusAction)
		} else {
			m.setStatus("Opened selected message in Slack", statusAction)
		}
		return m, nil
	case sendResult:
		return m.updateSend(v)
	case tickMsg:
		return m.updateTick(v)
	case tea.KeyPressMsg:
		return m.updateKey(v)
	}
	if m.help || m.refreshDetails {
		return m, nil
	}
	// Picker paste/blink messages take precedence even when the underlying send is pending.
	if m.mode != pickerClosed {
		return m, m.updatePickerInput(msg)
	}
	if d := m.deliveries[m.key()]; d != nil && d.State == core.DeliveryPending && m.focus == focusCompose {
		return m, nil
	}
	if m.focus == focusCompose {
		var c tea.Cmd
		m.composer, c = m.composer.Update(msg)
		m.rememberDraft()
		return m, c
	}
	return m, nil
}
