package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/siftpane/internal/core"
)

func TestSelectedMessageRailGroupsWrappedAndBlankBodyRows(t *testing.T) {
	msg := core.Message{TS: futureTimestamp, Author: "Reviewer", Text: "A long paragraph that wraps across several terminal cells and keeps its rail.\n\n> quoted context\n- list item\n```literal code\nsecond line```"}
	for _, width := range []int{38, 74} {
		for _, selectedMessage := range []bool{false, true} {
			rows := messageRows(msg, width, selectedMessage)
			for i, row := range rows[:len(rows)-1] {
				plain := ansi.Strip(row)
				if ansi.StringWidth(row) > historyContentWidth(width) {
					t.Fatal("message consumed its reserved right padding", width, plain)
				}
				if selectedMessage && !strings.HasPrefix(row, " "+accent.Render("│")) {
					t.Fatal("selected message row lost its colored rail", i, row)
				}
				if !selectedMessage && !strings.HasPrefix(plain, " ") {
					t.Fatal("unselected message lost its matching gutter", plain)
				}
			}
			if rows[len(rows)-1] != "" {
				t.Fatal("selection rail extended into the message separator")
			}
		}
	}
}

func TestHistoryAlwaysReservesRightPadding(t *testing.T) {
	m := fixture(t)
	m.cache[m.key()] = &timeline{Loaded: true, Messages: []core.Message{{TS: futureTimestamp, Author: "Reviewer", Text: strings.Repeat("wide内容 ", 30)}}}
	m.index, m.scroll = 0, 0
	for _, width := range []int{38, 74} {
		for _, row := range m.history(width, 20) {
			if ansi.StringWidth(row) != width || !strings.HasSuffix(ansi.Strip(row), " ") {
				t.Fatal("message touched its right border or changed width", row)
			}
		}
	}
}
