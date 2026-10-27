package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, api.ErrorResponse{Error: api.Error{Code: code, Message: msg}})
}

// server is a fake API that knows one account and one token.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	user := api.User{ID: id.New(), Kind: api.AccountPerson, Email: "ani@himasif.id", DisplayName: "Ani"}
	authed := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer tok-1" }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, api.Health{Status: "ok", Database: "ok", Version: "test"})
	})
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			apiError(w, 400, api.CodeBadRequest, "Bukan JSON.")
			return
		}
		var req api.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiError(w, 400, api.CodeBadRequest, "Permintaan tidak valid.")
			return
		}
		if req.Email == "limit@himasif.id" {
			apiError(w, 429, api.CodeRateLimited, "Terlalu banyak percobaan.")
			return
		}
		if req.Email != user.Email || req.Password != "rahasia" || req.Client != "desktop" {
			apiError(w, 401, api.CodeUnauthenticated, "Email atau kata sandi salah.")
			return
		}
		writeJSON(w, 200, api.LoginResponse{Token: "tok-1", ExpiresAt: time.Now().Add(time.Hour), User: user})
	})
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			apiError(w, 401, api.CodeUnauthenticated, "Sesi tidak valid.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			apiError(w, 401, api.CodeUnauthenticated, "Sesi tidak valid.")
			return
		}
		writeJSON(w, 200, user)
	})
	mux.HandleFunc("GET /api/periods", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, api.List[api.Period]{Items: []api.Period{{ID: id.New(), Label: "2025/2026", Status: api.PeriodActive,
			StartsOn: timeutil.Date{Year: 2025, Month: time.September, Day: 1}, EndsOn: timeutil.Date{Year: 2026, Month: time.August, Day: 31}, TimeZone: "Asia/Jakarta"}}})
	})
	mux.HandleFunc("GET /api/divisions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, api.List[api.Division]{Items: []api.Division{{ID: id.New(), Slug: "mediatek", Name: "Mediatek"}}})
	})
	mux.HandleFunc("GET /api/gateway", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	})
	mux.HandleFunc("GET /api/plain", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestLoginMeLogout(t *testing.T) {
	s := server(t)
	ctx := context.Background()
	c := New(s.URL + "/") // a trailing slash is fine
	r, err := c.Login(ctx, api.LoginRequest{Email: "ani@himasif.id", Password: "rahasia", Client: "desktop"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Token != "tok-1" || r.User.DisplayName != "Ani" {
		t.Fatalf("login = %+v", r)
	}
	if c.Token != "" {
		t.Error("Login set the token itself")
	}
	c.SetToken(r.Token)
	u, err := c.Me(ctx)
	if err != nil || u.Email != "ani@himasif.id" {
		t.Fatalf("Me = %+v, %v", u, err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestErrors(t *testing.T) {
	s := server(t)
	ctx := context.Background()
	c := New(s.URL)
	_, err := c.Login(ctx, api.LoginRequest{Email: "ani@himasif.id", Password: "salah", Client: "desktop"})
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("wrong password: %v (%T)", err, err)
	}
	if e.Status != 401 || e.Code != api.CodeUnauthenticated || e.Message != "Email atau kata sandi salah." {
		t.Errorf("wrong password: %+v", e)
	}
	if !IsUnauthenticated(err) || IsUnreachable(err) {
		t.Errorf("IsUnauthenticated %v, IsUnreachable %v", IsUnauthenticated(err), IsUnreachable(err))
	}

	_, err = c.Login(ctx, api.LoginRequest{Email: "limit@himasif.id", Password: "x", Client: "desktop"})
	if !errors.As(err, &e) || e.Status != 429 || e.Code != api.CodeRateLimited {
		t.Errorf("rate limited: %v", err)
	}

	c.SetToken("expired")
	if _, err := c.Me(ctx); !IsUnauthenticated(err) {
		t.Errorf("Me with a bad token: %v", err)
	}

	if err := c.do(ctx, http.MethodGet, "/api/gateway", nil, nil); !IsUnreachable(err) {
		t.Errorf("502 from a proxy: %v, want unreachable", err)
	}
	err = c.do(ctx, http.MethodGet, "/api/plain", nil, nil)
	if !errors.As(err, &e) || e.Status != 500 || e.Code != api.CodeInternal || e.Message == "" {
		t.Errorf("500 without JSON: %v", err)
	}
}

func TestLists(t *testing.T) {
	s := server(t)
	ctx := context.Background()
	c := New(s.URL)
	h, err := c.Health(ctx)
	if err != nil || h.Status != "ok" {
		t.Fatalf("Health = %+v, %v", h, err)
	}
	ps, err := c.Periods(ctx)
	if err != nil || len(ps) != 1 || ps[0].Status != api.PeriodActive {
		t.Fatalf("Periods = %+v, %v", ps, err)
	}
	ds, err := c.Divisions(ctx)
	if err != nil || len(ds) != 1 || ds[0].Slug != "mediatek" {
		t.Fatalf("Divisions = %+v, %v", ds, err)
	}
}

func TestUnreachable(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	url := s.URL
	s.Close() // nothing listens there now
	_, err := New(url).Health(context.Background())
	if !IsUnreachable(err) {
		t.Fatalf("closed server: %v (%T), want unreachable", err, err)
	}
	if IsUnauthenticated(err) {
		t.Error("closed server counts as unauthenticated")
	}
}

func TestTimeout(t *testing.T) {
	block := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer s.Close()
	defer close(block)
	c := &Client{BaseURL: s.URL, HTTP: &http.Client{Timeout: 50 * time.Millisecond}}
	if _, err := c.Me(context.Background()); !IsUnreachable(err) {
		t.Fatalf("slow server: %v, want unreachable", err)
	}
}

func TestCanceled(t *testing.T) {
	s := server(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(s.URL).Health(ctx); !errors.Is(err, context.Canceled) || IsUnreachable(err) {
		t.Fatalf("canceled: %v", err)
	}
}
