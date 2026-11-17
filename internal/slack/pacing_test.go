package slack

import (
	"context"
	"errors"
	"testing"
	"time"
)

const testPacingToken = "offline-test"

func TestForegroundReadsPrecedeQueuedBackgroundWithoutBypassingBackoff(t *testing.T) {
	c := New(testPacingToken)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.next[methodHistory] = time.Now().Add(time.Hour)
	completed := make(chan int, 4)
	for i := range 3 {
		go func() {
			if c.wait(ctx, methodHistory) == nil {
				completed <- i
			}
		}()
		awaitRequestCount(ctx, t, c, methodHistory, i+1)
	}
	go func() {
		if c.wait(ForegroundRead(ctx), methodHistory) == nil {
			completed <- 3
		}
	}()
	awaitRequestCount(ctx, t, c, methodHistory, 4)
	select {
	case <-completed:
		t.Fatal("foreground bypassed the existing deadline")
	default:
	}
	// Other methods remain independent of the blocked history queue.
	if err := c.wait(ctx, methodReplies); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{3, 0, 1, 2} {
		releaseRequest(c, methodHistory)
		select {
		case got := <-completed:
			if got != want {
				t.Fatalf("completed %d, want %d", got, want)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		c.mu.Lock()
		paced := time.Until(c.next[methodHistory]) > 0
		c.mu.Unlock()
		if !paced {
			t.Fatal("request did not preserve pacing")
		}
	}
	awaitRequestCount(ctx, t, c, methodHistory, 0)
}

func TestCancelForegroundWaitReleasesBackgroundQueue(t *testing.T) {
	c := New(testPacingToken)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.next[methodHistory] = time.Now().Add(time.Hour)
	foreground, cancelForeground := context.WithCancel(ForegroundRead(ctx))
	done := make(chan error, 1)
	go func() { done <- c.wait(foreground, methodHistory) }()
	awaitRequestCount(ctx, t, c, methodHistory, 1)
	cancelForeground()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	awaitRequestCount(ctx, t, c, methodHistory, 0)
	releaseRequest(c, methodHistory)
	if err := c.wait(ctx, methodHistory); err != nil {
		t.Fatal("canceled foreground stranded background work", err)
	}
}

func awaitRequestCount(ctx context.Context, t *testing.T, c *Client, method string, count int) {
	t.Helper()
	for {
		c.mu.Lock()
		got, changed := len(c.requests.methods[method]), c.requests.changed
		c.mu.Unlock()
		if got == count {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting requests = %d, want %d: %v", got, count, ctx.Err())
		case <-changed:
		}
	}
}

func releaseRequest(c *Client, method string) {
	c.mu.Lock()
	c.next[method] = time.Time{}
	c.requests.notify()
	c.mu.Unlock()
}
