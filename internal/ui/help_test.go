package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

func TestSidebarActivitySlots(t *testing.T) {
	for _, name := range []string{"short", strings.Repeat("界", 30)} {
		for _, unread := range []bool{false, true} {
			for _, draft := range []bool{false, true} {
				m := fixture(t)
				c := core.Conversation{ID: m.active, Name: name, Kind: core.ConversationChannel}
				m.unread[c.ID] = unread
				if draft {
					m.drafts[c.ID] = mainDraft
				}
				for _, width := range []int{sidebarColumns, 58, 38} {
					row := ansi.Strip(m.conversationRow(c, width))
					tail := "    "
					if unread {
						tail = " ●  "
					}
					if draft {
						tail = tail[:len(tail)-1] + "✎"
					}
					if ansi.StringWidth(row) != width || !strings.HasSuffix(row, tail) || !strings.HasPrefix(row, "▸ ") {
						t.Fatalf("markers lost/misaligned: %q want suffix %q", row, tail)
					}
				}
			}
		}
	}
}

func TestHelpConsumesInputAndRestoresContext(t *testing.T) {
	for _, focus := range []focusPane{focusChannels, focusMessages, focusCompose} {
		for _, mode := range []pickerMode{pickerClosed, pickerSwitch, pickerInclude} {
			t.Run(fmt.Sprint(focus, mode), func(t *testing.T) {
				assertHelpRestoresContext(t, focus, mode)
			})
		}
	}
	m := fixture(t)
	m.focus = focusCompose
	m.composer.Focus()
	m.Update(key(keyHelp))
	if m.help || m.composer.Value() != keyHelp {
		t.Fatal("question mark no longer types normally")
	}
	m.focus = focusMessages
	m.Update(key(keyHelp))
	if !m.help {
		t.Fatal("navigation help missing")
	}
}

func TestHelpAllRowsReachableAndResize(t *testing.T) {
	m := fixture(t)
	m.help = true
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}, {100, 30}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		w, h := m.pickerSize()
		rows := helpRows(w)
		m.helpScroll = 0
		for i, row := range rows {
			m.helpScroll = i
			if !strings.Contains(ansi.Strip(m.helpView(w, h)), row) {
				t.Fatalf("unreachable guide row %q", row)
			}
		}
		m.helpKey(keyNewest)
		if !strings.Contains(ansi.Strip(m.helpView(w, h)), "docs/USAGE.md") {
			t.Fatal("last guide row unreachable")
		}
		m.helpKey(keyPageUp)
		if m.helpScroll >= len(rows)-(h-frameRows-helpFooterRows) {
			t.Fatal("page up did not move")
		}
		view := ansi.Strip(m.View().Content)
		assertLayout(t, m, view, size, SnapshotHelp)
		if !strings.Contains(view, "Esc close") {
			t.Fatal("minimum layout clips help dismissal", view)
		}
	}
}

func TestHelpColorIsolationAndRecovery(t *testing.T) {
	m := fixture(t)
	m.help = true
	m.quitConfirm = true
	for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		w, h := m.pickerSize()
		x, y := (size[0]-w)/2, (size[1]-h)/2
		canvas := lipgloss.NewCanvas(size[0], size[1]).Compose(lipgloss.NewLayer(m.View().Content))
		for _, sample := range []struct {
			x, y int
			want color.Color
		}{{1, 0, backdrop.GetForeground()}, {x, y, accent.GetForeground()}, {x + 2, y, focusedTitle.GetForeground()}} {
			cell := canvas.CellAt(sample.x, sample.y)
			if cell == nil || cell.Style.Fg == nil || color.RGBAModel.Convert(cell.Style.Fg) != color.RGBAModel.Convert(sample.want) {
				t.Fatal("help colors leaked", sample, cell)
			}
		}
		last := strings.Split(ansi.Strip(m.View().Content), "\n")[size[1]-1]
		if !strings.Contains(last, "Unsent text") {
			t.Fatal("help hid discard warning")
		}
	}
}

func TestQuietChromeRetainsDeliveryRecovery(t *testing.T) {
	for _, state := range []core.DeliveryState{core.DeliveryPending, core.DeliveryFailed} {
		for _, size := range [][2]int{{100, 30}, {60, 20}, {40, 14}} {
			t.Run(fmt.Sprint(state, size), func(t *testing.T) {
				m := fixture(t)
				m.focus = focusCompose
				m.composer.SetValue(mainDraft)
				m.deliveries[m.key()] = &core.Delivery{State: state, Outgoing: core.Outgoing{Text: mainDraft}}
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				view := ansi.Strip(m.View().Content)
				want := "sending…"
				if state == core.DeliveryFailed {
					want = "retry"
				}
				if !strings.Contains(view, want) || !strings.Contains(view, "text kept") && !strings.Contains(view, "Text kept") {
					t.Fatal("delivery recovery disappeared", view)
				}
				if strings.Contains(view, "UNREAD HERE") || strings.Contains(view, "focus ·") || !strings.Contains(view, "F1 help") {
					t.Fatal("chrome still noisy or help undiscoverable")
				}
			})
		}
	}
}

func assertHelpRestoresContext(t *testing.T, focus focusPane, mode pickerMode) {
	t.Helper()
	m := fixture(t)
	m.thread = "123.456"
	m.focus = focus
	m.composer.SetValue(mainDraft)
	m.rememberDraft()
	if focus == focusCompose {
		m.composer.Focus()
	}
	if mode != pickerClosed {
		m.openPicker(mode)
		m.picker.SetValue("help query")
	}
	active, thread, index, scroll := m.active, m.thread, m.index, m.scroll
	priorFocus := m.focus
	m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !m.help {
		t.Fatal("F1 did not open help")
	}
	m.Update(tea.PasteMsg{Content: "must not reach inputs"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if !m.help || m.helpScroll == 0 {
		t.Fatal("help did not scroll")
	}
	m.Update(key(keySend))
	if m.help || len(m.deliveries) != 0 {
		t.Fatal("dismiss key sent text")
	}
	if m.active != active || m.thread != thread || m.index != index || m.scroll != scroll || m.focus != priorFocus || m.mode != mode || m.composer.Value() != mainDraft {
		t.Fatal("help changed underlying context")
	}
	if mode != pickerClosed && m.picker.Value() != "help query" {
		t.Fatal("help paste altered search")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.help || m.quitConfirm {
		t.Fatal("help close key triggered quit")
	}
}

func TestOpeningHelpCancelsQuitAndRetryConfirmation(t *testing.T) {
	m := fixture(t)
	m.quitConfirm, m.retryConfirm = true, true
	m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !m.help || m.quitConfirm || m.retryConfirm {
		t.Fatal("help retained a destructive confirmation")
	}
}

func TestHelpResumesFocusedInputCursor(t *testing.T) {
	for _, mode := range []pickerMode{pickerClosed, pickerSwitch} {
		t.Run(string(mode), func(t *testing.T) {
			m := fixture(t)
			m.focus = focusCompose
			styles := m.composer.Styles()
			styles.Cursor.BlinkSpeed = time.Millisecond
			m.composer.SetStyles(styles)
			pickerStyles := m.picker.Styles()
			pickerStyles.Cursor.BlinkSpeed = time.Millisecond
			m.picker.SetStyles(pickerStyles)
			start := m.composer.Focus()
			if mode != pickerClosed {
				start = m.openPicker(mode)
			}
			// Consume a hidden-phase timer while help owns input, then dismiss it.
			_, next := m.Update(cursorMessage(t, start))
			m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
			m.Update(cursorMessage(t, next))
			_, resume := m.Update(key(keyEscape))
			if resume == nil {
				t.Fatal("help did not restart the focused cursor")
			}
			// Verify the resumed timer is accepted and schedules another blink.
			_, next = m.Update(cursorMessage(t, resume))
			if next == nil {
				t.Fatal("cursor did not continue blinking after dismissal")
			}
		})
	}
}

func cursorMessage(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing cursor command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				return cursorMessage(t, c)
			}
		}
	}
	if _, ok := msg.(cursor.BlinkMsg); !ok {
		t.Fatalf("expected blink, got %T", msg)
	}
	return msg
}
