package slack

import (
	"context"
	"errors"
	"github.com/cliffom/siftpane/internal/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func clientFor(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c := New("xoxp-test")
	c.base = s.URL + "/"
	c.http = s.Client()
	return c
}
func TestHistoryPaginationAndThreadParams(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Header.Get("Authorization") != "Bearer xoxp-test" {
			t.Error("missing auth")
		}
		if r.Form.Get("channel") != "C1" || r.Form.Get("cursor") != "page2" || r.Form.Get("limit") != "50" {
			t.Error(r.Form)
		}
		if r.URL.Path == "/conversations.replies" && (r.Form.Get("ts") != "10.1" || r.Form.Get("oldest") != "12.1") {
			t.Error(r.Form)
		}
		w.Write([]byte(`{"ok":true,"messages":[{"ts":"14.1","text":"body"}],"has_more":true,"response_metadata":{"next_cursor":"page3"}}`))
	})
	for _, thread := range []string{"", "10.1"} {
		p, e := c.History(context.Background(), core.Query{Conversation: "C1", Thread: thread, Cursor: "page2", After: "12.1"})
		if e != nil || p.Next != "page3" || !p.More || len(p.Messages) != 1 {
			t.Fatal(p, e)
		}
	}
}
func TestDirectoryPassesCursorAndEmptyPage(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("cursor") != "next" || r.Form.Get("types") != "public_channel,private_channel,im,mpim" {
			t.Error(r.Form)
		}
		w.Write([]byte(`{"ok":true,"channels":[],"response_metadata":{"next_cursor":"still-more"}}`))
	})
	p, e := c.Directory(context.Background(), "next")
	if e != nil || p.Next != "still-more" {
		t.Fatal(p, e)
	}
}
func TestRateLimitBlocksMethodAndRespectsCancellation(t *testing.T) {
	calls := 0
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
	})
	e := c.call(context.Background(), "conversations.history", nil, &struct{}{})
	var a *APIError
	if !errors.As(e, &a) || a.RetryAfter != 2*time.Second || a.Ambiguous {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	e = c.call(ctx, "conversations.history", nil, &struct{}{})
	if !errors.Is(e, context.DeadlineExceeded) || calls != 1 {
		t.Fatal(e, calls)
	}
	if time.Until(c.next["conversations.history"]) < time.Second {
		t.Fatal("Retry-After not retained")
	}
}
func TestSendFieldsAndAmbiguousFailure(t *testing.T) {
	n := 0
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		r.ParseForm()
		if r.Form.Get("thread_ts") != "parent" || r.Form.Get("text") != "keep body" || r.Form.Get("client_msg_id") != "stable" {
			t.Error(r.Form)
		}
		w.WriteHeader(503)
	})
	_, e := c.Send(context.Background(), core.Outgoing{Conversation: "C", Thread: "parent", Text: "keep body", ID: "stable"})
	if !Uncertain(e) || n != 1 || strings.Contains(e.Error(), "keep body") {
		t.Fatal(e, n)
	}
}
func TestExpiredAuth(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":false,"error":"token_expired"}`)) })
	_, e := c.Identity(context.Background())
	if !IsAuth(e) {
		t.Fatal(e)
	}
}
func TestReadableAndSafe(t *testing.T) {
	got := Readable("<@U1> <#C1|studio> <https://example.com|notes> &amp; \x1b]52;c;bad\a\u202e")
	if !strings.Contains(got, "#studio") || !strings.Contains(got, "notes (https://example.com)") || strings.ContainsAny(got, "\x1b\a\u202e") {
		t.Fatal(got)
	}
	if !strings.Contains(Body(core.Message{Blocks: []map[string]any{{"type": "image"}}, Link: "slack://x"}), "slack://x") {
		t.Fatal("missing fallback link")
	}
}

func TestConversationMetadata(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.URL.Path {
		case "/conversations.info":
			if r.Form.Get("include_num_members") != "true" {
				t.Error("member count not requested")
			}
			w.Write([]byte(`{"ok":true,"channel":{"id":"G1","name":"crew","is_mpim":true,"num_members":3,"topic":{"value":"Launch review"},"purpose":{"value":"Review together"}}}`))
		case "/conversations.list":
			w.Write([]byte(`{"ok":true,"channels":[{"id":"C1","name":"studio","topic":{"value":"Daily work"},"purpose":{"value":"Collaborate"},"num_members":4}]}`))
		default:
			t.Error("unexpected method", r.URL.Path)
		}
	})
	group, e := c.Conversation(context.Background(), "G1")
	if e != nil || group.Members != 3 || group.Topic != "Launch review" || group.Kind != "group" {
		t.Fatal(group, e)
	}
	page, e := c.Directory(context.Background(), "")
	if e != nil || len(page.Conversations) != 1 || page.Conversations[0].Topic != "Daily work" || page.Conversations[0].Purpose != "Collaborate" {
		t.Fatal(page, e)
	}
}
