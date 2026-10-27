package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/seed"
	"github.com/himasifunej/himasif-space/internal/store"
)

const (
	adminEmail    = "Admin@Himasif.Test"
	adminPassword = "kata-sandi-admin-uji"
)

// testDB makes an empty database for one test, dropped when it ends. It
// needs SPACE_TEST_DATABASE_URL, a URL of a role that may create
// databases; without it the test is skipped.
func testDB(t *testing.T) *store.Store {
	t.Helper()
	base := os.Getenv("SPACE_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SPACE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	b := make([]byte, 6)
	rand.Read(b)
	name := "space_test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + name
	st, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		st.Close()
		admin.Exec(ctx, "drop database if exists "+name+" with (force)")
		admin.Close(ctx)
	})
	if _, err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

type env struct {
	t    *testing.T
	st   *store.Store
	srv  *Server
	url  string
	mail *captureMailer
}

// captureMailer keeps the emails the server sends, for tests to read the
// links in them; with fail set it refuses to send.
type captureMailer struct {
	mu   sync.Mutex
	msgs []mail.Message
	fail bool
}

func (m *captureMailer) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("test mailer: refused")
	}
	m.msgs = append(m.msgs, msg)
	return nil
}

// last returns the last email to an address.
func (m *captureMailer) last(t *testing.T, to string) mail.Message {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.msgs) - 1; i >= 0; i-- {
		if strings.EqualFold(m.msgs[i].To, to) {
			return m.msgs[i]
		}
	}
	t.Fatalf("no email to %s", to)
	return mail.Message{}
}

func (m *captureMailer) count(to string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, msg := range m.msgs {
		if strings.EqualFold(msg.To, to) {
			n++
		}
	}
	return n
}

func newEnv(t *testing.T, webDir string) *env {
	st := testDB(t)
	if _, err := seed.Run(context.Background(), st, seed.Options{AdminEmail: adminEmail, AdminPassword: adminPassword}); err != nil {
		t.Fatal(err)
	}
	var logOut io.Writer = io.Discard
	if os.Getenv("SPACE_TEST_LOG") != "" {
		logOut = os.Stderr
	}
	log := slog.New(slog.NewTextHandler(logOut, nil))
	cfg := config.Config{Env: "development", SessionTTL: time.Hour, WebDir: webDir, PublicURL: "http://space.test"}
	m := &captureMailer{}
	srv := New(cfg, st, m, log)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return &env{t: t, st: st, srv: srv, url: hs.URL, mail: m}
}

// call sends a request and decodes the JSON response into out, if not nil.
func (e *env) call(method, path, token string, body any, out any) int {
	e.t.Helper()
	var rd io.Reader
	if s, ok := body.(string); ok {
		rd = strings.NewReader(s)
	} else if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.url+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			e.t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
	return resp.StatusCode
}

func (e *env) login(email, password string) (int, api.LoginResponse, api.ErrorResponse) {
	e.t.Helper()
	var raw json.RawMessage
	code := e.call("POST", "/api/auth/login", "", api.LoginRequest{Email: email, Password: password, Client: "web"}, &raw)
	var ok api.LoginResponse
	var bad api.ErrorResponse
	if code == http.StatusOK {
		json.Unmarshal(raw, &ok)
	} else {
		json.Unmarshal(raw, &bad)
	}
	return code, ok, bad
}

func TestHealth(t *testing.T) {
	e := newEnv(t, "")
	var h api.Health
	if code := e.call("GET", "/api/health", "", nil, &h); code != 200 || h.Status != "ok" || h.Database != "ok" {
		t.Fatalf("health = %d %+v", code, h)
	}
}

func TestLoginSessionAndLogout(t *testing.T) {
	e := newEnv(t, "")

	// Emails are compared case-insensitively and trimmed.
	code, r, bad := e.login("  admin@HIMASIF.test ", adminPassword)
	if code != 200 {
		t.Fatalf("login = %d %+v", code, bad)
	}
	if r.Token == "" || r.User.Kind != api.AccountOrganizationAdmin || r.User.Email != "admin@himasif.test" || !r.User.EmailVerified {
		t.Fatalf("login response %+v", r)
	}
	if time.Until(r.ExpiresAt) > time.Hour+time.Minute {
		t.Fatalf("session outlives SessionTTL: %v", r.ExpiresAt)
	}

	var me api.User
	if code := e.call("GET", "/api/me", r.Token, nil, &me); code != 200 || me.ID != r.User.ID {
		t.Fatalf("me = %d %+v", code, me)
	}
	var periods api.List[api.Period]
	if code := e.call("GET", "/api/periods", r.Token, nil, &periods); code != 200 || len(periods.Items) != 1 {
		t.Fatalf("periods = %d %+v", code, periods)
	}
	p := periods.Items[0]
	if p.Label != "2025/2026" || p.Status != api.PeriodActive || p.StartsOn.String() != "2025-12-01" || p.EndsOn.String() != "2026-11-30" || p.TimeZone != "Asia/Jakarta" {
		t.Fatalf("period %+v", p)
	}
	var divisions api.List[api.Division]
	if code := e.call("GET", "/api/divisions", r.Token, nil, &divisions); code != 200 || len(divisions.Items) != len(seed.Divisions) {
		t.Fatalf("divisions = %d %+v", code, divisions)
	}

	if code := e.call("POST", "/api/auth/logout", r.Token, nil, nil); code != 204 {
		t.Fatalf("logout = %d", code)
	}
	var er api.ErrorResponse
	if code := e.call("GET", "/api/me", r.Token, nil, &er); code != 401 || er.Error.Code != api.CodeUnauthenticated {
		t.Fatalf("me after logout = %d %+v", code, er)
	}

	// Login and logout are in the audit log, with the account that acted.
	var n int
	err := e.st.DB().QueryRow(context.Background(),
		`select count(*) from audit_events where actor_user_id = $1 and actor_kind = 'organization_admin' and action in ('auth.login', 'auth.logout')`,
		r.User.ID).Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("audit events = %d, %v", n, err)
	}
}

func TestLoginFailuresLookAlike(t *testing.T) {
	e := newEnv(t, "")
	c1, _, wrong := e.login(adminEmail, "kata-sandi-salah-sekali")
	c2, _, unknown := e.login("tidak-ada@himasif.test", "kata-sandi-salah-sekali")
	if c1 != 401 || c2 != 401 || wrong.Error != unknown.Error || wrong.Error.Message != wrongCredentials {
		t.Fatalf("wrong password %d %+v, unknown email %d %+v", c1, wrong, c2, unknown)
	}
	if c, _, bad := e.login("", ""); c != 400 || bad.Error.Code != api.CodeBadRequest {
		t.Fatalf("empty = %d %+v", c, bad)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t, "")
	for i := 0; i < 10; i++ {
		if c, _, _ := e.login(adminEmail, "kata-sandi-salah-sekali"); c != 401 {
			t.Fatalf("attempt %d = %d", i+1, c)
		}
	}
	// Even the right password waits once the limit is reached.
	if c, _, bad := e.login(adminEmail, adminPassword); c != 429 || bad.Error.Code != api.CodeRateLimited {
		t.Fatalf("attempt 11 = %d %+v", c, bad)
	}
}

func TestRequestsWithoutAValidSession(t *testing.T) {
	e := newEnv(t, "")
	for _, tok := range []string{"", "bukan-token", strings.Repeat("x", 300)} {
		var er api.ErrorResponse
		if code := e.call("GET", "/api/periods", tok, nil, &er); code != 401 || er.Error.Code != api.CodeUnauthenticated {
			t.Errorf("token %.10q: %d %+v", tok, code, er)
		}
	}
	var er api.ErrorResponse
	if code := e.call("GET", "/api/tidak-ada", "", nil, &er); code != 404 || er.Error.Code != api.CodeNotFound {
		t.Errorf("unknown route = %d %+v", code, er)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	e := newEnv(t, "")
	var er api.ErrorResponse
	code := e.call("POST", "/api/auth/login", "", `{"email":"a@b.c","password":"x","isAdmin":true}`, &er)
	if code != 400 || er.Error.Code != api.CodeBadRequest {
		t.Fatalf("unknown field = %d %+v", code, er)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	if _, err := e.st.DB().Exec(ctx, `update audit_events set action = 'x'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("update: %v", err)
	}
	if _, err := e.st.DB().Exec(ctx, `delete from audit_events`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete: %v", err)
	}
}

func TestSeedIsIdempotentAndAdminIsSingle(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	rep, err := seed.Run(ctx, e.st, seed.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Period || rep.Admin || len(rep.Divisions) != 0 || len(rep.Spaces) != 0 {
		t.Fatalf("second seed added %+v", rep)
	}
	_, err = e.st.CreateUser(ctx, e.st.DB(), store.User{Kind: api.AccountOrganizationAdmin, Email: "admin2@himasif.test", DisplayName: "Admin 2"})
	if err == nil {
		t.Fatal("a second organization admin account was created")
	}
	_, err = e.st.CreateUser(ctx, e.st.DB(), store.User{Kind: api.AccountPerson, Email: "ADMIN@himasif.test", DisplayName: "Orang"})
	if err != store.ErrEmailTaken {
		t.Fatalf("duplicate email: %v", err)
	}
}

func TestWebApp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Space</title>"), 0o644)
	os.WriteFile(filepath.Join(dir, "space.abc123.wasm"), []byte("\x00asm"), 0o644)
	e := newEnv(t, dir)
	get := func(path string) *http.Response {
		resp, err := http.Get(e.url + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	if r := get("/"); r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("/ = %d %q", r.StatusCode, r.Header.Get("Cache-Control"))
	}
	if r := get("/space.abc123.wasm"); r.Header.Get("Content-Type") != "application/wasm" || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("wasm = %q %q", r.Header.Get("Content-Type"), r.Header.Get("Cache-Control"))
	}
	// With a gzipped copy beside it, clients that take gzip get the copy.
	os.WriteFile(filepath.Join(dir, "space.abc123.wasm.gz"), []byte("gzipped"), 0o644)
	req, _ := http.NewRequest("GET", e.url+"/space.abc123.wasm", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	tr := &http.Transport{DisableCompression: true}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" || string(body) != "gzipped" || resp.Header.Get("Content-Type") != "application/wasm" {
		t.Errorf("gzip wasm = %q %q %q", resp.Header.Get("Content-Encoding"), resp.Header.Get("Content-Type"), body)
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err = (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "" || string(body) != "\x00asm" {
		t.Errorf("identity wasm = %q %q", resp.Header.Get("Content-Encoding"), body)
	}
	if r := get("/../../etc/passwd"); r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("traversal = %d", r.StatusCode)
	}
	if r := get("/api/health"); r.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("API shadowed by the web app: %q", r.Header.Get("Content-Type"))
	}
}
