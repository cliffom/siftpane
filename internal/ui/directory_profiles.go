package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

type directoryProfileResult struct {
	Profile    core.Profile
	Generation uint64
	Err        error
}

// Directory pages never wait for users.info. Names arrive independently through
// one bounded, cancelable lookup stream while the conversation picker is open.
func (m *Model) directoryProfileCmd() tea.Cmd {
	if _, recognized, _ := m.pickerTarget(); recognized {
		return nil
	}
	svc, ok := m.service.(profileService)
	if !ok || m.mode == pickerClosed || m.stopped || m.directoryProfileBusy || strings.HasPrefix(strings.TrimSpace(m.picker.Value()), "#") {
		return nil
	}
	if m.directoryProfileAttempted == nil {
		m.directoryProfileAttempted = make(map[string]bool)
	}
	for _, c := range m.directory {
		if (!unresolvedDirectoryName(c) && !m.directoryProfilePending[c.User]) || m.directoryProfileAttempted[c.User] {
			continue
		}
		if len(m.directoryProfileAttempted) >= profileLookupLimit {
			return nil
		}
		m.directoryProfileAttempted[c.User] = true
		ctx, cancel := context.WithCancel(m.ctx)
		m.directoryProfileCancel = cancel
		m.directoryProfileBusy = true
		m.directoryProfileUser = c.User
		generation, user := m.directoryGeneration, c.User
		return func() tea.Msg {
			defer cancel()
			if err := ctx.Err(); err != nil {
				return directoryProfileResult{Generation: generation, Err: err}
			}
			p, err := svc.Profile(ctx, user)
			return directoryProfileResult{Profile: p, Generation: generation, Err: err}
		}
	}
	return nil
}

func unresolvedDirectoryName(c core.Conversation) bool {
	return c.Kind == core.ConversationDM && c.User != "" && (c.Name == "" || c.Name == c.User)
}

func (m *Model) directoryNamesPending() bool {
	for _, c := range m.directory {
		if unresolvedDirectoryName(c) || m.directoryProfilePending[c.User] {
			return true
		}
	}
	return false
}

func (m *Model) updateDirectoryProfile(v directoryProfileResult) (tea.Model, tea.Cmd) {
	if m.stopped || v.Generation != m.directoryGeneration {
		return m, nil
	}
	m.directoryProfileBusy = false
	m.directoryProfileUser = ""
	m.directoryProfileCancel = nil
	if slack.IsAuth(v.Err) {
		m.stopReads()
		return m, nil
	}
	picked := m.pickedID()
	if v.Err == nil && v.Profile.Name != "" {
		delete(m.directoryProfilePending, v.Profile.ID)
		updated := []core.Conversation{}
		for i, c := range m.directory {
			if c.Kind == core.ConversationDM && c.User == v.Profile.ID {
				m.directory[i].Name = v.Profile.Name
				m.directory[i].Handle = v.Profile.Handle
				updated = append(updated, m.directory[i])
			}
		}
		m.sortDirectory()
		m.reselectChoice(picked)
		m.syncProfiles()
		m.saveMetadata()
		m.saveDirectory(updated)
	}
	return m, m.directoryProfileCmd()
}
