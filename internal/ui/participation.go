package ui

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
)

type reactionService interface {
	ToggleReaction(context.Context, core.MessageTarget, core.QuickReaction, string) (bool, error)
}
type permalinkService interface {
	Permalink(context.Context, core.MessageTarget) (string, error)
}

type participationResult struct {
	reaction core.QuickReaction
	present  bool
	err      error
}

func (m *Model) messageTarget() (core.MessageTarget, bool) {
	line := m.cache[m.key()]
	if line == nil || m.index < 0 || m.index >= len(line.Messages) {
		return core.MessageTarget{}, false
	}
	return core.MessageTarget{Conversation: m.active, TS: line.Messages[m.index].TS}, true
}

func (m *Model) toggleReaction(reaction core.QuickReaction) tea.Cmd {
	svc, ok := m.service.(reactionService)
	if !ok {
		m.setStatus("Quick reactions unavailable for this service", statusAction)
		return nil
	}
	target, ok := m.messageTarget()
	if !ok || m.participationBusy {
		return nil
	}
	m.participationBusy = true
	m.setStatus(reaction.Label()+" · updating your reaction…", statusAction)
	ctx, user := m.sendCtx, m.identity.User
	return func() tea.Msg {
		present, err := svc.ToggleReaction(ctx, target, reaction, user)
		return participationResult{reaction: reaction, present: present, err: err}
	}
}

func (m *Model) updateParticipation(v participationResult) (tea.Model, tea.Cmd) {
	m.participationBusy = false
	if v.err != nil {
		m.setStatus("Reaction failed · inspect Slack before toggling again: "+v.err.Error(), statusAction)
		if slack.IsAuth(v.err) {
			m.stopReads()
		}
		return m, nil
	}
	action := "added"
	if !v.present {
		action = "removed"
	}
	m.setStatus(v.reaction.Label()+" "+action+" · local attention unchanged", statusAction)
	return m, nil
}

type openSlackResult struct{ err error }

func (m *Model) openSlack() tea.Cmd {
	svc, ok := m.service.(permalinkService)
	if !ok {
		m.setStatus("Open in Slack unavailable for this service", statusAction)
		return nil
	}
	target, ok := m.messageTarget()
	if !ok {
		return nil
	}
	ctx := m.sendCtx
	return func() tea.Msg {
		link, err := svc.Permalink(ctx, target)
		if err == nil {
			err = launchSlackURL(ctx, link)
		}
		return openSlackResult{err: err}
	}
}

// Launch only Slack HTTPS permalinks, as argv without a shell. Never launch
// untrusted message links or arbitrary schemes returned in message content.
func launchSlackURL(ctx context.Context, link string) error {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !strings.HasSuffix(u.Hostname(), ".slack.com") {
		return fmt.Errorf("invalid Slack permalink")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", link)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", link)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", link)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not open Slack; check your system browser")
	}
	return nil
}

func quickReaction(key string) core.QuickReaction {
	switch key {
	case keyAcknowledged:
		return core.ReactionAcknowledged
	case keyLooking:
		return core.ReactionLooking
	case keyResolved:
		return core.ReactionResolved
	}
	return ""
}

func quickReactionHelp() string {
	labels := []string{}
	for _, key := range []string{keyAcknowledged, keyLooking, keyResolved} {
		reaction := quickReaction(key)
		labels = append(labels, keyLabel(key)+" "+reaction.Label()+" ("+string(reaction)+")")
	}
	return strings.Join(labels, " / ") + ": toggle your reaction; never dismiss"
}
