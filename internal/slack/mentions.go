package slack

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	methodMentions    = "search.messages"
	mentionPageItems  = 100
	mentionMaxPage    = 10
	mentionInterval   = 3100 * time.Millisecond
	searchDatePadding = 24 * time.Hour
)

// Mentions searches a bounded interval for exact personal mentions, independently
// of local Kept membership. Search filters/indexing can still reduce coverage.
func (c *Client) Mentions(ctx context.Context, q core.MentionQuery) (core.MentionPage, error) {
	if q.User == "" || q.Page < 1 || q.Page > mentionMaxPage || q.From.IsZero() || !q.Until.After(q.From) || q.Until.Sub(q.From) > core.AttentionLookback {
		return core.MentionPage{}, errors.New("invalid bounded mention query")
	}
	token := "<@" + q.User + ">"
	query := token + " after:" + q.From.Add(-searchDatePadding).UTC().Format(time.DateOnly) + " before:" + q.Until.Add(searchDatePadding).UTC().Format(time.DateOnly)
	var r struct {
		Messages struct {
			Matches []mentionWireMatch
			Paging  struct{ Count, Total, Page, Pages int }
		}
	}

	v := url.Values{"query": {query}, "sort": {"timestamp"}, "sort_dir": {"asc"}, "highlight": {wireFalse}, "count": {strconv.Itoa(mentionPageItems)}, "page": {strconv.Itoa(q.Page)}}
	if err := c.call(ctx, methodMentions, v, &r); err != nil {
		return core.MentionPage{}, err
	}
	p := r.Messages.Paging
	if p.Page != q.Page || p.Count != mentionPageItems || p.Pages < 0 || p.Total < 0 || p.Pages > 0 && p.Pages < q.Page || len(r.Messages.Matches) > mentionPageItems {
		return core.MentionPage{}, errors.New("inconsistent mention pagination; retry discovery")
	}
	if p.Total > 0 && (p.Pages != (p.Total+mentionPageItems-1)/mentionPageItems || len(r.Messages.Matches) == 0) {
		return core.MentionPage{}, errors.New("incomplete mention pagination; retry discovery")
	}
	out := core.MentionPage{More: p.Pages > q.Page, Total: p.Total}
	for _, m := range r.Messages.Matches {
		match, err := decodeMention(m, token, q)
		if err != nil {
			return core.MentionPage{}, err
		}
		if match != nil {
			out.Matches = append(out.Matches, *match)
		}
	}
	return out, nil
}

type mentionWireMatch struct {
	TS, Text, Permalink string
	Thread              string `json:"thread_ts"`
	Channel             struct {
		ID, Name string
		Private  bool `json:"is_private"`
		IM       bool `json:"is_im"`
		MPIM     bool `json:"is_mpim"`
	}
}

func decodeMention(m mentionWireMatch, token string, q core.MentionQuery) (*core.MentionMatch, error) {
	if !strings.Contains(m.Text, token) {
		return nil, nil
	}
	at, err := core.MessageTime(m.TS)
	if err != nil || m.Channel.ID == "" {
		return nil, errors.New("invalid mention identity")
	}
	if at.Before(q.From) || !at.Before(q.Until) {
		return nil, nil
	}
	root := m.Thread
	if root == "" && m.Permalink != "" {
		u, err := url.Parse(m.Permalink)
		if err != nil {
			return nil, errors.New("invalid mention link")
		}
		root = u.Query().Get("thread_ts")
	}
	if root != "" {
		if _, err := core.MessageTime(root); err != nil {
			return nil, errors.New("invalid mention thread")
		}
	}
	if root == m.TS {
		root = ""
	}
	kind := core.ConversationChannel
	switch {
	case m.Channel.MPIM:
		kind = core.ConversationGroup
	case m.Channel.IM:
		kind = core.ConversationDM
	case m.Channel.Private:
		kind = core.ConversationPrivate
	}
	name := m.Channel.Name
	if name == "" {
		name = m.Channel.ID
	}
	return &core.MentionMatch{Conversation: core.Conversation{ID: m.Channel.ID, Name: name, Kind: kind}, Trigger: core.AttentionTrigger{ID: "mention:" + m.TS, TS: m.TS, Thread: root, Reason: core.AttentionMention, At: at}}, nil
}
