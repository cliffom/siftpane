package ui

import (
	"context"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

// SnapshotMode is an offline review identifier accepted by the CLI.
type SnapshotMode string

// Snapshot modes are independent of the UI's navigation state.
const (
	SnapshotMessages SnapshotMode = "messages"
	SnapshotCompose  SnapshotMode = "compose"
	SnapshotPicker   SnapshotMode = "picker"
	SnapshotSearch   SnapshotMode = "search"
	SnapshotThread   SnapshotMode = "thread"
	SnapshotHelp     SnapshotMode = "help"
)

// LoadSnapshot loads offline review data through the normal update path.
func LoadSnapshot(ctx context.Context, m *Model, s core.Service) error {
	cursor := ""
	for {
		p, err := s.Directory(ctx, cursor)
		if err != nil {
			return err
		}
		m.Update(directoryResult{Page: p})
		cursor = p.Next
		if cursor == "" {
			break
		}
	}
	p, err := s.History(ctx, core.Query{Conversation: m.active})
	if err != nil {
		return err
	}
	m.line(m.active)
	m.Update(historyResult{Key: m.active, Generation: m.generations[m.active], Page: p})
	return nil
}

// Snapshot renders a deterministic review state through the normal view path.
func Snapshot(m *Model, width, height int, mode SnapshotMode) string {
	m.width = width
	m.height = height
	m.resize()
	m.focus = focusMessages
	switch mode {
	case SnapshotPicker:
		m.openPicker(pickerInclude)
	case SnapshotSearch:
		m.openPicker(pickerSwitch)
		m.picker.SetValue("@leo")
	case SnapshotCompose:
		m.focus = focusCompose
		m.composer.SetValue("Thanks, Maya. I'll review the keyboard flow this afternoon.\nThe small details are coming together.")
		m.composer.Focus()
	case SnapshotHelp:
		m.help = true
	case SnapshotThread:
		t := m.cache[m.active]
		if t != nil {
			for _, msg := range t.Messages {
				if msg.Replies > 0 {
					m.thread = msg.TS
					p, err := m.service.History(m.ctx, core.Query{Conversation: m.active, Thread: m.thread})
					if err != nil {
						m.status = err.Error()
						break
					}
					m.cache[m.key()] = &timeline{Messages: core.Merge(nil, p.Messages, cacheLimit), Loaded: true}
					break
				}
			}
		}
	}
	return ansi.Strip(m.View().Content)
}
