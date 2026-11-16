package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	delayedPasteText = "original account clipboard text"
	widgetReturned   = "returned"
)

type opaqueWidgetResult struct{ text string }

func TestAccountWidgetCompletionsRetainOwnership(t *testing.T) {
	for _, scenario := range []string{"active", "inactive", widgetReturned, "selector"} {
		for _, picker := range []bool{false, true} {
			t.Run(fmt.Sprint(scenario, picker), func(t *testing.T) {
				checkWidgetCompletion(t, scenario, picker)
			})
		}
	}
}

func TestOpaqueWidgetResultsAndTerminalCommands(t *testing.T) {
	a, _ := accountFixture(t, 1)
	payload := opaqueWidgetResult{text: delayedPasteText}
	msg := a.wrap(0, a.sessions[0].epoch, func() tea.Msg { return payload })()
	event, ok := msg.(accountEvent)
	if !ok || event.message != payload {
		t.Fatalf("private widget result escaped ownership: %T", msg)
	}
	_, cmd := a.Update(key(keyCopy))
	if cmd == nil {
		t.Fatal("missing clipboard terminal command")
	}
	if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tea.SetClipboard("")()); got != want {
		t.Fatal("terminal clipboard command was hidden from Bubble Tea", got, want)
	}
	_, cmd = a.Update(key(keyQuitQuiet))
	if cmd == nil {
		t.Fatal("missing explicit quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit was hidden from Bubble Tea")
	}
}

func TestAccountSelectorCancelsEnablementConfirmations(t *testing.T) {
	for _, action := range []tea.KeyPressMsg{key(keyMentions), key(keyChannel), key(keyHere), {Code: 'b', Mod: tea.ModCtrl}} {
		for _, closeKey := range []tea.KeyPressMsg{key(keyEscape), {Code: 'w', Mod: tea.ModCtrl}, {Code: tea.KeyEnter}} {
			t.Run(action.String()+closeKey.String(), func(t *testing.T) {
				a, _ := accountFixture(t, 1)
				m := a.current()
				a.Update(action)
				if !m.mentionConfirm && m.broadcastConfirm == "" && !m.cacheConfirm {
					t.Fatal("first press did not disclose")
				}
				a.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
				a.Update(closeKey)
				a.Update(action)
				if m.attention.SearchEnabled() || m.attention.MessageCache {
					t.Fatal("selector bypassed consecutive confirmation")
				}
				a.Update(action)
				if !m.attention.SearchEnabled() && !m.attention.MessageCache {
					t.Fatal("new consecutive presses did not confirm")
				}
			})
		}
	}
}

func TestAccountCoordinatorInterruptsUncertainRetry(t *testing.T) {
	for _, interrupt := range []tea.KeyPressMsg{{Code: 'w', Mod: tea.ModCtrl}, key(keyQuitQuiet), {Code: 'c', Mod: tea.ModCtrl}} {
		t.Run(interrupt.String(), func(t *testing.T) {
			a, services := accountFixture(t, 1)
			m := a.current()
			m.deliveries[m.key()] = &core.Delivery{Outgoing: core.Outgoing{Conversation: m.active, Text: mainDraft}, State: core.DeliveryFailed, Uncertain: true}
			a.Update(key(keyRetry))
			if !m.retryConfirm {
				t.Fatal("retry did not warn")
			}
			a.Update(interrupt)
			if a.choosing {
				a.Update(key(keyEscape))
			}
			_, cmd := a.Update(key(keyRetry))
			if cmd != nil || !m.retryConfirm || services[0].sends != 0 || m.deliveries[m.key()].State != core.DeliveryFailed {
				t.Fatal("intervening coordinator key bypassed retry warning")
			}
		})
	}
}

func checkWidgetCompletion(t *testing.T, scenario string, picker bool) {
	t.Helper()
	a, _ := accountFixture(t, 2)
	first := a.current()
	first.focus = focusCompose
	first.composer.Focus()
	if picker {
		first.openPicker(pickerSwitch)
	}
	// Model widgets accept this public paste payload through the same update
	// path as their private asynchronous clipboard result types.
	cmd := a.wrap(0, a.sessions[0].epoch, func() tea.Msg { return tea.PasteMsg{Content: delayedPasteText} })
	switch scenario {
	case "inactive", widgetReturned:
		switchAccount(t, a, 1)
		a.current().focus = focusCompose
		a.current().composer.Focus()
		if scenario == widgetReturned {
			switchAccount(t, a, 0)
		}
	case "selector":
		a.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	}
	a.Update(cmd())
	value := first.composer.Value()
	if picker {
		value = first.picker.Value()
	}
	want := ""
	if scenario == "active" {
		want = delayedPasteText
	}
	if value != want {
		t.Fatal("widget result violated original session/epoch", value, want)
	}
	if second := a.sessions[1].model; second != nil && strings.Contains(second.composer.Value(), delayedPasteText) {
		t.Fatal("paste crossed accounts")
	}
}
