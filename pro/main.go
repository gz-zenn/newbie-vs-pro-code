package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func readUsernames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return lines, nil
}

func fetchUser(ctx context.Context, client *http.Client, baseURL, name string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/users/"+name, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for user %s", resp.StatusCode, name)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decoding user %s: %w", name, err)
	}
	return &u, nil
}

func main() {
	names, err := readUsernames("users.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, name := range names {
		user, err := fetchUser(ctx, client, "https://api.example.com", name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning:", err)
			continue
		}
		fmt.Printf("%+v\n", user)
	}
}