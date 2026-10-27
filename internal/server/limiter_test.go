package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

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
