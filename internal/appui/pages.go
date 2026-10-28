package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// page renders only data the current service can confirm. Unavailable task,
// membership and bot services never turn into local sample data or saves.
func (a *App) page(c *ui.Context, r *ui.Route) {
	switch {
	case r.Match(PathJobdesk):
		r.Title("Jobdesk saya")
		a.personalTasksView(c)
	case r.Match(PathReminder):
		r.Title("Reminder pribadi")
		a.remindersView(c)
	case r.Match(PathSpaces):
		a.spaces(c, r)
	case r.Match(PathMedhum):
		a.spaces(c, r)
	case r.Match(PathEvaluasi):
		r.Title("Evaluasi SOP")
		pageBody(c, "Evaluasi SOP", func() {
			a.periodLine(c)
			muted(c, "Tinjau kejadian, cocokkan bukti, dan pahami apa yang perlu diperbaiki bersama.")
			pageNotice(c, "shield", "Temuan perlu ditinjau", "Waktu penyerahan dan kecocokan bukti menjadi dasar evaluasi. Dugaan belum menjadi pelanggaran yang disahkan.")
			pageUnavailable(c, "archive", "Laporan belum tersedia", "Data penyerahan dan evaluasi belum dapat dimuat. Tidak ada kesimpulan SOP yang ditampilkan.")
		})
	case r.Match(PathArchiveAccess):
		r.Title("Izin arsip")
		a.archiveManagementView(c)
	case r.Match(PathArsip):
		a.archivePage(c, r)
	case r.Match(PathPengaturan):
		a.settings(c, r)
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
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		ui.ScrollHorizontal(c.Key("space-tabs")).FillWidth().Height(58).Shrink(0).Background(c.Theme().Background).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
			ui.Row(c).FillHeight().Padding(8, 22).Gap(5).AlignItems(ui.Center).Children(func() {
				for _, tab := range []struct{ title, icon string }{{"Beranda", "home"}, {"Board tugas", "board"}, {"Agenda", "calendar"}, {"Notes", "note"}, {"Anggota", "users"}, {"Pengaturan", "settings"}} {
					if spaceButton(c.Key(tab.title), tab.title, tab.icon, a.chrome.tab == tab.title).Clicked() {
						a.chrome.tab = tab.title
					}
				}
			})
		})
		if s := a.selectedSpace(); s != nil && s.ReadOnly {
			pageNotice(c, "lock", "Hanya baca", "Periode atau akses arsip ini tidak mengizinkan perubahan.")
		}

		switch a.chrome.tab {
		case "Board tugas":
			a.spaceTasksView(c)
		case "Agenda":
			a.taskAgenda(c)
		case "Notes":
			a.notesPage(c)
		case "Anggota":
			a.membersPage(c)
		case "Pengaturan":
			a.spaceSettings(c, shared)
		default:
			a.overviewPage(c, title, shared)
		}
	})
}

func (a *App) overviewPage(c *ui.Context, title string, shared bool) {
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(26).Gap(24)
		wide := body.Bounds().W >= 720
		body.Children(func() {
			heading := ui.Column(c).FillWidth().MinWidth(0).Gap(20)
			if wide {
				heading.Row().AlignItems(ui.Start)
			}
			heading.Children(func() {
				titles := ui.Column(c).MinWidth(0).Gap(8)
				if wide {
					titles.Grow(1)
				} else {
					titles.FillWidth()
				}
				titles.Children(func() {
					if p := a.period(); p != nil {
						ui.Text(c, "HIMASIF · "+p.Label).FontSize(10).TextColor(t.TextMuted)
					}
					ui.Text(c, "Ruang "+title+" kita.").FontSize(29).Bold()
					muted(c, "Biar setiap ide punya tempat, dan setiap tugas tetap dalam alurnya.")
				})
				ui.Column(c).Gap(6).Children(func() {
					if spaceButton(c, "Buka board", "board", true).Clicked() {
						a.chrome.tab = "Board tugas"
					}
					if spaceButton(c, "Lihat agenda", "calendar", false).Clicked() {
						a.chrome.tab = "Agenda"
					}
				})
			})
			a.tasksOverview(c)
			if shared {
				pageNotice(c, "link", "Ruang kolaborasi", "Akses ruang bersama tidak membuka pekerjaan internal divisi lain.")
			}

			ui.Column(c).FillWidth().Gap(16).Children(func() {

				sections := ui.Column(c).FillWidth().MinWidth(0).Gap(16)
				if wide {
					sections.Row().AlignItems(ui.Stretch)
				}
				sections.Children(func() {
					first := ui.Column(c).MinWidth(0)
					if wide {
						first.Grow(1)
					} else {
						first.FillWidth()
					}
					first.Children(func() {
						pageSection(c, "Agenda terdekat", "Tenggat, rapat, dan jadwal posting.", func() {
							pageEmpty(c, "calendar", "Agenda belum tersedia", "Jadwal bersama belum dapat dimuat.")
							if spaceButton(c, "Lihat agenda", "arrow", false).Clicked() {
								a.chrome.tab = "Agenda"
							}
						})
					})
					second := ui.Column(c).MinWidth(0)
					if wide {
						second.Grow(1)
					} else {
						second.FillWidth()
					}
					second.Children(func() {
						pageSection(c, "Catatan bersama", "Simpan hasil rapat dan arah pekerjaan.", func() {
							pageEmpty(c, "note", "Catatan belum tersedia", "Notes space belum dapat dimuat.")
							if spaceButton(c, "Buka notes", "arrow", false).Clicked() {
								a.chrome.tab = "Notes"
							}
						})
					})
				})
			})
		})
	})
}

func (a *App) agendaPage(c *ui.Context) {
	now := c.Now()
	if p := a.period(); p != nil {
		if zone, err := time.LoadLocation(p.TimeZone); err == nil {
			now = now.In(zone)
		}
	}
	start := now.AddDate(0, 0, -(int(now.Weekday())+6)%7)
	t := c.Theme()
	pageBody(c, "Selaraskan minggu kita.", func() {
		muted(c, "Ruang untuk deadline, rapat, dan semua yang perlu diingat.")
		pageNotice(c, "calendar", "Agenda belum tersedia", "Jadwal bersama belum dapat dimuat. Kalender di bawah hanya menunjukkan tanggal, tanpa kegiatan atau pengingat yang dijadwalkan.")
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, fmt.Sprintf("%d %s – %d %s %d", start.Day(), monthName(start.Month()), start.AddDate(0, 0, 6).Day(), monthName(start.AddDate(0, 0, 6).Month()), start.AddDate(0, 0, 6).Year())).FillWidth().FontSize(16).Bold()
			spaceButton(c, "Tambah agenda", "plus", false).Disabled(true)
		})
		ui.ScrollHorizontal(c).FillWidth().Height(380).Shrink(0).Children(func() {
			ui.Row(c).FillHeight().Gap(1).Padding(1).Background(t.Border).Radius(10).Clip().Children(func() {
				for offset := 0; offset < 7; offset++ {
					day := start.AddDate(0, 0, offset)
					ui.Column(c.Key(offset)).Width(126).FillHeight().Shrink(0).Background(t.Surface).Padding(14).Gap(10).Children(func() {
						ui.Text(c, []string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}[offset]).FontSize(11).TextColor(t.TextMuted)
						color := t.Text
						if day.Day() == now.Day() && day.Month() == now.Month() {
							color = t.Accent
						}
						ui.Text(c, fmt.Sprint(day.Day())).FontSize(25).Bold().TextColor(color)
						ui.Box(c).FillWidth().Height(1).Background(t.Border)
					})
				}
			})
		})
		muted(c, "Deadline desain dan jadwal posting ditampilkan terpisah saat data tersedia.")
	})
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

func (a *App) settings(c *ui.Context, r *ui.Route) {
	r.Title("Pengaturan")
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		ui.ScrollHorizontal(c).FillWidth().Height(58).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
			ui.Row(c).FillHeight().Padding(8, 22).Gap(5).Children(func() {
				for _, tab := range []struct{ title, icon string }{{"WhatsApp", "whatsapp"}, {"Pengingat", "bell"}, {"Command", "terminal"}, {"Aktivitas", "clock"}, {"Akun", "user"}} {
					if spaceButton(c.Key(tab.title), tab.title, tab.icon, a.chrome.settingsTab == tab.title).Clicked() {
						a.chrome.settingsTab = tab.title
					}
				}
			})
		})
		switch a.chrome.settingsTab {
		case "Akun":
			a.accountPage(c)
		case "Pengingat":
			a.reminderSettings(c)
		case "Command":
			a.commandSettings(c)
		case "Aktivitas":
			pageBody(c, "Aktivitas WhatsApp", func() {
				pageUnavailable(c, "clock", "Riwayat pengiriman belum tersedia", "Status antre, kirim, dan gagal belum dapat diperiksa. Tidak ada pengiriman yang diklaim berhasil.")
			})
		default:
			a.whatsappSettings(c)
		}
	})
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

func (a *App) whatsappSettings(c *ui.Context) {
	pageBody(c, "WhatsApp divisi", func() {
		muted(c, "Pengingat yang sampai ke tempat kita biasa berkoordinasi.")
		pageNotice(c, "info", "Layanan WhatsApp belum tersedia", "Status koneksi akun belum dapat diperiksa. Tidak ada nomor, grup, atau sesi WhatsApp yang dianggap terhubung.")
		pageSection(c, "Akun WhatsApp", "Gunakan nomor khusus divisi atau akun kadiv sesuai kebutuhan divisi.", func() {
			ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("whatsapp")).Size(36, 36).TextColor(c.Theme().Accent)
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					ui.Text(c, "Koneksi belum dapat diperiksa").Bold()
					muted(c, "QR akan tersedia setelah layanan koneksi dapat digunakan.")
				})
			})
			spaceButton(c, "Tampilkan QR", "qr", false).Disabled(true)
		})
		pageSection(c, "Identitas dalam pesan", "Nama bot dan gaya pesan dapat disesuaikan oleh pengelola yang memiliki izin.", func() {
			info(c, [][2]string{{"Label bot", "Belum dapat dimuat"}, {"Jenis akun", "Belum dapat dimuat"}, {"Gaya pesan", "Belum dapat dimuat"}})
		})
		pageSection(c, "Grup tujuan", "Satu koneksi divisi untuk grup yang ditetapkan.", func() {
			pageEmpty(c, "users", "Daftar grup belum tersedia", "Tidak ada grup tujuan yang dapat dikonfirmasi.")
			spaceButton(c, "Sinkronkan grup", "refresh", false).Disabled(true)
		})
		pageNotice(c, "link", "Alur Medhum", "Grup Medhum menggunakan bot Humas agar pengingat bersama tidak berganda. Pengaturan koneksi tetap mengikuti kewenangan divisi Humas.")
	})
}

func (a *App) reminderSettings(c *ui.Context) {
	pageBody(c, "Pengingat divisi", func() {
		pageUnavailable(c, "bell", "Jadwal belum dapat dimuat", "Pengaturan pengingat divisi belum tersedia. Tidak ada perubahan jadwal yang disimpan.")
		pageSection(c, "Jadwal standar Space", "Pilihan awal untuk pengingat tugas; konfigurasi divisi belum dapat dikonfirmasi.", func() {
			info(c, [][2]string{{"Menjelang tenggat", "H-2 dan H-1"}, {"Saat tenggat", "Hari deadline"}})
		})
		pageSection(c, "Pengingat perangkat", "Notifikasi perangkat dan WhatsApp memiliki status pengiriman masing-masing.", func() { muted(c, "Suara, waktu pengiriman, dan pilihan kanal belum dapat diatur di halaman ini.") })
	})
}

func (a *App) commandSettings(c *ui.Context) {
	pageBody(c, "Command bot", func() {
		muted(c, "Perintah kecil untuk kerja divisi dan selingan di grup.")
		pageUnavailable(c, "terminal", "Command belum tersedia", "Daftar command dan stiker divisi belum dapat dimuat atau disimpan. Tidak ada command yang diaktifkan dari halaman ini.")
		spaceButton(c, "Tambah command", "plus", false).Disabled(true)
	})
}

func (a *App) accountPage(c *ui.Context) {
	pageBody(c, "Akun dan aplikasi", func() {
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
