package appui

import (
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// The redesigned settings pages (canvas c66c26d3f3e1, the owner's picks):
// the rows of Perangkat ini (settings_rows.go) without its search and
// column of sections: a 22 px title and its line, then sections, each a
// box of rows with the label at the left and the value or control at the
// right.

// settingsPageView draws a settings page titled title. wide pages (a
// preview, an editor beside a list) take 1088 px instead of 820; status
// draws at the right of the title.
func settingsPageView(c *ui.Context, title, description string, wide bool, status func(), body func()) {
	w, _ := c.Size()
	phone := w < 600
	ui.Scroll(c.Key("settings-page")).Fill().Label("Isi pengaturan").Children(func() {
		page := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(820).Padding(28, 40, 64, 40).Gap(28)
		if wide {
			page.MaxWidth(1088)
		}
		if phone {
			page.Padding(20, 16, 48, 16).Gap(24)
		}
		page.Children(func() {
			var head ui.Element
			if phone {
				head = ui.Column(c.Key("head")).FillWidth().MinWidth(0).Gap(8)
			} else {
				head = ui.Row(c.Key("head")).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.End)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, title).FontSize(22).Bold()
					if description != "" {
						muted(c, description).FontSize(13)
					}
				})
				if status != nil {
					ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(status)
				}
			})
			// The rows of Perangkat ini, with no search of their own: a
			// query left on that page must not hide these.
			query, narrow := settingsPage.query, settingsPage.narrow
			settingsPage.query, settingsPage.narrow = "", w < 1100
			if settingsPage.words == nil {
				settingsPage.words = map[string][]string{}
			}
			body()
			settingsPage.query, settingsPage.narrow = query, narrow
		})
	})
}

// settingsValue is a row's value at its right.
func settingsValue(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(13).MaxLines(2).Selectable()
}

// settingsRowBody is what opens under a row, inside its box: a form, a
// list.
func settingsRowBody(c *ui.Context, key string, fn func()) {
	ui.Column(c.Key("body-"+key)).FillWidth().MinWidth(0).Padding(0, 16, 16, 16).Gap(12).Children(fn)
}

// accountPage is Akun, option A: Akunmu, Keamanan akun (the password form
// opens in its row), Aplikasi ini; the admin's handover in its own
// section.
func (a *App) accountPage(c *ui.Context) {
	t := c.Theme()
	settingsPageView(c, "Akun dan aplikasi", "Identitas dan keamanan akun yang sedang masuk, serta koneksi aplikasi ini.", false, nil, func() {
		SettingSection(c, "Akunmu", "Identitas akun yang sedang masuk.", func() {
			SettingRow(c, "Nama", "", func() { settingsValue(c, a.user.DisplayName) })
			SettingRow(c, "Email", "", func() {
				settingsValue(c, a.user.Email)
				if a.user.EmailVerified {
					statusPill(c, "Terverifikasi", t.Success)
				} else {
					statusPill(c, "Belum diverifikasi", t.Warning)
				}
			})
			kind, kindLine := "Pribadi", "Akun pribadi masuk ke Space divisinya."
			if a.user.Kind == api.AccountOrganizationAdmin {
				kind, kindLine = "Admin organisasi", "Akun bersama organisasi: mengelola infrastruktur, bukan tugas divisi."
			}
			SettingRow(c, "Jenis akun", kindLine, func() { settingsValue(c, kind) })
		})
		a.accountCredentialsView(c)
		SettingSection(c, "Aplikasi ini", "Koneksi Space pada perangkat ini.", func() {
			SettingRow(c, "Server", "", func() { settingsValue(c, a.ServerURL).Font(FontMono) })
			SettingRow(c, "Versi", "Pembaruan aplikasi diatur di Perangkat ini.", func() {
				settingsValue(c, a.Version).Font(FontMono)
				if a.DeviceSettings != nil && SettingButton(c, "Pembaruan aplikasi", "").Clicked() {
					a.chrome.settingsTab = "Perangkat"
				}
			})
			SettingRow(c, "Penampilan", "Tema, palet, aksen, dan warna board.", func() {
				if SettingButton(c, "Buka Tampilan", "palette").Clicked() {
					a.chrome.settingsTab = "Tampilan"
				}
			})
			label := "Keluar"
			if a.loggingOut {
				label = "Keluar…"
			}
			SettingRow(c, "Keluar dari akun", "Perangkat ini lupa sesimu. Perubahan offline yang belum terkirim tetap tersimpan di perangkat.", func() {
				if SettingButton(c, label, "logout").Disabled(a.loggingOut).TextColor(t.Danger).Clicked() {
					a.logout()
				}
			})
		})
	})
}

// statusPill is a small bordered state beside a value.
func statusPill(c *ui.Context, text string, color ui.Color) {
	ui.Row(c.Key("pill-"+text)).Shrink(0).Height(22).Padding(0, 8).Radius(6).Border(1, color).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(12).TextColor(color).MaxLines(1)
	})
}

// spaceKindText and spaceAccessText say a Space's kind and the caller's
// access in words.
func spaceKindText(kind string) (string, string) {
	if kind == api.SpaceShared {
		return "Kolaborasi lintas divisi", "Space bersama beberapa divisi."
	}
	return "Ruang divisi", "Tugas, rapat, dan catatan satu divisi."
}

func spaceAccessText(access string) (string, string) {
	switch access {
	case api.AccessArchive:
		return "Arsip", "Hanya membaca; periode atau aksesnya sudah selesai."
	case api.AccessAdmin:
		return "Admin", "Pengelola organisasi."
	}
	return "Anggota", "Kamu anggota Space ini sesuai peranmu di divisi."
}

// spaceSettings is Identitas space, option A: the Space in words, its
// division's logo, and where the rest is set.
func (a *App) spaceSettings(c *ui.Context, shared bool) {
	s := a.selectedSpace()
	desc := ""
	if s != nil {
		desc = s.Name
		if p := a.period(); p != nil {
			desc += ", periode " + periodLabel(*p)
		}
	}
	settingsPageView(c, "Identitas space", desc, false, nil, func() {
		if shared {
			pageNotice(c, "whatsapp", "Medhum dilayani bot Humas", "Bot Humas ditujukan untuk grup Humas dan Medhum. Bot Mediatek berada di grup divisinya sendiri.")
		}
		if s == nil {
			pageEmpty(c, "grid", "Pilih Space", "Pilih Space di sidebar untuk melihat identitasnya.")
			return
		}
		SettingSection(c, "Space", "", func() {
			SettingRow(c, "Nama", "", func() { settingsValue(c, s.Name) })
			kind, kindLine := spaceKindText(s.Kind)
			SettingRow(c, "Jenis", kindLine, func() { settingsValue(c, kind) })
			access, accessLine := spaceAccessText(s.Access)
			if s.ReadOnly {
				access += " · hanya baca"
			}
			SettingRow(c, "Akses", accessLine, func() { settingsValue(c, access) })
			if d := a.selectedDivision(); d != nil && !shared {
				SettingRow(c, "Logo", "Mengikuti identitas divisi. Logo dan nama kustom belum tersedia.", func() { pageDivisionMark(c, *d) })
				SettingRow(c, "Slug", "", func() { settingsValue(c, d.Slug).Font(FontMono) })
			}
		})
		SettingSection(c, "Diatur di tempat lain", "", func() {
			SettingRow(c, "Anggota dan akses", "Siapa saja di Space ini dan perannya.", func() {
				if SettingButton(c, "Kelola anggota dan akses", "users").Clicked() {
					a.chrome.tab = "Anggota"
					a.navigate(PathSpaces)
				}
			})
			SettingRow(c, "Bot WhatsApp", "Grup divisi, pengingat, dan perintah bot.", func() {
				if SettingButton(c, "Lihat pengaturan WhatsApp", "whatsapp").Clicked() {
					a.chrome.settingsTab = "WhatsApp"
					a.router.Push(PathPengaturan)
				}
			})
		})
	})
}
