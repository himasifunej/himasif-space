package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/config"
)

// A full limiter evicts the key used least recently instead of refusing
// every new key: filling the table with made-up emails must not lock
// everyone out of login (SEC-02).
func TestFullLimiterEvictsLeastRecentlyUsedKey(t *testing.T) {
	l := newLimiter(2, time.Minute)
	l.maxKeys = 2
	now := time.Now()
	if !l.allow("a", now) || !l.allow("b", now) || !l.allow("a", now) {
		t.Fatal("initial identities refused")
	}
	if !l.allow("c", now) {
		t.Fatal("unseen identity refused at capacity")
	}
	if _, ok := l.hits["b"]; ok {
		t.Fatal("least recently used key b was not evicted")
	}
	if l.allow("a", now) {
		t.Fatal("recently used key a lost its count")
	}
	if len(l.hits) != 2 || l.order.Len() != 2 {
		t.Fatalf("limiter holds %d keys, order %d", len(l.hits), l.order.Len())
	}
}

func TestFullLimiterKeepsAcceptingNewKeys(t *testing.T) {
	l := newLimiter(10, 15*time.Minute)
	l.maxKeys = 100
	now := time.Now()
	for i := range 5000 {
		if !l.allow(fmt.Sprintf("email:spam%d@example.test", i), now) {
			t.Fatalf("new key %d refused", i)
		}
	}
	if !l.allow("email:anggota@himasif.test", now) {
		t.Fatal("a real member is locked out after a flood of made-up keys")
	}
	if len(l.hits) > 100 || l.order.Len() != len(l.hits) {
		t.Fatalf("limiter grew to %d keys (order %d)", len(l.hits), l.order.Len())
	}
}

func TestLimiterForgive(t *testing.T) {
	l := newLimiter(2, time.Minute)
	t0 := time.Now()
	l.allow("k", t0)
	l.allow("k", t0.Add(time.Second))
	if l.allow("k", t0.Add(2*time.Second)) {
		t.Fatal("third allowed")
	}
	l.forgive("k", t0.Add(time.Second))
	if !l.allow("k", t0.Add(3*time.Second)) {
		t.Fatal("forgiven event still counted")
	}
	l.forgive("missing", t0) // no key: nothing happens
	l2 := newLimiter(2, time.Minute)
	l2.allow("only", t0)
	l2.forgive("only", t0)
	if len(l2.hits) != 0 || l2.order.Len() != 0 {
		t.Fatal("a key with no events left is kept")
	}
}

func TestBlockedLoginAddressDoesNotRetainNewEmails(t *testing.T) {
	now := time.Now()
	s := &Server{now: func() time.Time { return now }, loginLimit: newLimiter(10, 15*time.Minute)}
	for range 10 {
		s.loginLimit.allow("ip:198.51.100.7", now)
	}
	for i := range 20 {
		body, _ := json.Marshal(map[string]string{"email": strings.Repeat("a", i+1) + "@example.test", "password": "some-password"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		r.RemoteAddr = "198.51.100.7:1234"
		w := httptest.NewRecorder()
		s.login(w, r)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("blocked request = %d", w.Code)
		}
	}
	if len(s.loginLimit.hits) != 1 {
		t.Fatalf("blocked IP retained additional identities: %d", len(s.loginLimit.hits))
	}
}

func TestOversizedLoginEmailIsRejectedBeforeRetention(t *testing.T) {
	now := time.Now()
	s := &Server{now: func() time.Time { return now }, loginLimit: newLimiter(10, 15*time.Minute)}
	body, _ := json.Marshal(map[string]string{"email": strings.Repeat("a", 500_000) + "@example.test", "password": "some-password"})
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.login(w, r)
	if w.Code != http.StatusBadRequest || len(s.loginLimit.hits) != 0 {
		t.Fatalf("oversized email: status %d, retained identities %d", w.Code, len(s.loginLimit.hits))
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	l := newLimiter(2, time.Minute)
	t0 := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	if !l.allow("k", t0) || !l.allow("k", t0.Add(time.Second)) {
		t.Fatal("first two refused")
	}
	if l.allow("k", t0.Add(2*time.Second)) {
		t.Fatal("third allowed within the window")
	}
	if !l.allow("other", t0.Add(2*time.Second)) {
		t.Fatal("keys share a count")
	}
	if !l.allow("k", t0.Add(61*time.Second)) {
		t.Fatal("refused after the first left the window")
	}
}

func ipRequest(remote string, headers ...string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Add(headers[i], headers[i+1])
	}
	return r
}

// Behind Cloudflare Tunnel the client writes X-Forwarded-For itself; its
// leftmost entry must never become the rate limit key (SEC-02).
func TestClientIPIgnoresSpoofedForwardedFor(t *testing.T) {
	c := newClientIPs(config.Config{})
	cases := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"spoofed XFF through the tunnel", ipRequest("127.0.0.1:5000", "X-Forwarded-For", "1.2.3.4", "CF-Connecting-IP", "203.0.113.9"), "203.0.113.9"},
		{"XFF alone through the tunnel", ipRequest("127.0.0.1:5000", "X-Forwarded-For", "1.2.3.4"), "127.0.0.1"},
		{"CF header from a stranger", ipRequest("198.51.100.7:1234", "CF-Connecting-IP", "1.2.3.4"), "198.51.100.7"},
		{"XFF from a stranger", ipRequest("198.51.100.7:1234", "X-Forwarded-For", "1.2.3.4"), "198.51.100.7"},
		{"garbage CF header", ipRequest("127.0.0.1:5000", "CF-Connecting-IP", "not-an-ip"), "127.0.0.1"},
		{"IPv6 client", ipRequest("[::1]:5000", "CF-Connecting-IP", "2001:db8::1"), "2001:db8::1"},
		{"Coolify proxy on a private network", ipRequest("10.0.1.5:1234", "CF-Connecting-IP", "203.0.113.9"), "203.0.113.9"},
	}
	for _, tc := range cases {
		if got := c.resolve(tc.r); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestClientIPForwardedForTakesRightmostUntrustedHop(t *testing.T) {
	trusted, _ := config.ParsePrefixes("127.0.0.1,10.0.0.0/8")
	c := newClientIPs(config.Config{ClientIPHeader: "X-Forwarded-For", TrustedProxies: trusted})
	cases := []struct {
		r    *http.Request
		want string
	}{
		{ipRequest("127.0.0.1:1", "X-Forwarded-For", "1.2.3.4, 203.0.113.9, 10.0.1.5"), "203.0.113.9"},
		{ipRequest("127.0.0.1:1", "X-Forwarded-For", "1.2.3.4", "X-Forwarded-For", "203.0.113.9"), "203.0.113.9"},
		{ipRequest("127.0.0.1:1", "X-Forwarded-For", "10.0.0.2, 10.0.1.5"), "10.0.0.2"},
		{ipRequest("127.0.0.1:1", "X-Forwarded-For", "1.2.3.4, bogus, 10.0.1.5"), "127.0.0.1"},
		{ipRequest("127.0.0.1:1", "CF-Connecting-IP", "1.2.3.4"), "127.0.0.1"},
		{ipRequest("192.168.1.9:1", "X-Forwarded-For", "203.0.113.9"), "192.168.1.9"},
	}
	for i, tc := range cases {
		if got := c.resolve(tc.r); got != tc.want {
			t.Errorf("case %d: %s, want %s", i, got, tc.want)
		}
	}
	none := newClientIPs(config.Config{ClientIPHeader: "none", TrustedProxies: trusted})
	if got := none.resolve(ipRequest("127.0.0.1:1", "CF-Connecting-IP", "1.2.3.4")); got != "127.0.0.1" {
		t.Errorf("none: %s", got)
	}
}

// The middleware resolves the address once; handlers and limits read it.
func TestMiddlewareKeysLimitsByCloudflareAddress(t *testing.T) {
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil)), clientIPs: newClientIPs(config.Config{})}
	var got string
	h := s.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = clientIP(r) }))
	h.ServeHTTP(httptest.NewRecorder(), ipRequest("127.0.0.1:5000", "X-Forwarded-For", "1.2.3.4", "CF-Connecting-IP", "203.0.113.9"))
	if got != "203.0.113.9" {
		t.Fatalf("clientIP = %s", got)
	}
	if got := clientIP(ipRequest("198.51.100.7:1")); got != "198.51.100.7" {
		t.Fatalf("without middleware = %s", got)
	}
}

// With the database: a successful login does not count as a failure, so a
// shared campus address is not locked out by people logging in.
func TestSuccessfulLoginsAreNotCounted(t *testing.T) {
	e := newEnv(t, "")
	for i := range 15 {
		if c, _, bad := e.login(adminEmail, adminPassword); c != 200 {
			t.Fatalf("login %d = %d %+v", i+1, c, bad)
		}
	}
	for i := range 10 {
		if c, _, _ := e.login(adminEmail, "kata-sandi-salah-sekali"); c != 401 {
			t.Fatalf("failure %d = %d", i+1, c)
		}
	}
	if c, _, _ := e.login(adminEmail, adminPassword); c != 429 {
		t.Fatalf("lockout after 10 failures = %d", c)
	}
}

// With the database: N parallel logins never run more Argon2id hashes at
// once than the limit, and logins that find the gate busy get a clean 429
// that does not count as a failure.
func TestParallelLoginsStayWithinTheHashLimit(t *testing.T) {
	e := newEnv(t, "")
	auth.SetHashLimits(2, 64)
	t.Cleanup(func() { auth.SetHashLimits(auth.DefaultHashConcurrency, auth.DefaultHashQueue) })
	e.srv.loginLimit = newLimiter(100, 15*time.Minute)
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _, _ := e.login(adminEmail, adminPassword)
			codes <- c
		}()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 200 {
			t.Fatalf("parallel login = %d", c)
		}
	}
	if _, peak := auth.HashStats(); peak > 2 {
		t.Fatalf("%d hashes ran at once, limit 2", peak)
	}

}
