// Package main wires together configuration, the fetcher, and observability.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"myorg/userfetch"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	names, err := loadUsernames("users.txt")
	if err != nil {
		logger.Error("failed to load usernames", "error", err)
		os.Exit(1)
	}

	fetcher := &userfetch.APIFetcher{
		Client:  &http.Client{Timeout: 5 * time.Second},
		BaseURL: mustEnv("USER_API_BASE_URL"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	users, errs := userfetch.FetchAll(ctx, fetcher, names, 10)
	logger.Info("batch complete",
		"succeeded", len(users),
		"failed", len(errs),
		"duration", time.Since(start),
	)

	for name, err := range errs {
		logger.Warn("failed to fetch user", "user", name, "error", err)
	}
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