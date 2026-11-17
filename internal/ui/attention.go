package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

const (
	discoveryOverlap   = 48 * time.Hour // Revisit recent completed intervals for delayed indexing.
	discoveryInterval  = 3 * time.Minute
	discoveryPageLimit = 10 // Requests per check, independent of transport page size.
)

type discoveryPhase uint8

const (
	discoveryDisabled discoveryPhase = iota
	discoveryChecking
	discoveryChecked
	discoveryIncomplete
	discoveryFailed
)

type (
	mentionService interface {
		Mentions(context.Context, core.MentionQuery) (core.MentionPage, error)
	}
	mentionResult struct {
		Query      core.MentionQuery
		Generation uint64
		Page       core.MentionPage
		Err        error
	}
)

func (m *Model) initAttention() {
	a, err := m.store.Attention(m.identity)
	m.attention = a
	m.prefs.Included = map[string]bool{}
	for cid, v := range a.Kept {
		if v {
			m.prefs.Included[cid] = true
		}
	}
	if err != nil {
		m.attentionErr = err.Error()
		m.setStatus("Attention unavailable: "+err.Error(), statusAction)
	}
	m.opened = map[string]bool{}
	m.discoveryMetadata = map[string]bool{}
	m.openTargets = map[string]string{}
	// Included remains a runtime union. Legacy preferences mirror Kept only.
	m.included = map[string]bool{}
	for cid, v := range a.Kept {
		if v {
			m.included[cid] = true
		}
	}
	for cid := range a.Pending {
		m.included[cid] = true
	}
	if a.SearchEnabled() {
		m.discoveryPhase = discoveryIncomplete
	}
}

func (m *Model) persistAttention(a core.AttentionState) bool {
	if m.attentionErr != "" {
		m.setStatus("Attention unavailable: "+m.attentionErr, statusAction)
		return false
	}
	if err := m.store.SaveAttention(m.identity, a); err != nil {
		m.setStatus("Cannot save attention: "+err.Error(), statusAction)
		return false
	}
	m.attention = a
	return true
}

func (m *Model) syncAttentionInclusion() {
	for cid, v := range m.attention.Kept {
		if v && !m.included[cid] {
			m.included[cid] = true
			m.addContext(cid)
		}
	}
	for cid := range m.attention.Pending {
		if !m.included[cid] {
			m.included[cid] = true
			m.addContext(cid)
		}
	}
	m.prefs.Included = map[string]bool{}
	for cid, v := range m.attention.Kept {
		if v {
			m.prefs.Included[cid] = true
		}
	}
}

// toggle changes Kept independently of pending attention.
func (m *Model) toggle(cid string) {
	a := m.attention.Clone()
	if a.Kept[cid] {
		delete(a.Kept, cid)
		for key, f := range a.Followed {
			if f.Conversation == cid {
				delete(a.Followed, key)
			}
		}
	} else {
		if len(a.Kept) >= selectionLimit {
			m.setStatus(fmt.Sprintf("Limit: %d kept conversations. Remove one first.", selectionLimit), statusAction)
			return
		}
		a.Kept[cid] = true
	}
	if !m.persistAttention(a) {
		return
	}
	if !a.Kept[cid] {
		m.cancelFollowed()
	}
	if !a.Kept[cid] && len(a.Pending[cid]) == 0 {
		m.toggleIncluded(cid)
	} else if !m.included[cid] {
		m.toggleIncluded(cid)
	}
	m.syncAttentionInclusion()
	if !a.Kept[cid] {
		delete(m.persisted, cid)
		m.saveMessages()
	}
	m.save()
	switch {
	case a.Kept[cid]:
		m.setStatus("Kept conversation", statusAction)
	case len(a.Pending[cid]) > 0:
		m.setStatus("Unkept · attention retained", statusAction)
	default:
		m.setStatus("Unkept conversation", statusAction)
	}
}

func (m *Model) temporaryChoice(cs []core.Conversation) tea.Cmd {
	if len(cs) == 0 {
		return nil
	}
	c := cs[max(0, min(m.pickIndex, len(cs)-1))]
	if m.directoryCached[c.ID] {
		return m.resolveConversation(c.ID, directTemporary)
	}
	a := m.attention.Clone()
	if err := a.Add(c.ID, core.AttentionTrigger{ID: "manual:" + rand.Text(), Reason: core.AttentionManual, At: time.Now().UTC()}); err != nil {
		m.setStatus(err.Error(), statusAction)
		return nil
	}
	if !m.persistAttention(a) {
		return nil
	}
	m.syncAttentionInclusion()
	m.publishStartupConversation(c.ID)
	m.closePicker()
	m.setStatus("Temporary conversation · keep or dismiss explicitly", statusAction)
	return m.selectConversation(c.ID)
}

func (m *Model) toggleActiveKept() {
	if m.active == "" {
		return
	}
	if m.attention.Kept[m.active] && len(m.attention.Pending[m.active]) == 0 && m.attentionDirty(m.active) {
		m.setStatus("Draft/send · recover before unkeeping", statusAction)
		return
	}
	m.toggle(m.active)
	if m.active == "" {
		m.focus = focusChannels
	}
}

func (m *Model) attentionDirty(cid string) bool {
	m.rememberDraft()
	for key, text := range m.drafts {
		c, _ := split(key)
		if c == cid && text != "" {
			return true
		}
	}
	for key, d := range m.deliveries {
		c, _ := split(key)
		if c == cid && (d.State == core.DeliveryPending || d.State == core.DeliveryFailed) {
			return true
		}
	}
	return false
}

func (m *Model) dismissActive() tea.Cmd {
	cid := m.active
	if len(m.attention.Pending[cid]) == 0 {
		m.setStatus("No attention to dismiss", statusAction)
		return nil
	}
	if m.attentionDirty(cid) {
		m.setStatus("Draft/send retained · recover before "+keyLabel(keyDismiss), statusAction)
		return nil
	}
	a := m.attention.Clone()
	a.Dismiss(cid)
	if !m.persistAttention(a) {
		return nil
	}
	for key := range m.openTargets {
		c, _ := split(key)
		if c == cid {
			delete(m.openTargets, key)
		}
	}
	if !a.Kept[cid] {
		m.toggleIncluded(cid)
	}
	m.syncAttentionInclusion()
	m.setStatus("Dismissed locally; Slack membership and read position unchanged", statusAction)
	return nil
}

func (m *Model) nextAttention() tea.Cmd {
	cs := m.attentionOrder()
	if len(cs) == 0 {
		m.setStatus("No attention entries · "+m.discoveryStatus(), statusAction)
		return nil
	}
	next := 0
	for i, cid := range cs {
		if cid == m.active {
			next = (i + 1) % len(cs)
			break
		}
	}
	return m.selectConversation(cs[next])
}

func (m *Model) attentionOrder() []string {
	ids := make([]string, 0, len(m.attention.Pending))
	for cid, ts := range m.attention.Pending {
		if len(ts) > 0 {
			ids = append(ids, cid)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := m.attention.Pending[ids[i]][0], m.attention.Pending[ids[j]][0]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		return ids[i] < ids[j]
	})
	return ids
}

func (m *Model) canRead(cid string) bool {
	return m.included[cid] && (m.attention.Kept[cid] || m.opened[cid])
}

func (m *Model) toggleMentions() tea.Cmd {
	if !m.attention.Enabled && !m.mentionConfirm {
		m.mentionConfirm = true
		m.setStatus(keyLabel(keyMentions)+" again: unkept mentions · search:read", statusAction)
		return nil
	}
	m.mentionConfirm = false
	a := m.attention.Clone()
	a.Enabled = !a.Enabled
	if a.Enabled {
		a.Checkpoint = time.Time{}
	}
	if !m.persistAttention(a) {
		return nil
	}
	m.discoveryGeneration++
	if m.discoveryCancel != nil {
		m.discoveryCancel()
	}
	m.finishDiscovery()
	if !a.SearchEnabled() {
		m.discoveryPhase = discoveryDisabled
		m.setStatus("Mention discovery disabled; existing attention retained", statusAction)
		return nil
	}
	m.discoveryNext = time.Time{}
	text := "Direct mentions disabled · broadcasts unchanged"
	if a.Enabled {
		text = "Direct mentions enabled · limited search"
	}
	m.setStatus(text, statusAction)
	return m.checkMentions(time.Now())
}

func (m *Model) checkMentions(now time.Time) tea.Cmd {
	if !m.attention.SearchEnabled() || m.attentionErr != "" || m.stopped || m.discoveryBusy {
		return nil
	}
	if !m.discoveryNext.IsZero() && now.Before(m.discoveryNext) {
		return nil
	}
	from := now.Add(-core.AttentionLookback)
	if overlap := m.attention.Checkpoint.Add(-discoveryOverlap); !m.attention.Checkpoint.IsZero() && overlap.After(from) {
		from = overlap
	}
	if from.Before(m.attention.Floor) {
		from = m.attention.Floor
	}
	if !now.After(from) {
		m.discoveryPhase = discoveryFailed
		m.discoveryErr = "Clock precedes stored discovery floor"
		return nil
	}
	if !m.attention.Checkpoint.IsZero() && m.attention.Checkpoint.Before(from) {
		a := m.attention.Clone()
		a.RecordGap(a.Checkpoint, from)
		if !m.persistAttention(a) {
			return nil
		}
	}
	m.discoveryBusy = true
	m.discoveryPage = 1
	m.discoveryTotal = 0
	m.discoveryPhase = discoveryChecking
	m.discoveryErr = ""
	m.discoveryNext = now.Add(discoveryInterval)
	m.discoveryGeneration++
	ctx, cancel := context.WithCancel(m.ctx)
	m.discoveryCancel = cancel
	m.discoveryCtx = ctx
	m.discoverySources = m.searchSources()
	return m.mentionCmd(core.MentionQuery{Reason: m.discoverySources[0], User: m.identity.User, From: from, Until: now, Page: 1})
}

func (m *Model) mentionCmd(q core.MentionQuery) tea.Cmd {
	svc, ok := m.service.(mentionService)
	gen, ctx := m.discoveryGeneration, m.discoveryCtx
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return mentionResult{Query: q, Generation: gen, Err: err}
		}
		if !ok {
			return mentionResult{Query: q, Generation: gen, Err: fmt.Errorf("mention discovery unavailable for this service")}
		}
		page, err := svc.Mentions(ctx, q)
		return mentionResult{Query: q, Generation: gen, Page: page, Err: err}
	}
}

func (m *Model) updateMentions(v mentionResult) (tea.Model, tea.Cmd) {
	if v.Generation != m.discoveryGeneration || !m.attention.SearchEnabled() || !m.discoveryBusy || v.Query.Page != m.discoveryPage {
		return m, nil
	}
	reason := v.Query.Reason
	if reason == "" {
		reason = core.AttentionMention
	}
	if len(m.discoverySources) > 0 && reason != m.discoverySources[0] {
		return m, nil
	}

	if v.Err != nil {
		m.finishDiscovery()
		m.discoveryPhase = discoveryFailed
		m.discoveryErr = v.Err.Error()
		if slack.IsAuth(v.Err) {
			m.stopReads()
		}
		return m, nil
	}
	if v.Query.Page == 1 {
		m.discoveryTotal = v.Page.Total
	} else if v.Page.Total != m.discoveryTotal {
		m.finishDiscovery()
		m.discoveryPhase = discoveryIncomplete
		m.discoveryErr = "Search changed during pagination; retry"
		return m, nil
	}
	a := m.attention.Clone()
	floor := v.Query.From
	for _, f := range a.Followed {
		bound := f.Checkpoint
		if minimum := v.Query.Until.Add(-core.AttentionLookback); bound.Before(minimum) {
			bound = minimum
		}
		if bound.Before(floor) {
			floor = bound
		}
	}
	a.Prune(floor)
	for _, match := range v.Page.Matches {
		if err := a.Add(match.Conversation.ID, match.Trigger); err != nil {
			m.finishDiscovery()
			m.discoveryPhase = discoveryIncomplete
			m.discoveryErr = err.Error()
			return m, nil
		}
	}
	if !v.Page.More && len(m.discoverySources) <= 1 {
		a.Checkpoint = v.Query.Until
		a.LastCheck = v.Query.Until
	}
	if !m.persistAttention(a) {
		m.finishDiscovery()
		m.discoveryPhase = discoveryFailed
		m.discoveryErr = "Cannot save discovery identities; retry with " + keyLabel(keyRefresh)
		return m, nil
	}
	for _, match := range v.Page.Matches {
		m.mergeAttentionConversation(match.Conversation)
	}
	m.syncAttentionInclusion()
	for _, match := range v.Page.Matches {
		m.publishStartupConversation(match.Conversation.ID)
	}
	if v.Page.More && v.Query.Page < discoveryPageLimit {
		q := v.Query
		q.Page++
		m.discoveryPage = q.Page
		return m, m.mentionCmd(q)
	}
	if !v.Page.More && len(m.discoverySources) > 1 {
		m.discoverySources = m.discoverySources[1:]
		q := v.Query
		q.Reason, q.Page = m.discoverySources[0], 1
		m.discoveryPage, m.discoveryTotal = 1, 0
		return m, m.mentionCmd(q)
	}
	m.finishDiscovery()
	if v.Page.More {
		m.discoveryPhase = discoveryIncomplete
		m.discoveryErr = "Page budget reached; coverage incomplete"
	} else {
		m.discoveryPhase = discoveryChecked
	}
	return m, nil
}

func (m *Model) mergeAttentionConversation(c core.Conversation) {
	for _, old := range m.directory {
		if old.ID == c.ID {
			return
		}
	}
	if len(m.attention.Pending[c.ID]) > 0 {
		m.directory = append(m.directory, c)
		m.discoveryMetadata[c.ID] = true
	}
}

func (m *Model) discoveryStatus() string {
	if m.attentionErr != "" {
		return "Attention state unavailable · recover saved file"
	}
	if !m.attention.SearchEnabled() {
		return "Mentions off · " + keyLabel(keyMentions) + " enable"
	}
	if m.discoveryPhase == discoveryChecking {
		return "Checking attention search" + m.gapMarker() + "…"
	}
	if m.discoveryErr != "" {
		return "Search failed" + m.gapMarker() + " · " + keyLabel(keyRefresh) + " retry · " + m.discoveryErr
	}
	if m.attention.LastCheck.IsZero() {
		return "Search not checked" + m.gapMarker() + " · " + keyLabel(keyRefresh) + " retry"
	}
	text := "Search " + m.attention.LastCheck.Local().Format(statusTimeFormat) + " · limited search"
	if m.attention.Gap != nil {
		text = "Search " + m.attention.LastCheck.Local().Format(statusTimeFormat) + " · gap/limited"
		if m.width >= sidebarBreakpoint {
			text += " · " + m.attention.Gap.From.Local().Format("Jan 02") + "–" + m.attention.Gap.Until.Local().Format("Jan 02") + " · " + keyLabel(keyAcknowledgeGap) + " acknowledge"
		}
	}
	return text
}

func (m *Model) replaceConversation(c core.Conversation) {
	for i, old := range m.directory {
		if old.ID == c.ID {
			m.directory[i] = c
			return
		}
	}
	m.directory = append(m.directory, c)
}

// fetchTrigger requests a bounded page anchored at the actual mention, so old
// mentions and replies do not silently open only the newest channel page.
func (m *Model) fetchTrigger(t core.AttentionTrigger) tea.Cmd {
	if m.stopped || !m.canRead(m.active) {
		return nil
	}
	key := m.key()
	line := m.line(key)
	if line.Busy {
		m.openTargets[key] = t.TS
		return nil
	}
	line.Busy = true
	line.Err = nil
	m.openTargets[key] = t.TS
	q := core.Query{Conversation: m.active, Thread: m.thread, Target: t.TS}
	epoch := m.cacheEpoch
	ctx, svc, gen := m.contexts[m.active], m.service, m.generations[m.active]
	return func() tea.Msg {
		p, err := svc.History(ctx, q)
		after := ""
		if q.Thread != "" {
			after = q.Target
		}
		return historyResult{CacheEpoch: epoch, Key: key, Generation: gen, Page: p, Err: err, QueryAfter: after, Target: q.Target}
	}
}

func (m *Model) finishDiscovery() {
	m.discoveryBusy = false
	if m.discoveryCancel != nil {
		m.discoveryCancel()
		m.discoveryCancel = nil
	}
	m.discoveryCtx = nil
}

func (m *Model) acknowledgeGap() {
	gap := m.attention.Gap
	if gap == nil {
		m.setStatus("No discovery gap to acknowledge", statusAction)
		return
	}
	if m.gapConfirmation == nil || !m.gapConfirmation.From.Equal(gap.From) || !m.gapConfirmation.Until.Equal(gap.Until) {
		copyGap := *gap
		m.gapConfirmation = &copyGap
		m.setStatus(keyLabel(keyAcknowledgeGap)+" again: acknowledge gaps", statusAction)
		return
	}
	a := m.attention.Clone()
	if gap.Until.After(a.GapAcknowledgedUntil) {
		a.GapAcknowledgedUntil = gap.Until
	}
	a.Gap = nil
	if !m.persistAttention(a) {
		return
	}
	m.gapConfirmation = nil
	m.setStatus("Gaps acknowledged · limited search", statusAction)
}

func (m *Model) searchSources() []core.AttentionReason {
	sources := []core.AttentionReason{}
	if m.attention.Enabled {
		sources = append(sources, core.AttentionMention)
	}
	if m.attention.ChannelEnabled {
		sources = append(sources, core.AttentionChannel)
	}
	if m.attention.HereEnabled {
		sources = append(sources, core.AttentionHere)
	}
	return sources
}

func (m *Model) toggleBroadcast(k string) tea.Cmd {
	reason := broadcastReason(k)
	if reason == "" {
		return nil
	}
	a := m.attention.Clone()
	enabled := &a.ChannelEnabled
	label := "@channel"
	if k == keyHere {
		enabled, label = &a.HereEnabled, "@here"
	}
	if !*enabled && m.broadcastConfirm != reason {
		m.broadcastConfirm = reason
		m.setStatus(k+" again: unkept "+label+" · search:read", statusAction)
		return nil
	}
	m.broadcastConfirm = ""
	*enabled = !*enabled
	// A new source must revisit the supported interval; handled identities stay durable.
	if *enabled {
		a.Checkpoint = time.Time{}
	}
	if !m.persistAttention(a) {
		return nil
	}
	m.discoveryGeneration++
	m.finishDiscovery()
	m.discoveryNext = time.Time{}
	m.discoveryPhase = discoveryDisabled
	state := "disabled"
	if *enabled {
		state = "enabled"
	}
	m.setStatus(label+" "+state+" · direct mentions independent", statusAction)
	return m.checkMentions(time.Now())
}
