package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const mentionTestToken = "offline-token"

const mentionTestTS = "1794672000.000001"

func TestMentionsExactIdentityPaginationAndThreadTarget(t *testing.T) {
	at, err := core.MessageTime(mentionTestTS)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/search.messages" || r.Form.Get("sort") != "timestamp" || r.Form.Get("sort_dir") != "asc" || r.Form.Get("count") != "100" || r.Form.Get("page") != "1" {
			t.Error("incorrect search query", r.Form)
		}
		response := map[string]any{"ok": true, "messages": map[string]any{"paging": map[string]int{"count": 100, "total": 101, "page": 1, "pages": 2}, "matches": []any{
			map[string]any{"ts": mentionTestTS, paramText: "hello <@person>", "permalink": "https://test.slack.com/archives/test/p1794672000000001?thread_ts=1794671000.000001", paramChannel: map[string]any{"id": "outside-kept", "name": "test", "is_private": true}},
			map[string]any{"ts": mentionTestTS, paramText: "hello <@other>", paramChannel: map[string]any{"id": "chatter"}},
			map[string]any{"ts": "1794000000.000001", paramText: "<@person>", paramChannel: map[string]any{"id": "too-old"}},
		}}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := New(mentionTestToken)
	c.base = server.URL + "/"
	page, err := c.Mentions(context.Background(), core.MentionQuery{User: "person", From: at.Add(-time.Hour), Until: at.Add(time.Hour), Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !page.More || len(page.Matches) != 1 {
		t.Fatal("lost paging or admitted chatter/out-of-window activity", page)
	}
	match := page.Matches[0]
	if match.Conversation.ID != "outside-kept" || match.Conversation.Kind != core.ConversationPrivate || match.Trigger.TS != mentionTestTS || match.Trigger.Thread != "1794671000.000001" {
		t.Fatal("exact target lost", match)
	}
}

func TestMentionsRejectsIncompletePagingAndInvalidTargets(t *testing.T) {
	at, err := core.MessageTime(mentionTestTS)
	if err != nil {
		t.Fatal(err)
	}
	for _, response := range []string{
		`{"ok":true,"messages":{"matches":[]}}`,
		`{"ok":false,"error":"missing_scope"}`,
		`{"ok":true,"messages":{"paging":{"count":100,"total":1,"page":1,"pages":1},"matches":[{"ts":"invalid","text":"<@person>","channel":{"id":"test"}}]}}`,
	} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(response)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			c := New(mentionTestToken)
			c.base = server.URL + "/"
			if _, err := c.Mentions(context.Background(), core.MentionQuery{User: "person", From: at.Add(-time.Hour), Until: at.Add(time.Hour), Page: 1}); err == nil {
				t.Fatal("incomplete discovery accepted as successful")
			}
		})
	}
}

func TestHistoryOpeningTargetParameters(t *testing.T) {
	for _, thread := range []string{"", "1794671000.000001"} {
		t.Run(thread, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				bound := "latest"
				if thread != "" {
					bound = "oldest"
				}
				if r.Form.Get(bound) != mentionTestTS || r.Form.Get("inclusive") != "true" {
					t.Error("opening skipped trigger", r.Form)
				}
				if _, err := w.Write([]byte(`{"ok":true,"messages":[]}`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			c := New(mentionTestToken)
			c.base = server.URL + "/"
			if _, err := c.History(context.Background(), core.Query{Conversation: "test", Thread: thread, Target: mentionTestTS}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
