package appui

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// fakeAPI is a server of one account, in memory.
type fakeAPI struct {
	mu          sync.Mutex
	user        api.User
	password    string
	valid       map[string]bool // tokens the server accepts
	token       string          // what the app set
	unreachable bool
	logouts     int
	logins      []api.LoginRequest
	periods     []api.Period
	divisions   []api.Division
}

var errOffline = &client.UnreachableError{Err: errors.New("dial tcp: connection refused")}

func unauthenticated(msg string) error {
	return &client.Error{Status: 401, Code: api.CodeUnauthenticated, Message: msg}
}

func newFake(kind string) *fakeAPI {
	date := func(y int, m time.Month, d int) timeutil.Date { return timeutil.Date{Year: y, Month: m, Day: d} }
	return &fakeAPI{
		user:     api.User{ID: id.New(), Kind: kind, Email: "ani@himasif.id", DisplayName: "Ani Wulandari", EmailVerified: true},
		password: "rahasia",
		valid:    map[string]bool{},
		periods: []api.Period{
			{ID: id.New(), Label: "2024/2025", Status: api.PeriodClosed, StartsOn: date(2024, 9, 1), EndsOn: date(2025, 8, 31)},
			{ID: id.New(), Label: "2025/2026", Status: api.PeriodActive, StartsOn: date(2025, 9, 1), EndsOn: date(2026, 8, 31)},
			{ID: id.New(), Label: "2026/2027", Status: api.PeriodPreparing, StartsOn: date(2026, 9, 1), EndsOn: date(2027, 8, 31)},
		},
		divisions: []api.Division{
			{ID: id.New(), Slug: "mediatek", Name: "Media dan Teknologi"},
			{ID: id.New(), Slug: "humas", Name: "Hubungan Masyarakat"},
		},
	}
}

func (f *fakeAPI) SetToken(t string) { f.mu.Lock(); f.token = t; f.mu.Unlock() }

// check returns the error of a call needing a session.
func (f *fakeAPI) check() error {
	if f.unreachable {
		return errOffline
	}
	if !f.valid[f.token] {
		return unauthenticated("Sesi tidak valid.")
	}
	return nil
}

func (f *fakeAPI) Health(context.Context) (api.Health, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.unreachable {
		return api.Health{}, errOffline
	}
	return api.Health{Status: "ok"}, nil
}

func (f *fakeAPI) Login(_ context.Context, req api.LoginRequest) (api.LoginResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logins = append(f.logins, req)
	if f.unreachable {
		return api.LoginResponse{}, errOffline
	}
	if req.Email != f.user.Email || req.Password != f.password {
		return api.LoginResponse{}, unauthenticated("Email atau kata sandi salah.")
	}
	tok := "tok-" + id.New().String()
	f.valid[tok] = true
	return api.LoginResponse{Token: tok, User: f.user}, nil
}

func (f *fakeAPI) Logout(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logouts++
	if err := f.check(); err != nil {
		return err
	}
	delete(f.valid, f.token)
	return nil
}

func (f *fakeAPI) Me(context.Context) (api.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.user, f.check()
}

func (f *fakeAPI) Periods(context.Context) ([]api.Period, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(); err != nil {
		return nil, err
	}
	return f.periods, nil
}

func (f *fakeAPI) Divisions(context.Context) ([]api.Division, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(); err != nil {
		return nil, err
	}
	return f.divisions, nil
}

// memTokens is a token store in memory.
type memTokens struct {
	mu    sync.Mutex
	token string
}

func (m *memTokens) Load() (string, error) { m.mu.Lock(); defer m.mu.Unlock(); return m.token, nil }
func (m *memTokens) Save(t string) error   { m.mu.Lock(); m.token = t; m.mu.Unlock(); return nil }
func (m *memTokens) Clear() error          { m.mu.Lock(); m.token = ""; m.mu.Unlock(); return nil }
func (m *memTokens) get() string           { m.mu.Lock(); defer m.mu.Unlock(); return m.token }

// harness runs the app in a tester, with the results of its calls queued
// until settle applies them on the test's goroutine, as Window.Update
// applies them on the main thread.
type harness struct {
	t      *testing.T
	api    *fakeAPI
	tokens *memTokens
	app    *App
	queue  chan func()
	tt     *ui.Tester
}

func newHarness(t *testing.T, f *fakeAPI, saved string) *harness {
	h := &harness{t: t, api: f, tokens: &memTokens{token: saved}, queue: make(chan func(), 64)}
	h.app = New(f, h.tokens, func(fn func()) { h.queue <- fn })
	h.app.ServerURL = "https://api.contoh.test"
	h.app.Version = "0.1.0-test"
	h.tt = ui.NewTester(h.app.View, 1100, 720)
	return h
}

// settle applies the results of the calls in flight, and of those they
// start, then renders a frame. It waits on the calls' goroutines, not on
// the clock.
func (h *harness) settle() {
	h.t.Helper()
	for h.app.inflight > 0 {
		select {
		case fn := <-h.queue:
			fn()
		case <-time.After(5 * time.Second):
			h.t.Fatalf("%d calls never returned", h.app.inflight)
		}
	}
	h.tt.Frame()
}

func (h *harness) click(s string) {
	h.t.Helper()
	if err := h.tt.Click(s); err != nil {
		h.t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
}

func (h *harness) want(s string) {
	h.t.Helper()
	if !h.tt.HasText(s) {
		h.t.Errorf("no text %q in %q", s, h.tt.Texts())
	}
}

func (h *harness) dontWant(s string) {
	h.t.Helper()
	if h.tt.HasText(s) {
		h.t.Errorf("text %q shown in %q", s, h.tt.Texts())
	}
}

// login types the credentials and presses Masuk.
func (h *harness) login(email, password string) {
	h.t.Helper()
	h.click("Email")
	h.tt.Type(email)
	h.click("Kata sandi")
	h.tt.Type(password)
	h.click("Masuk")
	h.settle()
}

func TestLoginScreen(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	for _, s := range []string{"Space", "Ruang Kerja HIMASIF", "Email", "Kata sandi", "Masuk",
		"Akun dibuat lewat undangan dari kadiv.", "https://api.contoh.test"} {
		h.want(s)
	}
	h.dontWant("Daftar")
}

func TestLoginSuccess(t *testing.T) {
	f := newFake(api.AccountPerson)
	h := newHarness(t, f, "")
	h.app.Start()
	h.settle()
	h.login("ani@himasif.id", "rahasia")
	if h.app.screen != screenShell {
		t.Fatalf("screen %d after login, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Jobdesk saya")
	h.want("Ani Wulandari")
	h.want("Tersambung")
	h.want("Layanan tugas belum tersedia.")
	if len(f.logins) != 1 || f.logins[0].Client != "desktop" {
		t.Errorf("logins %+v", f.logins)
	}
	if tok := h.tokens.get(); tok == "" || !f.valid[tok] {
		t.Errorf("saved token %q", tok)
	}
	if h.app.password != "" {
		t.Error("the password stays in memory after login")
	}
}

func TestLoginWithEnter(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	h.click("Email")
	h.tt.Type("ani@himasif.id")
	h.click("Kata sandi")
	h.tt.Type("rahasia")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.app.screen != screenShell {
		t.Fatalf("Enter did not sign in; texts %q", h.tt.Texts())
	}
}

func TestWrongPassword(t *testing.T) {
	f := newFake(api.AccountPerson)
	h := newHarness(t, f, "")
	h.app.Start()
	h.settle()
	h.login("ani@himasif.id", "salah")
	h.want("Email atau kata sandi salah.")
	h.want("Masuk")
	h.dontWant("Jobdesk saya")
	if h.app.screen != screenLogin || h.app.busy {
		t.Errorf("screen %d, busy %v", h.app.screen, h.app.busy)
	}
	if h.tokens.get() != "" {
		t.Error("a token was saved")
	}
}

func TestLoginBusy(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	h.click("Email")
	h.tt.Type("ani@himasif.id")
	h.click("Kata sandi")
	h.tt.Type("rahasia")
	h.click("Masuk")
	h.tt.Frame()
	if !h.app.busy {
		t.Fatal("not busy while signing in")
	}
	h.click("Masuk") // disabled: no second login
	h.settle()
	if n := len(h.api.logins); n != 1 {
		t.Errorf("%d logins, want 1", n)
	}
}

func TestLoginEmpty(t *testing.T) {
	f := newFake(api.AccountPerson)
	h := newHarness(t, f, "")
	h.app.Start()
	h.settle()
	h.click("Masuk")
	h.settle()
	h.want("Isi email dan kata sandi.")
	if len(f.logins) != 0 {
		t.Error("signed in with nothing typed")
	}
}

func TestLoginOffline(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.unreachable = true
	h := newHarness(t, f, "")
	h.app.Start()
	h.settle()
	h.login("ani@himasif.id", "rahasia")
	h.want("Tidak dapat menghubungi server")
	if h.app.screen != screenLogin {
		t.Errorf("screen %d", h.app.screen)
	}
}

func TestSavedTokenValid(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok-saved"] = true
	h := newHarness(t, f, "tok-saved")
	h.app.Start()
	h.tt.Frame()
	h.want("Menyambungkan…")
	h.settle()
	if h.app.screen != screenShell {
		t.Fatalf("screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Jobdesk saya")
	h.dontWant("Kata sandi")
	if len(f.logins) != 0 {
		t.Error("signed in again")
	}
}

func TestSavedTokenRejected(t *testing.T) {
	f := newFake(api.AccountPerson)
	h := newHarness(t, f, "tok-expired")
	h.app.Start()
	h.settle()
	if h.app.screen != screenLogin {
		t.Fatalf("screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Masuk")
	h.want("Sesi berakhir")
	if tok := h.tokens.get(); tok != "" {
		t.Errorf("token %q kept", tok)
	}
}

func TestSavedTokenOffline(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok-saved"] = true
	f.unreachable = true
	h := newHarness(t, f, "tok-saved")
	h.app.Start()
	h.settle()
	if h.app.screen != screenOffline {
		t.Fatalf("screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Tidak ada koneksi")
	if h.tokens.get() != "tok-saved" {
		t.Error("the token was dropped while offline")
	}
	f.mu.Lock()
	f.unreachable = false
	f.mu.Unlock()
	h.click("Coba lagi")
	h.settle()
	if h.app.screen != screenShell {
		t.Fatalf("retry: screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Jobdesk saya")
}

func TestAdminSeesOrganization(t *testing.T) {
	for _, tc := range []struct {
		kind string
		sees bool
	}{{api.AccountOrganizationAdmin, true}, {api.AccountPerson, false}} {
		t.Run(tc.kind, func(t *testing.T) {
			f := newFake(tc.kind)
			f.valid["tok"] = true
			h := newHarness(t, f, "tok")
			h.app.Start()
			h.settle()
			h.click("Menu profil")
			h.want("Pengaturan")
			if got := h.tt.HasText("Pengaturan organisasi"); got != tc.sees {
				t.Errorf("%s sees Pengaturan organisasi: %v, want %v", tc.kind, got, tc.sees)
			}
			if tc.sees {
				h.click("Pengaturan organisasi")
				h.settle()
				h.want("Pengaturan organisasi")
			} else {
				// Nor does the path open it.
				h.app.router.Push(PathOrganisasi)
				h.tt.Frame()
				h.want("Halaman tidak ditemukan")
			}
		})
	}
}

func TestPeriodPicker(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := newHarness(t, f, "tok")
	h.app.Start()
	h.settle()
	if p := h.app.period(); p == nil || p.Label != "2025/2026" {
		t.Fatalf("period %+v, want 2025/2026", p)
	}
	h.want("2025/2026 (aktif)")
	h.click("2025/2026 (aktif)") // opens the custom period dropdown
	if !h.app.chrome.periodOpen {
		t.Fatal("period dropdown did not open")
	}
	h.click("2024/2025 (ditutup)")
	h.settle()
	if p := h.app.period(); p == nil || p.Label != "2024/2025" {
		t.Errorf("period %+v after choosing 2024/2025", p)
	}
	if h.app.chrome.periodOpen {
		t.Error("period dropdown stayed open after choosing a period")
	}
}

func TestDefaultPeriod(t *testing.T) {
	ps := []api.Period{{ID: id.New(), Status: api.PeriodClosed}, {ID: id.New(), Status: api.PeriodPreparing}}
	if got := defaultPeriod(ps); got != ps[1].ID {
		t.Error("without an active period, the one being prepared is not chosen")
	}
	if got := defaultPeriod(ps[:1]); got != ps[0].ID {
		t.Error("with closed periods only, the first is not chosen")
	}
}

func TestPages(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := newHarness(t, f, "tok")
	h.app.Start()
	h.settle()
	for _, tc := range []struct {
		item, want string
		profile    bool
	}{
		{"Reminder pribadi", "Reminder belum dapat dimuat", false},
		{"Evaluasi SOP", "Temuan perlu ditinjau", true},
		{"Arsip periode", "Informasi periode", false},
		{"Space saya", "Layanan akses Space belum tersedia.", false},
	} {
		if tc.profile {
			h.click("Menu profil")
		}
		h.click(tc.item)
		h.settle()
		h.want(tc.want)
		h.dontWant("Dibangun di")
	}
	h.dontWant("Media dan Teknologi")
	h.dontWant("Medhum")
	h.click("Menu profil")
	h.click("Pengaturan")
	h.settle()
	h.click("Akun")
	for _, s := range []string{"ani@himasif.id", "Pribadi", "Terverifikasi", "https://api.contoh.test", "0.1.0-test", "Keluar"} {
		h.want(s)
	}
}

func TestLogout(t *testing.T) {
	f := newFake(api.AccountPerson)
	h := newHarness(t, f, "")
	h.app.Start()
	h.settle()
	h.login("ani@himasif.id", "rahasia")
	tok := h.tokens.get()
	h.click("Menu profil")
	h.click("Keluar akun")
	h.settle()
	if h.app.screen != screenLogin {
		t.Fatalf("screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	h.want("Masuk")
	if h.tokens.get() != "" {
		t.Error("token kept after Keluar")
	}
	if f.logouts != 1 || f.valid[tok] {
		t.Errorf("logouts %d, server token still valid %v", f.logouts, f.valid[tok])
	}
	if f.token != "" {
		t.Errorf("the client still sends %q", f.token)
	}
	// The email stays for signing in again, which starts at the first page.
	if h.app.email != "ani@himasif.id" || h.app.password != "" {
		t.Errorf("email %q, password %q after Keluar", h.app.email, h.app.password)
	}
	h.click("Kata sandi")
	h.tt.Type("rahasia")
	h.click("Masuk")
	h.settle()
	if h.app.screen != screenShell {
		t.Fatalf("signing in again: screen %d, texts %q", h.app.screen, h.tt.Texts())
	}
	if h.app.router.Path() != PathJobdesk {
		t.Errorf("page %q after signing in again", h.app.router.Path())
	}
}

func TestLogoutOffline(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := newHarness(t, f, "tok")
	h.app.Start()
	h.settle()
	f.mu.Lock()
	f.unreachable = true
	f.mu.Unlock()
	h.click("Menu profil")
	h.click("Keluar akun")
	h.settle()
	if h.app.screen != screenLogin || h.tokens.get() != "" {
		t.Errorf("offline Keluar: screen %d, token %q", h.app.screen, h.tokens.get())
	}
}

func TestSessionExpiresInShell(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := newHarness(t, f, "tok")
	h.app.Start()
	h.settle()
	delete(f.valid, "tok")
	h.click("Space saya")
	h.app.loadDivisions()
	h.settle()
	if h.app.screen != screenLogin || h.tokens.get() != "" {
		t.Errorf("expired session: screen %d, token %q", h.app.screen, h.tokens.get())
	}
	h.want("Sesi berakhir")
}

func TestConnectionIndicator(t *testing.T) {
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := newHarness(t, f, "tok")
	h.app.Start()
	h.settle()
	h.want("Tersambung")
	f.mu.Lock()
	f.unreachable = true
	f.mu.Unlock()
	h.tt.Advance(pingEvery)
	h.settle()
	h.want("Tidak ada koneksi")
	f.mu.Lock()
	f.unreachable = false
	f.mu.Unlock()
	h.click("Coba lagi")
	h.tt.Frame()
	h.want("Menyambungkan…")
	h.settle()
	h.want("Tersambung")
}
