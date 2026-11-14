package ui

import (
	"context"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
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
func Snapshot(m *Model, width, height int, mode string) string {
	m.width = width
	m.height = height
	m.resize()
	m.focus = "messages"
	switch mode {
	case "picker":
		m.mode = "pick"
	case "compose":
		m.focus = "compose"
		m.composer.SetValue("Thanks, Maya. I'll review the keyboard flow this afternoon.\nThe small details are coming together.")
		m.composer.Focus()
	case "help":
		m.help = true
	case "thread":
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
