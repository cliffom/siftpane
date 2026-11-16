package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

type participationTestService struct {
	core.Service
	target core.MessageTarget
	user   string
	calls  int
}

func (s *participationTestService) ToggleReaction(_ context.Context, target core.MessageTarget, _ core.QuickReaction, user string) (bool, error) {
	s.target, s.user = target, user
	s.calls++
	return true, nil
}

func TestReactionCapturesAccountAndTargetWhileSwitching(t *testing.T) {
	a, _ := accountFixture(t, 2)
	first := a.current()
	svc := &participationTestService{Service: first.service}
	first.service = svc
	cmd := a.wrap(0, a.sessions[0].epoch, first.toggleReaction(core.ReactionLooking))
	if first.toggleReaction(core.ReactionAcknowledged) != nil {
		t.Fatal("duplicate pending mutation")
	}
	switchAccount(t, a, 1)
	status := a.current().status
	runAccountCommand(t, a, cmd)
	if svc.calls != 1 || svc.user != first.identity.User || svc.target.Conversation != demoChannel || svc.target.TS != attentionTestTS {
		t.Fatal("reaction target changed")
	}
	if first.participationBusy || a.current().status != status || !strings.Contains(first.status, "Looking added") {
		t.Fatal("reaction result changed other account")
	}
}

func TestParticipationKeysAreOrdinaryComposingAndConsumedByHelp(t *testing.T) {
	m := fixture(t)
	for _, k := range []string{keyAcknowledged, keyLooking, keyResolved, keyOpenSlack} {
		m.focus = focusCompose
		m.composer.Focus()
		m.composer.Reset()
		m.Update(key(k))
		if m.composer.Value() != k {
			t.Fatal("participation hijacked composer", k)
		}
		m.help = true
		m.Update(key(k))
		if m.help || m.participationBusy {
			t.Fatal("help did not consume", k)
		}
	}
}

func TestReactionFailureRetainsDraftAndAttention(t *testing.T) {
	m := fixture(t)
	m.composer.SetValue(mainDraft)
	snapshotAttention(m)
	count := len(m.attention.Pending)
	m.Update(participationResult{reaction: core.ReactionResolved, err: errors.New("outcome unknown")})
	if m.composer.Value() != mainDraft || len(m.attention.Pending) != count || !strings.Contains(m.status, "inspect Slack") {
		t.Fatal("reaction discarded local state")
	}
}

func TestInvalidOpenSlackURLNeverLaunches(t *testing.T) {
	for _, link := range []string{"file:///tmp/x", "https://evil.example", "https://slack.com.evil.example", "slack://channel?id=C1", "https://user@example.slack.com/path", "https://example.slack.com:444/path"} {
		if launchSlackURL(context.Background(), link) == nil {
			t.Fatal("unsafe URL accepted", link)
		}
	}
}

func TestFormattedMessageCodeQuotesLists(t *testing.T) {
	body := "> quoted text\n- list item\n```go\n    literal <@USER>\n```\n`inline`"
	rows := formattedBodyRows(body, 40)
	rendered := ansi.Strip(strings.Join(rows, "\n"))
	for _, want := range []string{"│ quoted text", "• list item", "┌ code", "│ go", "│     literal <@USER>", "└", "`inline`"} {
		if !strings.Contains(rendered, want) {
			t.Fatal("format lost", want, rendered)
		}
	}
	if got := ansi.Strip(strings.Join(formattedBodyRows("```single line literal```", 40), "\n")); !strings.Contains(got, "│ single line literal") {
		t.Fatal("single line code lost", got)
	}
	// Held navigation keys cannot repeat a remote mutation.
	m := fixture(t)
	m.focus = focusMessages
	_, cmd := m.Update(tea.KeyPressMsg{Code: '1', Text: "1", IsRepeat: true})
	if cmd != nil || m.participationBusy {
		t.Fatal("repeat started reaction")
	}
}
