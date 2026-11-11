package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// header is canvas A's topbar: the breadcrumb above the page title, the
// sync pill, search, the Space's members, the bell, Ping and Buat tugas.
// The connection and sync state live in the pill; there is no separate sync
// row or status bar.
func (a *App) header(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	if a.chrome.preferences.SidebarCollapsed || viewportMode(c) == layoutTablet {
		w -= 60
	} else {
		w -= 248
	}
	if a.taskBoardVisible() && !a.chrome.appearance && a.tasks.draft != nil && a.tasks.draft.taskID == id.Nil {
		if a.chrome.drawerWidth > 0 {
			w -= a.chrome.drawerWidth
		} else {
			w -= 416
		}
	}
	path := a.router.Path()
	title := pageTitle(path)
	if title == "" {
		title = a.spaceTitle()
	}
	crumb := ""
	if p := a.period(); p != nil {
		crumb = "Periode " + p.Label
	}
	if (path == PathSpaces || path == PathMedhum) && a.selectedSpace() != nil {
		crumb = strings.TrimSuffix("Space · "+crumb, " · ")
	}
	searchWidth := float32(250)
	switch {
	case w < 900:
		searchWidth = 150
	case w < 1100:
		searchWidth = 200
	}
	// Narrow (canvas A · Tablet, or a drawer beside the page): no
	// breadcrumb, the pill a dot, search and Ping as icon buttons, so the
	// title is not cut.
	narrow := w < 900
	bar := ui.Row(c.Key("topbar")).FillWidth().Height(60).Shrink(0).Padding(0, 28).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	if narrow {
		bar.Padding(0, 16).Gap(8)
	}
	bar.Children(func() {
		ui.Column(c).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			if crumb != "" && !narrow {
				ui.Text(c, crumb).FontSize(11).TextColor(faint(t)).MaxLines(1)
			}
			ui.Text(c, title).FontSize(20).FontWeight(600).MaxLines(1)
		})
		a.syncPill(c, narrow)
		ui.Box(c).Grow(1).MinWidth(0)
		if narrow {
			a.headerSearchButton(c)
		} else {
			a.headerSearch(c, searchWidth)
		}
		a.headerMembers(c, w >= 900)
		a.helpButton(c, 34)
		a.headerBell(c)
		if canPing(a.selectedSpace()) && path == PathSpaces {
			ping := ui.ButtonBase(c).Label("Ping").Tooltip("Kirim ping ke anggota").Row().AlignItems(ui.Center).Justify(ui.Center).Gap(8).Height(36).Shrink(0).Padding(0, 12).Radius(8).Border(1, t.Border).TextColor(t.Text)
			if narrow {
				ping.Width(36).Padding(0)
			}
			if ping.Hovered() || a.chrome.pingOpen {
				ping.Background(t.SurfaceHover)
			}
			ping.FocusRing(ping.FocusVisible()).Children(func() {
				ui.Icon(c, chromeIcon("ping")).Size(16, 16)
				if !narrow {
					ui.Text(c, "Ping")
				}
			})
			if ping.Clicked() {
				a.chrome.pingOpen = true
			}
		}
		a.taskHeaderAction(c)
	})
}

// searchPlaceholder says what quick navigation finds.
const searchPlaceholder = "Cari halaman, pengaturan, aksi, tugas"

func (a *App) headerSearch(c *ui.Context, width float32) {
	t := c.Theme()
	ui.Column(c).Width(width).Shrink(0).Children(func() {
		var input ui.Element
		shortcut := c.Shortcut(ui.Ctrl, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyK)
		anchor := ui.Row(c).FillWidth().Height(34).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(8).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Icon(c, chromeIcon("search")).Size(16, 16).TextColor(faint(t)).Shrink(0)
			input = ui.TextInput(c.Key("global-search"), &a.chrome.search).Label("Cari di Space").Placeholder(searchPlaceholder).Grow(1).MinWidth(0).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0)
			if shortcut && a.toggleSearch() {
				input.Focus()
			}
			if !a.chrome.searchOpen && (input.Clicked() || input.Changed()) {
				a.toggleSearch()
			}
			// Enter runs the chosen result only while the results show;
			// with them closed it shows them first.
			if input.Submitted() {
				if a.chrome.searchOpen {
					a.chrome.searchSubmit = true
				} else {
					a.toggleSearch()
				}
			}
			if width >= 200 {
				kbd(c, "Ctrl K")
			}
		})
		popover(c, anchor, &a.chrome.searchOpen, max(width, 280), func() { a.searchResults(c) })
	})
}

// headerSearchButton is the narrow topbar's search: an icon button whose
// popover holds the search field and its results.
func (a *App) headerSearchButton(c *ui.Context) {
	t := c.Theme()
	shortcut := c.Shortcut(ui.Ctrl, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyK)
	b := ui.ButtonBase(c.Key("search-button")).Label("Buka pencarian").Tooltip("Cari · Ctrl K").Size(34, 34).Shrink(0).Center().Radius(8).Border(1, t.Border).TextColor(t.TextMuted)
	if b.Hovered() || a.chrome.searchOpen {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Icon(c, chromeIcon("search")).Size(16, 16) })
	focus := (shortcut || b.Clicked()) && a.toggleSearch()
	popover(c, b, &a.chrome.searchOpen, 320, func() {
		input := ui.TextInput(c.Key("global-search"), &a.chrome.search).Label("Cari di Space").Placeholder(searchPlaceholder).FillWidth()
		if focus {
			input.Focus()
		}
		if input.Submitted() {
			a.chrome.searchSubmit = true
		}
		a.searchResults(c)
	})
}

func (a *App) headerBell(c *ui.Context) {
	t := c.Theme()
	unread := false
	for _, n := range a.tasks.notifications {
		if n.ReadAt == nil {
			unread = true
			break
		}
	}
	label := "Notifikasi"
	if unread {
		label = "Notifikasi · ada yang belum dibaca"
	}
	b := ui.ButtonBase(c.Key("bell")).Label("Notifikasi").Tooltip(label).Size(34, 34).Shrink(0).Center().Radius(8).TextColor(t.TextMuted)
	if b.Hovered() || a.chrome.notificationsOpen {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, chromeIcon("bell")).Size(16, 16)
		if unread {
			ui.Box(c).Absolute().Top(5).Left(20).Size(8, 8).Radius(4).Background(t.Danger).Border(2, t.Background)
		}
	})
	if b.Clicked() {
		a.chrome.membersOpen = false
		a.chrome.notificationsOpen = !a.chrome.notificationsOpen
		a.chrome.searchOpen = false
		a.chrome.profileOpen = false
	}
	popover(c, b, &a.chrome.notificationsOpen, 310, func() {
		ui.Text(c, "Notifikasi").FontSize(17).Bold()
		muted(c, "Pembaruan yang memerlukan perhatianmu.")
		a.deliveryNotifications(c)
		a.taskNotifications(c)
	})
}

// syncState is what the sync pill and the phone's status dot show.
type syncState struct {
	// text is the pill's, as "Tersinkron · 07.15" or "Offline · 3 antre";
	// label names the connection for assistive technology.
	text, label, tip string
	color            ui.Color
	// review is set when queued changes need the user: a conflict, a
	// rejection or an error.
	review bool
	queued int
}

func (a *App) syncState(t *ui.Theme) syncState {
	s := syncState{text: "Tersinkron", label: "Tersambung", color: t.Success}
	var last time.Time
	syncing, errText, offline := false, a.offlineSync.err, a.conn == connOffline
	if cl := a.offlineClient(); cl != nil {
		var off bool
		var err string
		off, syncing, last, err = cl.OfflineStatus()
		offline = offline || off
		if errText == "" {
			errText = err
		}
	}
	for _, p := range a.offlineSync.items {
		switch p.State {
		case "conflict", "rejected":
			s.review = true
		default:
			s.queued++
		}
	}
	for _, at := range []time.Time{a.syncedAt, a.realtime.lastRefresh} {
		if at.After(last) {
			last = at
		}
	}
	queued := ""
	if s.queued > 0 {
		queued = fmt.Sprintf("%d antre", s.queued)
	}
	switch {
	case offline:
		s.text, s.label, s.color = strings.TrimSuffix("Offline · "+queued, " · "), "Tidak ada koneksi", t.Danger
		s.tip = "Tidak ada koneksi. Perubahan yang bisa diantrekan tersimpan di perangkat. Klik untuk mencoba lagi."
	case a.conn == connConnecting:
		s.text, s.label, s.color = "Menyambungkan…", "Menyambungkan…", t.Warning
	case syncing && s.queued > 0:
		s.text, s.color = "Menyinkronkan…", t.Warning
	case s.review:
		s.text, s.color = "Perlu ditinjau", t.Danger
		s.tip = "Ada perubahan perangkat yang bentrok atau ditolak. Klik untuk meninjau."
	case errText != "":
		s.text, s.color, s.review = "Gagal sinkron", t.Danger, true
		s.tip = "Klik untuk meninjau antrean sinkronisasi."
	case s.queued > 0:
		s.text, s.color = queued, t.Warning
		s.tip = "Perubahan tersimpan di perangkat — belum tersinkron. Klik untuk sinkronkan."
	case !last.IsZero():
		s.text += " · " + last.Local().Format("15.04")
	}
	if s.tip == "" {
		s.tip = "Diperiksa setiap 30 detik. Klik untuk sinkronkan sekarang."
		if a.realtime.connected {
			s.tip = "Pembaruan langsung aktif. Klik untuk sinkronkan sekarang."
		}
	}
	if errText != "" {
		s.tip += "\n" + errText
	}
	return s
}

// syncPill shows the connection and sync state in the topbar. A click
// syncs; with changes queued or in conflict it opens Pengaturan ›
// Sinkronisasi too. compact keeps the dot and the time alone.
func (a *App) syncPill(c *ui.Context, compact bool) {
	t := c.Theme()
	s := a.syncState(t)
	b := ui.ButtonBase(c.Key("sync-pill")).Label(s.label).Tooltip(s.tip).Row().AlignItems(ui.Center).Gap(6).Shrink(0).Padding(4, 10).Radius(999).Background(t.SurfaceHover).TextColor(t.TextMuted)
	if b.Hovered() {
		b.TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Box(c).Size(7, 7).Radius(3.5).Background(s.color).Shrink(0)
		text := s.text
		if compact {
			// Synced: the dot alone, as canvas A · Tablet; the tooltip
			// keeps the time. Otherwise the part that needs attention.
			if strings.HasPrefix(text, "Tersinkron") {
				return
			}
			if i := strings.LastIndex(text, " · "); i >= 0 {
				text = text[i+len(" · "):]
			}
		}
		ui.Text(c, text).FontSize(12).MaxLines(1)
	})
	if b.Clicked() {
		a.ping(c.Now())
		a.refreshOffline(true)
		if s.review || s.queued > 0 {
			a.chrome.settingsTab = "Sinkronisasi"
			a.chrome.settingsDetail = true
			a.navigate(PathPengaturan)
		}
	}
}
