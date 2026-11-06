package appui

import (
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// PathLainnya lists, on phones, what the desktop sidebar and Space tabs hold.
const PathLainnya = "/lainnya"

// layoutMode follows the window width, not the device: the web app and a
// narrowed desktop window share one layout per width. The breakpoints match
// the approved canvas (phone 390, tablet 768, desktop 1440).
type layoutMode int

const (
	layoutDesktop layoutMode = iota
	layoutTablet
	layoutMobile
)

const (
	mobileMaxWidth = 720
	tabletMaxWidth = 1024
)

func viewportMode(c *ui.Context) layoutMode {
	w, _ := c.Size()
	switch {
	case w < mobileMaxWidth:
		return layoutMobile
	case w < tabletMaxWidth:
		return layoutTablet
	}
	return layoutDesktop
}

func isMobile(c *ui.Context) bool { return viewportMode(c) == layoutMobile }

// mobileShell is the phone layout: a top bar, the page, and a bottom tab
// bar. Panels that sit beside the board on desktop open full screen.
func (a *App) mobileShell(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	ui.Column(c.Key("mobile-shell")).Fill().Children(func() {
		a.mobileTopBar(c)
		a.offlineBanner(c)
		a.router.View(c, func(r *ui.Route) { a.page(c, r) }).Grow(1).MinHeight(0).FillWidth().MinWidth(0)
		a.mobileNav(c)
	})
	overlay := a.chrome.appearance || (a.taskBoardVisible() && ((a.tasks.draft != nil && a.tasks.draft.taskID == id.Nil) || (a.workspace != nil && a.workspace.SelectedID() != id.Nil)))
	if overlay {
		ui.Overlay(c, func() {
			ui.Column(c.Key("mobile-panel")).Absolute().Left(0).Top(0).Width(w).Height(h).Background(t.Background).Children(func() {
				switch {
				case a.chrome.appearance:
					a.appearance(c)
				case a.tasks.draft != nil && a.tasks.draft.taskID == id.Nil:
					a.taskCreatePanel(c)
				default:
					a.workspace.Inspector(c)
				}
			})
		})
		return
	}
	if a.mobileShowsCreate() {
		ui.Overlay(c, func() {
			b := ui.ButtonBase(c.Key("mobile-fab")).Label("Buat tugas").Tooltip("Buat tugas").Absolute().Left(w-72).Top(h-64-72).Size(56, 56).Radius(16).Center().Background(t.Accent).TextColor(t.AccentText).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .35))
			b.FocusRing(b.FocusVisible()).Children(func() { ui.Icon(c, spaceIcon("plus")).Size(24, 24) })
			if b.Clicked() {
				a.startTaskFromAnywhere()
			}
		})
	}
}

func (a *App) mobileShowsCreate() bool {
	if a.isAdmin() || len(a.manageableDivisions()) == 0 {
		return false
	}
	p := a.router.Path()
	return p == PathJobdesk || (p == PathSpaces && a.selectedSpace() != nil && (a.chrome.tab == "Beranda" || a.chrome.tab == "Board tugas"))
}

// startTaskFromAnywhere opens the creation form on the selected Space's
// board, or on the first Space the user may delegate in.
func (a *App) startTaskFromAnywhere() {
	if sp := a.selectedSpace(); sp == nil || sp.ReadOnly || !spaceCan(*sp, api.CapTasksManage) {
		for _, s := range a.access.spaces {
			if !s.ReadOnly && spaceCan(s, api.CapTasksManage) {
				a.selectSpace(s.ID)
				break
			}
		}
	}
	a.ensureTasks(false)
	a.beginTask()
	a.navigate(PathSpaces)
	a.chrome.tab = "Board tugas"
}

func spaceCan(s api.Space, capability string) bool {
	for _, r := range s.Roles {
		for _, c := range r.Capabilities {
			if c == capability {
				return true
			}
		}
	}
	for _, c := range s.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

func (a *App) mobileTopBar(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	ui.Row(c.Key("mobile-top")).FillWidth().Height(56).Shrink(0).Padding(0, 8, 0, 12).Gap(6).AlignItems(ui.Center).Background(t.Background).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Image(c, spaceBrand()).Size(30, 30).Shrink(0).Fit(ui.Contain)
		title := map[string]string{PathJobdesk: "Jobdesk saya", PathReminder: "Reminder pribadi", PathArsip: "Arsip periode", PathPengaturan: "Pengaturan", PathOrganisasi: "Organisasi", PathEvaluasi: "Evaluasi SOP", PathLainnya: "Lainnya"}[a.router.Path()]
		if title == "" {
			title = a.spaceTitle()
		}
		anchor := ui.ButtonBase(c.Key("mobile-space")).Label("Pindah ruang").Row().Gap(4).AlignItems(ui.Center).MinHeight(44).Padding(0, 6).Radius(8).MinWidth(0).Shrink(1)
		anchor.FocusRing(anchor.FocusVisible()).Children(func() {
			ui.Text(c, title).FontSize(16).Bold().MaxLines(1).MinWidth(0)
			ui.Icon(c, spaceIcon("down")).Size(14, 14).Shrink(0)
		})
		if anchor.Clicked() {
			a.chrome.periodOpen = !a.chrome.periodOpen
			a.chrome.notificationsOpen = false
		}
		popover(c, anchor, &a.chrome.periodOpen, min(w-24, 320), func() {
			ui.Text(c, "Pindah ruang").Bold().Padding(4)
			for _, s := range a.access.spaces {
				if sidebarRow(c.Key(s.ID), s.Name, a.selectedSpaceID() == s.ID && a.router.Path() == PathSpaces).Clicked() {
					a.selectSpace(s.ID)
					a.chrome.tab = defaultSpaceTab(s)
					a.navigate(PathSpaces)
					a.chrome.periodOpen = false
				}
			}
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			ui.Text(c, "Periode").Bold().Padding(4)
			for _, p := range a.periods {
				if sidebarRow(c.Key("period-"+p.ID.String()), periodLabel(p), a.periodID == p.ID).Clicked() {
					a.changePeriod(p.ID)
					a.chrome.periodOpen = false
				}
			}
		})
		ui.Box(c).Grow(1)
		color := t.Success
		switch a.conn {
		case connOffline:
			color = t.Danger
		case connConnecting:
			color = t.Warning
		}
		status := map[connState]string{connOffline: "Tidak ada koneksi", connConnecting: "Menyambungkan…"}[a.conn]
		if status == "" {
			status = "Tersambung"
		}
		ui.Box(c.Key("mobile-sync")).Label(status).Size(8, 8).Radius(4).Background(color).Margin(0, 6, 0, 0)
		bell := ui.ButtonBase(c.Key("mobile-bell")).Label("Notifikasi").Size(44, 44).Radius(10).Center().TextColor(t.TextMuted)
		bell.FocusRing(bell.FocusVisible()).Children(func() { ui.Icon(c, spaceIcon("bell")).Size(20, 20) })
		if bell.Clicked() {
			a.chrome.notificationsOpen = !a.chrome.notificationsOpen
			a.chrome.periodOpen = false
		}
		popover(c, bell, &a.chrome.notificationsOpen, min(w-24, 360), func() {
			ui.Text(c, "Notifikasi").FontSize(17).Bold()
			a.deliveryNotifications(c)
			a.taskNotifications(c)
		})
		me := ui.ButtonBase(c.Key("mobile-me")).Label("Menu profil").Size(44, 44).Radius(10).Center()
		me.FocusRing(me.FocusVisible()).Children(func() {
			ui.Box(c).Size(30, 30).Radius(15).Center().Background(t.Accent).Children(func() {
				ui.Text(c, personInitials(a.user.DisplayName)).FontSize(11).Bold().TextColor(t.AccentText)
			})
		})
		if me.Clicked() {
			a.navigate(PathLainnya)
		}
	})
}

func sidebarRow(c *ui.Context, label string, active bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).FillWidth().Row().AlignItems(ui.Center).MinHeight(44).Padding(0, 10).Radius(8).TextColor(t.Text)
	if active {
		b.Background(t.SurfaceHover)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, label).MaxLines(1) })
	return b
}

type mobileTab struct{ key, label, icon string }

func (a *App) mobileActiveTab() string {
	p := a.router.Path()
	switch {
	case p == PathJobdesk:
		return "jobdesk"
	case p == PathSpaces || p == PathMedhum:
		if sp := a.selectedSpace(); sp != nil && sp.Kind == api.SpaceShared {
			return "medhum"
		}
		if a.chrome.tab == "Board tugas" {
			return "board"
		}
		if a.chrome.tab == "Beranda" || a.chrome.tab == "" {
			return "home"
		}
	}
	return "more"
}

func (a *App) mobileNav(c *ui.Context) {
	t := c.Theme()
	active := a.mobileActiveTab()
	tabs := []mobileTab{{"home", "Beranda", "home"}, {"jobdesk", "Jobdesk", "task"}, {"board", "Board", "board"}, {"medhum", "Medhum", "link"}, {"more", "Lainnya", "more"}}
	ui.Row(c.Key("mobile-nav")).FillWidth().Height(64).Shrink(0).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		for _, tab := range tabs {
			on := active == tab.key
			b := ui.ButtonBase(c.Key(tab.key)).Label(tab.label).Grow(1).MinWidth(0).FillHeight().Column().Center().Gap(3).TextColor(t.TextMuted)
			if on {
				b.TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				icon := ui.Icon(c, spaceIcon(tab.icon)).Size(20, 20)
				if on {
					icon.TextColor(t.Accent)
				}
				ui.Text(c, tab.label).FontSize(11)
			})
			if b.Clicked() {
				a.openMobileTab(tab.key)
			}
		}
	})
}

func (a *App) openMobileTab(key string) {
	switch key {
	case "jobdesk":
		a.navigate(PathJobdesk)
	case "home", "board":
		if sp := a.selectedSpace(); sp == nil || sp.Kind == api.SpaceShared {
			for _, s := range a.access.spaces {
				if s.Kind != api.SpaceShared {
					a.selectSpace(s.ID)
					break
				}
			}
		}
		a.chrome.tab = map[string]string{"home": "Beranda", "board": "Board tugas"}[key]
		a.navigate(PathSpaces)
	case "medhum":
		for _, s := range a.access.spaces {
			if s.Kind == api.SpaceShared {
				a.selectSpace(s.ID)
				a.chrome.tab = defaultSpaceTab(s)
				break
			}
		}
		a.navigate(PathSpaces)
	default:
		a.navigate(PathLainnya)
	}
}

// lainnyaPage gathers, for phones, what the sidebar, the profile menu and
// the Space tabs reach on wider windows.
func (a *App) lainnyaPage(c *ui.Context) {
	t := c.Theme()
	type entry struct {
		label, detail, icon string
		open                func()
	}
	space := func(tab string) func() {
		return func() {
			if a.selectedSpace() == nil {
				a.openMobileTab("home")
			}
			a.chrome.tab = tab
			a.navigate(PathSpaces)
		}
	}
	entries := []entry{
		{"Reminder pribadi", "Privat, hanya ke perangkatmu", "bell", func() { a.navigate(PathReminder) }},
		{"Kirim ping", "Ingatkan anggota divisi", "send", func() {
			if a.selectedSpace() == nil {
				a.openMobileTab("home")
			}
			a.chrome.pingOpen = true
		}},
		{"Agenda", "Rapat dan kehadiran", "calendar", space("Agenda")},
		{"Notes", "Catatan dan template", "note", space("Notes")},
		{"Anggota", "Role dan undangan", "users", space("Anggota")},
		{"Pengaturan Space", "Bot WhatsApp dan pengingat", "whatsapp", space("Pengaturan")},
		{"Evaluasi SOP", "Temuan dan laporan", "shield", func() { a.navigate(PathEvaluasi) }},
		{"Arsip periode", "Periode lama", "archive", func() { a.navigate(PathArsip) }},
		{"Pengaturan", "Perangkat, akun, sinkronisasi", "settings", func() { a.navigate(PathPengaturan) }},
		{"Penampilan", "Tema, aksen, kepadatan", "palette", func() { a.chrome.appearance = true }},
	}
	if a.isAdmin() {
		entries = append(entries, entry{"Organisasi", "Periode dan kadiv", "shield", func() { a.navigate(PathOrganisasi) }})
	}
	ui.Scroll(c.Key("lainnya")).Fill().Label("Menu lainnya").Children(func() {
		ui.Column(c).FillWidth().Padding(16, 16, 32, 16).Gap(14).Children(func() {
			ui.Text(c, "Lainnya").FontSize(22).Bold()
			ui.Grid(c).FillWidth().Columns(2).Gap(10).Children(func() {
				for _, e := range entries {
					b := ui.ButtonBase(c.Key(e.label)).Label(e.label).Column().Gap(10).Padding(14).MinHeight(92).Radius(14).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Start)
					if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.FocusRing(b.FocusVisible()).Children(func() {
						ui.Icon(c, spaceIcon(e.icon)).Size(20, 20).TextColor(t.Accent)
						ui.Column(c).Gap(2).Children(func() {
							ui.Text(c, e.label).FontSize(14).MaxLines(1)
							ui.Text(c, e.detail).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
						})
					})
					if b.Clicked() {
						e.open()
					}
				}
			})
			ui.Row(c).FillWidth().Padding(12).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(36, 36).Radius(18).Center().Background(t.Accent).Children(func() {
					ui.Text(c, personInitials(a.user.DisplayName)).Bold().TextColor(t.AccentText)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, a.user.DisplayName).Bold().MaxLines(1)
					muted(c, a.user.Email).MaxLines(1)
				})
				if spaceButton(c, "Keluar akun", "logout", false).Disabled(a.loggingOut).Clicked() {
					a.logout()
				}
			})
			muted(c, "Space · "+a.Version)
		})
	})
}

// defaultSpaceTab is the tab a Space opens on: Medhum opens on its content,
// as on canvas A; division Spaces on their Beranda.
func defaultSpaceTab(s api.Space) string {
	if s.Code == "medhum" {
		return medhumSpaceTab
	}
	return "Beranda"
}
