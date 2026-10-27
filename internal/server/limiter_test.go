package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimiterBoundsNewIdentitiesWithoutResettingExistingLimits(t *testing.T) {
	l := newLimiter(2, time.Minute)
	l.maxKeys = 2
	now := time.Now()
	if !l.allow("a", now) || !l.allow("b", now) {
		t.Fatal("initial identities refused")
	}
	if l.allow("c", now) {
		t.Fatal("unseen identity allowed beyond capacity")
	}
	if !l.allow("a", now) || l.allow("a", now) {
		t.Fatal("existing identity did not keep its original limit")
	}
	if !l.allow("c", now.Add(2*time.Minute)) {
		t.Fatal("expired identities did not release capacity")
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

func TestClientIPTrustsForwardedOnlyFromPrivateProxies(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.1.5:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.1.5")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Errorf("behind proxy = %s", got)
	}
	r.RemoteAddr = "198.51.100.7:1234"
	if got := clientIP(r); got != "198.51.100.7" {
		t.Errorf("spoofed header trusted: %s", got)
	}
}
