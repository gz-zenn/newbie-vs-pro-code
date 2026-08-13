// Package userfetch fetches user profiles from a remote API,
// hardened with rate limiting, circuit breaking, and tracing.
package userfetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

// User is a user profile as returned by the API.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Fetcher fetches a single item by key; anything can implement it.
type Fetcher interface {
	Fetch(ctx context.Context, key string) (*User, error)
}

// ErrUpstream is returned when the upstream API responds with a non-OK status.
var ErrUpstream = fmt.Errorf("upstream API error")

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open: upstream considered unhealthy")

// APIFetcher fetches users from a concrete HTTP API.
type APIFetcher struct {
	Client  *http.Client
	BaseURL string
}

// Fetch retrieves a single user by username.
func (f *APIFetcher) Fetch(ctx context.Context, username string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.BaseURL+"/users/"+username, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", username, err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling API for %s: %w", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d for %s", ErrUpstream, resp.StatusCode, username)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decoding response for %s: %w", username, err)
	}
	return &u, nil
}

var tracer = otel.Tracer("userfetch")

// CircuitBreaker is a minimal three-state breaker (closed/open/half-open).
// In production you'd likely use sony/gobreaker or similar; shown inline for clarity.
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration

	failures int
	openedAt time.Time
	state    string // "closed", "open", "half-open"
}

// NewCircuitBreaker creates a breaker that opens after maxFailures consecutive
// failures and tries again after resetTimeout.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{maxFailures: maxFailures, resetTimeout: resetTimeout, state: "closed"}
}

// Allow reports whether a request may proceed through the breaker.
func (cb *CircuitBreaker) Allow() bool {
	if cb.state == "open" && time.Since(cb.openedAt) > cb.resetTimeout {
		cb.state = "half-open"
	}
	return cb.state != "open"
}

// RecordResult feeds the outcome of a request back into the breaker.
func (cb *CircuitBreaker) RecordResult(err error) {
	if err == nil {
		cb.failures = 0
		cb.state = "closed"
		return
	}
	cb.failures++
	if cb.failures >= cb.maxFailures {
		cb.state = "open"
		cb.openedAt = time.Now()
	}
}

// RateLimitedFetcher wraps any Fetcher with a token-bucket limiter,
// a circuit breaker, and distributed tracing spans.
type RateLimitedFetcher struct {
	next    Fetcher
	limiter *rate.Limiter
	breaker *CircuitBreaker
}

// NewRateLimitedFetcher wraps next with the given requests-per-second budget
// and burst size.
func NewRateLimitedFetcher(next Fetcher, rps float64, burst int) *RateLimitedFetcher {
	return &RateLimitedFetcher{
		next:    next,
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
		breaker: NewCircuitBreaker(5, 30*time.Second),
	}
}

// Fetch retrieves a user subject to rate limiting, circuit breaking, and tracing.
func (f *RateLimitedFetcher) Fetch(ctx context.Context, username string) (*User, error) {
	ctx, span := tracer.Start(ctx, "userfetch.Fetch",
		trace.WithAttributes(attribute.String("user.name", username)))
	defer span.End()

	if !f.breaker.Allow() {
		span.RecordError(ErrCircuitOpen)
		return nil, ErrCircuitOpen
	}

	if err := f.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	u, err := f.next.Fetch(ctx, username)
	f.breaker.RecordResult(err)
	if err != nil {
		span.RecordError(err)
	}
	return u, err
}