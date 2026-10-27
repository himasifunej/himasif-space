package appui

import (
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// page builds the page of the route shown.
func (a *App) page(c *ui.Context, r *ui.Route) {
	switch {
	case r.Match(PathJobdesk):
		a.placeholder(c, r, "Jobdesk saya",
			"Tugas organisasi yang menjadi bagianmu, dari semua ruang, dengan asal ruangnya terlihat.", "Sesi 2A")
	case r.Match(PathReminder):
		a.placeholder(c, r, "Reminder pribadi",
			"Jadwal pengingat pribadimu, beserta kontrol dengan siapa ia dibagikan.", "Sesi 2A")
	case r.Match(PathSpaces):
		a.spaces(c, r)
	case r.Match(PathMedhum):
		a.placeholder(c, r, "Medhum",
			"Kalender dan detail konten, tugas Humas–Mediatek, PJ, konsep dan caption, serta penyerahan dan buktinya.", "Tahap 4")
	case r.Match(PathEvaluasi):
		a.placeholder(c, r, "Evaluasi SOP",
			"Tinjauan pelaksanaan SOP dan laporan untuk pihak yang berwenang.", "Tahap 4")
	case r.Match(PathArsip):
		a.placeholder(c, r, "Arsip",
			"Periode-periode lama yang boleh kamu buka, hanya untuk dibaca.", "Sesi 1B")
	case r.Match(PathPengaturan):
		a.settings(c, r)
	case r.Match(PathOrganisasi) && a.isAdmin():
		a.placeholder(c, r, "Pengaturan organisasi",
			"Periode, pengurus dan kadiv, serta serah terima antarperiode.", "Sesi 1B")
	default:
		r.Title("Tidak ditemukan")
		pageBody(c, "Halaman tidak ditemukan", func() {
			ui.Text(c, "Halaman ini tidak ada atau tidak tersedia untuk akunmu.").TextColor(c.Theme().TextMuted)
			if ui.Button(c, "Ke Jobdesk saya").Clicked() {
				a.router.Replace(PathJobdesk)
			}
		})
	}
}

// pageBody builds a page: its title and, below it, what fn builds, in a
// column that scrolls.
func pageBody(c *ui.Context, title string, fn func()) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(760).Padding(28, 32).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(22).Bold()
			fn()
		})
	})
}

// builtIn builds the muted line naming the session that builds a page.
func builtIn(c *ui.Context, session string) {
	muted(c, "Dibangun di "+session+".")
}

// placeholder builds a page that a later session builds.
func (a *App) placeholder(c *ui.Context, r *ui.Route, title, what, session string) {
	r.Title(title)
	pageBody(c, title, func() {
		a.periodLine(c)
		ui.Text(c, what).TextColor(c.Theme().TextMuted)
		builtIn(c, session)
	})
}

// periodLine names the period chosen, which the pages will show.
func (a *App) periodLine(c *ui.Context) {
	if p := a.period(); p != nil {
		muted(c, "Periode "+periodLabel(*p))
	}
}

func (a *App) spaces(c *ui.Context, r *ui.Route) {
	t := c.Theme()
	r.Title("Space divisi")
	pageBody(c, "Space divisi", func() {
		a.periodLine(c)
		ui.Text(c, "Ruang tiap divisi: jobdesk bersama, notes dan template, rapat dan jadwal, anggota dan subdivisi, serta bot.").
			TextColor(t.TextMuted)
		builtIn(c, "Sesi 1C")
		switch {
		case a.divisionsLoading && len(a.divisions) == 0:
			muted(c, "Memuat divisi…")
		case a.divisionsErr != "":
			ui.Text(c, a.divisionsErr).TextColor(t.Danger)
			if ui.Button(c, "Muat ulang").Clicked() {
				a.loadDivisions()
			}
		case len(a.divisions) == 0:
			muted(c, "Belum ada divisi.")
		default:
			ui.Column(c).Radius(10).Border(1, t.Border).Background(t.Surface).Clip().
				Dividers(1, t.Border).Children(func() {
				for _, d := range a.divisions {
					ui.Row(c.Key(d.ID)).Padding(10, 14).Gap(12).Children(func() {
						ui.Text(c, d.Name).Grow(1)
						muted(c, d.Slug)
					})
				}
			})
		}
		muted(c, "Keanggotaan dan role menyusul di Sesi 1C.")
	})
}

func (a *App) settings(c *ui.Context, r *ui.Route) {
	t := c.Theme()
	r.Title("Pengaturan")
	pageBody(c, "Pengaturan", func() {
		ui.Text(c, "Akun").Bold().Margin(8, 0, 0, 0)
		kind := "Pribadi"
		if a.user.Kind == api.AccountOrganizationAdmin {
			kind = "Admin organisasi"
		}
		verified := "Belum diverifikasi"
		if a.user.EmailVerified {
			verified = "Terverifikasi"
		}
		info(c, [][2]string{
			{"Nama", a.user.DisplayName},
			{"Email", a.user.Email},
			{"Jenis akun", kind},
			{"Verifikasi email", verified},
		})
		ui.Text(c, "Aplikasi").Bold().Margin(8, 0, 0, 0)
		info(c, [][2]string{
			{"Server", a.ServerURL},
			{"Versi", a.Version},
		})
		muted(c, "Profil, suara, Ace, autostart dan sinkronisasi menyusul di Tahap 3.")
		ui.Row(c).Margin(8, 0, 0, 0).Children(func() {
			label := "Keluar"
			if a.loggingOut {
				label = "Keluar…"
			}
			if ui.Button(c, label).Disabled(a.loggingOut).TextColor(t.Danger).Clicked() {
				a.logout()
			}
		})
	})
}

// info builds rows of names and values.
func info(c *ui.Context, rows [][2]string) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		for _, row := range rows {
			ui.Row(c.Key(row[0])).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Text(c, row[0]).Width(140).Shrink(0).TextColor(t.TextMuted)
				ui.Text(c, row[1]).Grow(1).MinWidth(0).Selectable()
			})
		}
	})
}
