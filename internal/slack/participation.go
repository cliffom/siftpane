package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/cliffom/siftpane/internal/core"
)

const (
	methodReactionGet    = "reactions.get"
	methodReactionAdd    = "reactions.add"
	methodReactionRemove = "reactions.remove"
	methodPermalink      = "chat.getPermalink"
	paramTimestamp       = "timestamp"
)

// ToggleReaction reads the authenticated user's current reaction, then performs
// one explicit mutation. Races are reconciled to the intended state, never retried.
func (c *Client) ToggleReaction(ctx context.Context, target core.MessageTarget, reaction core.QuickReaction, user string) (bool, error) {
	if reaction.Label() == "" {
		return false, fmt.Errorf("unsupported quick reaction")
	}
	var r struct {
		Message struct {
			Reactions []struct {
				Name  string   `json:"name"`
				Users []string `json:"users"`
			} `json:"reactions"`
		} `json:"message"`
	}
	values := url.Values{paramChannel: {target.Conversation}, paramTimestamp: {target.TS}}
	if err := c.callHTTP(ctx, http.MethodGet, methodReactionGet, values, &r); err != nil {
		return false, err
	}
	present := false
	for _, entry := range r.Message.Reactions {
		if entry.Name == string(reaction) && slices.Contains(entry.Users, user) {
			present = true
		}
	}
	method, settled := methodReactionAdd, "already_reacted"
	if present {
		method, settled = methodReactionRemove, "no_reaction"
	}
	values.Set("name", string(reaction))
	err := c.call(ctx, method, values, &struct{}{})
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == settled {
		err = nil
	}
	return !present, err
}

// Permalink retrieves Slack's canonical URL for the exact selected message.
func (c *Client) Permalink(ctx context.Context, target core.MessageTarget) (string, error) {
	var r struct {
		URL string `json:"permalink"`
	}
	err := c.callHTTP(ctx, http.MethodGet, methodPermalink, url.Values{paramChannel: {target.Conversation}, "message_ts": {target.TS}}, &r)
	return r.URL, err
}
