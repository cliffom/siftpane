package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cliffom/siftpane/internal/core"
)

func TestBodyAddsSlackLinkOnlyForUnrenderedContent(t *testing.T) {
	for _, tc := range []struct {
		name, payload, wantText string
		wantLink                bool
	}{
		{name: "plain message", payload: `{"text":"hi"}`, wantText: "hi"},
		{name: "ordinary rich text", payload: `{"text":"hi","blocks":[{"type":"rich_text"}]}`, wantText: "hi"},
		{name: "readable subtype", payload: `{"text":"Deployment finished","subtype":"bot_message","blocks":[{"type":"rich_text"}]}`, wantText: "Deployment finished"},
		{name: "readable preview", payload: `{"text":"<https://example.com|Notes>","attachments":[{"fallback":"Review notes"}]}`, wantText: "Review notes"},
		{name: "linked file", payload: `{"text":"Attached notes","subtype":"file_share","files":[{"title":"Notes","permalink":"https://example.com/file"}]}`, wantText: "https://example.com/file"},
		{name: "image with caption", payload: `{"text":"Architecture drawing","blocks":[{"type":"image"}]}`, wantText: "Architecture drawing", wantLink: true},
		{name: "block without fallback", payload: `{"blocks":[{"type":"rich_text"}]}`, wantText: "Rich content", wantLink: true},
		{name: "attachment without fallback", payload: `{"text":"See this","attachments":[{}]}`, wantText: "See this", wantLink: true},
		{name: "file without link", payload: `{"files":[{"title":"Notes"}]}`, wantText: "File: Notes", wantLink: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var msg core.Message
			if err := json.Unmarshal([]byte(tc.payload), &msg); err != nil {
				t.Fatal(err)
			}
			msg.Link = "slack://channel?id=C_TEST&message=123.456"
			got := Body(msg)
			if !strings.Contains(got, tc.wantText) {
				t.Fatalf("body lost readable content: %q", got)
			}
			if hasLink := strings.Contains(got, msg.Link); hasLink != tc.wantLink {
				t.Fatalf("Slack link present=%v, want %v: %q", hasLink, tc.wantLink, got)
			}
		})
	}
}

func TestCodePreservesLiteralMentionsAndLinks(t *testing.T) {
	raw := "<@U1> `literal <@U1>`\n```go\n<@U1> <https://example.com|label>\n```"
	m := WithProfiles(core.Message{Text: raw}, map[string]core.Profile{"U1": {ID: "U1", Name: "Code Tester"}})
	got := Body(m)
	if !strings.HasPrefix(got, "@Code Tester `literal <@U1>`") || !strings.Contains(got, "<@U1> <https://example.com|label>") {
		t.Fatal("code interpreted as prose", got)
	}
}

func TestUnclosedCodeKeepsMentionsAndLinksLiteral(t *testing.T) {
	raw := "Before <@U1>\n```literal <@U1> <https://example.com|label>"
	m := WithProfiles(core.Message{Text: raw}, map[string]core.Profile{"U1": {ID: "U1", Name: "Code Tester"}})
	got := Body(m)
	if !strings.HasPrefix(got, "Before @Code Tester") || !strings.Contains(got, "literal <@U1> <https://example.com|label>") {
		t.Fatal("unfinished code decorated as prose", got)
	}
}
