// Package client talks to Space's API server for the desktop and web apps.
// It builds for js/wasm too, where net/http goes through the browser's
// fetch.
package client

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

	"github.com/himasifunej/himasif-space/internal/api"
)

// DefaultTimeout bounds each request when Client.HTTP is nil.
const DefaultTimeout = 15 * time.Second

// maxBody bounds the bytes read from a response.
const maxBody = 4 << 20

// Client calls the API at BaseURL, as "https://app.himasif.id", sending
// Token, when set, as a bearer token.
//
// Its methods are safe for concurrent use. Set the Token field only before
// the first request; afterwards, as when signing in or out while other
// requests may run, change it with SetToken.
type Client struct {
	BaseURL string
	Token   string
	// HTTP sends the requests; nil uses a client with DefaultTimeout.
	HTTP *http.Client

	mu sync.Mutex // guards Token after the first request
}

// New returns a client of the API at baseURL.
func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: DefaultTimeout}}
}

// Error is a response of the server other than 2xx. Message is in
// Indonesian and can be shown to users.
type Error struct {
	Status  int
	Code    string
	Message string
	Data    any
}

func (e *Error) Error() string {
	return fmt.Sprintf("api: %d %s: %s", e.Status, e.Code, e.Message)
}

// UnreachableError is a request that got no answer from the API: the
// network is down, the server does not answer in time, or a proxy in front
// of it reports it unavailable.
type UnreachableError struct {
	Err error
}

func (e *UnreachableError) Error() string { return "api tidak dapat dihubungi: " + e.Err.Error() }
func (e *UnreachableError) Unwrap() error { return e.Err }

// IsUnreachable reports whether err means the API could not be reached, as
// opposed to the API answering with an error.
func IsUnreachable(err error) bool {
	var u *UnreachableError
	return errors.As(err, &u)
}

// IsUnauthenticated reports whether err is the server rejecting the
// session or the credentials (401).
func IsUnauthenticated(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.Status == http.StatusUnauthorized || e.Code == api.CodeUnauthenticated)
}

// SetToken sets the session token that later requests send; "" sends none.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.Token = token
	c.mu.Unlock()
}

// token returns the session token.
func (c *Client) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Token
}

// Health reports whether the server and its database answer.
func (c *Client) Health(ctx context.Context) (api.Health, error) {
	var h api.Health
	err := c.do(ctx, http.MethodGet, "/api/health", nil, &h)
	return h, err
}

// Login signs in. It does not set the client's token: the caller keeps the
// token it returns, and passes it to SetToken.
func (c *Client) Login(ctx context.Context, req api.LoginRequest) (api.LoginResponse, error) {
	var r api.LoginResponse
	err := c.do(ctx, http.MethodPost, "/api/auth/login", req, &r)
	return r, err
}

// Logout ends the session of the client's token on the server. It leaves
// the token set; the caller clears it.
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/auth/logout", nil, nil)
}

// Me returns the account of the client's token.
func (c *Client) Me(ctx context.Context) (api.User, error) {
	var u api.User
	err := c.do(ctx, http.MethodGet, "/api/me", nil, &u)
	return u, err
}

// Periods lists the periods (kepengurusan).
func (c *Client) Periods(ctx context.Context) ([]api.Period, error) {
	var l api.List[api.Period]
	err := c.do(ctx, http.MethodGet, "/api/periods", nil, &l)
	return l.Items, err
}

// Divisions lists the divisions.
func (c *Client) Divisions(ctx context.Context) ([]api.Division, error) {
	var l api.List[api.Division]
	err := c.do(ctx, http.MethodGet, "/api/divisions", nil, &l)
	return l.Items, err
}

// do sends a request with body as JSON, when not nil, and decodes a 2xx
// response into out, when not nil.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: DefaultTimeout}
	}
	resp, err := hc.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return &UnreachableError{Err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return &UnreachableError{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return responseError(resp.StatusCode, data)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("api: decoding %s %s: %w", method, path, err)
	}
	return nil
}

// responseError returns the error of a failed response: the server's
// api.ErrorResponse, or, for a body that is not one, a gateway error as
// unreachable and anything else as an *Error of its status.
func responseError(status int, data []byte) error {
	var er api.ErrorResponse
	if json.Unmarshal(data, &er) == nil && er.Error.Code != "" {
		return &Error{Status: status, Code: er.Error.Code, Message: er.Error.Message, Data: er.Error.Data}
	}
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return &UnreachableError{Err: fmt.Errorf("status %d", status)}
	}
	return &Error{Status: status, Code: codeOf(status), Message: fmt.Sprintf("Server menjawab dengan kesalahan (%d).", status)}
}

// codeOf returns the error code that matches an HTTP status.
func codeOf(status int) string {
	switch status {
	case http.StatusBadRequest:
		return api.CodeBadRequest
	case http.StatusUnauthorized:
		return api.CodeUnauthenticated
	case http.StatusForbidden:
		return api.CodeForbidden
	case http.StatusNotFound:
		return api.CodeNotFound
	case http.StatusTooManyRequests:
		return api.CodeRateLimited
	}
	return api.CodeInternal
}
