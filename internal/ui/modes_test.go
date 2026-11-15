package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNavigationModesRetainDraft(t *testing.T) {
	m := fixture(t)
	m.focus = focusChannels
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focus != focusMessages {
		t.Fatal("Tab did not focus messages")
	}
	m.Update(key(keyWrite))
	m.composer.SetValue("draft across picker")
	if m.focus != focusCompose {
		t.Fatal("write did not focus composer")
	}
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.mode != pickerSwitch || m.drafts[m.key()] != m.composer.Value() {
		t.Fatal("switch picker lost draft or mode")
	}
	m.Update(key(keyEscape))
	if m.mode != pickerClosed || m.focus != focusCompose {
		t.Fatal("picker close changed prior focus")
	}
	m.Update(key(keyEscape))
	m.Update(key(keyInclude))
	if m.mode != pickerInclude || m.composer.Value() != "draft across picker" {
		t.Fatal("inclusion picker changed draft or mode")
	}
}

func TestSelectionLimitUsesSameValueInStatus(t *testing.T) {
	m := fixture(t)
	for len(m.included) < selectionLimit {
		cid := fmt.Sprint("extra", len(m.included))
		m.included[cid] = true
		m.attention.Kept[cid] = true
	}
	m.toggle("overflow")
	if m.included["overflow"] || !strings.Contains(m.status, fmt.Sprint(selectionLimit)) {
		t.Fatal("selection limit or explanatory status changed", m.status)
	}
}
