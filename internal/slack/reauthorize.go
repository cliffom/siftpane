package slack

import (
	"context"
	"fmt"

	"github.com/cliffom/siftpane/internal/core"
)

// Reauthorize validates a replacement without mutating the original client.
// Both clients share method deadlines and name caches under the same lock, so
// in-flight writes keep their original token and Retry-After survives reload.
func (c *Client) Reauthorize(ctx context.Context, token string, expected core.Identity) (*Client, core.Identity, error) {
	next := &Client{token: token, base: c.base, http: c.http, mu: c.mu, next: c.next, names: c.names, profilesInFlight: c.profilesInFlight}
	id, err := next.Identity(ctx)
	if err != nil {
		return nil, core.Identity{}, err
	}
	if id.Workspace != expected.Workspace || id.User != expected.User {
		return nil, core.Identity{}, fmt.Errorf("replacement belongs to another account; run --login --account for the intended workspace/user")
	}
	return next, id, nil
}
