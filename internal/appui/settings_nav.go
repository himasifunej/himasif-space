package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

type settingsEntry struct{ key, label, detail, icon string }
type settingsGroup struct {
	title   string
	entries []settingsEntry
}

// settingsGroups is the navigation of canvas A: the account, this device,
// then the selected division. Keys stay the tab names other pages link to.
func (a *App) settingsGroups() []settingsGroup {
	division := "Divisi"
	sp := a.selectedSpace()
	if sp == nil {
		for i := range a.access.spaces {
			if a.access.spaces[i].Kind != api.SpaceShared {
				sp = &a.access.spaces[i]
				break
			}
		}
	}
	if sp != nil {
		division = "Divisi " + sp.Name
		if strings.HasPrefix(strings.ToLower(sp.Name), "divisi") {
			division = sp.Name
		}
	}
	return []settingsGroup{
		{"Akun", []settingsEntry{
			{"Akun", "Profil dan keamanan", "Nama, email, kata sandi, keluar", "user"},
		}},
		{"Perangkat ini", []settingsEntry{
			{"Perangkat", "Notifikasi dan Dynamic Island", "Island, toast, bel, aplikasi desktop", "bell"},
			{"Tampilan", "Tampilan dan tema", "Tema, palet, aksen, warna board", "palette"},
			{"Sinkronisasi", "Offline dan sinkronisasi", "Antrean perubahan di perangkat ini", "refresh"},
		}},
		{division, []settingsEntry{
			{"Space", "Identitas space", "Jenis, akses, dan logo divisi", "grid"},
			{"WhatsApp", "Tautan dan bot WhatsApp", "Kode !link, perangkat bot, grup", "whatsapp"},
			{"Pengingat", "Pengingat dan ping", "Jadwal pengingat bawaan, kirim ping", "clock"},
			{"Command", "Command bot", "Pemicu dan template balasan", "terminal"},
			{"Aktivitas", "Aktivitas WhatsApp", "Antrean dan status pengiriman", "message"},
			{"Status layanan", "Status layanan", "API, bot, penjadwal, sumber", "shield"},
			{"Darurat", "Spreadsheet darurat", "Salinan saat server tidak tersedia", "archive"},
		}},
		{"Tentang", []settingsEntry{
			{"Tentang", "Tentang Space", "Versi dan riwayat perubahan", "book"},
		}},
	}
}

func (a *App) settings(c *ui.Context, r *ui.Route) {
	r.Title("Pengaturan")
	t := c.Theme()
	groups := a.settingsGroups()
	content := func() {
		switch a.chrome.settingsTab {
		case "Sinkronisasi":
			a.offlineView(c)
		case "Darurat":
			a.emergencyView(c)
		case "Tampilan":
			a.appearanceSettings(c)
		case "Status layanan":
			a.operationsView(c)
		case "Perangkat":
			if a.DeviceSettings != nil {
				a.DeviceSettings(c)
			} else {
				settingsBody(c, "Notifikasi dan Dynamic Island", "Pengaturan ini hanya untuk perangkat ini.", func() {
					pageNotice(c, "monitor", "Tersedia di aplikasi desktop", "Dynamic Island, toast, dan bel diatur pada aplikasi Space untuk Windows. Di browser, notifikasi mengikuti izin browser.")
				})
			}
		case "Akun":
			a.accountPage(c)
		case "Pengingat":
			a.notificationSettings(c)
		case "Command":
			a.commandSettings(c)
		case "Aktivitas":
			a.whatsappActivity(c)
		case "Tentang":
			a.changelogSettings(c)
		case "Kredit":
			a.kreditSettings(c)
		case "Space":
			sp := a.selectedSpace()
			a.spaceSettings(c, sp != nil && sp.Kind == api.SpaceShared)
		default:
			a.whatsappSettings(c)
		}
	}
	w, _ := c.Size()
	if w >= 900 {
		ui.Row(c).Fill().MinWidth(0).Children(func() {
			ui.Scroll(c.Key("settings-navigation")).Width(248).Shrink(0).FillHeight().Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Label("Tab pengaturan").Children(func() {
				ui.Column(c).FillWidth().Padding(20, 12).Gap(2).Children(func() {
					ui.Text(c, "Pengaturan").FontSize(18).Bold().Padding(0, 8)
					muted(c, "Pengaturan perangkat tidak otomatis mengubah perangkat lain.").FontSize(11).Padding(2, 8, 10, 8)
					for _, g := range groups {
						muted(c.Key("g"+g.title), strings.ToUpper(g.title)).FontSize(10).Padding(12, 8, 6, 8).MaxLines(1)
						for _, e := range g.entries {
							// Kredit opens from Tentang, which stays lit.
							on := a.chrome.settingsTab == e.key || a.chrome.settingsTab == "Kredit" && e.key == "Tentang"
							if spaceButton(c.Key(e.key), e.label, e.icon, on).FillWidth().Clicked() {
								a.chrome.settingsTab = e.key
							}
						}
					}
				})
			})
			ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).FillHeight().Children(content)
		})
		return
	}
	if a.chrome.settingsDetail {
		ui.Column(c).Fill().MinWidth(0).Children(func() {
			ui.Row(c.Key("settings-back")).FillWidth().Shrink(0).Padding(6, 10).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				if spaceButton(c, "Semua pengaturan", "left", false).Clicked() {
					a.chrome.settingsDetail = false
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).FillWidth().Children(content)
		})
		return
	}
	settingsBody(c, "Pengaturan", "Akun, perangkat ini, dan divisi. Pengaturan perangkat tidak otomatis mengubah perangkat lain.", func() {
		for _, g := range groups {
			ui.Column(c.Key("g" + g.title)).FillWidth().Gap(8).Children(func() {
				muted(c, strings.ToUpper(g.title)).FontSize(10).Padding(0, 4)
				ui.Column(c).FillWidth().Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
					for i, e := range g.entries {
						b := ui.ButtonBase(c.Key(e.key)).Label(e.label).FillWidth().Row().Gap(12).Padding(12, 14).AlignItems(ui.Center)
						if i > 0 {
							b.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
						}
						if b.Hovered() {
							b.Background(t.SurfaceHover)
						}
						b.FocusRing(b.FocusVisible()).Children(func() {
							ui.Icon(c, spaceIcon(e.icon)).Size(18, 18).TextColor(t.TextMuted).Shrink(0)
							ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
								ui.Text(c, e.label).MaxLines(1)
								muted(c, e.detail).FontSize(11).MaxLines(1)
							})
							ui.Icon(c, spaceIcon("chevron")).Size(16, 16).TextColor(t.TextMuted).Shrink(0)
						})
						if b.Clicked() {
							a.chrome.settingsTab = e.key
							a.chrome.settingsDetail = true
						}
					}
				})
			})
		}
	})
}
