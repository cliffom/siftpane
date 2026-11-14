package slack

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

func TestHistoryDoesNotWaitForUncachedNames(t *testing.T) {
	var lookups atomic.Int32
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			lookups.Add(1)
			t.Error("history requested a profile")
			return
		}
		writeResponse(t, w, `{"ok":true,"messages":[{"ts":"1.1","user":"U1","text":"hello <@U2>"}]}`)
	})
	p, err := c.History(context.Background(), core.Query{Conversation: "C1"})
	if err != nil || len(p.Messages) != 1 {
		t.Fatal(p, err)
	}
	if lookups.Load() != 0 || p.Messages[0].Text != rawMentionText || p.Messages[0].User != "U1" || p.Messages[0].Link == "" {
		t.Fatal("history blocked on or rewrote uncached names", p)
	}
}

func TestConcurrentProfilesShareLookupAndCancelIndependently(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		close(entered)
		<-release
		writeResponse(t, w, `{"ok":true,"user":{"name":"maya","profile":{"display_name":"Maya"}}}`)
	})
	done := make(chan error, 1)
	go func() { _, err := c.Profile(context.Background(), "U1"); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	if _, err := c.Profile(ctx, "U1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	waiter := make(chan error, 1)
	go func() { _, err := c.Profile(context.Background(), "U1"); waiter <- err }()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-waiter; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate profile lookup", calls.Load())
	}
	if len(c.CachedProfiles()) != 1 {
		t.Fatal("successful profile not cached")
	}
}

func TestRestoredProfilesAvoidNetworkAndRemainBounded(t *testing.T) {
	c := clientFor(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("cached name made network request") })
	now := time.Now()
	c.RestoreProfiles([]core.Profile{{ID: "U1", Name: "Cached Maya", Updated: now}})
	p, err := c.Profile(context.Background(), "U1")
	if err != nil || p.Name != "Cached Maya" {
		t.Fatal(p, err)
	}
	cs := make([]core.Profile, authorCacheEntries+1)
	for i := range cs {
		cs[i] = core.Profile{ID: time.Unix(int64(i), 0).String(), Name: "Name", Updated: now}
	}
	c.RestoreProfiles(cs)
	if len(c.CachedProfiles()) > authorCacheEntries {
		t.Fatal("unbounded restored names")
	}
}

func TestProfileErrorsRemainRetryableAndHonorBackoff(t *testing.T) {
	var calls atomic.Int32
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if _, err := c.Profile(context.Background(), "U1"); err == nil {
		t.Fatal("rate limit ignored")
	}
	if len(c.CachedProfiles()) != 0 {
		t.Fatal("failed profile persisted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	if _, err := c.Profile(ctx, "U1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("profile bypassed Retry-After")
	}
}

func TestWithProfilesPreservesRawTextAndEscapesNames(t *testing.T) {
	raw := core.Message{User: "U1", Text: rawMentionText}
	ps := map[string]core.Profile{"U1": {Name: "Maya"}, "U2": {Name: "<https://example.com|fake>"}}
	shown := WithProfiles(raw, ps)
	if shown.Author != "Maya" || Body(shown) != "hello @<https://example.com|fake>" || raw.Text != rawMentionText {
		t.Fatal("unsafe or mutated presentation", shown, Body(shown))
	}
}

const rawMentionText = "hello <@U2>"
