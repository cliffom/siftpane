package slack

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

func TestReauthorizeRetainsOldTokenAndSharedBackoff(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer xoxp-replacement" {
			t.Error("replacement identity used wrong token")
		}
		writeResponse(t, w, `{"ok":true,"team_id":"T1","user_id":"U1"}`)
	})
	c.next[methodHistory] = time.Now().Add(time.Minute)
	id := core.Identity{Workspace: "T1", User: "U1"}
	next, actual, err := c.Reauthorize(context.Background(), "xoxp-replacement", id)
	if err != nil || actual.Workspace != id.Workspace || c.token != "xoxp-test" || next.token != "xoxp-replacement" {
		t.Fatal("credential replacement mutated old client or identity", err)
	}
	if next.requests != c.requests {
		t.Fatal("replacement did not share queued read priorities")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err = next.wait(ctx, methodHistory); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("replacement bypassed old backoff", err)
	}
	// A later deadline from the original in-flight transport is shared as well.
	c.mu.Lock()
	c.next[methodReplies] = time.Now().Add(time.Minute)
	c.mu.Unlock()
	if err = next.wait(ctx, methodReplies); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("late old-client backoff was not shared", err)
	}
}

func TestReauthorizeRejectsAnotherIdentityAndMissingScopeIsActionable(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, methodIdentity) {
			writeResponse(t, w, `{"ok":true,"team_id":"OTHER","user_id":"U1"}`)
		} else {
			writeResponse(t, w, `{"ok":false,"error":"missing_scope"}`)
		}
	})
	if next, _, err := c.Reauthorize(context.Background(), "xoxp-other", core.Identity{Workspace: "T1", User: "U1"}); err == nil || next != nil {
		t.Fatal("accepted another workspace")
	}
	_, err := c.History(context.Background(), core.Query{Conversation: "C1"})
	if err == nil || !strings.Contains(err.Error(), methodHistory) || !strings.Contains(err.Error(), "reinstall") || IsAuth(err) {
		t.Fatal("scope failure lacked method/recovery or stopped all reads", err)
	}
}

func TestReplyCursorIndicatesIncompletePageWithoutHasMore(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{"ok":true,"messages":[],"has_more":false,"response_metadata":{"next_cursor":"remaining"}}`)
	})
	page, err := c.History(context.Background(), core.Query{Conversation: "C1", Thread: "1.0"})
	if err != nil || !page.More || page.Next != "remaining" {
		t.Fatal("nonempty cursor was silently marked complete", page, err)
	}
}
