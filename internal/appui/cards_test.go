package appui

import (
	"context"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// fakeWeb is a browser: a Windows computer or not, its storage, and
// whether the app answers space://.
type fakeWeb struct {
	windows bool
	store   map[string]string
	opens   []string
	answer  bool
	went    []string
}

func newFakeWeb(windows, answer bool) *fakeWeb {
	return &fakeWeb{windows: windows, answer: answer, store: map[string]string{}}
}
func (w *fakeWeb) WindowsDesktop() bool  { return w.windows }
func (w *fakeWeb) Get(key string) string { return w.store[key] }
func (w *fakeWeb) Set(key, value string) { w.store[key] = value }
func (w *fakeWeb) Go(path string)        { w.went = append(w.went, path) }
func (w *fakeWeb) OpenApp(link string, _ time.Duration) <-chan bool {
	w.opens = append(w.opens, link)
	out := make(chan bool, 1)
	out <- w.answer
	return out
}

// cardsFake answers GetTask and GetMeeting.
type cardsFake struct {
	*fakeAPI
	task    api.Task
	meeting api.Meeting
	err     error
}

func (f *cardsFake) GetTask(context.Context, id.ID) (api.Task, error) { return f.task, f.err }
func (f *cardsFake) GetMeeting(context.Context, id.ID) (api.Meeting, error) {
	return f.meeting, f.err
}

func cardHarness(t *testing.T, web *fakeWeb, f func(*cardsFake)) (*harness, *cardsFake) {
	t.Helper()
	h := newChromeHarness(t, nil)
	cf := &cardsFake{fakeAPI: h.api}
	if f != nil {
		f(cf)
	}
	h.app.api = cf
	if web != nil {
		h.app.Web = web
	}
	h.tt.Frame()
	return h, cf
}

// A task's card on a Windows browser: its facts, Buka di aplikasi Space,
// which once it worked is remembered, tried by itself next time, and
// hides the install bar.
func TestCardPageHandoffRemembered(t *testing.T) {
	web := newFakeWeb(true, true)
	tid := id.New()
	h, _ := cardHarness(t, web, func(f *cardsFake) {
		f.task = api.Task{ID: tid, Title: "Poster open recruitment", Progress: api.TaskDoing, Assignee: api.PersonRef{DisplayName: "Rani Pratiwi"}}
	})
	h.want("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	cardGo(h, "/tugas/"+tid.String())
	h.want("Poster open recruitment")
	h.want("Rani Pratiwi")
	if len(web.opens) != 0 {
		t.Fatal("tried the app before it ever worked")
	}
	h.click("Buka di aplikasi Space")
	h.settle()
	h.tt.Frame()
	if len(web.opens) != 1 || web.opens[0] != "space://tugas/"+tid.String() {
		t.Fatalf("opens %q", web.opens)
	}
	h.want("Terbuka di aplikasi Space. Halaman ini boleh ditutup.")
	if web.store[webHandoffKey] != "1" {
		t.Fatal("handoff not remembered")
	}
	h.dontWant("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	// Another card: tried by itself.
	other := id.New()
	cardGo(h, "/tugas/"+other.String())
	h.settle()
	if len(web.opens) != 2 || web.opens[1] != "space://tugas/"+other.String() {
		t.Fatalf("auto opens %q", web.opens)
	}
}

// When the app does not answer, the page stays and offers the installer.
func TestCardPageHandoffFails(t *testing.T) {
	web := newFakeWeb(true, false)
	mid := id.New()
	h, _ := cardHarness(t, web, func(f *cardsFake) {
		f.meeting = api.Meeting{ID: mid, Title: "Rapat Divisi", StartsAt: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC), TimeZone: "Asia/Jakarta", Location: "Sekret"}
	})
	cardGo(h, "/rapat/"+mid.String())
	h.want("Rapat Divisi")
	h.want("Sekret")
	h.click("Buka di aplikasi Space")
	h.settle()
	h.tt.Frame()
	h.want("Aplikasi Space tidak terbuka. Belum terpasang? Unduh Space untuk Windows, atau lanjutkan di web.")
	if web.store[webHandoffKey] != "" {
		t.Fatal("a failed handoff was remembered")
	}
	h.click("Unduh Space untuk Windows")
	if len(web.went) == 0 || web.went[len(web.went)-1] != "/unduh/windows" {
		t.Fatalf("went %q", web.went)
	}
}

// Who may not see the card is told so, politely; a phone never tries the
// app.
func TestCardPageDeniedAndPhone(t *testing.T) {
	web := newFakeWeb(false, true)
	h, _ := cardHarness(t, web, func(f *cardsFake) {
		f.err = &client.Error{Status: 403, Code: api.CodeForbidden, Message: "x"}
	})
	h.dontWant("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	cardGo(h, "/tugas/"+id.New().String())
	h.want("Kamu tidak punya akses ke tugas ini")
	h.dontWant("Buka di aplikasi Space")
	web.store[webHandoffKey] = "1"
	cardGo(h, "/rapat/"+id.New().String())
	if len(web.opens) != 0 {
		t.Fatalf("a phone tried the app: %q", web.opens)
	}
}

// The install bar: Nanti hides it for 14 days, Jangan ingatkan lagi for
// good.
func TestWebBannerLaterAndOff(t *testing.T) {
	web := newFakeWeb(true, false)
	h, _ := cardHarness(t, web, nil)
	h.want("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	h.click("Unduh Space untuk Windows")
	if len(web.went) != 1 || web.went[0] != "/unduh/windows" {
		t.Fatalf("went %q", web.went)
	}
	h.click("Nanti, sembunyikan 14 hari")
	h.tt.Frame()
	h.dontWant("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	h.tt.Advance(webBannerLater + time.Hour)
	h.tt.Frame()
	h.want("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	h.click("Jangan ingatkan lagi")
	h.tt.Frame()
	h.dontWant("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
	if web.store[webBannerOffKey] != "1" {
		t.Fatal("not kept")
	}
	// The account's mark hides it in another browser too.
	other := newFakeWeb(true, false)
	h.app.Web = other
	h.app.onb.data.Marks = map[string]time.Time{api.OnboardWebAppBannerOff: time.Now()}
	h.tt.Frame()
	h.dontWant("Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat")
}

// cardGo opens a page and lets it load.
func cardGo(h *harness, path string) {
	h.t.Helper()
	h.app.router.Push(path)
	h.tt.Advance(500 * time.Millisecond)
	h.settle()
	h.tt.Advance(500 * time.Millisecond)
	h.settle()
	h.tt.Frame()
}
