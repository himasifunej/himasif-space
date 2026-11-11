//go:build !js

package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
)

// The desktop's Pengaturan › Perangkat ini, canvas A (cb14fa7a715d): the
// sections in a column with their state, rows of a label and a switch
// saved as they change, and a small preview of the island. Ace is the
// Dynamic Island; the star mascot's window is gone.

// islandSummary is the island's state in the column of sections.
func (p devicePreferences) islandSummary() string {
	if !p.IslandEnabled {
		return "Mati"
	}
	if p.IslandAlwaysOn {
		return "Selalu tampil"
	}
	return "Saat ada kabar"
}

// toastSummary says when Windows shows its own toast.
func (p devicePreferences) toastSummary() string {
	if p.IslandEnabled && p.IslandReplaceToast {
		return "Cadangan saat layar penuh"
	}
	return "Setiap kabar"
}

func (p devicePreferences) bellSummary() string {
	if !p.Sound {
		return "Mati"
	}
	return fmt.Sprintf("%d%%", int(math.Round(p.Volume*100)))
}

func onOff(on bool) string {
	if on {
		return "Aktif"
	}
	return "Mati"
}

func (d *desktopDevice) settings(c *ui.Context) {
	d.saveVolumeAtRest(c)
	nav := []appui.SettingsNavItem{
		{Title: "Dynamic Island", Icon: "spark", Status: d.prefs.islandSummary()},
		{Title: "Notifikasi Windows", Icon: "bell", Status: d.prefs.toastSummary()},
		{Title: "Mulai bersama Windows", Icon: "monitor", Status: onOff(d.prefs.Autostart)},
		{Title: "Suara bel", Icon: "play", Status: d.prefs.bellSummary()},
	}
	appui.SettingsLayout(c, "Notifikasi dan Dynamic Island", "Laptop ini · Space untuk Windows. Tidak ikut ke perangkat lain.", nav, func() { appui.SettingsSaved(c, d.savedAt) }, func() {
		if d.err != "" {
			ui.Text(c, d.err).FontSize(13).TextColor(c.Theme().Danger)
		}
		d.islandSettings(c)
		d.notificationSettings(c)
		appui.SettingSection(c, "Mulai bersama Windows", "", func() {
			on := d.prefs.Autostart
			if appui.SettingSwitch(c, "Jalankan saat masuk Windows", "Space berjalan di tray; kabar tetap masuk walau jendelanya ditutup.", &on) {
				if err := setDeviceAutostart(on); err != nil {
					d.err = err.Error()
				} else {
					d.err = ""
					d.prefs.Autostart = on
					d.save()
				}
			}
		})
		d.bellSettings(c)
		appui.SettingsNote(c, "Notifikasi mengikuti izin dan mode senyap Windows. Keluar aplikasi tersedia di ikon tray.")
	})
}

func (d *desktopDevice) islandSettings(c *ui.Context) {
	appui.SettingSection(c, "Dynamic Island", "Ace di atas tengah layar: kabar tugas, ping, dan rapat.", func() {
		appui.SettingPreview(c, 120, func() { d.islandPreview(c) })
		if appui.SettingSwitch(c, "Tampilkan Dynamic Island", "Saat mati, kabar muncul sebagai toast Windows.", &d.prefs.IslandEnabled) {
			d.save()
			if !d.prefs.IslandEnabled {
				d.island.reset(true)
			} else {
				d.island.wake()
			}
		}
		if !d.prefs.IslandEnabled {
			return
		}
		if appui.SettingSwitch(c, "Selalu tampil", "Ace menunggu sebagai pil bermata saat tidak ada kabar, dan menyingkir selama aplikasi layar penuh.", &d.prefs.IslandAlwaysOn) {
			d.save()
			if d.prefs.IslandAlwaysOn {
				d.island.wake()
			} else if d.island.win != nil {
				d.island.win.Update(func() {})
			}
		}
		if appui.SettingSwitch(c, "Gantikan toast Windows", "Kabar cukup muncul di island. Saat layar penuh, presentasi, atau Focus Assist, Windows tetap menampilkan toast.", &d.prefs.IslandReplaceToast) {
			d.save()
		}
		motion := 0
		if d.prefs.IslandReduceMotion {
			motion = 1
		}
		appui.SettingRow(c, "Kurangi gerak", "Ikut pengaturan animasi Windows, atau selalu kurangi gerak island.", func() {
			if i := appui.Segmented(c, "Kurangi gerak", []string{"Ikut Windows", "Kurangi"}, motion); i >= 0 && i != motion {
				d.prefs.IslandReduceMotion = i == 1
				d.save()
				if d.island.win != nil {
					d.island.win.Update(func() {})
				}
			}
		})
		appui.SettingRow(c, "Coba Dynamic Island", "Kirim kabar percobaan. Arahkan kursor 300 ms atau klik untuk membuka detail.", func() {
			if appui.SettingButton(c, "Coba island", "play").Clicked() {
				_ = d.app.Notifier.Present(appui.Notice{Kind: appui.NoticeTest, Title: "Dynamic Island siap", Body: "Arahkan kursor selama 300 ms atau klik untuk membuka detail.", Tag: "island-test"})
			}
		})
	})
}

// islandPreview is Ace at rest as the island looks: the black pill with
// its eyes (Ace island A has one look; the old styles A/B/C are gone), or
// what shows instead when it is off.
func (d *desktopDevice) islandPreview(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Center).Padding(16, 16).Gap(12).Children(func() {
		if !d.prefs.IslandEnabled {
			ui.Column(c).Grow(1).Justify(ui.Center).Children(func() {
				ui.Text(c, "Island mati · kabar tampil sebagai toast Windows").FontSize(13).TextColor(t.TextMuted)
			})
			return
		}
		fg := ui.RGB(245, 245, 247)
		pill := ui.Row(c.Key("pill")).Size(112, 30).Radius(15).Background(islandInk).Border(1, islandRing).Center().Gap(6).Label("Pratinjau Dynamic Island")
		if !d.prefs.IslandAlwaysOn {
			pill.Background(ui.RGBA(0, 0, 0, 0)).Border(1, t.Border)
			fg = t.TextMuted
		}
		pill.Children(func() {
			for i := 0; i < 2; i++ {
				ui.Column(c.Key(i)).Size(6, 9).Radius(3).Background(fg)
			}
		})
		caption := "Ace beristirahat di atas tengah layar"
		if !d.prefs.IslandAlwaysOn {
			caption = "Island muncul hanya saat ada kabar"
		}
		ui.Text(c, caption).FontSize(12).TextColor(t.TextMuted)
	})
}

func (d *desktopDevice) notificationSettings(c *ui.Context) {
	appui.SettingSection(c, "Notifikasi Windows", "Jalur cadangan island, dan jalur utama saat island mati.", func() {
		appui.SettingRow(c, "Pengaturan notifikasi Windows", "Izinkan Space dan periksa mode Jangan Ganggu bila notifikasi tidak terlihat.", func() {
			if appui.SettingButton(c, "Buka pengaturan Windows", "link").Clicked() {
				go func() {
					if err := openNotificationSettings(); err != nil {
						d.win.Update(func() { d.err = "Pengaturan notifikasi tidak dapat dibuka: " + err.Error() })
					}
				}()
			}
		})
		status := d.notificationTest
		if status == "" {
			status = "Mengirim satu notifikasi lewat Windows, bukan lewat island."
		}
		appui.SettingRow(c, "Coba notifikasi perangkat", status, func() {
			if appui.SettingButton(c, "Kirim percobaan", "send").Clicked() && !d.testingNotification {
				d.testingNotification = true
				d.notificationTest = "Mengirim notifikasi percobaan ke perangkat ini…"
				go func() {
					err := showTestDeviceNotification()
					d.win.Update(func() {
						d.testingNotification = false
						if err != nil {
							d.notificationTest = "Notifikasi belum diterima sistem: " + err.Error()
						} else {
							d.notificationTest = "Windows menerima. Tidak terlihat? Periksa mode Jangan Ganggu."
						}
					})
				}()
			}
		})
	})
}

func (d *desktopDevice) bellSettings(c *ui.Context) {
	appui.SettingSection(c, "Suara bel", "", func() {
		if appui.SettingSwitch(c, "Suara bel", "Bunyi saat pengingat dan ping masuk.", &d.prefs.Sound) {
			d.save()
		}
		if !d.prefs.Sound {
			return
		}
		appui.SettingRow(c, "Volume bel", "", func() {
			before := d.volume
			ui.StepSlider(c.Key("volume"), &d.volume, 0, 100, 5).Width(180).Label("Volume bel")
			if d.volume != before {
				d.volumeAt = time.Now()
				d.prefs.Volume = d.volume / 100
				c.After(500 * time.Millisecond)
			}
			ui.Text(c, fmt.Sprintf("%d%%", int(d.volume))).Font(appui.FontMono).FontSize(13).Width(40)
			if appui.SettingButton(c, "Putar", "play").Clicked() {
				d.playBell(d.prefs)
			}
		})
		name := "Bel bawaan Space"
		if d.prefs.Audio != "" {
			name = d.prefs.Audio
		}
		appui.SettingRow(c, "Bunyi bel", name+" · WAV, maksimal 5 detik", func() {
			if appui.SettingButton(c, "Ganti…", "upload").Clicked() {
				d.importBell()
			}
			if d.prefs.Audio != "" && appui.SettingButton(c, "Pakai bawaan", "").Clicked() {
				d.prefs.Audio = ""
				d.save()
			}
		})
	})
}

// saveVolumeAtRest saves the volume once the slider has rested for half a
// second, not on every step of a drag.
func (d *desktopDevice) saveVolumeAtRest(c *ui.Context) {
	if d.volumeAt.IsZero() {
		return
	}
	if rest := 500*time.Millisecond - time.Since(d.volumeAt); rest > 0 {
		c.After(rest)
		return
	}
	d.volumeAt = time.Time{}
	d.save()
}
