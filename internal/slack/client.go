// Package slack uses documented Web APIs with a user OAuth token. It never logs payloads.
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

type APIError struct {
	Code       string
	RetryAfter time.Duration
	Ambiguous  bool
}

func (e *APIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited; retry in %s", e.RetryAfter.Round(time.Second))
	}
	return "Slack: " + e.Code
}
func IsAuth(err error) bool {
	var e *APIError
	return errors.As(err, &e) && (e.Code == "invalid_auth" || e.Code == "token_revoked" || e.Code == "token_expired" || e.Code == "account_inactive" || e.Code == "not_authed")
}
func Uncertain(err error) bool {
	var e *APIError
	if errors.As(err, &e) {
		return e.Ambiguous
	}
	return err != nil
}

type Client struct {
	token string
	base  string
	http  *http.Client
	mu    sync.Mutex
	next  map[string]time.Time
	names map[string]string
}

func New(token string) *Client {
	return &Client{token: token, base: "https://slack.com/api/", http: &http.Client{Timeout: 20 * time.Second}, next: map[string]time.Time{}, names: map[string]string{}}
}

// Pacing is per method and workspace. A 429 blocks that method, including manual calls.
func (c *Client) wait(ctx context.Context, method string) error {
	for {
		c.mu.Lock()
		delay := time.Until(c.next[method])
		if delay <= 0 {
			gap := 1300 * time.Millisecond
			if method == "users.info" {
				gap = 650 * time.Millisecond
			}
			if method == "conversations.list" {
				gap = 3100 * time.Millisecond
			}
			c.next[method] = time.Now().Add(gap)
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}
func (c *Client) call(ctx context.Context, method string, v url.Values, out any) error {
	if e := c.wait(ctx, method); e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.base+method, strings.NewReader(v.Encode()))
	if e != nil {
		return errors.New("cannot create Slack request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, e := c.http.Do(req)
	if e != nil {
		return &APIError{Code: "connection failed; refresh will reconnect", Ambiguous: method == "chat.postMessage"}
	}
	defer r.Body.Close()
	if r.StatusCode == 429 {
		n, _ := strconv.Atoi(r.Header.Get("Retry-After"))
		if n < 1 {
			n = 30
		}
		d := time.Duration(n) * time.Second
		c.mu.Lock()
		until := time.Now().Add(d)
		if until.After(c.next[method]) {
			c.next[method] = until
		}
		c.mu.Unlock()
		return &APIError{Code: "ratelimited", RetryAfter: d}
	}
	if r.StatusCode != 200 {
		return &APIError{Code: fmt.Sprintf("HTTP %d", r.StatusCode), Ambiguous: method == "chat.postMessage"}
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if e != nil {
		return &APIError{Code: "incomplete response", Ambiguous: method == "chat.postMessage"}
	}
	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return &APIError{Code: "invalid response", Ambiguous: method == "chat.postMessage"}
	}
	if !envelope.OK {
		code := envelope.Error // Slack error codes only, never server payloads.
		if len(code) > 80 || strings.ContainsAny(code, " \n\r\x1b") {
			code = "request_failed"
		}
		return &APIError{Code: code, Ambiguous: code == "internal_error" || code == "fatal_error" || code == "request_timeout" || code == "service_unavailable"}
	}
	if e = json.Unmarshal(b, out); e != nil {
		return &APIError{Code: "invalid payload", Ambiguous: method == "chat.postMessage"}
	}
	return nil
}
func (c *Client) Identity(ctx context.Context) (core.Identity, error) {
	var r struct {
		TeamID   string `json:"team_id"`
		Team     string `json:"team"`
		UserID   string `json:"user_id"`
		Username string `json:"user"`
		BotID    string `json:"bot_id"`
	}
	e := c.call(ctx, "auth.test", url.Values{}, &r)
	if e != nil {
		return core.Identity{}, e
	}
	if r.BotID != "" {
		return core.Identity{}, errors.New("a user OAuth token is required; bot tokens cannot act as you")
	}
	return core.Identity{Workspace: r.TeamID, WorkspaceName: r.Team, User: r.UserID, Username: r.Username}, nil
}
func (c *Client) name(ctx context.Context, id string) string {
	if id == "" {
		return "Slack"
	}
	c.mu.Lock()
	n := c.names[id]
	c.mu.Unlock()
	if n != "" {
		return n
	}
	var r struct {
		User struct {
			Name    string `json:"name"`
			Real    string `json:"real_name"`
			Profile struct {
				Display string `json:"display_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if c.call(ctx, "users.info", url.Values{"user": {id}}, &r) != nil {
		return id
	}
	n = r.User.Profile.Display
	if n == "" {
		n = r.User.Real
	}
	if n == "" {
		n = r.User.Name
	}
	if n == "" {
		n = id
	}
	c.mu.Lock()
	if len(c.names) >= 512 {
		c.names = map[string]string{}
	}
	c.names[id] = n
	c.mu.Unlock()
	return n
}
func (c *Client) Directory(ctx context.Context, cursor string) (core.DirectoryPage, error) {
	var r struct {
		Channels []struct {
			ID, Name, User string
			IM             bool `json:"is_im"`
			MPIM           bool `json:"is_mpim"`
			Private        bool `json:"is_private"`
			Topic          struct {
				Value string `json:"value"`
			} `json:"topic"`
			Purpose struct {
				Value string `json:"value"`
			} `json:"purpose"`
			Members int `json:"num_members"`
		} `json:"channels"`
		Meta struct {
			Next string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
	e := c.call(ctx, "conversations.list", url.Values{"types": {"public_channel,private_channel,im,mpim"}, "exclude_archived": {"true"}, "limit": {"30"}, "cursor": {cursor}}, &r)
	if e != nil {
		return core.DirectoryPage{}, e
	}
	p := core.DirectoryPage{Next: r.Meta.Next}
	for _, v := range r.Channels {
		kind := "channel"
		name := v.Name
		if v.Private {
			kind = "private"
		}
		if v.MPIM {
			kind = "group"
		}
		if v.IM {
			kind = "dm"
			name = c.name(ctx, v.User)
		}
		p.Conversations = append(p.Conversations, core.Conversation{ID: v.ID, Name: name, Kind: kind, Topic: v.Topic.Value, Purpose: v.Purpose.Value, Members: v.Members})
	}
	return p, nil
}
func (c *Client) History(ctx context.Context, q core.Query) (core.Page, error) {
	method := "conversations.history"
	v := url.Values{"channel": {q.Conversation}, "limit": {"50"}, "cursor": {q.Cursor}}
	if q.After != "" {
		v.Set("oldest", q.After)
	}
	if q.Before != "" {
		v.Set("latest", q.Before)
	}
	if q.Thread != "" {
		method = "conversations.replies"
		v.Set("ts", q.Thread)
	}
	var r struct {
		Messages []core.Message `json:"messages"`
		More     bool           `json:"has_more"`
		Meta     struct {
			Next string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
	if e := c.call(ctx, method, v, &r); e != nil {
		return core.Page{}, e
	}
	// Resolve only authors and mentions in this page, never a workspace user download.
	for i := range r.Messages {
		m := &r.Messages[i]
		m.Author = c.name(ctx, m.User)
		m.Link = "slack://channel?id=" + url.QueryEscape(q.Conversation) + "&message=" + url.QueryEscape(m.TS)
		m.Text = mentionRE.ReplaceAllStringFunc(m.Text, func(s string) string { return "@" + c.name(ctx, strings.TrimSuffix(strings.TrimPrefix(s, "<@"), ">")) })
	}
	return core.Page{Messages: r.Messages, Next: r.Meta.Next, More: r.More}, nil
}
func (c *Client) Send(ctx context.Context, o core.Outgoing) (core.Message, error) {
	v := url.Values{"channel": {o.Conversation}, "text": {o.Text}, "unfurl_links": {"false"}, "unfurl_media": {"false"}, "client_msg_id": {o.ID}}
	if o.Thread != "" {
		v.Set("thread_ts", o.Thread)
	}
	var r struct {
		TS      string       `json:"ts"`
		Message core.Message `json:"message"`
	}
	if e := c.call(ctx, "chat.postMessage", v, &r); e != nil {
		return core.Message{}, e
	}
	r.Message.TS = r.TS
	r.Message.Text = o.Text
	r.Message.Thread = o.Thread
	r.Message.ClientID = o.ID
	return r.Message, nil
}

// Conversation restores a saved selection without scanning directory pages.
func (c *Client) Conversation(ctx context.Context, id string) (core.Conversation, error) {
	var r struct {
		Channel struct {
			ID, Name, User string
			IM             bool `json:"is_im"`
			MPIM           bool `json:"is_mpim"`
			Private        bool `json:"is_private"`
			Topic          struct {
				Value string `json:"value"`
			} `json:"topic"`
			Purpose struct {
				Value string `json:"value"`
			} `json:"purpose"`
			Members int `json:"num_members"`
		} `json:"channel"`
	}
	if e := c.call(ctx, "conversations.info", url.Values{"channel": {id}, "include_num_members": {"true"}}, &r); e != nil {
		return core.Conversation{}, e
	}
	v := r.Channel
	kind := "channel"
	name := v.Name
	if v.Private {
		kind = "private"
	}
	if v.MPIM {
		kind = "group"
	}
	if v.IM {
		kind = "dm"
		name = c.name(ctx, v.User)
	}
	return core.Conversation{ID: v.ID, Name: name, Kind: kind, Topic: v.Topic.Value, Purpose: v.Purpose.Value, Members: v.Members}, nil
}
