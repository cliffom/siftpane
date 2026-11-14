package demo

import (
	"context"
	"strings"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

func TestDistinctConversationFixturesAndParticipants(t *testing.T) {
	s := New()
	seen := map[string]string{}
	for _, c := range s.conversations {
		p, err := s.History(context.Background(), core.Query{Conversation: c.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Messages) != 15 || !p.More {
			t.Fatal("missing older history", c.ID)
		}
		var body strings.Builder
		for _, m := range p.Messages {
			body.WriteString(m.Text)
			if c.Kind == core.ConversationDM && m.Author != "You" && m.Author != c.Name {
				t.Fatalf("unexpected DM participant: %s in %s", m.Author, c.Name)
			}
		}
		if other := seen[body.String()]; other != "" {
			t.Fatalf("duplicate conversation histories: %s, %s", other, c.ID)
		}
		seen[body.String()] = c.ID
		for _, m := range p.Messages {
			if m.Replies > 0 {
				thread, err := s.History(context.Background(), core.Query{Conversation: c.ID, Thread: m.TS})
				if err != nil {
					t.Fatal(err)
				}
				if len(thread.Messages) != 4 || thread.Messages[0].Text != m.Text {
					t.Fatal("thread doesn't match parent", c.ID)
				}
			}
		}
	}
}

func TestInvalidPaginationCursorsReturnErrors(t *testing.T) {
	for _, cursor := range []string{"invalid", "-1", "10000"} {
		t.Run(cursor, func(t *testing.T) {
			s := New()
			if _, err := s.Directory(context.Background(), cursor); err == nil {
				t.Fatal("directory accepted invalid cursor")
			}
			if _, err := s.History(context.Background(), core.Query{Conversation: "Cstudio", Cursor: cursor}); err == nil {
				t.Fatal("history accepted invalid cursor")
			}
		})
	}
}
