package demo

import (
	"context"
	"github.com/cliffom/siftpane/internal/core"
	"strings"
	"testing"
)

func TestDistinctConversationFixturesAndParticipants(t *testing.T) {
	s := New()
	seen := map[string]string{}
	for _, c := range s.conversations {
		p, e := s.History(context.Background(), core.Query{Conversation: c.ID})
		if e != nil {
			t.Fatal(e)
		}
		if len(p.Messages) != 15 || !p.More {
			t.Fatal("missing older history", c.ID)
		}
		var body strings.Builder
		for _, m := range p.Messages {
			body.WriteString(m.Text)
			if c.Kind == "dm" && m.Author != "You" && m.Author != c.Name {
				t.Fatalf("unexpected DM participant: %s in %s", m.Author, c.Name)
			}
		}
		if other := seen[body.String()]; other != "" {
			t.Fatalf("duplicate conversation histories: %s, %s", other, c.ID)
		}
		seen[body.String()] = c.ID
		for _, m := range p.Messages {
			if m.Replies > 0 {
				thread, _ := s.History(context.Background(), core.Query{Conversation: c.ID, Thread: m.TS})
				if len(thread.Messages) != 4 || thread.Messages[0].Text != m.Text {
					t.Fatal("thread doesn't match parent", c.ID)
				}
			}
		}
	}
}
