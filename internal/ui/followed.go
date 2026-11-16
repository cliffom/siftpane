package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

type followedResult struct {
	Key         string
	Generation  uint64
	Query       core.Query
	From, Until time.Time
	Number      int
	Page        core.Page
	Err         error
}

func (m *Model) toggleFollow() tea.Cmd {
	if m.active == "" || m.thread == "" {
		m.setStatus("Open a thread first with "+keyLabel(keyThread), statusAction)
		return nil
	}
	a := m.attention.Clone()
	if a.Followed == nil {
		a.Followed = map[string]core.FollowedThread{}
	}
	key := m.key()
	if _, exists := a.Followed[key]; exists {
		delete(a.Followed, key)
	} else {
		if len(a.Followed) >= core.FollowedThreads {
			m.setStatus(fmt.Sprintf("Limit: %d locally followed threads", core.FollowedThreads), statusAction)
			return nil
		}
		a.Followed[key] = core.FollowedThread{Conversation: m.active, Root: m.thread, Checkpoint: time.Now().UTC().Truncate(time.Microsecond)}
	}
	if !m.persistAttention(a) {
		return nil
	}
	m.cancelFollowed()
	delete(m.followNext, key)
	delete(m.followErrors, key)
	text := "Thread unfollowed locally · attention retained"
	if _, exists := a.Followed[key]; exists {
		text = "Following thread locally · future replies from others"
	}
	m.setStatus(text, statusAction)
	return m.checkFollowed(time.Now())
}

func (m *Model) cancelFollowed() {
	m.followGeneration++
	if m.followCancel != nil {
		m.followCancel()
		m.followCancel = nil
	}
	m.followBusy = false
	m.followKey = ""
	m.followCtx = nil
	for key := range m.followErrors {
		if _, exists := m.attention.Followed[key]; !exists {
			delete(m.followErrors, key)
		}
	}
	for key := range m.followNext {
		if _, exists := m.attention.Followed[key]; !exists {
			delete(m.followNext, key)
		}
	}
}

func (m *Model) retryFollowed() tea.Cmd {
	m.followNext = map[string]time.Time{}
	return m.checkFollowed(time.Now())
}

func messageBound(at time.Time) string {
	return fmt.Sprintf("%d.%06d", at.Unix(), at.Nanosecond()/int(time.Microsecond))
}

func (m *Model) checkFollowed(now time.Time) tea.Cmd {
	if m.stopped || m.attentionErr != "" || m.followBusy || len(m.attention.Followed) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.attention.Followed))
	for key := range m.attention.Followed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for offset := range keys {
		idx := (m.followIndex + offset) % len(keys)
		key := keys[idx]
		if now.Before(m.followNext[key]) {
			continue
		}
		f := m.attention.Followed[key]
		until := now.UTC().Truncate(time.Microsecond)
		from := f.Checkpoint
		floor := until.Add(-core.AttentionLookback)
		if floor.Before(m.attention.Floor) {
			floor = m.attention.Floor
		}
		if from.Before(floor) {
			a := m.attention.Clone()
			a.RecordGap(from, floor)
			if !m.persistAttention(a) {
				return nil
			}
			from = floor
		}
		if !until.After(from) {
			continue
		}
		m.followIndex = (idx + 1) % len(keys)
		m.followBusy, m.followKey, m.followPage = true, key, 1
		m.followNext[key] = now.Add(discoveryInterval)
		ctx, cancel := context.WithCancel(m.ctx)
		m.followCancel = cancel
		m.followCtx = ctx
		q := core.Query{Conversation: f.Conversation, Thread: f.Root, After: messageBound(from.Add(-time.Microsecond)), Before: messageBound(until)}
		return m.followedCmd(ctx, followedResult{Key: key, Generation: m.followGeneration, Query: q, From: from, Until: until, Number: 1})
	}
	return nil
}

func (m *Model) followedCmd(ctx context.Context, result followedResult) tea.Cmd {
	svc := m.service
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			result.Err = err
			return result
		}
		result.Page, result.Err = svc.History(ctx, result.Query)
		return result
	}
}

func (m *Model) failFollowed(key, text string) {
	m.followErrors[key] = text
	m.cancelFollowed()
}

func (m *Model) updateFollowed(v followedResult) (tea.Model, tea.Cmd) {
	f, exists := m.attention.Followed[v.Key]
	if m.stopped || !exists || !m.followBusy || v.Key != m.followKey || v.Generation != m.followGeneration || v.Number != m.followPage {
		return m, nil
	}
	if v.Err != nil {
		m.failFollowed(v.Key, v.Err.Error())
		if slack.IsAuth(v.Err) {
			m.stopReads()
		}
		if slack.IsAccess(v.Err) {
			m.purgeConversationCache(f.Conversation, v.Err)
		}
		return m, nil
	}
	a := m.attention.Clone()
	a.Prune(v.Until.Add(-core.AttentionLookback))
	for _, msg := range v.Page.Messages {
		// APIs may include the root; it is never a new reply. Self replies aren't requests.
		if msg.TS == f.Root || msg.User == m.identity.User {
			continue
		}
		trigger, err := core.MessageTrigger(msg.TS, f.Root, core.AttentionReply)
		if err != nil {
			m.failFollowed(v.Key, "Invalid reply identity · incomplete")
			return m, nil
		}
		if trigger.At.Before(v.From) || !trigger.At.Before(v.Until) {
			continue
		}
		if err = a.Add(f.Conversation, trigger); err != nil {
			m.failFollowed(v.Key, err.Error())
			return m, nil
		}
	}
	more := v.Page.More || v.Page.Next != ""
	if !more {
		f.Checkpoint, f.LastCheck = v.Until, v.Until
		a.Followed[v.Key] = f
	}
	if !m.persistAttention(a) {
		m.failFollowed(v.Key, "Cannot save reply identities")
		return m, nil
	}
	m.mergeAttentionConversation(core.Conversation{ID: f.Conversation, Name: f.Conversation, Kind: core.ConversationChannel})
	m.syncAttentionInclusion()
	m.publishStartupConversation(f.Conversation)
	if more {
		if v.Number >= discoveryPageLimit || v.Page.Next == "" || v.Page.Next == v.Query.Cursor || len(v.Page.Messages) == 0 {
			m.failFollowed(v.Key, "Reply page budget/pagination incomplete")
			return m, nil
		}
		v.Query.Cursor = v.Page.Next
		v.Number++
		m.followPage = v.Number
		return m, m.followedCmd(m.followCtx, v)
	}
	delete(m.followErrors, v.Key)
	m.cancelFollowed()
	return m, nil
}

func (m *Model) followStatus() string {
	if len(m.attention.Followed) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m.followErrors))
	for key := range m.followErrors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return "Replies incomplete" + m.gapMarker() + " · R retry · " + m.followErrors[keys[0]]
	}
	if m.followBusy {
		return "Checking followed replies" + m.gapMarker() + "…"
	}
	var oldest time.Time
	for _, f := range m.attention.Followed {
		if f.LastCheck.IsZero() {
			return "Followed replies not checked" + m.gapMarker() + " · R retry"
		}
		if oldest.IsZero() || f.LastCheck.Before(oldest) {
			oldest = f.LastCheck
		}
	}
	text := fmt.Sprintf("%d local threads · %s", len(m.attention.Followed), oldest.Local().Format(statusTimeFormat))
	if m.attention.Gap != nil {
		text += " · gap · " + keyLabel(keyAcknowledgeGap)
	}
	if m.attention.SearchEnabled() {
		text += " · " + m.discoveryStatus()
	}
	return text
}
