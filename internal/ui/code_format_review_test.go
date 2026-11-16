package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSlackFencesRetainAllCodeContent(t *testing.T) {
	first := "SELECT important_column FROM some_table WHERE important_condition = true;"
	for _, body := range []string{
		"```" + first + "\nlast_line```",
		"```\n" + first + "\nlast_line\n```",
		"before ```" + first + "\nlast_line``` after",
		"```" + first + "\nlast_line",
	} {
		for _, width := range []int{36, 56, 74} {
			rows := formattedBodyRows(body, width)
			var content strings.Builder
			for _, row := range rows {
				plain := ansi.Strip(row)
				if ansi.StringWidth(plain) > width {
					t.Fatal("code exceeded display width", plain, width)
				}
				if strings.HasPrefix(plain, "  │ ") {
					content.WriteString(strings.TrimPrefix(plain, "  │ "))
				}
			}
			got := content.String()
			// Wraps may split either side of spaces; neither boundary may erase text.
			normalized := strings.ReplaceAll(got, " ", "")
			if !strings.Contains(normalized, strings.ReplaceAll(first, " ", "")) || !strings.Contains(normalized, "last_line") {
				t.Fatal("code content lost", width, got)
			}
			if strings.Contains(got, codeFence) {
				t.Fatal("closing fence leaked into code", got)
			}
			rendered := ansi.Strip(strings.Join(rows, "\n"))
			if strings.HasPrefix(body, "before") && (!strings.Contains(rendered, "before") || !strings.Contains(rendered, "after")) {
				t.Fatal("surrounding prose lost", rendered)
			}
		}
	}
}

func TestSlackFencesPreserveLiteralIndentationAndBlankLines(t *testing.T) {
	body := "```\n    first\n\n        second\n```\nprose"
	got := ansi.Strip(strings.Join(formattedBodyRows(body, 40), "\n"))
	for _, want := range []string{"│     first", "│ \n", "│         second", "└\n  prose"} {
		if !strings.Contains(got, want) {
			t.Fatal("code structure lost", want, got)
		}
	}
}
