package userfetch

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubFetcher struct {
	fn func(ctx context.Context, key string) (*User, error)
}

func (s stubFetcher) Fetch(ctx context.Context, key string) (*User, error) {
	return s.fn(ctx, key)
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("iteration %d: Allow() = false, want true", i)
		}
		cb.RecordResult(errors.New("boom"))
	}
	if cb.Allow() {
		t.Fatal("Allow() = true, want false after max failures")
	}

	cb.RecordResult(nil)
	if !cb.Allow() {
		t.Fatal("Allow() = false, want true after successful result")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)
	cb.RecordResult(errors.New("boom"))
	cb.RecordResult(errors.New("boom"))
	if cb.Allow() {
		t.Fatal("Allow() = true, want false while open")
	}
	time.Sleep(15 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("Allow() = false, want true after reset timeout (half-open)")
	}
}

func TestRateLimitedFetcherOpenBreaker(t *testing.T) {
	fail := errors.New("upstream down")
	f := NewRateLimitedFetcher(stubFetcher{fn: func(context.Context, string) (*User, error) {
		return nil, fail
	}}, 1000, 100)

	// Open the breaker after 5 consecutive failures.
	for i := 0; i < 5; i++ {
		if _, err := f.Fetch(context.Background(), "ada"); err == nil {
			t.Fatal("expected error from failing upstream")
		}
	}

	_, err := f.Fetch(context.Background(), "ada")
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("got %v, want ErrCircuitOpen", err)
	}
}

func TestRateLimitedFetcherSuccess(t *testing.T) {
	f := NewRateLimitedFetcher(stubFetcher{fn: func(_ context.Context, key string) (*User, error) {
		return &User{ID: key, Name: key}, nil
	}}, 1000, 100)

	u, err := f.Fetch(context.Background(), "grace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "grace" {
		t.Fatalf("got %q, want %q", u.Name, "grace")
	}
}