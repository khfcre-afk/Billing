package pterodactyl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/application/users" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing auth")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"attributes": map[string]any{"id": 42, "email": "a@b.c", "username": "ab"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "app-key", "")
	u, err := c.CreateUser(context.Background(), CreateUserInput{Email: "a@b.c", Username: "ab", FirstName: "A", LastName: "B", Password: "xx"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 42 {
		t.Fatalf("got %d want 42", u.ID)
	}
}

func TestRetryOn5xxThenSuccess(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"attributes": map[string]any{"id": 1}})
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.CreateUser(ctx, CreateUserInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"code":"validation"}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k", "")
	if _, err := c.CreateUser(context.Background(), CreateUserInput{}); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", calls)
	}
}
