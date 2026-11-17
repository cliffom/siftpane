package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

func TestKeptShortcutRemovesSelectedAndIgnoresRepeat(t *testing.T) {
	m := fixture(t)
	cid := m.active
	m.focus = focusMessages
	m.Update(tea.KeyPressMsg{Code: 'K', Text: keyKeep, IsRepeat: true})
	if !m.attention.Kept[cid] {
		t.Fatal("held key unkept conversation")
	}
	m.Update(key(keyKeep))
	if m.attention.Kept[cid] || m.included[cid] || m.active != "" || m.focus != focusChannels || !strings.Contains(m.status, "Unkept") {
		t.Fatal("shortcut did not remove selected Kept conversation safely")
	}
}

func TestKeptShortcutRetainsAttentionAndDraft(t *testing.T) {
	m := fixture(t)
	cid := m.active
	trigger := core.AttentionTrigger{ID: "manual-keyboard", Reason: core.AttentionManual, At: time.Now().UTC()}
	m.attention.Pending[cid] = []core.AttentionTrigger{trigger}
	m.composer.SetValue("draft retained in attention")
	m.Update(key(keyKeep))
	if m.attention.Kept[cid] || !m.included[cid] || m.active != cid || len(m.attention.Pending[cid]) != 1 || m.composer.Value() != "draft retained in attention" {
		t.Fatal("unkeep dismissed attention or hid draft")
	}
	m.Update(key(keyKeep))
	if !m.attention.Kept[cid] || len(m.attention.Pending[cid]) != 1 {
		t.Fatal("shortcut did not keep again independently of attention")
	}
}

func TestKeptShortcutDoesNotHideRecoverableText(t *testing.T) {
	for _, recovery := range []string{"conversation draft", "draft in thread", "pending send", "failed send"} {
		t.Run(recovery, func(t *testing.T) {
			m := fixture(t)
			cid := m.active
			switch recovery {
			case "conversation draft":
				m.composer.SetValue(mainDraft)
			case "draft in thread":
				m.drafts[cid+"/thread"] = mainDraft
			case "pending send":
				m.deliveries[cid+"/thread"] = &core.Delivery{State: core.DeliveryPending}
			case "failed send":
				m.deliveries[cid+"/thread"] = &core.Delivery{State: core.DeliveryFailed}
			}
			m.Update(key(keyKeep))
			if !m.attention.Kept[cid] || !m.included[cid] || m.active != cid || !strings.Contains(m.status, "recover") {
				t.Fatal("unkeep hid recoverable text", recovery)
			}
		})
	}
}

func TestKeptShortcutIsTextWhileComposing(t *testing.T) {
	m := fixture(t)
	m.focus = focusCompose
	m.composer.Focus()
	cid := m.active
	m.Update(tea.KeyPressMsg{Code: 'K', Text: keyKeep, IsRepeat: true})
	if !m.attention.Kept[cid] || m.composer.Value() != keyKeep {
		t.Fatal("composing K toggled Kept instead of entering text")
	}
}
