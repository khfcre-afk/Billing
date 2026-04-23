// Package pterodactyl is a typed client for Pterodactyl Panel Application API.
//
// It implements the subset of calls needed by the billing platform:
// users, servers (create/suspend/unsuspend/delete), and power actions via
// the Client API. All outbound calls are wrapped in an exponential-backoff
// retry and a simple circuit breaker; secrets are kept in the struct and
// never logged.
package pterodactyl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Client struct {
	base         string
	appKey       string
	clientKey    string
	hc           *http.Client
	breaker      *breaker
}

func New(baseURL, appKey, clientKey string) *Client {
	return &Client{
		base:      strings.TrimRight(baseURL, "/"),
		appKey:    appKey,
		clientKey: clientKey,
		hc:        &http.Client{Timeout: 20 * time.Second},
		breaker:   newBreaker(5, 30*time.Second),
	}
}

type CreateUserInput struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

type User struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type CreateServerInput struct {
	Name           string            `json:"name"`
	UserID         int               `json:"user"`
	EggID          int               `json:"egg"`
	NestID         int               `json:"nest"`
	DockerImage    string            `json:"docker_image"`
	StartupCommand string            `json:"startup"`
	Environment    map[string]string `json:"environment"`
	Limits         Limits            `json:"limits"`
	FeatureLimits  FeatureLimits     `json:"feature_limits"`
	Allocation     Allocation        `json:"allocation"`
}

type Limits struct {
	Memory int `json:"memory"`
	Swap   int `json:"swap"`
	Disk   int `json:"disk"`
	IO     int `json:"io"`
	CPU    int `json:"cpu"`
}

type FeatureLimits struct {
	Databases   int `json:"databases"`
	Backups     int `json:"backups"`
	Allocations int `json:"allocations"`
}

type Allocation struct {
	Default int `json:"default"`
}

type Server struct {
	ID          int    `json:"id"`
	Identifier  string `json:"identifier"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Suspended   bool   `json:"suspended"`
	Allocation  int    `json:"allocation"`
}

type AllocationInfo struct {
	ID   int    `json:"id"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type PowerAction string

const (
	PowerStart   PowerAction = "start"
	PowerStop    PowerAction = "stop"
	PowerRestart PowerAction = "restart"
	PowerKill    PowerAction = "kill"
)

// --- Application API ---

func (c *Client) CreateUser(ctx context.Context, in CreateUserInput) (*User, error) {
	var resp struct {
		Attributes User `json:"attributes"`
	}
	if err := c.doApp(ctx, http.MethodPost, "/api/application/users", in, &resp); err != nil {
		return nil, err
	}
	return &resp.Attributes, nil
}

func (c *Client) CreateServer(ctx context.Context, in CreateServerInput) (*Server, error) {
	var resp struct {
		Attributes Server `json:"attributes"`
	}
	if err := c.doApp(ctx, http.MethodPost, "/api/application/servers", in, &resp); err != nil {
		return nil, err
	}
	return &resp.Attributes, nil
}

func (c *Client) Suspend(ctx context.Context, serverID int) error {
	return c.doApp(ctx, http.MethodPost, fmt.Sprintf("/api/application/servers/%d/suspend", serverID), nil, nil)
}

func (c *Client) Unsuspend(ctx context.Context, serverID int) error {
	return c.doApp(ctx, http.MethodPost, fmt.Sprintf("/api/application/servers/%d/unsuspend", serverID), nil, nil)
}

func (c *Client) DeleteServer(ctx context.Context, serverID int) error {
	return c.doApp(ctx, http.MethodDelete, fmt.Sprintf("/api/application/servers/%d", serverID), nil, nil)
}

func (c *Client) ListFreeAllocations(ctx context.Context, nodeID int) ([]AllocationInfo, error) {
	var resp struct {
		Data []struct {
			Attributes struct {
				AllocationInfo
				Assigned bool `json:"assigned"`
			} `json:"attributes"`
		} `json:"data"`
	}
	path := fmt.Sprintf("/api/application/nodes/%d/allocations?per_page=100", nodeID)
	if err := c.doApp(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]AllocationInfo, 0, len(resp.Data))
	for _, d := range resp.Data {
		if !d.Attributes.Assigned {
			out = append(out, d.Attributes.AllocationInfo)
		}
	}
	return out, nil
}

// --- Client API (power actions on behalf of server owners) ---

func (c *Client) Power(ctx context.Context, identifier string, action PowerAction) error {
	if c.clientKey == "" {
		return errors.New("pterodactyl client api key not configured")
	}
	payload := map[string]string{"signal": string(action)}
	return c.doClient(ctx, http.MethodPost, fmt.Sprintf("/api/client/servers/%s/power", identifier), payload, nil)
}

// --- internal helpers ---

func (c *Client) doApp(ctx context.Context, method, path string, body, out any) error {
	return c.do(ctx, method, path, body, out, c.appKey, "application/vnd.pterodactyl.v1+json")
}

func (c *Client) doClient(ctx context.Context, method, path string, body, out any) error {
	return c.do(ctx, method, path, body, out, c.clientKey, "application/vnd.pterodactyl.v1+json")
}

func (c *Client) do(ctx context.Context, method, path string, body, out any, token, accept string) error {
	if !c.breaker.allow() {
		return fmt.Errorf("pterodactyl: circuit breaker open")
	}

	const maxAttempts = 3
	var lastErr error
	backoff := 250 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := c.roundtrip(ctx, method, path, body, out, token, accept)
		if err == nil {
			c.breaker.success()
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			c.breaker.failure()
			return err
		}
		log.Warn().Err(err).Int("attempt", attempt).Str("path", path).Msg("pterodactyl retrying")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	c.breaker.failure()
	return fmt.Errorf("pterodactyl: max retries exceeded: %w", lastErr)
}

func (c *Client) roundtrip(ctx context.Context, method, path string, body, out any, token, accept string) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return &retryableError{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		raw, _ := io.ReadAll(resp.Body)
		return &retryableError{err: fmt.Errorf("ptero %d: %s", resp.StatusCode, string(raw))}
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ptero %d: %s", resp.StatusCode, string(raw))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

type retryableError struct{ err error }

func (r *retryableError) Error() string { return r.err.Error() }
func (r *retryableError) Unwrap() error { return r.err }

func isRetryable(err error) bool {
	var r *retryableError
	return errors.As(err, &r)
}

// --- circuit breaker ---

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

type breaker struct {
	mu           sync.Mutex
	state        breakerState
	failures     int
	threshold    int
	openUntil    time.Time
	cooldown     time.Duration
}

func newBreaker(threshold int, cooldown time.Duration) *breaker {
	return &breaker{threshold: threshold, cooldown: cooldown}
}

func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == breakerOpen {
		if time.Now().After(b.openUntil) {
			b.state = breakerHalfOpen
			return true
		}
		return false
	}
	return true
}

func (b *breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = breakerClosed
}

func (b *breaker) failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.state = breakerOpen
		b.openUntil = time.Now().Add(b.cooldown)
	}
}
