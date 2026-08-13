package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRun(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "e"}
	got := make(map[string]int)
	job := func(_ context.Context, key string) (int, error) {
		if key == "c" {
			return 0, errors.New("boom")
		}
		return len(key), nil
	}

	results := Run(context.Background(), keys, 2, job)
	for r := range results {
		got[r.Key] = r.Value
		if r.Key == "c" && r.Err == nil {
			t.Error("expected error for key c, got nil")
		}
		if r.Key != "c" && r.Err != nil {
			t.Errorf("unexpected error for key %s: %v", r.Key, r.Err)
		}
	}
	for _, k := range keys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing result for key %s", k)
		}
	}
}

func TestRunConcurrencyBounded(t *testing.T) {
	const max = 2
	tr := &tracker{max: max}
	job := func(ctx context.Context, _ string) (int, error) {
		tr.inc()
		defer tr.dec()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		return 1, nil
	}

	results := Run(context.Background(), []string{"a", "b", "c", "d", "e"}, max, job)
	for r := range results {
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
	}
	tr.assertMax(t)
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	job := func(ctx context.Context, _ string) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	results := Run(ctx, []string{"a", "b", "c"}, 2, job)
	_ = results
	cancel()

	// The producer should stop sending and close the channel once ctx is done.
	for r := range results {
		if r.Err == nil {
			t.Errorf("expected cancelled error, got nil result %+v", r)
		}
	}
}

type tracker struct {
	mu      sync.Mutex
	max     int
	current int
	peak    int
}

func (t *tracker) inc() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current++
	if t.current > t.peak {
		t.peak = t.current
	}
}

func (t *tracker) dec() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current--
}

func (t *tracker) assertMax(test *testing.T) {
	test.Helper()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.peak > t.max {
		test.Errorf("peak concurrency %d exceeded limit %d", t.peak, t.max)
	}
}