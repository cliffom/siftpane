package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

// Account provides an authenticated service; credentials never enter UI state.
type Account struct {
	Service  core.Service
	Identity core.Identity
	Offline  bool
	// Reload reads and validates the credential at the CLI boundary. No token is returned.
	Reload func(context.Context) (core.Service, core.Identity, error)
}

type accountSession struct {
	account   Account
	model     *Model
	cancel    context.CancelFunc
	epoch     uint64
	reloading bool
}

type accountEvent struct {
	index   int
	epoch   uint64
	message tea.Msg
}

// Accounts owns retained sessions and routes asynchronous results to their owner.
// Only the active session receives input, ticks, or read completions.
type Accounts struct {
	ctx           context.Context
	store         *store.Store
	sessions      []accountSession
	active        int
	choosing      bool
	choice        int
	width, height int
	quitConfirm   bool
}

// NewAccounts creates an explicit account switcher around isolated UI sessions.
func NewAccounts(ctx context.Context, accounts []Account, st *store.Store) *Accounts {
	a := &Accounts{ctx: ctx, store: st, width: DefaultWidth, height: DefaultHeight}
	for _, account := range accounts {
		a.sessions = append(a.sessions, accountSession{account: account})
	}
	return a
}

func (a *Accounts) current() *Model { return a.sessions[a.active].model }

func (a *Accounts) activate(index int) tea.Cmd {
	s := &a.sessions[index]
	a.active = index
	ctx, cancel := context.WithCancel(a.ctx)
	s.cancel = cancel
	s.epoch++
	if s.model == nil {
		s.model = New(ctx, s.account.Service, s.account.Identity, a.store, s.account.Offline)
		s.model.sendCtx = a.ctx
		s.model.multipleAccounts = true
		s.model.width, s.model.height = a.width, a.height
		s.model.resize()
		return a.wrap(index, s.epoch, s.model.Init())
	}
	m := s.model
	m.ctx = ctx
	for cid := range m.included {
		m.addContext(cid)
	}
	m.directoryBusy = false
	m.discoveryNext = time.Time{}
	m.followNext = map[string]time.Time{}
	// Keep stopped authentication local; activation never retries a failed send.
	if m.stopped {
		return a.wrap(index, s.epoch, m.resumeInput())
	}
	cmds := []tea.Cmd{m.resumeInput(), m.profileCmd(), m.checkMentions(time.Now()), m.checkFollowed(time.Now())}
	if !m.directoryLoaded {
		cmds = append(cmds, m.directoryCmd(""))
	}
	if m.scroll == 0 {
		cmds = append(cmds, m.fetch(m.key(), false))
	}
	for cid := range m.included {
		cmds = append(cmds, m.conversationCmd(cid))
	}
	return a.wrap(index, s.epoch, tea.Batch(cmds...))
}

func (a *Accounts) deactivate() {
	s := &a.sessions[a.active]
	m := s.model
	m.rememberDraft()
	m.cancelDirectory()
	s.cancel()
	s.epoch++
	m.cancelProfileLookup()
	m.discoveryGeneration++
	m.finishDiscovery()
	m.cancelFollowed()
	m.metadataBusy = map[string]bool{}
	m.startupRestoring = false
	m.startupSidebar = nil
	for _, line := range m.cache {
		line.Busy = false
	}
	m.cancelConfirmations()
}

// wrap recursively tags batched results while leaving terminal control messages
// visible to Bubble Tea. Tick chains belong to Accounts, never retained sessions.
func (a *Accounts) wrap(index int, epoch uint64, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch v := msg.(type) {
		case tea.BatchMsg:
			cmds := make([]tea.Cmd, 0, len(v))
			for _, c := range v {
				cmds = append(cmds, a.wrap(index, epoch, c))
			}
			return tea.BatchMsg(cmds)
		case tickMsg:
			return nil
		case nil:
			return nil
		case tea.QuitMsg:
			return v
		case terminalResult:
			return v.message
		default:
			return accountEvent{index, epoch, msg}
		}
	}
}

// Init starts one active account and a single application refresh clock.
func (a *Accounts) Init() tea.Cmd { return tea.Batch(a.activate(a.active), tick()) }

// Update preserves ownership across switches, including delayed send completion.
func (a *Accounts) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case credentialResult:
		return a, a.updateCredential(v)
	case accountEvent:
		s := &a.sessions[v.index]
		if accountWriteResult(v.message) {
			_, cmd := s.model.Update(v.message)
			return a, a.wrap(v.index, s.epoch, cmd)
		}
		if v.index != a.active || v.epoch != s.epoch || (a.choosing && !accountReadResult(v.message)) {
			return a, nil
		}
		_, cmd := s.model.Update(v.message)
		return a, a.wrap(v.index, s.epoch, cmd)
	case tickMsg:
		_, cmd := a.current().Update(v)
		return a, tea.Batch(tick(), a.wrap(a.active, a.sessions[a.active].epoch, cmd))
	case tea.WindowSizeMsg:
		a.width, a.height = v.Width, v.Height
		for i := range a.sessions {
			if m := a.sessions[i].model; m != nil {
				_, _ = m.Update(v)
			}
		}
		return a, nil
	case tea.KeyPressMsg:
		return a.key(v)
	}
	if a.choosing {
		return a, nil
	}
	_, cmd := a.current().Update(msg)
	return a, a.wrap(a.active, a.sessions[a.active].epoch, cmd)
}

func (a *Accounts) dirty() bool {
	for _, s := range a.sessions {
		if s.model == nil {
			continue
		}
		s.model.rememberDraft()
		for _, text := range s.model.drafts {
			if text != "" {
				return true
			}
		}
		for _, d := range s.model.deliveries {
			if d.State == core.DeliveryPending || d.State == core.DeliveryFailed {
				return true
			}
		}
	}
	return false
}

func (a *Accounts) key(v tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := v.String()
	m := a.current()
	if k == keyQuit && !m.help && !m.refreshDetails {
		if v.Key().IsRepeat {
			return a, nil
		}
		m.cancelConfirmations()
		if a.dirty() && !a.quitConfirm {
			a.quitConfirm = true
			m.quitConfirm = true
			return a, nil
		}
		return a, tea.Quit
	}
	a.quitConfirm, m.quitConfirm = false, false
	if a.choosing {
		m.cancelConfirmations()
		return a.chooseKey(k, v.Key().IsRepeat)
	}
	if k == keyReloadCredential && !m.help && !m.refreshDetails && m.mode == pickerClosed && m.focus != focusCompose {
		m.cancelConfirmations()
		if v.Key().IsRepeat {
			return a, nil
		}
		return a, a.reloadCredential()
	}
	if k == keyAccounts && !m.help && !m.refreshDetails {
		if v.Key().IsRepeat {
			return a, nil
		}
		m.cancelConfirmations()
		m.rememberDraft()
		m.cancelDirectory()
		a.choosing, a.choice = true, a.active
		return a, nil
	}
	if k == keyQuitQuiet && !m.help && !m.refreshDetails && m.mode == pickerClosed && m.focus != focusCompose && a.dirty() {
		m.cancelConfirmations()
		m.setStatus("Unsent text in an account · "+keyLabel(keyQuit)+" twice to exit", statusAction)
		return a, nil
	}
	_, cmd := m.Update(v)
	return a, a.wrap(a.active, a.sessions[a.active].epoch, cmd)
}

func (a *Accounts) chooseKey(k string, repeat bool) (tea.Model, tea.Cmd) {
	switch k {
	case keyEscape, keyAccounts:
		a.choosing = false
		return a, a.wrap(a.active, a.sessions[a.active].epoch, a.current().resumeInput())
	case keyUp, keyPrevious:
		a.choice = (a.choice + len(a.sessions) - 1) % len(a.sessions)
	case keyDown, keyNext:
		a.choice = (a.choice + 1) % len(a.sessions)
	case keyCompose:
		if repeat {
			return a, nil
		}
		a.choosing = false
		if a.choice != a.active {
			a.deactivate()
			return a, a.activate(a.choice)
		}
		return a, a.wrap(a.active, a.sessions[a.active].epoch, a.current().resumeInput())
	}
	return a, nil
}

// View renders the active account and an explicit identity-only selector.
// Inactive counts are omitted: inactive accounts are never polled.
func (a *Accounts) View() tea.View {
	v := a.current().View()
	if !a.choosing || a.width < MinWidth || a.height < MinHeight {
		return v
	}
	w, h := a.current().pickerSize()
	rows := []string{" " + keyLabel(keyCompose) + " switch · " + keyLabel(keyEscape) + " close", " Inactive accounts pause reads", ""}
	visible := max(1, h-frameRows-len(rows))
	start := max(0, a.choice-visible+1)
	const separator = " · "
	markerColumns := ansi.StringWidth("▸ ")
	nameColumns := w - frameColumns - markerColumns - ansi.StringWidth(separator)
	workspaceColumns := 0
	for _, session := range a.sessions {
		workspaceColumns = max(workspaceColumns, ansi.StringWidth(accountText(session.account.Identity.WorkspaceName)))
	}
	workspaceColumns = min(workspaceColumns, nameColumns/accountNameColumns)
	for i := start; i < len(a.sessions) && i < start+visible; i++ {
		id := a.sessions[i].account.Identity
		marker := "  "
		if i == a.choice {
			marker = "▸ "
		}
		label := marker + fit(accountText(id.WorkspaceName), workspaceColumns) + separator + "@" + accountText(id.Username)
		rows = append(rows, fit(label, w-frameColumns))
	}
	content := a.current().modalOverlay(v.Content, dialogFrame("Workspaces", rows, w, h, lipgloss.RoundedBorder()), a.width, a.height)
	v.Content = strings.TrimSuffix(content, "\n")
	return v
}

func accountWriteResult(msg tea.Msg) bool {
	switch msg.(type) {
	case sendResult, participationResult, openSlackResult:
		return true
	}
	return false
}

func accountReadResult(msg tea.Msg) bool {
	switch msg.(type) {
	case historyResult, conversationResult, directoryResult, directoryProfileResult, directConversationResult, mentionResult, followedResult, profileResult:
		return true
	}
	return false
}
