package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/emergency/sheet"
	"github.com/himasifunej/himasif-space/internal/id"
)

// emergencyView is Pusat darurat (canvas dcb4562af51c, option B): Buka
// sheet beside the title, the three phases as tabs, and the sheet's link
// one tap from being shared.
func (a *App) emergencyView(c *ui.Context) {
	st := a.sheetEnsure()
	if e := &a.emergency; e.user != a.user.ID || e.period != a.periodID {
		*e = emergencyUIState{seq: e.seq + 1, user: a.user.ID, period: a.periodID, space: a.selectedSpaceID()}
	}
	phase := a.sheetPhase()
	settingsPageView(c, "Pusat darurat", "Google Sheet tempat tim mencatat progres tugas saat server Space mati. Space membacanya; perubahan masuk server lewat rekonsiliasi.", true, func() {
		if st.cfg.Edit != "" {
			if primarySpaceButton(c.Key("open-sheet"), "Buka sheet", "external").Clicked() {
				c.OpenURL(st.cfg.Edit)
			}
		}
		if i := Segmented(c, "Fase darurat", []string{"Persiapan", "Saat gangguan", "Setelah pulih"}, phase); i >= 0 {
			st.phase = i
		}
	}, func() {
		if a.period() == nil {
			muted(c, "Pilih periode terlebih dahulu.")
			return
		}
		if a.isAdmin() {
			muted(c, "Gunakan akun pribadi dengan akses tugas untuk sheet darurat.")
			return
		}
		if st.err != "" {
			ui.Text(c, st.err).FontSize(13).TextColor(c.Theme().Danger)
		}
		if st.notice != "" {
			muted(c, st.notice).FontSize(13)
		}
		if a.Web != nil {
			// The browser may not read Google's published sheet (CORS),
			// and is down with the server anyway.
			pageNotice(c, "monitor", "Sheet darurat ada di aplikasi desktop", "Aplikasi Space untuk Windows menyimpan link sheet, membacanya saat server mati, dan tetap bekerja dari tray.")
			phase = -1
		}
		switch phase {
		case -1:
		case 0:
			a.sheetPrepareView(c)
		case 1:
			a.sheetDuringView(c)
		default:
			a.sheetAfterView(c)
		}
		ui.Column(c.Key("legacy")).FillWidth().Gap(12).Children(func() {
			label := "Tampilkan"
			if st.legacy {
				label = "Sembunyikan"
			}
			SettingSection(c, "Cara lama: salinan CSV", "Snapshot server sebagai CSV dan rekonsiliasi dari CSV, untuk sheet yang belum disiapkan.", func() {
				SettingRow(c, "Salinan CSV", "Butuh server hidup untuk membuat snapshot.", func() {
					if SettingButton(c, label, "").Clicked() {
						st.legacy = !st.legacy
					}
				})
			})
			if st.legacy {
				a.emergencyLegacy(c)
			}
		})
	})
}

// sheetPrepareView is Persiapan: the steps in Google, the links, the
// share card.
func (a *App) sheetPrepareView(c *ui.Context) {
	st := &a.sheet
	if !st.linked() || st.editing {
		SettingSection(c, "Siapkan sheet", "Sekali, selagi server hidup. Butuh akun Google pemilik sheet; tim tidak perlu login.", func() {
			SettingRow(c, "1 · Unduh template", "Tab Meta (jam sheet) dan Tugas; kolom ID disembunyikan.", func() {
				if a.ExportFile != nil && SettingButton(c, "Simpan template (.xlsx)", "archive").Disabled(st.busy).Clicked() {
					a.sheetTemplate()
				}
			})
			SettingRow(c, "2 · Buka di Google Sheets", "Drive → Baru → Upload file. Klik kanan → Buka dengan → Google Spreadsheet, lalu File → Simpan sebagai Google Spreadsheet.", func() {})
			SettingRow(c, "3 · Atur sheet", "File → Setelan: zona waktu (GMT+07:00) Jakarta; Penghitungan: Saat ada perubahan dan setiap menit.", func() {})
			SettingRow(c, "4 · Bagikan ke tim", "Bagikan → Siapa saja yang memiliki link → Editor. Data → Lindungi sheet dan rentang: kolom I–K (ID) hanya pemilik.", func() {})
			SettingRow(c, "5 · Publikasikan ke web", "File → Bagikan → Publikasikan ke web → Seluruh dokumen → Microsoft Excel (.xlsx); publikasi ulang otomatis dicentang.", func() {})
		})
		SettingSection(c, "Link sheet", "Disimpan di perangkat ini saja. Space menguji link publish sekali sebelum menyimpan.", func() {
			settingsRowBody(c, "links", func() {
				ui.Box(c).Height(12)
				muted(c, "Link publish (dibaca Space)").FontSize(12)
				ui.TextInput(c.Key("publish"), &st.publishIn).Label("Link publish (dibaca Space)").Placeholder("https://docs.google.com/spreadsheets/d/e/…/pub?output=xlsx").FillWidth()
				muted(c, "Link edit (untuk tim)").FontSize(12)
				ui.TextInput(c.Key("edit"), &st.editIn).Label("Link edit (untuk tim)").Placeholder("https://docs.google.com/spreadsheets/d/…/edit").FillWidth()
				ui.Row(c).Gap(8).Children(func() {
					label := "Uji dan simpan"
					if st.busy {
						label = "Menguji…"
					}
					if primarySpaceButton(c.Key("save-links"), label, "check").Disabled(st.busy || strings.TrimSpace(st.publishIn) == "" || strings.TrimSpace(st.editIn) == "").Clicked() {
						a.sheetSaveLinks()
					}
					if st.linked() && SettingButton(c, "Batal", "").Clicked() {
						st.editing = false
						st.publishIn, st.editIn = st.cfg.Publish, st.cfg.Edit
					}
				})
			})
		})
		return
	}
	ui.Row(c.Key("prepare")).FillWidth().Gap(16).AlignItems(ui.Start).Wrap().Children(func() {
		ui.Column(c.Key("ready")).Grow(1).MinWidth(320).Gap(16).Children(func() {
			SettingSection(c, "Kesiapan", "Link disimpan di perangkat ini. Bagikan ke pengelola lain selagi server hidup.", func() {
				SettingRow(c, "Link publish (dibaca Space)", shortLink(st.cfg.Publish), func() {
					if !st.cfg.Tested.IsZero() {
						statusPill(c, "Teruji "+st.cfg.Tested.Format("2 Jan 15.04"), c.Theme().Success)
					}
				})
				SettingRow(c, "Link edit (untuk tim)", shortLink(st.cfg.Edit), func() {})
				SettingRow(c, "Uji terakhir", st.cfg.TestNote, func() {
					if SettingButton(c, "Uji lagi", "refresh").Disabled(st.busy).Clicked() {
						a.sheetSaveLinks()
					}
				})
				SettingRow(c, "Ganti link", "Sheet baru, atau link yang dicabut.", func() {
					if SettingButton(c, "Ganti", "").Clicked() {
						st.editing = true
					}
					if SettingButton(c, "Hapus dari perangkat", "close").Clicked() {
						a.sheetForgetLinks()
					}
				})
			})
			SettingSection(c, "Yang masuk ke sheet", "Saat gangguan, pengelola mengisi sheet dari cache perangkat.", func() {
				SettingRow(c, "Tugas aktif", "Judul, PJ, Space, tenggat, progres; ID tersembunyi untuk mencocokkan baris.", func() {})
				SettingRow(c, "Tidak pernah", "Kontak, catatan, lampiran, reminder pribadi. Link sheet bersifat publik bagi yang memegangnya.", func() {})
			})
		})
		a.sheetShareCard(c)
	})
}

func shortLink(u string) string {
	if len(u) <= 64 {
		return u
	}
	return u[:40] + "…" + u[len(u)-18:]
}

// sheetShareCard is Bagikan sheet ke tim: one big Salin link, WhatsApp, a
// QR to scan from a phone.
func (a *App) sheetShareCard(c *ui.Context) {
	st := &a.sheet
	t := c.Theme()
	if st.cfg.Edit == "" {
		return
	}
	ui.Column(c.Key("share")).Width(380).MinWidth(300).Shrink(1).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Column(c).FillWidth().Padding(14, 16).Gap(2).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Bagikan sheet ke tim").FontSize(15).Bold()
			muted(c, "Siapa saja dengan link bisa mengedit").FontSize(13)
		})
		ui.Row(c).FillWidth().Padding(16).Gap(16).AlignItems(ui.Start).Children(func() {
			if bm := a.sheetQR(); bm != nil {
				ui.Box(c.Key("qr")).Size(120, 120).Radius(8).Background(ui.Hex("#ffffff")).Padding(4).Shrink(0).Label("QR link edit sheet").Children(func() {
					ui.Image(c, bm).Size(112, 112).Fit(ui.Contain)
				})
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
				label := "Salin link"
				if time.Since(st.copied) < 4*time.Second {
					label = "Tersalin, tempel di grup"
					c.After(time.Second)
				}
				if primarySpaceButton(c.Key("copy-link"), label, "link").FillWidth().Height(44).Clicked() {
					c.WriteClipboard(st.cfg.Edit)
					st.copied = time.Now()
				}
				if spaceButton(c.Key("wa"), "Bagikan ke WhatsApp", "whatsapp", false).FillWidth().Height(44).Clicked() {
					c.OpenURL(sheet.WhatsAppURL(sheet.ShareText(st.cfg.Edit)))
				}
				muted(c, "Atau pindai QR dari ponsel.").FontSize(12)
			})
		})
		ui.Row(c).FillWidth().Padding(0, 16, 14, 16).Gap(8).Children(func() {
			ui.Text(c, shortLink(st.cfg.Edit)).FontSize(12).Font(FontMono).TextColor(t.TextMuted).MaxLines(1).Grow(1).MinWidth(0)
			muted(c, "Hanya ke grup pengurus").FontSize(12)
		})
	})
}

// sheetDuringView is Saat gangguan: how fresh the sheet is, what it
// changed, filling it from the cache, sharing it.
func (a *App) sheetDuringView(c *ui.Context) {
	st := &a.sheet
	t := c.Theme()
	if !st.linked() {
		pageNotice(c, "alert", "Sheet darurat belum disiapkan", "Siapkan sheet di Persiapan selagi server hidup. Tanpa sheet, gunakan Cara lama: salinan CSV di bawah.")
		return
	}
	now := c.Now()
	f := &st.sess.Follow
	ui.Row(c.Key("stats")).FillWidth().Gap(16).Wrap().Children(func() {
		fetched, next := "—", "belum diambil"
		if !st.lastFetch.IsZero() {
			fetched = st.lastFetch.Format("15.04")
			next = "berikutnya ±" + st.nextAt.Format("15.04")
		}
		if st.fetching {
			next = "mengambil…"
		}
		sheetStat(c, "Diambil dari Google", fetched, next, t.Text)
		clock, lag := "—", "jam sheet belum terbaca"
		if !f.Clock.IsZero() {
			clock = f.Clock.Format("15.04")
			lag = strings.TrimPrefix(sheet.Lag(f.Clock, now), "data sheet per "+clock)
			lag = strings.Trim(lag, " ()")
			if lag == "" {
				lag = "segar"
			}
		}
		sheetStat(c, "Jam sheet (tab Meta)", clock, lag, t.Text)
		sheetStat(c, "Diterapkan di perangkat", fmt.Sprint(len(f.Changed())), "tugas berubah di sheet", t.Text)
		sheetStat(c, "Ditolak", fmt.Sprint(len(f.Issues)+len(f.NoID)), "perlu diperbaiki di sheet", t.Danger)
	})
	if st.lastErr != "" {
		pageNotice(c, "alert", st.lastErr, fmt.Sprintf("Data di perangkat tidak berubah. Dicoba lagi sekitar %s.", st.nextAt.Format("15.04")))
	}
	if f.Held != nil {
		ui.Column(c.Key("held")).FillWidth().Gap(10).Padding(14, 16).Radius(12).Border(1, t.Warning).Children(func() {
			ui.Text(c, "Salinan ditahan: terlalu banyak baris berubah sekaligus").Bold()
			muted(c, fmt.Sprintf("Salinan jam %s mengubah lebih dari %d baris. Periksa sheet (dan Riwayat versi Google) sebelum menerapkannya.", f.Held.Clock.Format("15.04"), sheet.MassChange)).FontSize(13)
			ui.Row(c).Gap(8).Children(func() {
				if primarySpaceButton(c.Key("accept-held"), "Terapkan salinan ini", "check").Clicked() {
					out := f.AcceptHeld()
					if out.Kind == sheet.OutApplied {
						a.sheetApplied(out, -1, time.Now())
					}
					a.sheetSave()
				}
				if SettingButton(c, "Abaikan salinan ini", "close").Clicked() {
					f.Held = nil
					a.sheetSave()
				}
			})
		})
	}
	ui.Row(c.Key("during")).FillWidth().Gap(16).AlignItems(ui.Start).Wrap().Children(func() {
		ui.Column(c.Key("feed")).Grow(1).MinWidth(320).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).FillWidth().Padding(12, 16).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "Perubahan dari sheet").FontSize(15).Bold().Grow(1)
				if SettingButton(c, "Ambil sekarang", "refresh").Disabled(st.fetching).Clicked() {
					a.sheetFetch(time.Now())
				}
			})
			if len(st.sess.Feed) == 0 {
				ui.Column(c).Padding(16).Children(func() {
					muted(c, "Belum ada perubahan terbaca. Isi sheet dari cache bila masih kosong.").FontSize(13)
				})
			}
			for i, item := range st.sess.Feed {
				if i >= 12 {
					break
				}
				sheetFeedRow(c, i, item)
			}
		})
		ui.Column(c.Key("side")).Width(380).MinWidth(300).Shrink(1).Gap(16).Children(func() {
			a.sheetShareCard(c)
			a.sheetFillCard(c)
		})
	})
	if len(f.NoID) > 0 || len(f.Issues) > 0 {
		SettingSection(c, "Baris yang belum diterapkan", "Diperbaiki di sheet, atau ditinjau setelah pulih.", func() {
			for _, is := range f.Issues {
				SettingRow(c.Key(fmt.Sprint("is", is.Line)), fmt.Sprintf("Baris %d · %s", is.Line, is.Title), is.Reason, func() {})
			}
			for _, r := range f.NoID {
				SettingRow(c.Key(fmt.Sprint("noid", r.Line)), fmt.Sprintf("Baris %d · %s", r.Line, r.Title), "Tanpa ID: tugas yang diketik di sheet, ditinjau setelah pulih.", func() {})
			}
		})
	}
}

func sheetStat(c *ui.Context, label, value, line string, color ui.Color) {
	t := c.Theme()
	ui.Column(c.Key("stat-"+label)).Grow(1).MinWidth(180).Padding(14, 16).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		muted(c, label).FontSize(12)
		ui.Text(c, value).FontSize(20).FontWeight(600).Font(FontMono).TextColor(color)
		muted(c, line).FontSize(12).MaxLines(1)
	})
}

func sheetFeedRow(c *ui.Context, i int, item sheetFeed) {
	t := c.Theme()
	ui.Row(c.Key(i)).FillWidth().Padding(12, 16).Gap(14).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, item.At.Format("15.04")).FontSize(12).Font(FontMono).TextColor(t.TextMuted).Width(44).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, item.Title).FontSize(14).FontWeight(500).MaxLines(2)
			if item.Detail != "" {
				muted(c, item.Detail).FontSize(12).MaxLines(2)
			}
		})
		tag, color := map[string]string{"applied": "diterapkan", "issue": "ditolak", "held": "ditahan", "manual": "info", "error": "gagal"}[item.Kind], t.TextMuted
		switch item.Kind {
		case "applied":
			color = t.Success
		case "issue", "error":
			color = t.Danger
		case "held":
			color = t.Warning
		}
		statusPill(c, tag, color)
	})
}

// sheetFillCard is Isi sheet dari cache: the Spaces, then the rows to
// paste at A1 of Tugas, or the workbook to import.
func (a *App) sheetFillCard(c *ui.Context) {
	st := &a.sheet
	t := c.Theme()
	if !a.sheetManager() {
		return
	}
	ui.Column(c.Key("fill")).FillWidth().Radius(12).Background(t.Surface).Border(1, t.Border).Padding(14, 16).Gap(10).Children(func() {
		ui.Text(c, "Isi sheet dari cache").FontSize(15).Bold()
		muted(c, "Tugas aktif yang tersimpan di perangkat ini. Satu pengelola cukup; yang lain hanya membaca.").FontSize(13)
		if st.fill.spaces == nil {
			st.fill.spaces = map[id.ID]bool{}
			if sid := a.selectedSpaceID(); sid != id.Nil {
				st.fill.spaces[sid] = true
			}
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, s := range a.access.spaces {
				if !spaceCan(s, api.CapTasksManage) {
					continue
				}
				if spaceButton(c.Key(s.ID.String()), s.Name, "", st.fill.spaces[s.ID]).Disabled(st.fill.loading).Clicked() {
					st.fill.spaces[s.ID] = !st.fill.spaces[s.ID]
					st.fill.tasks = nil
				}
			}
		})
		if st.fill.err != "" {
			ui.Text(c, st.fill.err).FontSize(12).TextColor(t.Danger)
		}
		if st.fill.tasks == nil {
			label := "Siapkan baris"
			if st.fill.loading {
				label = "Membaca cache…"
			}
			if SettingButton(c, label, "refresh").Disabled(st.fill.loading || len(st.fill.spaces) == 0).Clicked() {
				a.sheetLoadFill()
			}
			return
		}
		n := len(st.fill.tasks)
		if primarySpaceButton(c.Key("copy-rows"), fmt.Sprintf("Salin %d baris tugas", n), "board").FillWidth().Height(40).Disabled(n == 0).Clicked() {
			c.WriteClipboard(sheet.TSV(st.fill.tasks))
			a.sheetFilled(time.Now())
			st.notice = "Baris tersalin. Di sheet: tab Tugas, klik sel A1, tempel (Ctrl+V). Isi lama tertimpa."
		}
		if a.ExportFile != nil && SettingButton(c, "Simpan .xlsx untuk diimpor", "archive").Disabled(st.busy || n == 0).Clicked() {
			if b, err := sheet.Workbook(st.fill.tasks, time.Now().In(a.sheetZone())); err == nil {
				a.sheetExport("space-darurat-"+time.Now().Format("20060102-1504")+".xlsx", b)
				a.sheetFilled(time.Now())
			} else {
				st.err = err.Error()
			}
		}
		muted(c, "Tempel di A1 tab Tugas, atau File → Impor → Upload → Ganti spreadsheet.").FontSize(12)
	})
}

// sheetAfterView is Setelah pulih: freeze, the final copy, reconcile,
// close.
func (a *App) sheetAfterView(c *ui.Context) {
	st := &a.sheet
	t := c.Theme()
	if !st.sess.Active {
		pageEmpty(c, "check", "Tidak ada mode darurat berjalan", "Setelah gangguan, langkah rekonsiliasi muncul di sini.")
		return
	}
	f := &st.sess.Follow
	final := !st.sess.Frozen.IsZero() && !f.Clock.Before(st.sess.Frozen)
	down := a.serverDown()
	changed := f.Changed()
	SettingSection(c, "Rekonsiliasi", "Perubahan sheet masuk server lewat rekonsiliasi yang sudah ada: hanya progres tugas yang versinya tidak berubah di server.", func() {
		frozen := "Ubah link edit menjadi Pelihat supaya tidak ada perubahan baru."
		if !st.sess.Frozen.IsZero() {
			frozen = "Dibekukan " + st.sess.Frozen.Format("15.04") + ". "
			if final {
				frozen += "Versi final: " + sheet.Lag(f.Clock, c.Now()) + "."
			} else {
				frozen += "Menunggu salinan sesudah jam itu (Google bisa tertinggal sampai ±30 menit)."
			}
		}
		SettingRow(c, "1 · Bekukan sheet", frozen, func() {
			if st.sess.Frozen.IsZero() {
				if primarySpaceButton(c.Key("frozen"), "Sudah dibekukan", "check").Clicked() {
					st.sess.Frozen = time.Now().In(a.sheetZone()).Truncate(time.Minute)
					st.nextAt = time.Time{}
					a.sheetSave()
				}
			} else if !final && SettingButton(c, "Ambil sekarang", "refresh").Disabled(st.fetching).Clicked() {
				a.sheetFetch(time.Now())
			}
		})
		line := fmt.Sprintf("%d baris berubah di sheet. Space membuat snapshot server, lalu memasangkan baris yang versinya masih sama.", len(changed))
		if down {
			line = "Menunggu server terjangkau."
		}
		SettingRow(c, "2 · Siapkan rekonsiliasi", line, func() {
			label := "Siapkan"
			if st.busy {
				label = "Menyiapkan…"
			}
			if primarySpaceButton(c.Key("prepare-recon"), label, "refresh").Disabled(!final || down || st.busy || len(changed)+len(f.NoID) == 0).Clicked() {
				a.sheetPrepareReconcile()
			}
		})
	})
	if st.recon.prepared {
		if len(st.recon.manual) > 0 {
			SettingSection(c, "Ditinjau manual", "Tidak masuk rekonsiliasi otomatis; buka tugasnya dan perbarui di Space.", func() {
				for i, m := range st.recon.manual {
					SettingRow(c.Key(i), m.Title, m.Reason, func() {
						if m.TaskID != id.Nil && SettingButton(c, "Buka tugas", "task").Clicked() {
							a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: m.TaskID})
						}
					})
				}
			})
		}
		if st.recon.count > 0 {
			ui.Column(c.Key("recon-results")).FillWidth().Gap(12).Children(func() {
				ui.Text(c, fmt.Sprintf("Pratinjau server · %d baris", st.recon.count)).FontSize(15).Bold()
				if a.emergency.err != "" {
					ui.Text(c, a.emergency.err).FontSize(13).TextColor(t.Danger)
				}
				a.emergencyResults(c)
			})
		}
	}
	SettingSection(c, "Tutup sheet", "Setelah rekonsiliasi: sheet kembali tertutup, perangkat kembali ke data server.", func() {
		if st.sess.Closed == nil {
			st.sess.Closed = map[string]bool{}
		}
		for _, item := range []struct{ key, title, line string }{
			{"unpublish", "Publikasi dihentikan", "File → Bagikan → Publikasikan ke web → Hentikan publikasi."},
			{"restrict", "Akses dibatasi", "Bagikan → Akses umum: Dibatasi."},
		} {
			on := st.sess.Closed[item.key]
			if SettingSwitch(c, item.title, item.line, &on) {
				st.sess.Closed[item.key] = on
				a.sheetSave()
			}
		}
		SettingRow(c, "Akhiri mode darurat", "Kunci progres dibuka, tugas dimuat ulang dari server.", func() {
			if SettingButton(c, "Akhiri", "check").Disabled(down || !st.sess.Closed["unpublish"] || !st.sess.Closed["restrict"]).Clicked() {
				a.sheetEnd()
			}
		})
	})
}

// emergencyBanner is the line under the header while the emergency lasts.
func (a *App) emergencyBanner(c *ui.Context) {
	st := &a.sheet
	if !st.loaded || a.isAdmin() {
		return
	}
	down := a.serverDown()
	if !st.sess.Active && !down {
		return
	}
	t := c.Theme()
	text := "Server Space tidak terjangkau."
	switch {
	case st.sess.Active && down:
		text = "Mode darurat · progres tugas mengikuti sheet · " + sheet.Lag(st.sess.Follow.Clock, c.Now())
	case st.sess.Active:
		text = "Server pulih · bekukan sheet, lalu rekonsiliasi di Pusat darurat"
	case !st.linked():
		text = "Server Space tidak terjangkau · sheet darurat belum disiapkan"
	}
	ui.Row(c.Key("emergency-banner")).FillWidth().Padding(8, 24).Gap(12).AlignItems(ui.Center).Background(t.Danger.Alpha(0.12)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(t.Danger).Shrink(0)
		ui.Text(c, text).FontSize(13).MaxLines(1).Grow(1).MinWidth(0)
		if st.cfg.Edit != "" && spaceButton(c.Key("banner-open"), "Buka sheet", "external", false).Height(28).Clicked() {
			c.OpenURL(st.cfg.Edit)
		}
		if spaceButton(c.Key("banner-center"), "Pusat darurat", "", false).Height(28).Clicked() {
			a.OpenEmergencyCenter()
		}
	})
}
