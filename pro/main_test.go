package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(User{ID: "1", Name: "Ada"})
	}))
	defer server.Close()

	client := server.Client()
	u, err := fetchUser(context.Background(), client, server.URL, "ada")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "Ada" {
		t.Errorf("got %q, want %q", u.Name, "Ada")
	}
}

func TestReadUsernames(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []string
	}{
		{"single", "ada", []string{"ada"}},
		{"multiple", "ada\ngrace\nlinus", []string{"ada", "grace", "linus"}},
		{"trailing newline", "ada\n", []string{"ada"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := t.TempDir() + "/users.txt"
			if err := os.WriteFile(path, []byte(tt.data), 0o644); err != nil {
				t.Fatalf("writing fixture: %v", err)
			}
			got, err := readUsernames(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %q, want %q", got[i], tt.want[i])
				}
			}
		})
	}
}