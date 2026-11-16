package ui

import (
	"errors"
	"slices"

	"github.com/cliffom/siftpane/internal/core"
)

// Accept useful partial pages, but abandon a nonadvancing cursor. The next
// explicit page request can resume after the last retained reply, with no loop.
func (t *timeline) checkReplyPage(v historyResult) {
	if v.QueryCursor == "" {
		t.replyCursors = nil
	}
	repeated := v.Page.Next == v.QueryCursor || slices.Contains(t.replyCursors, v.Page.Next)
	if v.QueryCursor != "" {
		t.replyCursors = append(t.replyCursors, v.QueryCursor)
		if len(t.replyCursors) > replyCursorHistory {
			t.replyCursors = t.replyCursors[len(t.replyCursors)-replyCursorHistory:]
		}
	}
	last := ""
	for _, msg := range v.Page.Messages {
		if msg.TS > last {
			last = msg.TS
		}
	}
	if !v.Page.More || (v.Page.Next != "" && !repeated) {
		return
	}
	t.Next = ""
	if len(t.Messages) > 0 {
		t.After = t.Messages[len(t.Messages)-1].TS
	}
	if last == "" || last <= v.QueryAfter {
		t.Err = errors.New("reply pagination stalled; o retry or O inspect Slack")
		t.PageErr = t.Err
	}
}

func (t *timeline) recordReadResult(v historyResult) {
	if v.Older {
		t.PageErr = v.Err
		return
	}
	t.RecentErr = v.Err
	if v.Err == nil {
		t.Checked = t.Saved
	}
}

// Keep a bounded thread window, retaining its root separately for context.
func mergeReplies(old, fresh []core.Message, root string) []core.Message {
	var parent *core.Message
	for _, msg := range append(append([]core.Message{}, old...), fresh...) {
		if msg.TS == root {
			parentMessage := msg
			parent = &parentMessage
		}
	}
	merged := core.Merge(old, fresh, cacheLimit)
	if parent != nil && len(merged) > 0 && merged[0].TS != root {
		merged = append([]core.Message{*parent}, merged[max(0, len(merged)-(cacheLimit-1)):]...)
	}
	return merged
}
