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

// Local operational policy for internal-app traffic; server Retry-After wins.
const (
	apiBaseURL         = "https://slack.com/api/"
	requestTimeout     = 20 * time.Second
	methodInterval     = 1300 * time.Millisecond
	userLookupInterval = 650 * time.Millisecond
	directoryInterval  = 3100 * time.Millisecond
	fallbackRetryAfter = 30 * time.Second
	responseByteLimit  = 4 << 20 // Bound decoded response input to 4 MiB.
	errorCodeByteLimit = 80
	authorCacheEntries = 512
	directoryPageItems = 200 // Slack recommends at most 200 results per directory page.
	historyPageItems   = 50
)

// Documented method identifiers belong to the transport, not core domain state.
const (
	wireTrue           = "true"
	wireFalse          = "false"
	paramCursor        = "cursor"
	paramOldest        = "oldest"
	paramPage          = "page"
	paramCount         = "count"
	paramText          = "text"
	paramLatest        = "latest"
	methodIdentity     = "auth.test"
	methodUser         = "users.info"
	methodDirectory    = "conversations.list"
	methodHistory      = "conversations.history"
	methodReplies      = "conversations.replies"
	methodSend         = "chat.postMessage"
	methodConversation = "conversations.info"
)

// APIError reports a sanitized Slack failure, retry delay, and delivery uncertainty.
type APIError struct {
	Code       string
	Method     string
	RetryAfter time.Duration
	Ambiguous  bool
}

func (err *APIError) Error() string {
	if err.RetryAfter > 0 {
		return fmt.Sprintf("rate limited; retry in %s", err.RetryAfter.Round(time.Second))
	}
	if err.Code == "missing_scope" && err.Method != "" {
		return "Slack: missing_scope for " + err.Method + "; reinstall app with documented user scopes, then replace its saved token (--login --account)"
	}
	if err.Code == "not_allowed_token_type" {
		return "Slack: use the internal app's xoxp- User OAuth Token, not a bot or app token"
	}
	return "Slack: " + err.Code
}

// IsAccess reports explicit conversation access revocation; cached content is disposable.
func IsAccess(err error) bool {
	var e *APIError
	if !errors.As(err, &e) {
		return false
	}
	switch e.Code {
	case "channel_not_found", "not_in_channel", "access_denied", "ekm_access_denied", "restricted_action":
		return true
	}
	return false
}

// IsAuth reports whether the error requires replacing the authenticated credential.
func IsAuth(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Code == "invalid_auth" || apiErr.Code == "token_revoked" || apiErr.Code == "token_expired" || apiErr.Code == "account_inactive" || apiErr.Code == "not_authed")
}

// IsInvalidCursor identifies an expired/rejected pagination cursor.
func IsInvalidCursor(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == "invalid_cursor"
}

// DeliveryUncertain reports whether Slack might have accepted the failed write.
func (err *APIError) DeliveryUncertain() bool { return err.Ambiguous }

// Client is a concurrency-safe, paced Web API client for one user and workspace.
type Client struct {
	token            string
	base             string
	http             *http.Client
	mu               *sync.Mutex
	next             map[string]time.Time
	names            map[string]core.Profile
	profilesInFlight map[string]*profileCall
}

// New constructs a user-token client without making network requests.
func New(token string) *Client {
	return &Client{mu: &sync.Mutex{}, token: token, base: apiBaseURL, http: &http.Client{Timeout: requestTimeout}, next: map[string]time.Time{}, names: map[string]core.Profile{}, profilesInFlight: map[string]*profileCall{}}
}

// Pacing is per method and workspace. A 429 blocks that method, including manual calls.
func (c *Client) wait(ctx context.Context, method string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		delay := time.Until(c.next[method])
		if delay <= 0 {
			gap := methodInterval
			if method == methodUser {
				gap = userLookupInterval
			}
			if method == methodDirectory {
				gap = directoryInterval
			}
			if method == methodReactionRemove {
				gap = reactionRemoveInterval
			}
			if method == methodMentions {
				gap = mentionInterval
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
	return c.callHTTP(ctx, http.MethodPost, method, v, out)
}

func (c *Client) callHTTP(ctx context.Context, httpMethod, method string, v url.Values, out any) error {
	if err := c.wait(ctx, method); err != nil {
		return err
	}
	endpoint := c.base + method
	body := v.Encode()
	if httpMethod == http.MethodGet {
		endpoint += "?" + body
		body = ""
	}
	req, err := http.NewRequestWithContext(ctx, httpMethod, endpoint, strings.NewReader(body))
	if err != nil {
		return errors.New("cannot create Slack request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := c.http.Do(req)
	if err != nil {
		return &APIError{Code: "connection failed; refresh will reconnect", Ambiguous: isWriteMethod(method)}
	}
	defer func() {
		// Closing an already-read response cannot change its API result.
		_ = r.Body.Close()
	}()
	if r.StatusCode == http.StatusTooManyRequests {
		n, err := strconv.Atoi(r.Header.Get("Retry-After"))
		if err != nil || n < 1 {
			n = int(fallbackRetryAfter / time.Second)
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
	if r.StatusCode != http.StatusOK {
		return &APIError{Code: fmt.Sprintf("HTTP %d", r.StatusCode), Ambiguous: isWriteMethod(method)}
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, responseByteLimit))
	if err != nil {
		return &APIError{Code: "incomplete response", Ambiguous: isWriteMethod(method)}
	}
	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return &APIError{Code: "invalid response", Ambiguous: isWriteMethod(method)}
	}
	if !envelope.OK {
		code := envelope.Error // Slack error codes only, never server payloads.
		if len(code) > errorCodeByteLimit || strings.ContainsAny(code, " \n\r\x1b") {
			code = "request_failed"
		}
		return &APIError{Code: code, Method: method, Ambiguous: code == "internal_error" || code == "fatal_error" || code == "request_timeout" || code == "service_unavailable"}
	}
	if err = json.Unmarshal(b, out); err != nil {
		return &APIError{Code: "invalid payload", Ambiguous: isWriteMethod(method)}
	}
	return nil
}

// Identity validates the token and returns the workspace and authenticated user.
func (c *Client) Identity(ctx context.Context) (core.Identity, error) {
	var r struct {
		TeamID   string `json:"team_id"`
		Team     string `json:"team"`
		UserID   string `json:"user_id"`
		Username string `json:"user"`
		BotID    string `json:"bot_id"`
	}
	err := c.call(ctx, methodIdentity, url.Values{}, &r)
	if err != nil {
		return core.Identity{}, err
	}
	if r.TeamID == "" || r.UserID == "" {
		return core.Identity{}, errors.New("slack authentication returned no workspace/user identity")
	}
	if r.BotID != "" {
		return core.Identity{}, errors.New("a user OAuth token is required; bot tokens cannot act as you")
	}
	return core.Identity{Workspace: r.TeamID, WorkspaceName: r.Team, User: r.UserID, Username: r.Username}, nil
}

// Directory fetches one page of accessible channels, DMs, and group DMs.
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
	err := c.call(ctx, methodDirectory, url.Values{"types": {"public_channel,private_channel,im,mpim"}, "exclude_archived": {wireTrue}, "limit": {strconv.Itoa(directoryPageItems)}, paramCursor: {cursor}}, &r)
	if err != nil {
		return core.DirectoryPage{}, err
	}
	p := core.DirectoryPage{Next: r.Meta.Next}
	for _, v := range r.Channels {
		kind := core.ConversationChannel
		name := v.Name
		handle := ""
		if v.Private {
			kind = core.ConversationPrivate
		}
		if v.MPIM {
			kind = core.ConversationGroup
		}
		if v.IM {
			kind = core.ConversationDM
			c.mu.Lock()
			profile := c.names[v.User]
			c.mu.Unlock()
			name, handle = profile.Name, profile.Handle
			if name == "" {
				name = v.User
			}
		}
		p.Conversations = append(p.Conversations, core.Conversation{ID: v.ID, Name: name, Handle: handle, User: v.User, Kind: kind, Topic: v.Topic.Value, Purpose: v.Purpose.Value, Members: v.Members})
	}
	return p, nil
}

// History returns a bounded page immediately, without waiting for author lookups.
func (c *Client) History(ctx context.Context, q core.Query) (core.Page, error) {
	method := methodHistory
	v := url.Values{paramChannel: {q.Conversation}, "limit": {strconv.Itoa(historyPageItems)}, paramCursor: {q.Cursor}}
	if q.Target != "" {
		v.Set(paramLatest, q.Target)
		v.Set("inclusive", wireTrue)
		if q.Thread != "" {
			v.Del(paramLatest)
			v.Set(paramOldest, q.Target)
		}
	}
	if q.After != "" {
		v.Set(paramOldest, q.After)
	}
	if q.Before != "" {
		v.Set(paramLatest, q.Before)
	}
	if q.Thread != "" {
		method = methodReplies
		v.Set("ts", q.Thread)
	}
	var r struct {
		Messages []core.Message `json:"messages"`
		More     bool           `json:"has_more"`
		Meta     struct {
			Next string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
	if err := c.call(ctx, method, v, &r); err != nil {
		return core.Page{}, err
	}
	for i := range r.Messages {
		m := &r.Messages[i]
		m.Link = "slack://channel?id=" + url.QueryEscape(q.Conversation) + "&message=" + url.QueryEscape(m.TS)
	}

	return core.Page{Messages: r.Messages, Next: r.Meta.Next, More: r.More || r.Meta.Next != ""}, nil
}

// Send posts once as the authenticated user; it never retries a write.
func (c *Client) Send(ctx context.Context, o core.Outgoing) (core.Message, error) {
	v := url.Values{paramChannel: {o.Conversation}, paramText: {o.Text}, "unfurl_links": {wireFalse}, "unfurl_media": {wireFalse}, "client_msg_id": {o.ID}}
	if o.Thread != "" {
		v.Set("thread_ts", o.Thread)
	}
	var r struct {
		TS      string       `json:"ts"`
		Message core.Message `json:"message"`
	}
	if err := c.call(ctx, methodSend, v, &r); err != nil {
		return core.Message{}, err
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
	if err := c.call(ctx, methodConversation, url.Values{paramChannel: {id}, "include_num_members": {wireTrue}}, &r); err != nil {
		return core.Conversation{}, err
	}
	v := r.Channel
	kind := core.ConversationChannel
	name := v.Name
	handle := ""
	if v.Private {
		kind = core.ConversationPrivate
	}
	if v.MPIM {
		kind = core.ConversationGroup
	}
	if v.IM {
		kind = core.ConversationDM
		profile, _ := c.Profile(ctx, v.User)
		name, handle = profile.Name, profile.Handle
		if name == "" {
			name = v.User
		}
	}
	return core.Conversation{ID: v.ID, Name: name, Handle: handle, Kind: kind, Topic: v.Topic.Value, Purpose: v.Purpose.Value, Members: v.Members}, nil
}

// paramChannel is the shared Web API conversation parameter.
const paramChannel = "channel"

func isWriteMethod(method string) bool {
	return method == methodSend || method == methodReactionAdd || method == methodReactionRemove
}
