package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// page renders only data the current service can confirm. Unavailable task,
// membership and bot services never turn into local sample data or saves.
func (a *App) page(c *ui.Context, r *ui.Route) {
	switch {
	case r.Match(PathJobdesk):
		r.Title("Jobdesk saya")
		a.jobdeskView(c)
	case r.Match(PathReminder):
		r.Title("Reminder pribadi")
		a.remindersView(c)
	case r.Match(PathSpaces):
		a.spaces(c, r)
	case r.Match(PathMedhum):
		a.spaces(c, r)
	case r.Match(PathEvaluasi):
		r.Title("Evaluasi SOP")
		a.sopView(c)
	case r.Match(PathArchiveAccess):
		r.Title("Izin arsip")
		a.archiveManagementView(c)
	case r.Match(PathArsip):
		a.archivePage(c, r)
	case r.Match(PathPengaturan):
		a.settings(c, r)
	case r.Match(PathLainnya):
		r.Title("Lainnya")
		a.lainnyaPage(c)
	case r.Match(PathOrganisasi) && a.isAdmin():
		a.organizationPage(c, r)
	default:
		r.Title("Tidak ditemukan")
		pageBody(c, "Halaman tidak ditemukan", func() {
			muted(c, "Halaman ini tidak ada atau tidak tersedia untuk akunmu.")
			if spaceButton(c, "Ke Jobdesk saya", "arrow", false).Clicked() {
				a.router.Replace(PathJobdesk)
			}
		})
	}
}

func pageBody(c *ui.Context, title string, fn func()) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1280).Padding(26).Gap(20).Children(func() {
			ui.Text(c, title).FontSize(26).Bold()
			fn()
		})
	})
}

// settingsBody gives each settings page the same readable header and content
// width while allowing the page to present its own actions and state.
func settingsBody(c *ui.Context, title, description string, fn func()) {
	ui.Scroll(c).Fill().Label("Isi pengaturan").Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1000).Padding(26).Gap(20).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(6).Children(func() {
				ui.Text(c, title).FontSize(26).Bold()
				muted(c, description)
			})
			fn()
		})
	})
}

func (a *App) periodLine(c *ui.Context) {
	if p := a.period(); p != nil {
		muted(c, "Periode "+periodLabel(*p))
	}
}

func (a *App) spaces(c *ui.Context, r *ui.Route) {
	if s := a.selectedSpace(); s != nil {
		a.spacePage(c, r, s.Kind == api.SpaceShared)
		return
	}
	r.Title("Space saya")
	pageBody(c, "Space saya", func() {
		a.periodLine(c)
		muted(c, "Ruang yang dapat kamu akses pada kepengurusan ini.")
		if a.access.notice != "" {
			pageNotice(c, "lock", "Akses diperbarui", a.access.notice)
		}
		if a.access.loading {
			pageUnavailable(c, "refresh", "Memuat Space…", "Memeriksa akses dari server.")
			return
		}
		if a.access.err != "" {
			pageNotice(c, "alert", "Space belum dapat dimuat", a.access.err)
		}
		if spaceButton(c, "Muat ulang Space", "refresh", false).Disabled(a.access.loading).Clicked() {
			a.loadSpaces()
		}
		if len(a.access.spaces) == 0 && a.access.err == "" {
			pageEmpty(c, "lock", "Belum ada Space", "Gunakan undangan dari kadiv atau hubungi pengelola untuk mendapatkan akses.")
		}
		for _, s := range a.access.spaces {
			desc := "Ruang divisi"
			if s.Kind == api.SpaceShared {
				desc = "Kolaborasi lintas divisi"
			}
			if s.ReadOnly {
				desc += " · Hanya baca"
			}
			pageSection(c.Key(s.ID), s.Name, desc, func() {
				for _, d := range s.Divisions {
					pageDivisionMark(c, d)
				}
				if spaceButton(c, "Buka "+s.Name, "arrow", false).Clicked() {
					a.selectSpace(s.ID)
				}
			})
		}
	})
}

func (a *App) divisionCatalog(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.divisionsLoading && len(a.divisions) == 0:
		pageUnavailable(c, "refresh", "Memuat divisi…", "Mengambil direktori divisi HIMASIF.")
	case a.divisionsErr != "":
		pageNotice(c, "alert", "Divisi belum dapat dimuat", a.divisionsErr)
		if spaceButton(c, "Muat ulang divisi", "refresh", false).Disabled(a.divisionsLoading).Clicked() {
			a.loadDivisions()
		}
	case len(a.divisions) == 0:
		pageUnavailable(c, "grid", "Belum ada divisi", "Direktori divisi masih kosong.")
	default:
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			for _, division := range a.divisions {
				entry := ui.Column(c.Key(division.ID)).FillWidth().MinWidth(0).Padding(16).Gap(14).Radius(11).Background(t.Surface).Border(1, t.Border)
				wide := entry.Bounds().W >= 480
				if wide {
					entry.Row().AlignItems(ui.Center)
				}
				entry.Children(func() {
					details := ui.Row(c).MinWidth(0).Gap(14).AlignItems(ui.Center)
					if wide {
						details.Grow(1)
					} else {
						details.FillWidth()
					}
					details.Children(func() {
						pageDivisionMark(c, division)
						ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
							ui.Text(c, division.Name).FontSize(14).Bold()
							muted(c, division.Slug)
						})
					})
					muted(c, "Identitas organisasi")
				})
			}
		})
	}
}

func (a *App) spacePage(c *ui.Context, r *ui.Route, shared bool) {
	title := a.spaceTitle()
	if shared {
		title = "Medhum"
	}
	r.Title(title)
	sp := a.selectedSpace()
	medhum := sp != nil && sp.Code == "medhum"
	if medhum {
		// Older entry points name the tabs that are now Medhum sub-tabs.
		if sub, ok := map[string]string{"Kalender konten": medhumTabList, "Konsep dan bukti": medhumTabInbox, "Sumber kalender": medhumTabSources}[a.chrome.tab]; ok {
			a.ensureMedhum()
			a.chrome.tab, a.medhum.tab = medhumSpaceTab, sub
		}
	}
	a.resolveSpaceTab(sp)
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		a.spaceTabBar(c, sp)
		if s := a.selectedSpace(); s != nil && s.ReadOnly {
			pageNotice(c, "lock", "Hanya baca", "Periode atau akses arsip ini tidak mengizinkan perubahan.")
		}

		switch a.chrome.tab {
		case medhumSpaceTab:
			if medhum {
				a.medhumPage(c)
			} else {
				a.overviewPage(c, title, shared)
			}
		case "Board tugas":
			a.spaceTasksView(c)
		case "Agenda":
			a.agendaCalendar(c)
		case "Catatan":
			a.notesPage(c)
		case "Rapat":
			a.meetingsView(c)
		case "Tinjauan":
			a.spaceReviewPage(c)
		case "Ping":
			a.spacePingPage(c)
		case "Anggota":
			a.membersPage(c)
		case "Bot WhatsApp":
			a.whatsappSettings(c)
		default:
			a.overviewPage(c, title, shared)
		}
	})
}

func (a *App) agendaPage(c *ui.Context) {
	a.meetingsView(c)
}

func (a *App) notesPage(c *ui.Context) {
	a.notesView(c)
}

func (a *App) archivePage(c *ui.Context, r *ui.Route) {
	r.Title("Arsip periode")
	pageBody(c, "Arsip periode", func() {
		muted(c, "Ruang untuk melihat perjalanan kepengurusan sebelumnya.")
		if len(a.archiveManagedDivisions()) > 0 {
			if spaceButton(c, "Kelola izin arsip", "shield", false).Clicked() {
				a.navigate(PathArchiveAccess)
			}
		}
		pageNotice(c, "lock", "Informasi periode", "Daftar di bawah hanya berisi metadata periode yang telah ditutup. Akses arsip pekerjaan tetap memerlukan izin.")
		if a.periodsErr != "" {
			pageNotice(c, "alert", "Periode belum dapat dimuat", a.periodsErr)
			if spaceButton(c, "Muat ulang periode", "refresh", false).Disabled(a.periodsLoading).Clicked() {
				a.loadPeriods()
			}
			return
		}
		closed := 0
		for _, period := range a.periods {
			if period.Status == api.PeriodClosed {
				closed++
				pagePeriodCard(c, period)
				if spaceButton(c, "Lihat ruang "+period.Label, "archive", false).Clicked() {
					a.changePeriod(period.ID)
					a.navigate(PathSpaces)
				}
			}
		}
		if closed == 0 {
			if a.periodsLoading {
				pageUnavailable(c, "refresh", "Memuat periode…", "Mengambil daftar kepengurusan.")
			} else {
				pageUnavailable(c, "archive", "Belum ada periode ditutup", "Periode yang telah ditutup akan tercantum di sini.")
			}
		}
	})
}

func (a *App) organizationPage(c *ui.Context, r *ui.Route) {
	r.Title("Pengaturan organisasi")
	a.organizationAdminView(c)
}

func (a *App) spaceSettings(c *ui.Context, shared bool) {
	pageBody(c, "Pengaturan space", func() {
		if shared {
			pageNotice(c, "whatsapp", "Medhum dilayani bot Humas", "Bot Humas ditujukan untuk grup Humas dan Medhum. Bot Mediatek berada di grup divisinya sendiri.")
		}
		if s := a.selectedSpace(); s != nil {
			info(c, [][2]string{{"Space", s.Name}, {"Jenis", s.Kind}, {"Akses", s.Access}})
		}
		if spaceButton(c, "Kelola anggota dan akses", "users", false).Clicked() {
			a.chrome.tab = "Anggota"
			a.navigate(PathSpaces)
		}
		if spaceButton(c, "Lihat pengaturan WhatsApp", "whatsapp", false).Clicked() {
			a.chrome.settingsTab = "WhatsApp"
			a.router.Push(PathPengaturan)
		}
		pageSection(c, "Identitas space", "Logo bawaan mengikuti identitas divisi. Pengaturan logo dan nama kustom belum tersedia.", func() {
			if d := a.selectedDivision(); d != nil && !shared {
				pageDivisionMark(c, *d)
				info(c, [][2]string{{"Nama divisi", d.Name}, {"Slug", d.Slug}})
			}
		})
	})
}

func (a *App) reminderSettings(c *ui.Context) { a.notificationSettings(c) }

func (a *App) accountPage(c *ui.Context) {
	settingsBody(c, "Akun dan aplikasi", "Kelola identitas dan keamanan, serta periksa koneksi aplikasi ini.", func() {
		kind := "Pribadi"
		if a.user.Kind == api.AccountOrganizationAdmin {
			kind = "Admin organisasi"
		}
		verified := "Belum diverifikasi"
		if a.user.EmailVerified {
			verified = "Terverifikasi"
		}
		pageSection(c, "Akunmu", "Identitas akun yang sedang masuk.", func() {
			info(c, [][2]string{{"Nama", a.user.DisplayName}, {"Email", a.user.Email}, {"Jenis akun", kind}, {"Verifikasi email", verified}})
		})
		a.accountCredentialsView(c)
		pageSection(c, "Aplikasi", "Informasi koneksi Space pada perangkat ini.", func() { info(c, [][2]string{{"Server", a.ServerURL}, {"Versi", a.Version}}) })
		if spaceButton(c, "Penampilan", "palette", false).Clicked() {
			a.chrome.appearance = true
		}
		label := "Keluar"
		if a.loggingOut {
			label = "Keluar…"
		}
		if spaceButton(c, label, "logout", false).Disabled(a.loggingOut).TextColor(c.Theme().Danger).Clicked() {
			a.logout()
		}
	})
}

func info(c *ui.Context, rows [][2]string) {
	ui.Column(c).FillWidth().Gap(13).Children(func() {
		for _, row := range rows {
			entry := ui.Column(c.Key(row[0])).FillWidth().MinWidth(0).Gap(7)
			wide := entry.Bounds().W >= 370
			if wide {
				entry.Row().Gap(16).AlignItems(ui.Start)
			}
			entry.Children(func() {
				label := ui.Text(c, row[0]).FontSize(12).TextColor(c.Theme().TextMuted)
				if wide {
					label.Width(140).Shrink(0)
				}
				value := strings.TrimSpace(row[1])
				if value == "" {
					value = "Belum tersedia"
				}
				text := ui.Text(c, value).MinWidth(0).FontSize(12).Selectable()
				if wide {
					text.Grow(1)
				} else {
					text.FillWidth()
				}
			})
		}
	})
}

func pageSection(c *ui.Context, title, description string, fn func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().MinWidth(0).Padding(20).Gap(15).Radius(11).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(15).Bold()
		if description != "" {
			muted(c, description)
		}
		fn()
	})
}
func pageNotice(c *ui.Context, icon, title, description string) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Padding(15).Gap(12).Radius(10).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, spaceIcon(icon)).Size(18, 18).Shrink(0).TextColor(t.Accent)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() { ui.Text(c, title).FontSize(12).Bold(); muted(c, description) })
	})
}
func pageEmpty(c *ui.Context, icon, title, description string) {
	ui.Column(c).FillWidth().MinWidth(0).Gap(9).Padding(12, 0).Children(func() {
		ui.Icon(c, spaceIcon(icon)).Size(24, 24).TextColor(c.Theme().TextMuted)
		ui.Text(c, title).FontSize(14).Bold()
		muted(c, description)
	})
}
func pageUnavailable(c *ui.Context, icon, title, description string) {
	pageSection(c, title, description, func() { ui.Icon(c, spaceIcon(icon)).Size(28, 28).TextColor(c.Theme().TextMuted) })
}
func pagePeriodCard(c *ui.Context, p api.Period) {
	pageSection(c, p.Label, "", func() {
		status := "Persiapan"
		if p.Status == api.PeriodActive {
			status = "Aktif"
		} else if p.Status == api.PeriodClosed {
			status = "Ditutup"
		}
		info(c, [][2]string{{"Status", status}, {"Mulai", p.StartsOn.String()}, {"Berakhir", p.EndsOn.String()}, {"Zona waktu", p.TimeZone}})
	})
}
func pageDivisionMark(c *ui.Context, d api.Division) {
	if logo := divisionLogo(d.Slug); logo != nil {
		ui.Image(c, logo).Size(44, 44).Shrink(0)
	} else {
		ui.Box(c).Size(44, 44).Radius(10).Background(c.Theme().Background).Children(func() { ui.Icon(c, spaceIcon("grid")).Size(24, 24).TextColor(c.Theme().Accent) })
	}
}
func monthName(month time.Month) string {
	names := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if month < time.January || month > time.December {
		return ""
	}
	return names[month-1]
}

func (a *App) unavailableBoard(c *ui.Context) {
	t := c.Theme()
	pageBody(c, "Board tugas", func() {
		pageNotice(c, "board", "Daftar tugas belum dapat dimuat", "Board belum tersedia. Jumlah dan status pekerjaan belum dapat dipastikan.")
		ui.ScrollHorizontal(c).FillWidth().Height(190).Shrink(0).Children(func() {
			ui.Row(c).FillHeight().Gap(14).Children(func() {
				for lane := workspaceui.Todo; lane <= workspaceui.Done; lane++ {
					ui.Column(c.Key(lane)).Width(244).Shrink(0).FillHeight().Padding(15).Gap(17).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Row(c).FillWidth().Gap(8).Children(func() { ui.Text(c, lane.String()).Grow(1).FontSize(12).Bold(); muted(c, "—") })
						muted(c, "Data belum tersedia")
					})
				}
			})
		})
	})
}

func (a *App) homeAgenda(c *ui.Context) {
	a.ensureMeetings()
	t := c.Theme()
	if !a.meetings.loaded {
		muted(c, "Memuat rapat…")
		return
	}
	now := c.Now()
	shown := 0
	for _, m := range a.meetings.items {
		if m.Cancelled || m.StartsAt.Before(now.Add(-2*time.Hour)) || shown == 4 {
			continue
		}
		shown++
		loc, _ := timeutil.LoadZone(m.TimeZone)
		if loc == nil {
			loc = time.UTC
		}
		at := m.StartsAt.In(loc)
		ui.Row(c.Key(m.ID)).FillWidth().Gap(12).AlignItems(ui.Center).MinHeight(40).Children(func() {
			ui.Text(c, fmt.Sprintf("%s %d", shortDays[at.Weekday()], at.Day())).FontSize(12).TextColor(t.TextMuted).Width(52).Shrink(0)
			ui.Text(c, m.Title).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
			ui.Text(c, at.Format("15.04")).FontSize(11).Padding(3, 8).Radius(6).Background(t.Background).Shrink(0)
		})
	}
	if shown == 0 {
		muted(c, "Belum ada rapat terjadwal.")
	}
}

func (a *App) homeNotes(c *ui.Context) {
	a.ensureNotes()
	if !a.notes.loaded {
		muted(c, "Memuat catatan…")
		return
	}
	for i, n := range a.notes.items {
		if i == 4 {
			break
		}
		if spaceButton(c.Key(n.ID), n.Title, "note", false).FillWidth().Clicked() {
			a.chrome.tab = "Catatan"
		}
	}
	if len(a.notes.items) == 0 {
		muted(c, "Belum ada catatan.")
	}
}
