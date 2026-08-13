// Package userfetch fetches user profiles from a remote API.
package userfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/sync/errgroup"
)

// User is a user profile as returned by the API.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Fetcher fetches user profiles from a remote API.
// It's a small interface so callers can mock it in tests.
type Fetcher interface {
	Fetch(ctx context.Context, username string) (*User, error)
}

// ErrUpstream is returned when the upstream API responds with a non-OK status.
var ErrUpstream = fmt.Errorf("upstream API error")

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

// FetchAll fetches many users concurrently, bounded by maxConcurrency.
// Partial failures are collected rather than aborting the whole batch.
func FetchAll(ctx context.Context, f Fetcher, usernames []string, maxConcurrency int) ([]*User, map[string]error) {
	sem := make(chan struct{}, maxConcurrency)
	var mu sync.Mutex
	users := make([]*User, 0, len(usernames))
	errs := make(map[string]error)

	g, ctx := errgroup.WithContext(ctx)

	for _, name := range usernames {
		name := name
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			u, err := f.Fetch(ctx, name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[name] = err
				return nil // don't abort the whole group on one failure
			}
			users = append(users, u)
			return nil
		})
	}
	_ = g.Wait() // errors are collected per-user, not returned here

	return users, errs
}