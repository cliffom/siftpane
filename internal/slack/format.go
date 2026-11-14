package slack

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/cliffom/siftpane/internal/core"
)

var mentionRE = regexp.MustCompile(`<@[A-Z0-9]+>`)
var angleRE = regexp.MustCompile(`<([^<>]+)>`)

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
func Readable(s string) string {
	s = angleRE.ReplaceAllStringFunc(s, func(v string) string {
		parts := strings.SplitN(v[1:len(v)-1], "|", 2)
		a := parts[0]
		if len(parts) == 2 {
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
func Body(m core.Message) string {
	s := Readable(m.Text)
	for _, a := range m.Attachments {
		t := a.Fallback
		if t == "" {
			t = a.Text
		}
		if t != "" {
			s += "\n" + Readable(t)
		}
	}
	for _, f := range m.Files {
		s += "\n[File: " + Safe(f.Title) + "] " + Safe(f.URL)
	}
	if len(m.Blocks) > 0 && strings.TrimSpace(s) == "" {
		s = "[Rich content — open in Slack]"
	}
	if strings.TrimSpace(s) == "" {
		s = "[" + Safe(m.Subtype) + " message — open in Slack]"
	}
	if len(m.Files) > 0 || len(m.Blocks) > 0 || len(m.Attachments) > 0 || m.Subtype != "" {
		if m.Link != "" {
			s += "\n↗ " + Safe(m.Link)
		}
	}
	return strings.TrimSpace(s)
}
