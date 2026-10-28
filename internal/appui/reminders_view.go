package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (a *App) remindersView(c *ui.Context) {
	if a.isAdmin() {
		pageBody(c, "Reminder pribadi", func() {
			pageNotice(c, "lock", "Gunakan akun pribadi", "Akun organisasi tidak dapat mengakses reminder pribadi pengurus.")
		})
		return
	}
	a.ensureReminders()
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Gap(0).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Padding(12, 22).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Reminder pribadi").FontSize(16).Bold().Grow(1).MinWidth(0)
			if spaceIconButton(c, "Muat ulang reminder", "refresh").Disabled(a.reminders.busy || a.reminders.loading).Clicked() {
				a.loadReminders()
			}
			if spaceButton(c, "Buat reminder", "plus", true).Disabled(a.reminderWriteBlocked() || !a.reminders.loaded || !a.reminders.prefsLoaded).Clicked() {
				a.beginReminder()
			}
		})
		a.reminderFeedback(c)
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c).Grow(1).MinWidth(0).MinHeight(0).Label("Daftar reminder pribadi").Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Padding(22).Gap(18).Children(func() {
					a.reminderPrivacyPreference(c)
					muted(c, "Reminder disimpan ke akunmu. Pengiriman notifikasi terjadwal belum tersedia.")
					ui.TextInput(c.Key("reminder-filter"), &a.reminders.filter).FillWidth().Label("Cari reminder").Placeholder("Cari judul atau catatan…")
					a.reminderList(c)
				})
			})
			if (a.reminders.draft != nil || a.reminders.selected != id.Nil) && !a.chrome.appearance {
				a.reminderPanel(c)
			}
		})
	})
}

func (a *App) reminderFeedback(c *ui.Context) {
	if a.reminders.err != "" {
		ui.Text(c, a.reminders.err).FillWidth().MinWidth(0).Padding(8, 22).FontSize(12).TextColor(c.Theme().Danger)
	}
	if a.reminders.notice != "" {
		ui.Row(c).FillWidth().MinWidth(0).Padding(8, 22).Gap(8).Children(func() {
			ui.Text(c, a.reminders.notice).FontSize(12).TextColor(c.Theme().Accent).Grow(1).MinWidth(0)
			if spaceIconButton(c, "Tutup informasi reminder", "close").Clicked() {
				a.reminders.notice = ""
			}
		})
	}
	if a.reminders.busy {
		muted(c, "Menyimpan perubahan ke server…").Padding(6, 22)
	}
	if a.conn == connOffline {
		muted(c, "Tidak terhubung. Data terakhir ditampilkan; perubahan belum dikirim ke cloud.").Padding(6, 22)
	}
}

func (a *App) reminderPrivacyPreference(c *ui.Context) {
	ui.Column(c).FillWidth().MinWidth(0).Padding(15).Gap(8).Radius(10).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("lock")).Size(18, 18).Shrink(0).TextColor(c.Theme().Accent)
			ui.Text(c, "Privasi di tanganmu").FontSize(13).Bold().Grow(1).MinWidth(0)
		})
		muted(c, "Privat secara default. Kadiv, role 45, admin, dan pengurus periode baru tidak mendapat akses otomatis.")
		if a.reminders.prefsErr != "" {
			ui.Text(c, a.reminders.prefsErr).FontSize(12).TextColor(c.Theme().Danger)
		}
		if !a.reminders.prefsLoaded {
			muted(c, "Preferensi privasi belum dimuat. Gunakan muat ulang untuk mencoba kembali.")
			return
		}
		label := "Default berbagi: nonaktif"
		if a.reminders.prefs.DefaultShare {
			label = "Default berbagi: aktif"
		}
		if spaceButton(c, label, "users", a.reminders.prefs.DefaultShare).FillWidth().Disabled(a.reminderWriteBlocked()).Clicked() {
			a.setReminderDefaultShare(!a.reminders.prefs.DefaultShare)
		}
		muted(c, "Berlaku untuk reminder baru saja. Penerima tetap dipilih per akun; reminder lama tidak berubah.")
	})
}

func (a *App) reminderList(c *ui.Context) {
	if !a.reminders.loaded {
		title, detail := "Memuat reminder…", "Mengambil reminder milikmu dan yang dibagikan kepadamu."
		if a.reminders.err != "" {
			title, detail = "Reminder belum dapat dimuat", "Gunakan tombol muat ulang untuk mencoba kembali."
		}
		pageEmpty(c, "bell", title, detail)
		return
	}
	q := strings.ToLower(strings.TrimSpace(a.reminders.filter))
	count := 0
	for _, completed := range []bool{false, true} {
		section := false
		for _, r := range a.reminders.items {
			if r.Completed != completed || (q != "" && !strings.Contains(strings.ToLower(r.Title+" "+r.Notes), q)) {
				continue
			}
			if !section {
				label := "Ingin diingat"
				if completed {
					label = "Sudah selesai"
				}
				ui.Text(c, label).FontSize(12).Bold().TextColor(c.Theme().TextMuted)
				section = true
			}
			count++
			a.reminderCard(c.Key(r.ID), r)
		}
	}
	if count == 0 {
		title, detail := "Belum ada reminder", "Buat pengingat pribadi pertamamu. Reminder ini bukan jobdesk divisi."
		if q != "" {
			title, detail = "Reminder tidak ditemukan", "Coba judul atau catatan lain."
		}
		pageEmpty(c, "bell", title, detail)
	}
}

func (a *App) reminderCard(c *ui.Context, r api.Reminder) {
	t := c.Theme()
	b := ui.ButtonBase(c).Label("Reminder "+r.Title).Column().AlignItems(ui.Start).FillWidth().MinWidth(0).Gap(8).Padding(15).Radius(10).Border(1, t.Border).Background(t.Surface)
	if a.reminders.selected == r.ID || b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(10).AlignItems(ui.Start).Children(func() {
			icon := "bell"
			if r.Completed {
				icon = "check"
			}
			ui.Icon(c, spaceIcon(icon)).Size(18, 18).Shrink(0).TextColor(t.Accent)
			ui.Text(c, r.Title).FontSize(14).Bold().Grow(1).MinWidth(0)
		})
		muted(c, taskDate(r.DueOn, r.DueAt, r.TimeZone)).FontSize(12)
		label := "Milik saya · privat"
		if r.Relation != api.ReminderOwner {
			label = "Dibagikan kepada saya · hanya baca"
		} else if r.Shared {
			label = "Milik saya · berbagi aktif"
		}
		muted(c, label).FontSize(11)
	})
	if b.Clicked() && !a.reminders.busy {
		a.openReminder(r.ID)
	}
}

func (a *App) reminderPanel(c *ui.Context) {
	r := a.reminderByID(a.reminders.selected)
	owner := r != nil && r.Relation == api.ReminderOwner && r.OwnerID == a.user.ID
	creating := a.reminders.draft != nil && a.reminders.draft.reminderID == id.Nil
	title := "Detail reminder"
	if creating {
		title = "Reminder baru"
	}
	ui.Column(c).Width(350).Shrink(0).MinWidth(0).MinHeight(0).FillHeight().Background(c.Theme().Surface).BorderWidth(0, 0, 0, 1).BorderColor(c.Theme().Border).Label("Panel reminder").Children(func() {
		ui.Row(c).FillWidth().Padding(16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).Bold().Grow(1).MinWidth(0)
			if spaceIconButton(c, "Tutup reminder", "close").Disabled(a.reminders.busy).Clicked() {
				a.closeReminder()
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Area scroll detail reminder").Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Padding(18).Gap(16).Children(func() {
				if a.reminders.detailsLoading {
					muted(c, "Memuat rincian reminder…")
				}
				if a.reminders.detailsErr != "" {
					ui.Text(c, a.reminders.detailsErr).FontSize(12).TextColor(c.Theme().Danger)
					if spaceButton(c, "Muat ulang detail reminder", "refresh", false).FillWidth().Disabled(a.reminders.detailsLoading || a.reminders.busy).Clicked() {
						a.loadReminderDetails()
					}
				}
				if creating || owner {
					a.reminderEditor(c)
					if owner {
						a.reminderOwnerActions(c, *r)
					}
				} else if r != nil {
					a.reminderReadOnly(c, *r)
				} else {
					pageEmpty(c, "lock", "Reminder tidak tersedia", "Muat ulang daftar untuk memeriksa akses terbaru.")
				}
			})
		})
		if (creating || owner) && a.reminders.draft != nil {
			ui.Row(c).FillWidth().Padding(12, 18).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
				if spaceButton(c, "Simpan reminder", "check", true).FillWidth().Disabled(a.reminderWriteBlocked() || a.reminders.detailsLoading || a.reminders.detailsErr != "").Clicked() {
					a.saveReminder()
				}
			})
		}
	})
}

func (a *App) reminderEditor(c *ui.Context) {
	d := a.reminders.draft
	if d == nil {
		return
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(13).Disabled(a.reminderWriteBlocked() || a.reminders.detailsLoading || a.reminders.detailsErr != "").Children(func() {
		taskInput(c, "reminder-title", "Judul reminder *", &d.title)
		taskLongInput(c, "reminder-notes", "Catatan reminder", &d.notes)
		taskInput(c, "reminder-date", "Tanggal reminder *", &d.date)
		muted(c, "Gunakan YYYY-MM-DD, misalnya 2026-10-31.").FontSize(11)
		taskInput(c, "reminder-clock", "Jam reminder (opsional)", &d.clock)
		muted(c, "Gunakan HH:MM. Kosongkan untuk menyimpan tanggal tanpa jam.").FontSize(11)
		taskInput(c, "reminder-zone", "Zona waktu reminder", &d.zone)
		muted(c, "Zona IANA, misalnya Asia/Jakarta. Tidak mengikuti perubahan periode.").FontSize(11)
	})
	if d.reminderID == id.Nil {
		label := "Reminder ini akan dibuat privat."
		if a.reminders.prefs.DefaultShare {
			label = "Berbagi akan aktif sesuai preferensimu. Setelah menyimpan, pilih akun penerimanya."
		}
		pageNotice(c, "lock", "Privasi reminder baru", label)
	}
}

func (a *App) reminderReadOnly(c *ui.Context, r api.Reminder) {
	ui.Text(c, r.Title).FontSize(21).Bold()
	pageNotice(c, "lock", "Dibagikan kepadamu", "Aksesmu hanya baca. Hanya pemilik yang dapat mengubah, menyelesaikan, atau membagikan reminder ini.")
	ui.Text(c, taskDate(r.DueOn, r.DueAt, r.TimeZone)).FontSize(13)
	muted(c, "Zona waktu: "+r.TimeZone)
	if r.Notes != "" {
		ui.Text(c, "Catatan").FontSize(12).Bold()
		ui.Text(c, r.Notes).FontSize(13)
	}
	status := "Belum selesai"
	if r.Completed {
		status = "Sudah selesai"
	}
	muted(c, status)
}

func (a *App) reminderOwnerActions(c *ui.Context, r api.Reminder) {
	blocked := a.reminderWriteBlocked() || a.reminders.detailsLoading || a.reminders.detailsErr != ""
	label := "Tandai reminder selesai"
	if r.Completed {
		label = "Buka kembali reminder"
	}
	if spaceButton(c, label, "check", r.Completed).FillWidth().Disabled(blocked).Clicked() {
		a.completeReminder()
	}
	if r.CompletedAt != nil {
		muted(c, "Selesai "+taskTimestamp(*r.CompletedAt, r.TimeZone)).FontSize(11)
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(10).Padding(14).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(func() {
		ui.Text(c, "Akses berbagi").FontSize(13).Bold()
		shareLabel := "Berbagi: nonaktif"
		if r.Shared {
			shareLabel = "Berbagi: aktif"
		}
		if spaceButton(c, shareLabel, "users", r.Shared).FillWidth().Disabled(blocked).Clicked() {
			a.setReminderSharing(!r.Shared)
		}
		if r.Shared {
			muted(c, "Hanya akun di daftar penerima yang dapat membaca. Daftar kosong berarti belum ada yang mendapat akses.")
		} else {
			muted(c, "Reminder tersembunyi dari semua penerima. Daftar izin tetap disimpan sampai kamu mencabutnya.")
		}
		a.reminderRecipientsView(c, blocked)
	})
	ui.Column(c).FillWidth().MinWidth(0).Gap(8).Children(func() {
		ui.Text(c, "Riwayat reminder").FontSize(13).Bold()
		if !a.reminders.detailsLoading && a.reminders.detailsErr == "" {
			if len(a.reminders.history) == 0 {
				muted(c, "Belum ada riwayat yang tersedia.")
			}
			for i, h := range a.reminders.history {
				ui.Column(c.Key(i)).FillWidth().MinWidth(0).Gap(4).Padding(8, 0).Children(func() {
					ui.Text(c, reminderAuditLabel(h.Action)).FontSize(12)
					muted(c, taskTimestamp(h.OccurredAt, r.TimeZone)).FontSize(11)
				})
			}
		}
	})
	if !a.reminders.deleteConfirm {
		if spaceButton(c, "Hapus reminder", "close", false).FillWidth().Disabled(blocked).Clicked() {
			a.reminders.deleteConfirm = true
		}
	} else {
		ui.Column(c).FillWidth().MinWidth(0).Gap(8).Padding(14).Radius(10).Border(1, c.Theme().Danger).Children(func() {
			ui.Text(c, "Hapus reminder ini?").FontSize(13).Bold()
			muted(c, "Reminder akan dihapus dari akunmu dan tidak lagi dapat dibaca penerima.")
			if spaceButton(c, "Hapus permanen", "close", true).FillWidth().Disabled(blocked).Clicked() {
				a.deleteReminder()
			}
			if spaceButton(c, "Batal hapus", "arrow", false).FillWidth().Disabled(a.reminders.busy).Clicked() {
				a.reminders.deleteConfirm = false
			}
		})
	}
}

func (a *App) reminderRecipientsView(c *ui.Context, blocked bool) {
	ui.Text(c, "Penerima reminder").FontSize(12).Bold()
	if !a.reminders.recipientsLoaded {
		muted(c, "Daftar penerima belum dimuat.")
	} else if len(a.reminders.recipients) == 0 {
		muted(c, "Belum ada penerima yang dipilih.")
	}
	for _, s := range a.reminders.recipients {
		ui.Column(c.Key(s.RecipientID)).FillWidth().MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, s.DisplayName).FontSize(12).Bold()
			muted(c, s.RecipientID.String()).FontSize(10)
			if spaceButton(c, "Cabut akses "+s.DisplayName, "unlink", false).FillWidth().Disabled(blocked).Clicked() {
				a.removeReminderRecipient(s.RecipientID)
			}
		})
	}
	b := spaceButton(c, "Tambah penerima", "plus", false).FillWidth().Disabled(blocked || !a.reminders.recipientsLoaded)
	if b.Clicked() {
		a.reminders.recipientOpen = !a.reminders.recipientOpen
		if a.reminders.recipientOpen {
			a.loadReminderCandidates()
		}
	}
	popover(c, b, &a.reminders.recipientOpen, 300, func() {
		ui.Text(c, "Pilih akun penerima").FontSize(13).Bold()
		muted(c, "Akses hanya baca, terikat ke akun ini dan tidak diwariskan ke jabatan berikutnya.")
		if a.reminders.candidatesLoading {
			muted(c, "Memuat akun dari direktori yang dapat kamu akses…")
			return
		}
		if a.reminders.candidatesErr != "" {
			ui.Text(c, a.reminders.candidatesErr).FontSize(12).TextColor(c.Theme().Danger)
			if spaceButton(c, "Muat ulang calon penerima", "refresh", false).FillWidth().Disabled(a.reminders.busy).Clicked() {
				a.loadReminderCandidates()
			}
			return
		}
		ui.TextInput(c.Key("reminder-recipient-filter"), &a.reminders.recipientFilter).FillWidth().Label("Cari akun penerima").Placeholder("Nama atau ID akun…")
		q := strings.ToLower(strings.TrimSpace(a.reminders.recipientFilter))
		count := 0
		ui.Scroll(c).MaxHeight(200).FillWidth().Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
				for _, m := range a.reminders.candidates {
					if q != "" && !strings.Contains(strings.ToLower(m.DisplayName+" "+m.UserID.String()), q) {
						continue
					}
					already := false
					for _, existing := range a.reminders.recipients {
						if existing.RecipientID == m.UserID {
							already = true
							break
						}
					}
					if already {
						continue
					}
					count++
					choice := ui.ButtonBase(c.Key(m.UserID)).Label("Pilih penerima " + m.DisplayName).Column().AlignItems(ui.Start).FillWidth().MinWidth(0).Padding(9).Gap(3).Radius(7).Disabled(blocked)
					if choice.Hovered() {
						choice.Background(c.Theme().SurfaceHover)
					}
					choice.FocusRing(choice.FocusVisible()).Children(func() {
						ui.Text(c, m.DisplayName).FontSize(12)
						muted(c, m.UserID.String()).FontSize(10)
					})
					if choice.Clicked() {
						a.addReminderRecipient(m.UserID)
						a.reminders.recipientOpen = false
					}
				}
			})
		})
		if count == 0 && a.reminders.candidatesLoaded {
			muted(c, "Tidak ada akun lain yang cocok di direktori yang dapat kamu akses.")
		}
	})
}

func reminderAuditLabel(action string) string {
	labels := map[string]string{
		"reminder.create":       "Reminder dibuat",
		"reminder.update":       "Reminder diperbarui",
		"reminder.complete":     "Reminder diselesaikan",
		"reminder.reopen":       "Reminder dibuka kembali",
		"reminder.sharing":      "Pengaturan berbagi diubah",
		"reminder.share.add":    "Penerima ditambahkan",
		"reminder.share.remove": "Akses penerima dicabut",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return "Perubahan: " + action
}
