package slack

import (
	"context"
	"time"
)

type foregroundReadKey struct{}

// ForegroundRead gives a selected view's reads priority over queued background
// work for the same method. It never bypasses pacing or server Retry-After.
func ForegroundRead(ctx context.Context) context.Context {
	return context.WithValue(ctx, foregroundReadKey{}, true)
}

type requestWaiter struct {
	foreground bool
}

// Protected by Client.mu and shared with replacement credentials.
type requestQueue struct {
	methods map[string][]*requestWaiter
	changed chan struct{}
}

func newRequestQueue() *requestQueue {
	return &requestQueue{methods: map[string][]*requestWaiter{}, changed: make(chan struct{})}
}

func (q *requestQueue) notify() {
	close(q.changed)
	q.changed = make(chan struct{})
}

func (q *requestQueue) first(method string) *requestWaiter {
	queue := q.methods[method]
	for _, w := range queue {
		if w.foreground {
			return w
		}
	}
	if len(queue) > 0 {
		return queue[0]
	}
	return nil
}

func (q *requestQueue) remove(method string, waiter *requestWaiter) {
	queue := q.methods[method]
	for i, w := range queue {
		if w == waiter {
			q.methods[method] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	if len(q.methods[method]) == 0 {
		delete(q.methods, method)
	}
	q.notify()
}

// Waiting requests have an explicit order instead of racing at every deadline.
// Only the first eligible waiter owns a timer; cancellation wakes its successor.
func (c *Client) wait(ctx context.Context, method string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	foreground, _ := ctx.Value(foregroundReadKey{}).(bool)
	w := &requestWaiter{foreground: foreground && !isWriteMethod(method)}
	c.mu.Lock()
	c.requests.methods[method] = append(c.requests.methods[method], w)
	c.requests.notify()
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.requests.remove(method, w)
		c.mu.Unlock()
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		first := c.requests.first(method) == w
		delay := time.Until(c.next[method])
		if first && delay <= 0 {
			c.next[method] = time.Now().Add(methodGap(method))
			c.mu.Unlock()
			return nil
		}
		changed := c.requests.changed
		c.mu.Unlock()
		if err := waitForRequest(ctx, changed, first, delay); err != nil {
			return err
		}
	}
}

func waitForRequest(ctx context.Context, changed <-chan struct{}, first bool, delay time.Duration) error {
	var ready <-chan time.Time
	if first {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		ready = timer.C
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-changed:
	case <-ready:
	}
	return nil
}

func methodGap(method string) time.Duration {
	switch method {
	case methodUser:
		return userLookupInterval
	case methodDirectory:
		return directoryInterval
	case methodReactionRemove:
		return reactionRemoveInterval
	case methodMentions:
		return mentionInterval
	default:
		return methodInterval
	}
}
