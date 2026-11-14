package slack

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
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
		if err := r.ParseForm(); err != nil {
			t.Errorf(parseRequestFailure, err)
			return
		}
		if r.Header.Get("Authorization") != "Bearer xoxp-test" {
			t.Error("missing auth")
		}
		if r.Form.Get("channel") != "C1" || r.Form.Get("cursor") != "page2" || r.Form.Get("limit") != "50" {
			t.Error(r.Form)
		}
		if r.URL.Path == "/conversations.replies" && (r.Form.Get("ts") != "10.1" || r.Form.Get("oldest") != "12.1") {
			t.Error(r.Form)
		}
		writeResponse(t, w, `{"ok":true,"messages":[{"ts":"14.1","text":"body"}],"has_more":true,"response_metadata":{"next_cursor":"page3"}}`)
	})
	for _, thread := range []string{"", "10.1"} {
		p, err := c.History(context.Background(), core.Query{Conversation: "C1", Thread: thread, Cursor: "page2", After: "12.1"})
		if err != nil || p.Next != "page3" || !p.More || len(p.Messages) != 1 {
			t.Fatal(p, err)
		}
	}
}

func TestDirectoryPassesCursorAndEmptyPage(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf(parseRequestFailure, err)
			return
		}
		if r.Form.Get("limit") != "30" || r.Form.Get("cursor") != "next" || r.Form.Get("types") != "public_channel,private_channel,im,mpim" {
			t.Error(r.Form)
		}
		writeResponse(t, w, `{"ok":true,"channels":[],"response_metadata":{"next_cursor":"still-more"}}`)
	})
	p, err := c.Directory(context.Background(), "next")
	if err != nil || p.Next != "still-more" {
		t.Fatal(p, err)
	}
}

func TestRateLimitBlocksMethodAndRespectsCancellation(t *testing.T) {
	calls := 0
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
	})
	err := c.call(context.Background(), "conversations.history", nil, &struct{}{})
	var a *APIError
	if !errors.As(err, &a) || a.RetryAfter != 2*time.Second || a.Ambiguous {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	err = c.call(ctx, "conversations.history", nil, &struct{}{})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatal(err, calls)
	}
	if time.Until(c.next["conversations.history"]) < time.Second {
		t.Fatal("Retry-After not retained")
	}
}

func TestSendFieldsAndAmbiguousFailure(t *testing.T) {
	n := 0
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		if err := r.ParseForm(); err != nil {
			t.Errorf(parseRequestFailure, err)
			return
		}
		if r.Form.Get("thread_ts") != "parent" || r.Form.Get("text") != outgoingBody || r.Form.Get("client_msg_id") != "stable" {
			t.Error(r.Form)
		}
		w.WriteHeader(503)
	})
	_, err := c.Send(context.Background(), core.Outgoing{Conversation: "C", Thread: "parent", Text: outgoingBody, ID: "stable"})
	if !core.UncertainDelivery(err) || n != 1 || strings.Contains(err.Error(), outgoingBody) {
		t.Fatal(err, n)
	}
}

func TestExpiredAuth(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{"ok":false,"error":"token_expired"}`)
	})
	_, err := c.Identity(context.Background())
	if !IsAuth(err) {
		t.Fatal(err)
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
		if err := r.ParseForm(); err != nil {
			t.Errorf(parseRequestFailure, err)
			return
		}
		switch r.URL.Path {
		case "/conversations.info":
			if r.Form.Get("include_num_members") != "true" {
				t.Error("member count not requested")
			}
			writeResponse(t, w, `{"ok":true,"channel":{"id":"G1","name":"crew","is_mpim":true,"num_members":3,"topic":{"value":"Launch review"},"purpose":{"value":"Review together"}}}`)
		case "/conversations.list":
			writeResponse(t, w, `{"ok":true,"channels":[{"id":"C1","name":"studio","topic":{"value":"Daily work"},"purpose":{"value":"Collaborate"},"num_members":4}]}`)
		default:
			t.Error("unexpected method", r.URL.Path)
		}
	})
	group, err := c.Conversation(context.Background(), "G1")
	if err != nil || group.Members != 3 || group.Topic != "Launch review" || group.Kind != core.ConversationGroup {
		t.Fatal(group, err)
	}
	page, err := c.Directory(context.Background(), "")
	if err != nil || len(page.Conversations) != 1 || page.Conversations[0].Topic != "Daily work" || page.Conversations[0].Purpose != "Collaborate" {
		t.Fatal(page, err)
	}
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}

const (
	parseRequestFailure = "parse request: %v"
	outgoingBody        = "keep body"
)

func TestPacingPolicyPreservesMethodIntervals(t *testing.T) {
	for _, tc := range []struct {
		method string
		gap    time.Duration
	}{
		{methodIdentity, 1300 * time.Millisecond},
		{methodHistory, 1300 * time.Millisecond},
		{methodReplies, 1300 * time.Millisecond},
		{methodSend, 1300 * time.Millisecond},
		{methodConversation, 1300 * time.Millisecond},
		{methodUser, 650 * time.Millisecond},
		{methodDirectory, 3100 * time.Millisecond},
	} {
		t.Run(tc.method, func(t *testing.T) {
			c := New("offline-test")
			if c.http.Timeout != 20*time.Second {
				t.Fatal("request timeout changed", c.http.Timeout)
			}
			before := time.Now()
			if err := c.wait(context.Background(), tc.method); err != nil {
				t.Fatal(err)
			}
			after := time.Now()
			deadline := c.next[tc.method]
			if deadline.Before(before.Add(tc.gap)) || deadline.After(after.Add(tc.gap)) {
				t.Fatalf("%s pacing deadline %v outside expected interval", tc.method, deadline)
			}
		})
	}
}

func TestSearchMetadataRetainsDisplayNameAndHandle(t *testing.T) {
	var userCalls atomic.Int32
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users.info":
			userCalls.Add(1)
			writeResponse(t, w, `{"ok":true,"user":{"name":"mchen","real_name":"Maya Chen","profile":{"display_name":"Maya at work"}}}`)
		case "/conversations.list":
			writeResponse(t, w, `{"ok":true,"channels":[{"id":"D_SEARCH","is_im":true,"user":"U_SEARCH"}]}`)
		case "/conversations.info":
			writeResponse(t, w, `{"ok":true,"channel":{"id":"D_SEARCH","is_im":true,"user":"U_SEARCH"}}`)
		default:
			t.Error("unexpected request", r.URL.Path)
		}
	})
	page, err := c.Directory(context.Background(), "")
	if err != nil || len(page.Conversations) != 1 {
		t.Fatal(page, err)
	}
	conversation, err := c.Conversation(context.Background(), "D_SEARCH")
	if err != nil || conversation.Name != "Maya at work" || conversation.Handle != "mchen" || conversation.Kind != core.ConversationDM || userCalls.Load() != 1 {
		t.Fatal("display name/handle lost or lookup was not cached", conversation, err, userCalls.Load())
	}
	for _, query := range []string{"@MCHEN", "@maya AT WORK"} {
		if got := core.Filter(page.Conversations, query, nil, false); len(got) != 1 || got[0].ID != conversation.ID {
			t.Fatal("directory metadata not searchable by available name/handle", query, got)
		}
	}
}
