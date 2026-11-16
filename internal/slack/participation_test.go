package slack

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

func TestQuickReactionTogglesOnlyAuthenticatedUser(t *testing.T) {
	for _, own := range []bool{false, true} {
		for _, reaction := range []core.QuickReaction{core.ReactionAcknowledged, core.ReactionLooking, core.ReactionResolved} {
			t.Run(fmt.Sprint(own)+string(reaction), func(t *testing.T) {
				testQuickReaction(t, own, reaction)
			})
		}
	}
}

const attentionWireTS = "1794672000.000001"

func TestReactionReadFailureNeverWritesAndWriteNeverRetries(t *testing.T) {
	for _, readFail := range []bool{false, true} {
		calls := 0
		c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Path == "/"+methodReactionGet && !readFail {
				writeResponse(t, w, `{"ok":true,"message":{}}`)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		_, err := c.ToggleReaction(context.Background(), core.MessageTarget{Conversation: "C1", TS: attentionWireTS}, core.ReactionLooking, "SELF")
		expected := 2
		if readFail {
			expected = 1
		}
		if err == nil || calls != expected {
			t.Fatal("reaction retried or wrote after read failure", err, calls)
		}
		if !readFail && !core.UncertainDelivery(err) {
			t.Fatal("write failure lost uncertainty")
		}
	}
}

func TestPermalinkUsesExactSelectedTarget(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/"+methodPermalink || r.URL.Query().Get(paramChannel) != "C1" || r.URL.Query().Get("message_ts") != attentionWireTS {
			t.Error(r.Method, r.URL)
		}
		writeResponse(t, w, `{"ok":true,"permalink":"https://example.slack.com/archives/C1/p1794672000000001?thread_ts=1794671000.000001"}`)
	})
	link, err := c.Permalink(context.Background(), core.MessageTarget{Conversation: "C1", TS: attentionWireTS})
	if err != nil || link == "" {
		t.Fatal(link, err)
	}
}

func testQuickReaction(t *testing.T, own bool, reaction core.QuickReaction) {
	t.Helper()
	calls := 0
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.Form.Get(paramChannel) != "C1" || r.Form.Get(paramTimestamp) != attentionWireTS {
			t.Error("target changed", r.Form)
		}
		if r.URL.Path == "/"+methodReactionGet {
			if r.Method != http.MethodGet {
				t.Error("reaction read not GET")
			}
			users := `["OTHER"]`
			if own {
				users = `["SELF","OTHER"]`
			}
			writeResponse(t, w, fmt.Sprintf(`{"ok":true,"message":{"reactions":[{"name":%q,"users":%s,"count":50}]}}`, reaction, users))
			return
		}
		method := methodReactionAdd
		if own {
			method = methodReactionRemove
		}
		if r.URL.Path != "/"+method || r.Method != http.MethodPost || r.Form.Get("name") != string(reaction) || r.Form.Get("user") != "" {
			t.Error("incorrect mutation", r.URL.Path, r.Form)
		}
		writeResponse(t, w, `{"ok":true}`)
	})
	present, err := c.ToggleReaction(context.Background(), core.MessageTarget{Conversation: "C1", TS: attentionWireTS}, reaction, "SELF")
	if err != nil || present == own || calls != 2 {
		t.Fatal(present, err, calls)
	}
}
