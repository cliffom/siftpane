package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

func TestCredentialReloadPreservesDraftsAndOriginalPendingSend(t *testing.T) {
	a, services := accountFixture(t, 2)
	m := a.current()
	m.composer.SetValue(mainDraft)
	m.rememberDraft()
	pending := a.wrap(0, a.sessions[0].epoch, m.send(false))
	oldRead := a.wrap(0, a.sessions[0].epoch, m.conversationCmd(m.active))
	identity := m.identity
	replacement := &accountTestService{Service: services[0].Service}
	calls := 0
	a.sessions[0].account.Reload = func(context.Context) (core.Service, core.Identity, error) {
		calls++
		return replacement, identity, nil
	}
	cmd := a.reloadCredential()
	if a.reloadCredential() != nil {
		t.Fatal("duplicate credential check allowed")
	}
	m.focus, m.scroll = focusMessages, -1
	before := m.line(m.key()).Messages[m.index].TS
	runAccountCommand(t, a, cmd)
	if calls != 1 || m.service != replacement || m.composer.Value() != mainDraft || m.line(m.key()).Messages[m.index].TS != before || m.scroll != -1 {
		t.Fatal("reload changed draft/reading identity")
	}
	if oldRead != nil {
		runAccountCommand(t, a, oldRead)
	}
	runAccountCommand(t, a, pending)
	if services[0].sends != 1 || replacement.sends != 0 || m.deliveries[m.key()].State != core.DeliverySent {
		t.Fatal("send did not finish through its original service")
	}
}

func TestCredentialReloadCompletionAfterSwitchAndLateAuthFailure(t *testing.T) {
	a, services := accountFixture(t, 2)
	m := a.current()
	identity := m.identity
	replacement := &accountTestService{Service: services[0].Service}
	a.sessions[0].account.Reload = func(context.Context) (core.Service, core.Identity, error) {
		return replacement, identity, nil
	}
	m.composer.SetValue(mainDraft)
	m.rememberDraft()
	m.deliveries[m.key()] = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Text: mainDraft}, State: core.DeliveryPending}
	cmd := a.reloadCredential()
	switchAccount(t, a, 1)
	runAccountCommand(t, a, cmd)
	if replacement.reads != 0 || a.current() == m || a.current().service == replacement {
		t.Fatal("inactive reload issued reads or crossed accounts")
	}
	a.Update(accountEvent{index: 0, message: sendResult{Key: m.key(), Err: &slack.APIError{Code: "account_inactive"}}})
	if m.stopped || m.deliveries[m.key()].State != core.DeliveryFailed || m.composer.Value() != mainDraft {
		t.Fatal("late old-token failure stopped replacement or lost body")
	}
	switchAccount(t, a, 0)
	if m.service != replacement || replacement.sends != 0 || m.composer.Value() != mainDraft {
		t.Fatal("switch back lost replacement or resent delivery")
	}
}

func TestCredentialReloadFailureAndWrongIdentityRetainStoppedAccount(t *testing.T) {
	for _, failure := range []string{"store", "identity"} {
		t.Run(failure, func(t *testing.T) {
			a, services := accountFixture(t, 2)
			m := a.current()
			m.composer.SetValue(mainDraft)
			m.stopReads()
			a.sessions[0].account.Reload = func(context.Context) (core.Service, core.Identity, error) {
				if failure == "store" {
					return nil, core.Identity{}, errors.New("credential store locked")
				}
				return services[1], a.sessions[1].account.Identity, nil
			}
			runAccountCommand(t, a, a.reloadCredential())
			if !m.stopped || m.service != services[0] || m.composer.Value() != mainDraft || a.sessions[0].reloading {
				t.Fatal("failed replacement changed original state")
			}
			switchAccount(t, a, 1)
			if a.current().stopped {
				t.Fatal("replacement failure stopped healthy account")
			}
		})
	}
}

func TestCredentialKeyConsumedByInputsAndModals(t *testing.T) {
	a, services := accountFixture(t, 1)
	m := a.current()
	calls := 0
	a.sessions[0].account.Reload = func(context.Context) (core.Service, core.Identity, error) {
		calls++
		return services[0], m.identity, nil
	}
	for _, state := range []string{"compose", "picker", "help", "details", "workspace-modal", "repeat"} {
		m.focus, m.mode, m.help, m.refreshDetails, a.choosing = focusMessages, pickerClosed, false, false, false
		switch state {
		case "compose":
			m.focus = focusCompose
		case "picker":
			m.openPicker(pickerSwitch)
		case "help":
			m.help = true
		case "details":
			m.refreshDetails = true
		case "workspace-modal":
			a.choosing = true
		}
		m.composer.SetValue(mainDraft)
		_, cmd := a.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl, IsRepeat: state == "repeat"}))
		runAccountCommand(t, a, cmd)
		if calls != 0 || !strings.Contains(m.composer.Value(), mainDraft) {
			t.Fatal("modal/input/repeat triggered credential reload", state)
		}
	}
}
