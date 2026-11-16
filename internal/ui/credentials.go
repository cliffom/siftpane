package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

type credentialResult struct {
	index    int
	service  core.Service
	identity core.Identity
	err      error
}

func (a *Accounts) reloadCredential() tea.Cmd {
	s := &a.sessions[a.active]
	if s.reloading {
		return nil
	}
	if s.account.Reload == nil {
		s.model.setStatus("Credential reload unavailable in this session", statusAction)
		return nil
	}
	s.reloading = true
	s.model.setStatus("Checking saved credential · drafts retained", statusAction)
	index, reload, ctx := a.active, s.account.Reload, a.ctx
	return func() tea.Msg {
		service, id, err := reload(ctx)
		return credentialResult{index: index, service: service, identity: id, err: err}
	}
}

func (a *Accounts) updateCredential(v credentialResult) tea.Cmd {
	s := &a.sessions[v.index]
	s.reloading = false
	m := s.model
	if v.err != nil {
		m.setStatus("Credential reload failed: "+v.err.Error(), statusAction)
		return nil
	}
	if v.service == nil || v.identity.Workspace != s.account.Identity.Workspace || v.identity.User != s.account.Identity.User {
		m.setStatus("Credential belongs to another account; original session retained", statusAction)
		return nil
	}
	if v.index == a.active {
		a.deactivate()
	}
	s.account.Service, s.account.Identity = v.service, v.identity
	m.service, m.identity, m.stopped = v.service, v.identity, false
	m.serviceGeneration++
	m.setStatus("Credential reloaded · drafts retained · sends require explicit retry", statusAction)
	if v.index == a.active {
		return a.activate(v.index)
	}
	return nil
}
