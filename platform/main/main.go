// Package main wires together the fetcher, tracer, logger, and pipeline.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"myorg/pipeline"
	"myorg/userfetch"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	names, err := loadUsernames("users.txt")
	if err != nil {
		logger.Error("failed to load usernames", "error", err)
		os.Exit(1)
	}

	fetcher := userfetch.NewRateLimitedFetcher(
		&userfetch.APIFetcher{
			Client:  &http.Client{Timeout: 5 * time.Second},
			BaseURL: mustEnv("USER_API_BASE_URL"),
		},
		10, // requests per second
		5,  // burst
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results := pipeline.Run(ctx, names, 10, func(ctx context.Context, name string) (*userfetch.User, error) {
		return fetcher.Fetch(ctx, name)
	})

	var succeeded, failed int
	for r := range results {
		if r.Err != nil {
			failed++
			logger.Warn("fetch failed", "user", r.Key, "error", r.Err)
			continue
		}
		succeeded++
		process(r.Value)
	}
	logger.Info("batch complete", "succeeded", succeeded, "failed", failed)
}

func process(u *userfetch.User) {
	slog.Info("user fetched", "id", u.ID, "name", u.Name, "email", u.Email)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing required env var: " + key)
	}
	return v
}

func loadUsernames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return lines, nil
}