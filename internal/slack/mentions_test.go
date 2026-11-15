package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if r.URL.Path != "/search.messages" || r.Form.Get("sort") != "timestamp" || r.Form.Get("sort_dir") != "asc" || r.Form.Get(paramCount) != "100" || r.Form.Get(paramPage) != "1" {
			t.Error("incorrect search query", r.Form)
		}
		response := map[string]any{"ok": true, "messages": map[string]any{"paging": map[string]int{paramCount: 100, "total": 101, paramPage: 1, "pages": 2}, "matches": []any{
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
	page, err := c.Mentions(context.Background(), core.MentionQuery{User: mentionTestUser, From: at.Add(-time.Hour), Until: at.Add(time.Hour), Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !page.More || len(page.Matches) != 1 {
		t.Fatal("lost paging or admitted chatter/out-of-window activity", page)
	}
	match := page.Matches[0]
	if match.Conversation.ID != "outside-kept" || match.Conversation.Kind != core.ConversationPrivate || match.Trigger.TS != mentionTestTS || match.Trigger.Thread != mentionTestRoot {
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
		`{"ok":true,"messages":{"paging":{paramCount:100,"total":1,paramPage:1,"pages":1},"matches":[{"ts":"invalid",paramText:"<@person>","channel":{"id":"test"}}]}}`,
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
			if _, err := c.Mentions(context.Background(), core.MentionQuery{User: mentionTestUser, From: at.Add(-time.Hour), Until: at.Add(time.Hour), Page: 1}); err == nil {
				t.Fatal("incomplete discovery accepted as successful")
			}
		})
	}
}

func TestHistoryOpeningTargetParameters(t *testing.T) {
	for _, thread := range []string{"", mentionTestRoot} {
		t.Run(thread, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				bound := "latest"
				if thread != "" {
					bound = paramOldest
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

func TestBroadcastSearchExactTokensAndStableCrossSourceIdentity(t *testing.T) {
	at, err := core.MessageTime(mentionTestTS)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		reason core.AttentionReason
		token  string
	}{{core.AttentionChannel, "<!channel>"}, {core.AttentionHere, "<!here>"}} {
		t.Run(string(tc.reason), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				if !strings.HasPrefix(r.Form.Get("query"), tc.token+" after:") {
					t.Error("source query coupled to direct mentions", r.Form.Get("query"))
				}
				response := map[string]any{"ok": true, "messages": map[string]any{"paging": map[string]int{paramCount: mentionPageItems, "total": 3, paramPage: 1, "pages": 1}, "matches": []any{
					map[string]any{"ts": mentionTestTS, paramText: "request " + tc.token, "thread_ts": mentionTestRoot, paramChannel: map[string]any{"id": "broadcast-source"}},
					map[string]any{"ts": mentionTestTS, paramText: "literal " + string(tc.reason), paramChannel: map[string]any{"id": "plain-text"}},
					map[string]any{"ts": mentionTestTS, paramText: "<@person>", paramChannel: map[string]any{"id": "direct-only"}},
				}}}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			c := New(mentionTestToken)
			c.base = server.URL + "/"
			page, err := c.Mentions(context.Background(), core.MentionQuery{User: mentionTestUser, Reason: tc.reason, From: at.Add(-time.Hour), Until: at.Add(time.Hour), Page: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Matches) != 1 || page.Matches[0].Trigger.Reason != tc.reason || page.Matches[0].Trigger.ID != "mention:"+mentionTestTS || page.Matches[0].Trigger.Thread != mentionTestRoot {
				t.Fatal("broadcast validation/identity", page)
			}
		})
	}
}

func TestFollowedThreadWindowParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/conversations.replies" || r.Form.Get("ts") != mentionTestTS || r.Form.Get(paramOldest) != "1794672001.000001" || r.Form.Get("latest") != "1794673000.000001" || r.Form.Get(paramCursor) != "replies-cursor" || r.Form.Get("inclusive") != "" {
			t.Error("followed window lost", r.Form)
		}
		if _, err := w.Write([]byte(`{"ok":true,"messages":[]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := New(mentionTestToken)
	c.base = server.URL + "/"
	if _, err := c.History(context.Background(), core.Query{Conversation: "followed-source", Thread: mentionTestTS, After: "1794672001.000001", Before: "1794673000.000001", Cursor: "replies-cursor"}); err != nil {
		t.Fatal(err)
	}
}

const (
	mentionTestUser = "person"
	mentionTestRoot = "1794671000.000001"
)
