package appui

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Card links (0.4.6). A WhatsApp message about a task or a meeting links
// <web>/t/<id> or /r/<id>; the server sends the browser on to the pages
// below, which a sign-in comes back to. On the web the page offers to open
// the card in Space for Windows (space://tugas/<id>), and does so by itself
// once that worked in this browser. In the desktop app the page opens the
// card on the board or in Rapat at once.

// Pages of a card.
const (
	PathTaskCard    = "/tugas/{id}"
	PathMeetingCard = "/rapat/{id}"
)

// AppScheme is Space for Windows' link scheme for cards: space://tugas/<id>.
const AppScheme = "space"

// Browser storage keys of the web (WebPlatform.Get).
const (
	webHandoffKey     = "space.app.handoff"     // "1" once the app opened from the web
	webBannerUntilKey = "space.app.bannerUntil" // Nanti: hidden until (Unix seconds)
	webBannerOffKey   = "space.app.bannerOff"   // Jangan ingatkan lagi, before the server knows
)

// webBannerLater is how long Nanti hides the web's install bar.
const webBannerLater = 14 * 24 * time.Hour

// webHandoffWait is how long the web waits for the browser to leave for the
// app before it stays.
const webHandoffWait = 2500 * time.Millisecond

// WebPlatform is what only the browser has: the web build sets it, the
// desktop app does not.
type WebPlatform interface {
	// WindowsDesktop tells whether the browser runs on a Windows computer,
	// never a phone or a tablet.
	WindowsDesktop() bool
	// Get and Set keep small values in this browser (localStorage).
	Get(key string) string
	Set(key, value string)
	// OpenApp opens link (space://…) and reports, after wait, whether the
	// browser left for the app.
	OpenApp(link string, wait time.Duration) <-chan bool
	// Go opens path of the web's own origin in this tab (a download).
	Go(path string)
}

// CardsAPI reads one task or meeting, within the caller's access.
type CardsAPI interface {
	GetTask(context.Context, id.ID) (api.Task, error)
	GetMeeting(context.Context, id.ID) (api.Meeting, error)
}

func (a *App) cardsService() (CardsAPI, bool) {
	if cl, ok := a.api.(*client.Client); ok {
		return cl.AccountSession(), true
	}
	s, ok := a.api.(CardsAPI)
	return s, ok
}

// cardState is the card page shown.
type cardState struct {
	kind    string // "task" or "meeting"
	id      id.ID
	loading bool
	loaded  bool
	denied  bool
	err     string
	task    api.Task
	meeting api.Meeting
	// handoff: "" idle, "trying", "opened", "failed"; auto: tried by
	// itself for this card already; opened: the desktop app went on.
	handoff string
	auto    bool
	opened  bool
}

// cardLink is the app's link of a card: space://tugas/<id>.
func cardAppLink(kind string, cid id.ID) string {
	page := "tugas"
	if kind == "meeting" {
		page = "rapat"
	}
	return AppScheme + "://" + page + "/" + cid.String()
}

// OpenCard shows a card's page: the desktop app does, given a space://
// link; it then opens the card on the board or in Rapat.
func (a *App) OpenCard(kind string, cid id.ID) {
	page := "/tugas/"
	if kind == "meeting" {
		page = "/rapat/"
	}
	a.router.Push(page + cid.String())
}

// cardPage is a card's page: what it is, and where to open it.
func (a *App) cardPage(c *ui.Context, r *ui.Route, kind string) {
	cid, err := id.Parse(r.Param("id"))
	st := &a.card
	if err != nil {
		r.Title("Tidak ditemukan")
		pageBody(c, "Tautan tidak dikenal", func() {
			muted(c, "Tautan ini tidak lengkap. Buka lagi dari pesannya, atau cari di Space.")
		})
		return
	}
	if st.id != cid || st.kind != kind {
		*st = cardState{kind: kind, id: cid}
	}
	if !st.loaded && !st.loading && st.err == "" {
		a.loadCard()
	}
	title := map[string]string{"task": "Tugas", "meeting": "Rapat"}[kind]
	r.Title(title)
	// The desktop app goes on to the card itself, on the board or in Rapat.
	if a.Web == nil && st.loaded && !st.denied && !st.opened {
		st.opened = true
		a.OpenNotification(api.Delivery{SourceKind: kind, SourceID: cid})
	}
	if a.Web != nil && st.loaded && !st.denied && !st.auto && a.Web.WindowsDesktop() && a.Web.Get(webHandoffKey) == "1" {
		st.auto = true
		a.cardHandoff()
	}
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640).Padding(32, 24).Gap(16).AlignSelf(ui.Center).Children(func() {
			ui.Text(c, map[string]string{"task": "TUGAS", "meeting": "RAPAT"}[kind]).FontSize(11).TextColor(faint(t))
			switch {
			case st.loading || (!st.loaded && st.err == ""):
				muted(c, "Memuat…")
			case st.denied:
				ui.Text(c, "Kamu tidak punya akses ke "+map[string]string{"task": "tugas", "meeting": "rapat"}[kind]+" ini").FontSize(20).FontWeight(600)
				muted(c, "Mungkin ini milik divisi lain, kamu belum bergabung di Space-nya, atau sudah dihapus. Minta pengirim pesan menambahkanmu bila perlu.").FontSize(14)
				if spaceButton(c, "Ke Untukku", "arrow", false).Border(1, t.Border).Clicked() {
					a.router.Replace(PathUntukku)
				}
			case st.err != "":
				ui.Text(c, "Belum dapat dimuat").FontSize(20).FontWeight(600)
				muted(c, st.err).FontSize(14)
				if spaceButton(c, "Coba lagi", "refresh", false).Border(1, t.Border).Clicked() {
					st.err = ""
				}
			default:
				a.cardDetails(c)
				a.cardActions(c)
			}
		})
	})
}

func (a *App) loadCard() {
	svc, ok := a.cardsService()
	st := &a.card
	if !ok {
		st.err = "Layanan Space belum tersedia."
		return
	}
	kind, cid, gen := st.kind, st.id, a.gen
	st.loading = true
	a.async(func() func() {
		var task api.Task
		var meeting api.Meeting
		var err error
		if kind == "meeting" {
			meeting, err = svc.GetMeeting(context.Background(), cid)
		} else {
			task, err = svc.GetTask(context.Background(), cid)
		}
		return func() {
			st := &a.card
			if gen != a.gen || st.id != cid || st.kind != kind {
				return
			}
			st.loading = false
			var ce *client.Error
			switch {
			case errors.As(err, &ce) && (ce.Status == http.StatusForbidden || ce.Status == http.StatusNotFound || ce.Code == api.CodeForbidden || ce.Code == api.CodeNotFound):
				st.loaded, st.denied = true, true
			case err != nil:
				st.err = a.failed(err)
			default:
				st.loaded, st.task, st.meeting = true, task, meeting
			}
		}
	})
}

// cardDetails is the card's title and its facts.
func (a *App) cardDetails(c *ui.Context) {
	st := &a.card
	facts := [][2]string{}
	if st.kind == "meeting" {
		m := st.meeting
		ui.Text(c, m.Title).FontSize(22).FontWeight(600)
		when := taskTimestamp(m.StartsAt, m.TimeZone)
		if m.Cancelled {
			when += " · dibatalkan"
		}
		facts = append(facts, [2]string{"Waktu", when})
		if m.Location != "" {
			facts = append(facts, [2]string{"Tempat", m.Location})
		}
		if m.JoinURL != "" {
			facts = append(facts, [2]string{"Tautan", m.JoinURL})
		}
		facts = append(facts, [2]string{"Peserta", strconv.Itoa(len(m.Participants)) + " orang"})
	} else {
		t := st.task
		ui.Text(c, t.Title).FontSize(22).FontWeight(600)
		facts = append(facts, [2]string{"Status", progressLabel(t.Progress)})
		if t.Assignee.DisplayName != "" {
			facts = append(facts, [2]string{"PJ", t.Assignee.DisplayName})
		}
		if t.DeadlineAt != nil || t.DeadlineDate != nil {
			due := taskDue(t)
			label := taskTimestamp(due, t.TimeZone)
			if t.DeadlineAt == nil {
				label = t.DeadlineDate.String()
			}
			facts = append(facts, [2]string{"Tenggat", label})
		}
	}
	info(c, facts)
}

// cardActions are Buka di Space and, on a Windows computer's browser,
// Buka di aplikasi Space with what came of it.
func (a *App) cardActions(c *ui.Context) {
	t := c.Theme()
	st := &a.card
	app := a.Web != nil && a.Web.WindowsDesktop()
	ui.Row(c.Key("card-actions")).FillWidth().Wrap().Gap(8).Children(func() {
		if app && onbPrimary(c, "Buka di aplikasi Space", "Buka di aplikasi Space", false).Disabled(st.handoff == "trying").Clicked() {
			a.cardHandoff()
		}
		open := onbSecondary(c, "Buka di Space", "Buka di Space", false)
		if !app {
			open = onbPrimary(c, "Buka di Space", "Buka di Space", false)
		}
		if open.Clicked() {
			st.handoff = ""
			a.OpenNotification(api.Delivery{SourceKind: st.kind, SourceID: st.id})
		}
	})
	switch st.handoff {
	case "trying":
		muted(c, "Membuka di aplikasi Space… Bila tidak terjadi apa-apa, kamu tetap di sini.").FontSize(13)
	case "opened":
		ui.Text(c, "Terbuka di aplikasi Space. Halaman ini boleh ditutup.").FontSize(13).TextColor(t.Success)
	case "failed":
		muted(c, "Aplikasi Space tidak terbuka. Belum terpasang? Unduh Space untuk Windows, atau lanjutkan di web.").FontSize(13)
		if onbLink(c, "Unduh Space untuk Windows", "Unduh Space untuk Windows").Clicked() {
			a.Web.Go("/unduh/windows")
		}
	}
}

// cardHandoff tries space:// for the card; once it worked, this browser
// tries it by itself next time, and stops showing the install bar.
func (a *App) cardHandoff() {
	st := &a.card
	if a.Web == nil || st.handoff == "trying" {
		return
	}
	// Tried once for this card: by hand or by itself, not both.
	st.handoff, st.auto = "trying", true
	cid, done := st.id, a.Web.OpenApp(cardAppLink(st.kind, st.id), webHandoffWait)
	a.async(func() func() {
		opened := <-done
		return func() {
			if a.card.id != cid || a.card.handoff != "trying" {
				return
			}
			if opened {
				a.card.handoff = "opened"
				a.Web.Set(webHandoffKey, "1")
			} else {
				a.card.handoff = "failed"
			}
		}
	})
}

// webBannerShown tells whether the web's install bar shows: a Windows
// computer's browser that has not opened the app from Space, has not put
// the bar off, and whose account has not turned it off.
func (a *App) webBannerShown(c *ui.Context) bool {
	w := a.Web
	if w == nil || a.screen != screenShell || isMobile(c) || !w.WindowsDesktop() {
		return false
	}
	if w.Get(webHandoffKey) == "1" || w.Get(webBannerOffKey) == "1" || a.onb.data.Done(api.OnboardWebAppBannerOff) {
		return false
	}
	if until, err := strconv.ParseInt(w.Get(webBannerUntilKey), 10, 64); err == nil && c.Now().Unix() < until {
		return false
	}
	return true
}

// webBanner is the thin bar over the web: Space for Windows, with Unduh,
// Nanti (14 days) and Jangan ingatkan lagi (the account, every browser).
func (a *App) webBanner(c *ui.Context) {
	if !a.webBannerShown(c) {
		return
	}
	t := c.Theme()
	ui.Row(c.Key("web-banner")).FillWidth().Height(40).Shrink(0).Padding(0, 12, 0, 16).Gap(10).AlignItems(ui.Center).Background(spaceMix(t.Accent, t.Background, .92)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Label("Space untuk Windows").Children(func() {
		ui.Icon(c, spaceIcon("monitor")).Size(16, 16).TextColor(t.Accent).Shrink(0)
		ui.Text(c, "Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat").FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
		if onbPrimary(c, "Unduh Space untuk Windows", "Unduh", true).Clicked() {
			a.Web.Go("/unduh/windows")
		}
		if onbLink(c, "Nanti, sembunyikan 14 hari", "Nanti").Clicked() {
			a.Web.Set(webBannerUntilKey, strconv.FormatInt(c.Now().Add(webBannerLater).Unix(), 10))
		}
		if onbLink(c, "Jangan ingatkan lagi", "Jangan ingatkan lagi").Clicked() {
			a.Web.Set(webBannerOffKey, "1")
			if a.onboardingActive() || a.onb.loaded {
				a.markOnboarding(api.OnboardWebAppBannerOff)
			}
		}
	})
}
