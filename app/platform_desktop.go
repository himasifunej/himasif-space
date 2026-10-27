//go:build !js

package main

import (
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// clientName names this app to the server at login.
const clientName = "desktop"

// defaultAPIURL is the server of the desktop app, unless SPACE_API_URL
// names another. Builds may set it:
//
//	-ldflags "-X main.defaultAPIURL=https://staging.himasif.id"
var defaultAPIURL = "https://app.himasif.id"

// secretKey is the key of the session token in the credential store.
const secretKey = "session-token"

// apiURL returns the URL of the API server.
func apiURL() string {
	if u := strings.TrimSpace(os.Getenv("SPACE_API_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return defaultAPIURL
}

// secretTokens keeps the token in the system's credential store, or in
// memory where there is none, as on Linux without a Secret Service.
type secretTokens struct {
	mu          sync.Mutex
	unsupported bool
	mem         string
}

func newTokenStore() *secretTokens { return &secretTokens{} }

// fallback reports whether err means there is no credential store, and
// from then on keeps the token in memory.
func (s *secretTokens) fallback(err error) bool {
	if errors.Is(err, errors.ErrUnsupported) {
		s.unsupported = true
		return true
	}
	return false
}

func (s *secretTokens) Load() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unsupported {
		return s.mem, nil
	}
	b, err := mygo.Secrets.Get(secretKey)
	switch {
	case err == nil:
		return string(b), nil
	case errors.Is(err, mygo.ErrSecretNotFound):
		return "", nil
	case s.fallback(err):
		return s.mem, nil
	}
	return "", err
}

func (s *secretTokens) Save(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = token
	if s.unsupported {
		return nil
	}
	if err := mygo.Secrets.Set(secretKey, []byte(token)); err != nil && !s.fallback(err) {
		return err
	}
	return nil
}

func (s *secretTokens) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = ""
	if s.unsupported {
		return nil
	}
	if err := mygo.Secrets.Delete(secretKey); err != nil && !s.fallback(err) {
		return err
	}
	return nil
}
