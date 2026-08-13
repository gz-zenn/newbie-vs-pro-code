package userfetch

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type stubFetcher struct {
	errs map[string]error
}

func (s stubFetcher) Fetch(_ context.Context, username string) (*User, error) {
	if err := s.errs[username]; err != nil {
		return nil, err
	}
	return &User{ID: username, Name: username}, nil
}

func TestFetchAll(t *testing.T) {
	f := stubFetcher{errs: map[string]error{"bad": errors.New("boom")}}
	names := []string{"ada", "grace", "bad", "linus"}

	users, errs := FetchAll(context.Background(), f, names, 2)

	if len(users) != 3 {
		t.Errorf("got %d users, want 3", len(users))
	}
	if len(errs) != 1 {
		t.Errorf("got %d errors, want 1", len(errs))
	}
	if _, ok := errs["bad"]; !ok {
		t.Errorf("missing expected error for user %q", "bad")
	}
}

func TestFetchAllConcurrencyBounded(t *testing.T) {
	const max = 2
	tracker := &concurrencyTracker{max: max}
	f := trackingFetcher{max: max, inFlight: tracker}

	users, errs := FetchAll(context.Background(), f, []string{"a", "b", "c", "d", "e"}, max)
	if len(users) != 5 || len(errs) != 0 {
		t.Fatalf("got %d users, %d errors; want 5, 0", len(users), len(errs))
	}
	tracker.assertMax(t)
}

type concurrencyTracker struct {
	mu      sync.Mutex
	max     int
	current int
	peak    int
}

func (c *concurrencyTracker) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current++
	if c.current > c.peak {
		c.peak = c.current
	}
}

func (c *concurrencyTracker) dec() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current--
}

func (c *concurrencyTracker) assertMax(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.peak > c.max {
		t.Errorf("peak concurrency %d exceeded limit %d", c.peak, c.max)
	}
}

type trackingFetcher struct {
	max      int
	inFlight *concurrencyTracker
}

func (f trackingFetcher) Fetch(ctx context.Context, _ string) (*User, error) {
	f.inFlight.inc()
	defer f.inFlight.dec()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &User{}, nil
}