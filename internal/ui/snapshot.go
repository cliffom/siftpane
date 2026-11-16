package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

const demoShippingConversation = "Cshipping" // Shared offline seed identity, not a domain state.

// SnapshotMode is an offline review identifier accepted by the CLI.
type SnapshotMode string

// Snapshot modes are independent of the UI's navigation state.
const (
	SnapshotWorkspaces     SnapshotMode = "workspaces"
	SnapshotAccountCompose SnapshotMode = "account-compose"
	SnapshotAccountFailed  SnapshotMode = "account-failed"
	SnapshotReadability    SnapshotMode = "readability"
	SnapshotCached         SnapshotMode = "cached"
	SnapshotCacheError     SnapshotMode = "cache-error"
	SnapshotCoverage       SnapshotMode = "coverage"
	SnapshotFollowed       SnapshotMode = "followed"
	SnapshotAttention      SnapshotMode = "attention"
	SnapshotMessages       SnapshotMode = "messages"
	SnapshotCompose        SnapshotMode = "compose"
	SnapshotPicker         SnapshotMode = "picker"
	SnapshotSearch         SnapshotMode = "search"
	SnapshotThread         SnapshotMode = "thread"
	SnapshotHelp           SnapshotMode = "help"
	SnapshotStartup        SnapshotMode = "startup"
	SnapshotLoading        SnapshotMode = "loading"
	SnapshotLoadError      SnapshotMode = "load-error"
)

// LoadSnapshot loads offline review data through the normal update path.
func LoadSnapshot(ctx context.Context, m *Model, s core.Service) error {
	cursor := ""
	for {
		p, err := s.Directory(ctx, cursor)
		if err != nil {
			return err
		}
		m.Update(directoryResult{Page: p})
		cursor = p.Next
		if cursor == "" {
			break
		}
	}
	p, err := s.History(ctx, core.Query{Conversation: m.active})
	if err != nil {
		return err
	}
	m.line(m.active)
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: p})
	return nil
}

// Snapshot renders a deterministic review state through the normal view path.
func Snapshot(m *Model, width, height int, mode SnapshotMode) string {
	m.width = width
	m.height = height
	m.resize()
	m.focus = focusMessages
	switch mode {
	case SnapshotWorkspaces:
		m.multipleAccounts = true
		other := m.identity
		other.Workspace, other.WorkspaceName, other.Username = "DEMO2", "Workshop · demo", "maya"
		a := &Accounts{ctx: m.ctx, store: m.store, sessions: []accountSession{{model: m, account: Account{Identity: m.identity}}, {account: Account{Identity: other}}}, choosing: true, choice: 1, width: width, height: height}
		return ansi.Strip(a.View().Content)
	case SnapshotAccountCompose, SnapshotAccountFailed:
		m.multipleAccounts = true
		m.focus = focusCompose
		m.composer.SetValue("Draft for this account, retained while switching.")
		m.composer.Focus()
		if mode == SnapshotAccountFailed {
			m.focus = focusMessages
			m.deliveries[m.key()] = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Text: m.composer.Value()}, State: core.DeliveryFailed, Uncertain: true}
			m.setStatus("Delivery unknown · check Slack before r retry", statusAction)
		}
	case SnapshotReadability:
		m.line(m.key()).Messages = []core.Message{{Author: "Maya", TS: "1794672000.000001", Text: "@you Please review:\n> Keep the old contract.\n- Check the parser\n- Preserve literal code mentions\n```if ready {\n    send(\"<@U2>\")\n}```\nUse `dryRun` before sending."}}
		m.line(m.key()).Loaded = true
		m.index, m.scroll = 0, -1

	case SnapshotCached, SnapshotCacheError:
		line := m.line(m.key())
		line.Cached, line.Busy, line.Saved = true, true, time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
		if mode == SnapshotCacheError {
			line.Busy = false
			line.Err = errors.New("connection unavailable")
		}
		m.setStatus("Cached history", statusRoutine)
	case SnapshotCoverage:
		snapshotCoverage(m)
	case SnapshotAttention:
		snapshotAttention(m)
	case SnapshotStartup:
		m.focus = focusChannels
		m.directory = nil
		m.active = ""
		for cid, included := range m.included {
			if included {
				m.metadataBusy[cid] = true
			}
		}
		m.setStatus("Restoring saved conversations", statusRoutine)
	case SnapshotLoading:
		m.startupSidebar = m.selected()
		m.startupRestoring = true
		for cid, included := range m.included {
			if included {
				m.metadataBusy[cid] = true
			}
		}
		m.cache[m.key()] = &timeline{Busy: true}
		m.setStatus("Loading conversation history", statusRoutine)
	case SnapshotLoadError:
		m.cache[m.key()] = &timeline{Err: errors.New("connection unavailable")}
		m.setStatus("History unavailable · R retry / refresh", statusRoutine)

	case SnapshotPicker:
		m.openPicker(pickerInclude)
	case SnapshotSearch:
		m.openPicker(pickerSwitch)
		m.picker.SetValue("@leo")
	case SnapshotCompose:
		m.focus = focusCompose
		m.composer.SetValue("Thanks, Maya. I'll review the keyboard flow this afternoon.\nThe small details are coming together.")
		m.composer.Focus()
	case SnapshotHelp:
		m.help = true
	case SnapshotThread, SnapshotFollowed:
		snapshotThread(m)
	}
	if mode == SnapshotFollowed && m.thread != "" {
		at := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
		m.attention.Followed = map[string]core.FollowedThread{m.key(): {Conversation: m.active, Root: m.thread, Checkpoint: at, LastCheck: at}}
		m.setStatus("", statusRoutine)
	}
	return ansi.Strip(m.View().Content)
}

func snapshotAttention(m *Model) {
	m.focus = focusChannels
	at := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	if err := m.attention.Add(demoShippingConversation, core.AttentionTrigger{ID: "review:mention", TS: "1794672000.000001", Reason: core.AttentionMention, At: at}); err != nil {
		m.setStatus(err.Error(), statusAction)
	}
	if err := m.attention.Add("Dleo", core.AttentionTrigger{ID: "review:temporary", Reason: core.AttentionManual, At: at.Add(time.Second)}); err != nil {
		m.setStatus(err.Error(), statusAction)
	}
	m.attention.Enabled = true
	m.attention.LastCheck = at
	m.discoveryPhase = discoveryChecked
	m.syncAttentionInclusion()
	m.active = demoShippingConversation
	p, err := m.service.History(m.ctx, core.Query{Conversation: m.active})
	if err != nil {
		m.setStatus(err.Error(), statusAction)
	} else {
		m.line(m.active)
		m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: p})
	}
}

func snapshotCoverage(m *Model) {
	snapshotAttention(m)
	for i, reason := range []core.AttentionReason{core.AttentionReply, core.AttentionChannel, core.AttentionHere} {
		ts := fmt.Sprintf("1794672001.%06d", i+1)
		trigger, err := core.MessageTrigger(ts, "1794671000.000001", reason)
		if err != nil {
			panic(err)
		}
		if err = m.attention.Add(m.active, trigger); err != nil {
			panic(err)
		}
	}
	m.attention.ChannelEnabled, m.attention.HereEnabled = true, true
	m.setStatus("", statusRoutine)
}

func snapshotThread(m *Model) {
	t := m.cache[m.active]
	if t != nil {
		for _, msg := range t.Messages {
			if msg.Replies > 0 {
				m.thread = msg.TS
				p, err := m.service.History(m.ctx, core.Query{Conversation: m.active, Thread: m.thread})
				if err != nil {
					m.setStatus(err.Error(), statusRoutine)
					break
				}
				m.cache[m.key()] = &timeline{Messages: core.Merge(nil, p.Messages, cacheLimit), Loaded: true}
				break
			}
		}
	}
}
