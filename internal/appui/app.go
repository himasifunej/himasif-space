// Package appui is Space's interface, drawn with MyGo's native UI: the
// login, and the shell of pages after it. It holds the app's state and
// views and runs without a window, so tests drive it with ui.Tester.
//
// The view runs on the window's main thread. Network calls run on
// goroutines and hand their results back through the update function given
// to New, as mygo's Window.Update does.
package appui

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// API is what the interface needs of the server; *client.Client implements
// it. Its errors are those of package client: *client.Error for answers of
// the server, and errors for which client.IsUnreachable holds for no answer.
type API interface {
	Health(ctx context.Context) (api.Health, error)
	Login(ctx context.Context, req api.LoginRequest) (api.LoginResponse, error)
	Logout(ctx context.Context) error
	Me(ctx context.Context) (api.User, error)
	Periods(ctx context.Context) ([]api.Period, error)
	Divisions(ctx context.Context) ([]api.Division, error)
	// SetToken sets the session token later calls send; "" sends none.
	SetToken(token string)
}

// TokenStore keeps the session token between launches. Load returns "" and
// no error when there is none.
type TokenStore interface {
	Load() (string, error)
	Save(token string) error
	Clear() error
}

var _ API = (*client.Client)(nil)

// Timeouts of the calls the app makes, within the client's own.
const (
	logoutTimeout = 5 * time.Second
	// pingEvery is how often the shell checks the connection.
	pingEvery = 30 * time.Second
)

// Paths of the shell's pages.
const (
	PathJobdesk    = "/jobdesk"
	PathReminder   = "/reminder"
	PathSpaces     = "/spaces"
	PathMedhum     = "/medhum"
	PathEvaluasi   = "/evaluasi"
	PathArsip      = "/arsip"
	PathPengaturan = "/pengaturan"
	PathOrganisasi = "/organisasi"
)

// screen is what the window shows.
type screen int

const (
	screenStarting screen = iota // loading the saved session
	screenLogin
	screenOffline // a saved session the server could not confirm
	screenShell
)

// Connection states, shown at the bottom of the sidebar.
type connState int

const (
	connConnecting connState = iota
	connOnline
	connOffline
)

// App is the app's state. Make it with New, set the exported fields, then
// call Start once the window exists.
type App struct {
	// ServerURL is shown on the login and settings pages.
	ServerURL string
	// Version is the app's version, shown in the settings.
	Version string
	// ClientName names the app to the server at login: "desktop" or "web".
	ClientName string
	// PreferencesStore persists appearance on this device, separately from tokens.
	PreferencesStore  PreferencesStore
	preferencesLoaded bool
	chrome            chromeState
	workspace         *workspaceui.State

	api    API
	tokens TokenStore
	update func(func())
	// inflight counts the calls whose results have not been applied yet.
	inflight int
	// gen changes as the session does, so that results of an earlier
	// session are dropped.
	gen int
	// tokenSeq numbers the writes of the token store as they are asked for,
	// on the view's thread; tokenDone, guarded by tokenMu, is the last one
	// done, so that a write asked for earlier never undoes a later one.
	tokenSeq  uint64
	tokenMu   sync.Mutex
	tokenDone uint64

	screen screen
	notice string // shown on the login page, as why the session ended
	// startErr is why a saved session could not be confirmed.
	startErr string

	email, password string
	busy            bool
	loginErr        string

	user       api.User
	conn       connState
	pinging    bool
	lastPing   time.Time // by the view's clock
	loggingOut bool

	periods        []api.Period
	periodsLoading bool
	periodsErr     string
	periodID       id.ID

	divisions        []api.Division
	divisionsLoading bool
	divisionsErr     string

	router *ui.Router
}

// New returns the app's state. update runs a function on the view's thread
// and redraws, as mygo's Window.Update.
func New(api API, tokens TokenStore, update func(func())) *App {
	return &App{
		ClientName: "desktop",
		api:        api,
		tokens:     tokens,
		update:     update,
		router:     ui.NewRouter(PathJobdesk),
		chrome:     chromeState{preferences: DefaultPreferences(), tab: "Beranda", settingsTab: "WhatsApp"},
		workspace:  workspaceui.New(),
	}
}

// async runs work on a goroutine, then the function it returns, if not
// nil, through update.
func (a *App) async(work func() func()) {
	a.inflight++
	go func() {
		apply := work()
		a.update(func() {
			a.inflight--
			if apply != nil {
				apply()
			}
		})
	}()
}

// Start shows the saved session, if there is one the server confirms, or
// the login.
func (a *App) Start() {
	a.loadPreferences()
	a.screen = screenStarting
	a.startErr = ""
	a.conn = connConnecting
	a.gen++
	gen := a.gen
	write := a.tokenWrite()
	a.async(func() func() {
		token, err := a.tokens.Load()
		if err != nil || token == "" {
			return func() {
				if gen == a.gen {
					a.screen = screenLogin
				}
			}
		}
		a.api.SetToken(token)
		user, err := a.api.Me(context.Background())
		if client.IsUnauthenticated(err) {
			a.api.SetToken("")
			write("")
		}
		return func() {
			if gen != a.gen {
				return
			}
			switch {
			case err == nil:
				a.enterShell(user)
			case client.IsUnauthenticated(err):
				a.notice = "Sesi berakhir. Silakan masuk lagi."
				a.screen = screenLogin
			default:
				// Unreachable, or the server failed: keep the token, to
				// try again.
				a.conn = connOffline
				a.startErr = errorText(err)
				a.screen = screenOffline
			}
		}
	})
}

// login signs in with the email and password typed.
func (a *App) login() {
	if a.busy {
		return
	}
	a.notice = ""
	if a.email == "" || a.password == "" {
		a.loginErr = "Isi email dan kata sandi."
		return
	}
	a.busy, a.loginErr = true, ""
	a.gen++
	gen := a.gen
	req := api.LoginRequest{Email: a.email, Password: a.password, Client: a.ClientName}
	write := a.tokenWrite()
	a.async(func() func() {
		r, err := a.api.Login(context.Background(), req)
		if err == nil {
			a.api.SetToken(r.Token)
			write(r.Token)
		}
		return func() {
			if gen != a.gen {
				return
			}
			a.busy = false
			if err != nil {
				a.loginErr = errorText(err)
				return
			}
			a.password = ""
			a.enterShell(r.User)
		}
	})
}

// enterShell shows the shell of the user's pages, at the page the router
// shows.
func (a *App) enterShell(u api.User) {
	a.resetWorkspace()
	a.user = u
	a.screen = screenShell
	a.conn = connOnline
	a.notice, a.startErr, a.loginErr = "", "", ""
	a.lastPing = time.Time{}
	a.periods, a.periodID, a.periodsErr = nil, id.ID{}, ""
	a.divisions, a.divisionsErr = nil, ""
	a.loadPeriods()
	a.loadDivisions()
}

// failed handles an error of a call of the shell's: a rejected session
// ends it, no answer shows the app offline. It returns the text to show.
func (a *App) failed(err error) string {
	switch {
	case client.IsUnauthenticated(err):
		a.expire()
	case client.IsUnreachable(err):
		a.conn = connOffline
	}
	return errorText(err)
}

// expire ends a session the server no longer accepts.
func (a *App) expire() {
	a.toLogin("Sesi berakhir. Silakan masuk lagi.")
	write := a.tokenWrite()
	a.async(func() func() {
		write("")
		return nil
	})
}

// toLogin forgets the session and shows the login with a notice.
func (a *App) toLogin(notice string) {
	a.resetWorkspace()
	a.gen++
	a.api.SetToken("")
	a.user = api.User{}
	a.screen = screenLogin
	a.notice = notice
	a.busy, a.loggingOut, a.pinging = false, false, false
	a.periods, a.divisions = nil, nil
	// The next session starts at the first page. A session restored at
	// launch keeps the page the router starts at, as a browser's reload
	// of "#/spaces".
	a.router.Replace(PathJobdesk)
}

func (a *App) loadPeriods() {
	a.periodsLoading, a.periodsErr = true, ""
	gen := a.gen
	a.async(func() func() {
		ps, err := a.api.Periods(context.Background())
		return func() {
			if gen != a.gen {
				return
			}
			a.periodsLoading = false
			if err != nil {
				a.periodsErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.periods = ps
			if a.period() == nil {
				a.periodID = defaultPeriod(ps)
			}
		}
	})
}

func (a *App) loadDivisions() {
	a.divisionsLoading, a.divisionsErr = true, ""
	gen := a.gen
	a.async(func() func() {
		ds, err := a.api.Divisions(context.Background())
		return func() {
			if gen != a.gen {
				return
			}
			a.divisionsLoading = false
			if err != nil {
				a.divisionsErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.divisions = ds
		}
	})
}

// ping checks the connection, and loads again what failed for want of it.
func (a *App) ping(now time.Time) {
	if a.pinging {
		return
	}
	a.pinging, a.lastPing = true, now
	if a.conn == connOffline {
		a.conn = connConnecting
	}
	gen := a.gen
	a.async(func() func() {
		_, err := a.api.Health(context.Background())
		return func() {
			if gen != a.gen {
				return
			}
			a.pinging = false
			if err != nil {
				a.conn = connOffline
				return
			}
			a.conn = connOnline
			if a.periodsErr != "" && !a.periodsLoading {
				a.loadPeriods()
			}
			if a.divisionsErr != "" && !a.divisionsLoading {
				a.loadDivisions()
			}
		}
	})
}

// logout ends the session on the server, as it can, forgets the token and
// shows the login.
func (a *App) logout() {
	if a.loggingOut {
		return
	}
	a.loggingOut = true
	gen := a.gen
	write := a.tokenWrite()
	a.async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), logoutTimeout)
		_ = a.api.Logout(ctx) // best effort: the token is forgotten anyway
		cancel()
		write("")
		return func() {
			if gen == a.gen {
				a.toLogin("")
			}
		}
	})
}

// tokenWrite returns a function, for any goroutine, that saves a token in
// the store, or clears it for "", unless a write asked for after this one
// has been done already. Failures are left alone: a token that cannot be
// kept still serves this launch.
func (a *App) tokenWrite() func(token string) {
	a.tokenSeq++
	seq := a.tokenSeq
	return func(token string) {
		a.tokenMu.Lock()
		defer a.tokenMu.Unlock()
		if seq < a.tokenDone {
			return
		}
		a.tokenDone = seq
		if token == "" {
			_ = a.tokens.Clear()
		} else {
			_ = a.tokens.Save(token)
		}
	}
}

// period returns the period chosen, or nil.
func (a *App) period() *api.Period {
	for i := range a.periods {
		if a.periods[i].ID == a.periodID {
			return &a.periods[i]
		}
	}
	return nil
}

// defaultPeriod returns the active period, else the one being prepared,
// else the first.
func defaultPeriod(ps []api.Period) id.ID {
	for _, status := range []string{api.PeriodActive, api.PeriodPreparing} {
		for _, p := range ps {
			if p.Status == status {
				return p.ID
			}
		}
	}
	if len(ps) > 0 {
		return ps[0].ID
	}
	return id.ID{}
}

// periodLabel returns a period's label with its status, as
// "2025/2026 (aktif)".
func periodLabel(p api.Period) string {
	switch p.Status {
	case api.PeriodActive:
		return p.Label + " (aktif)"
	case api.PeriodClosed:
		return p.Label + " (ditutup)"
	case api.PeriodPreparing:
		return p.Label + " (persiapan)"
	}
	return p.Label
}

// errorText returns what to tell the user of err.
func errorText(err error) string {
	if client.IsUnreachable(err) {
		return "Tidak dapat menghubungi server. Periksa koneksi, lalu coba lagi."
	}
	var e *client.Error
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	return "Terjadi kesalahan. Coba lagi."
}
