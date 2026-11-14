package slack

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

type profileCall struct {
	done    chan struct{}
	profile core.Profile
	err     error
}

// Profile resolves one display identity, coalescing concurrent lookups and using
// the same paced users.info calls as directory metadata. Failures are not cached.
func (c *Client) Profile(ctx context.Context, id string) (core.Profile, error) {
	if id == "" {
		return core.Profile{Name: "Slack"}, nil
	}
	if err := ctx.Err(); err != nil {
		return core.Profile{}, err
	}
	for {
		c.mu.Lock()
		if p := c.names[id]; p.Name != "" {
			c.mu.Unlock()
			return p, nil
		}
		if pending := c.profilesInFlight[id]; pending != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return core.Profile{}, ctx.Err()
			case <-pending.done:
				// A canceled leader must not cancel other consumers' live contexts.
				if (errors.Is(pending.err, context.Canceled) || errors.Is(pending.err, context.DeadlineExceeded)) && ctx.Err() == nil {
					continue
				}
				return pending.profile, pending.err
			}
		}
		pending := &profileCall{done: make(chan struct{})}
		c.profilesInFlight[id] = pending
		c.mu.Unlock()
		return c.finishProfile(ctx, id, pending)
	}
}

func (c *Client) finishProfile(ctx context.Context, id string, pending *profileCall) (core.Profile, error) {
	p, err := c.lookupProfile(ctx, id)
	c.mu.Lock()
	if err == nil {
		if len(c.names) >= authorCacheEntries {
			c.evictProfile()
		}
		c.names[id] = p
	}
	pending.profile, pending.err = p, err
	delete(c.profilesInFlight, id)
	close(pending.done)
	c.mu.Unlock()
	return p, err
}

func (c *Client) lookupProfile(ctx context.Context, id string) (core.Profile, error) {
	var r struct {
		User struct {
			Name    string `json:"name"`
			Real    string `json:"real_name"`
			Profile struct {
				Display string `json:"display_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := c.call(ctx, methodUser, url.Values{"user": {id}}, &r); err != nil {
		if ctx.Err() != nil {
			return core.Profile{}, ctx.Err()
		}
		return core.Profile{}, err
	}
	name := r.User.Profile.Display
	if name == "" {
		name = r.User.Real
	}
	if name == "" {
		name = r.User.Name
	}
	if name == "" {
		name = id
	}
	return core.Profile{ID: id, Name: name, Handle: r.User.Name, Updated: time.Now()}, nil
}

// CachedProfiles returns a deterministic copy of the bounded display cache.
func (c *Client) CachedProfiles() []core.Profile {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]core.Profile, 0, len(c.names))
	for _, p := range c.names {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RestoreProfiles seeds this authenticated client's cache from validated summaries.
// The caller must isolate the data by workspace and user before calling it.
func (c *Client) RestoreProfiles(ps []core.Profile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range ps {
		if len(c.names) >= authorCacheEntries {
			break
		}
		if p.ID != "" && p.Name != "" {
			c.names[p.ID] = p
		}
	}
}

// Called with mu held; evict the oldest successful lookup, keeping useful names.
func (c *Client) evictProfile() {
	var oldest core.Profile
	for _, p := range c.names {
		if oldest.ID == "" || p.Updated.Before(oldest.Updated) {
			oldest = p
		}
	}
	delete(c.names, oldest.ID)
}
