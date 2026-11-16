package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

// Only transports with lazy identity lookup need asynchronous enrichment.
type profileService interface {
	Profile(context.Context, string) (core.Profile, error)
	CachedProfiles() []core.Profile
	RestoreProfiles([]core.Profile)
}

type profileResult struct {
	Profile                      core.Profile
	ID, Key                      string
	Generation, LookupGeneration uint64
	Err                          error
}

func (m *Model) cancelProfileLookup() {
	if m.profileCancel != nil {
		m.profileCancel()
	}
	m.profileCancel = nil
	m.profileBusy = false
	m.profileKey = ""
	m.profileLookupGeneration++
	m.profileAttempted = map[string]bool{}
}

func (m *Model) syncProfiles() {
	svc, ok := m.service.(profileService)
	if !ok {
		return
	}
	ps := svc.CachedProfiles()
	next := make(map[string]core.Profile, len(ps))
	changed := len(ps) != len(m.profiles)
	for _, p := range ps {
		next[p.ID] = p
		if m.profiles[p.ID] != p {
			changed = true
		}
	}
	m.profiles = next
	if changed || m.profileCacheErr != "" {
		m.profileCacheErr = ""
		if err := m.store.SaveProfiles(m.identity, ps); err != nil {
			m.profileCacheErr = err.Error()
		}
	}
}

func (m *Model) profileCmd() tea.Cmd {
	svc, ok := m.service.(profileService)
	if !ok || m.stopped || m.profileBusy || !m.included[m.active] {
		return nil
	}
	m.syncProfiles()
	line := m.cache[m.key()]
	if line == nil {
		return nil
	}
	for i := len(line.Messages) - 1; i >= 0; i-- {
		msg := line.Messages[i]
		ids := []string{msg.User}
		ids = append(ids, slack.MentionIDs(msg.Text)...)
		for _, id := range ids {
			if id == "" || m.profiles[id].Name != "" || m.profileAttempted[id] {
				continue
			}
			if len(m.profileAttempted) >= profileLookupLimit {
				return nil
			}
			m.profileAttempted[id] = true
			ctx, cancel := context.WithCancel(m.contexts[m.active])
			m.profileCancel = cancel
			m.profileBusy = true
			m.profileKey = m.key()
			key, gen, lookupGen := m.key(), m.generations[m.active], m.profileLookupGeneration
			return func() tea.Msg {
				if err := ctx.Err(); err != nil {
					return profileResult{ID: id, Key: key, Generation: gen, LookupGeneration: lookupGen, Err: err}
				}
				p, err := svc.Profile(ctx, id)
				return profileResult{ID: id, Key: key, Generation: gen, LookupGeneration: lookupGen, Profile: p, Err: err}
			}
		}
	}
	return nil
}

func (m *Model) updateProfile(v profileResult) (tea.Model, tea.Cmd) {
	cid, _ := split(v.Key)
	if !m.included[cid] || m.generations[cid] != v.Generation || m.profileLookupGeneration != v.LookupGeneration {
		return m, nil
	}
	m.profileBusy = false
	if m.profileCancel != nil {
		m.profileCancel()
		m.profileCancel = nil
	}
	m.profileKey = ""
	if slack.IsAuth(v.Err) {
		m.stopReads()
		return m, nil
	}
	if errors.Is(v.Err, context.Canceled) {
		delete(m.profileAttempted, v.ID)
	} else {
		m.profileAttempted[v.ID] = true
	}
	m.syncProfiles()
	return m, m.profileCmd()
}
