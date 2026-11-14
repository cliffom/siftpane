package slack

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/cliffom/siftpane/internal/core"
)

var (
	mentionRE = regexp.MustCompile(`<@[A-Z0-9]+>`)
	angleRE   = regexp.MustCompile(`<([^<>]+)>`)
)

// Safe removes control sequences, bidi overrides and terminal controls from untrusted text.
func Safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s)
}

// Readable renders Slack mentions, links, and entities as safe terminal text.
func Readable(s string) string {
	s = angleRE.ReplaceAllStringFunc(s, func(v string) string {
		parts := strings.SplitN(v[1:len(v)-1], "|", linkParts)
		a := parts[0]
		if len(parts) == linkParts {
			if strings.HasPrefix(a, "http") {
				return parts[1] + " (" + a + ")"
			}
			if strings.HasPrefix(a, "#") {
				return "#" + parts[1]
			}
			if strings.HasPrefix(a, "@") {
				return "@" + parts[1]
			}
			return parts[1]
		}
		if strings.HasPrefix(a, "!") {
			return "@" + strings.TrimPrefix(a, "!")
		}
		if strings.HasPrefix(a, "@") {
			return a
		}
		return a
	})
	return Safe(html.UnescapeString(s))
}

// Body returns readable text with fallbacks for unsupported message content.
func Body(m core.Message) string {
	s := Readable(m.Text)
	needsSlackLink := false
	// Text already represents ordinary rich-text blocks. Other block types may
	// contain controls or media that the terminal cannot display.
	for _, block := range m.Blocks {
		if block["type"] != "rich_text" {
			needsSlackLink = true
		}
	}
	for _, a := range m.Attachments {
		t := a.Fallback
		if t == "" {
			t = a.Text
		}
		if t != "" {
			s += "\n" + Readable(t)
		} else {
			needsSlackLink = true
		}
	}
	for _, f := range m.Files {
		s += "\n[File: " + Safe(f.Title) + "] " + Safe(f.URL)
		if strings.TrimSpace(f.URL) == "" {
			needsSlackLink = true
		}
	}
	if len(m.Blocks) > 0 && strings.TrimSpace(s) == "" {
		s = "[Rich content — open in Slack]"
		needsSlackLink = true
	}
	if strings.TrimSpace(s) == "" {
		s = "[" + Safe(m.Subtype) + " message — open in Slack]"
		needsSlackLink = true
	}
	if needsSlackLink && m.Link != "" {
		s += "\n↗ " + Safe(m.Link)
	}
	return strings.TrimSpace(s)
}

// A Slack link has a target and an optional label separated by one pipe.
const linkParts = 2

// MentionIDs extracts explicit user mentions without making network calls.
func MentionIDs(text string) []string {
	out := []string{}
	for _, m := range mentionRE.FindAllString(text, -1) {
		out = append(out, m[2:len(m)-1])
	}
	return out
}

// WithProfiles decorates a copy for presentation, retaining the raw cached text.
// Escaping names prevents profile text from being interpreted as Slack markup.
func WithProfiles(m core.Message, profiles map[string]core.Profile) core.Message {
	if p := profiles[m.User]; p.Name != "" && m.Author != "You" {
		m.Author = p.Name
	}
	m.Text = mentionRE.ReplaceAllStringFunc(m.Text, func(mention string) string {
		id := mention[2 : len(mention)-1]
		if p := profiles[id]; p.Name != "" {
			return "@" + html.EscapeString(p.Name)
		}
		return mention
	})
	return m
}
