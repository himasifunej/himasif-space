package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (a *App) organizationAdminView(c *ui.Context) {
	if !a.isAdmin() {
		pageBody(c, "Pengaturan organisasi", func() {
			pageNotice(c, "lock", "Khusus admin organisasi", "Pengelolaan periode, penetapan kadiv dan checklist serah terima memakai akun admin organisasi.")
		})
		return
	}
	a.ensureOrganization()
	pageBody(c, "Pengaturan organisasi", func() {
		a.periodLine(c)
		muted(c, "Siapkan kepengurusan berikutnya sambil menjaga riwayat periode sebelumnya.")
		ui.ScrollHorizontal(c).FillWidth().Height(46).Shrink(0).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, tab := range []struct{ name, icon string }{{"Periode", "calendar"}, {"Kadiv", "users"}, {"Serah terima", "archive"}} {
					if spaceButton(c.Key(tab.name), tab.name, tab.icon, a.organization.tab == tab.name).Clicked() {
						a.cancelOrganizationConfirmation()
						a.organization.tab = tab.name
					}
				}
			})
		})
		if a.organization.err != "" {
			pageNotice(c, "alert", "Permintaan belum berhasil", a.organization.err)
		}
		if a.organization.notice != "" {
			pageNotice(c, "check", "Perubahan tersimpan", a.organization.notice)
		}
		if a.organization.loading {
			muted(c, "Memuat data administrasi dari server…")
		}
		if a.organization.busy {
			muted(c, "Menyimpan perubahan ke server…")
		}
		if a.conn != connOnline {
			pageNotice(c, "wifi", "Tidak terhubung", "Data terakhir dapat dibaca. Perubahan administrasi memerlukan koneksi dan belum dikirim ke cloud.")
		}
		if spaceButton(c, "Muat ulang administrasi", "refresh", false).Disabled(a.organization.loading || a.organization.busy).Clicked() {
			a.loadOrganization()
		}
		if a.organization.confirm != nil {
			a.organizationConfirmationView(c)
		}
		switch a.organization.tab {
		case "Kadiv":
			a.organizationKadivView(c)
		case "Serah terima":
			a.organizationHandoverView(c)
		default:
			a.organizationPeriodsView(c)
		}
	})
}

func (a *App) organizationPeriodsView(c *ui.Context) {
	if a.periodsErr != "" {
		pageNotice(c, "alert", "Periode belum dapat dimuat", a.periodsErr)
	}
	if p := a.period(); p != nil {
		pageSection(c, "Periode terpilih", "Ganti periode dari pemilih di sidebar untuk mengelola kepengurusan lain.", func() {
			pagePeriodCard(c, *p)
			switch p.Status {
			case api.PeriodPreparing:
				muted(c, "Aktivasi tidak menutup periode lain. Penutupan periode lama dilakukan terpisah.")
				if spaceButton(c, "Aktifkan periode", "check", true).FillWidth().Disabled(a.organizationWriteBlocked()).Clicked() {
					a.requestOrganizationAction("activate", id.Nil)
				}
			case api.PeriodClosed:
				pageNotice(c, "lock", "Periode ditutup", "Data periode ini hanya dapat dibaca. Periode baru tetap dapat disiapkan melalui formulir di bawah.")
			}
			if p.Status != api.PeriodClosed {
				if spaceButton(c, "Tutup periode", "archive", false).FillWidth().Disabled(a.organizationWriteBlocked()).Clicked() {
					a.requestOrganizationAction("close", id.Nil)
				}
			}
		})
	} else if !a.periodsLoading && a.periodsErr == "" {
		pageEmpty(c, "calendar", "Belum ada periode", "Siapkan periode pertama untuk mulai menyusun kepengurusan.")
	}
	if a.organization.create == nil {
		if spaceButton(c, "Buat periode", "plus", true).FillWidth().Disabled(a.organizationWriteBlocked() || a.divisionsLoading || a.divisionsErr != "").Clicked() {
			a.beginPeriod()
		}
		if a.divisionsErr != "" {
			pageNotice(c, "alert", "Struktur divisi belum dapat dimuat", a.divisionsErr)
		}
		return
	}
	d := a.organization.create
	pageSection(c, "Periode baru", "Periode dimulai dalam persiapan. Anggota, role dan pekerjaan periode lama tidak disalin otomatis.", func() {
		ui.Column(c).FillWidth().MinWidth(0).Gap(13).Disabled(a.organizationWriteBlocked()).Children(func() {
			taskInput(c, "period-label", "Nama periode *", &d.label)
			taskInput(c, "period-start", "Tanggal mulai *", &d.start)
			taskInput(c, "period-end", "Tanggal selesai *", &d.end)
			muted(c, "Gunakan YYYY-MM-DD untuk kedua tanggal, misalnya 2026-11-01.").FontSize(11)
			taskInput(c, "period-zone", "Zona waktu periode *", &d.zone)
			muted(c, "Misalnya Asia/Jakarta. Tanggal tanpa jam tetap disimpan sebagai tanggal.").FontSize(11)
			ui.Text(c, "Divisi dalam periode *").FontSize(12).Bold()
			if len(a.divisions) == 0 {
				muted(c, "Struktur divisi belum tersedia. Muat ulang sebelum membuat periode.")
			}
			for _, div := range a.divisions {
				selected := d.divisions[div.ID]
				icon := "plus"
				if selected {
					icon = "check"
				}
				if spaceButton(c.Key(div.ID), div.Name, icon, selected).FillWidth().Clicked() {
					d.divisions[div.ID] = !selected
				}
			}
			muted(c, "Pilih setidaknya satu divisi. Medhum dibuat bila Humas dan Mediatek sama-sama dipilih.").FontSize(11)
		})
		if spaceButton(c, "Simpan periode baru", "check", true).FillWidth().Disabled(a.organizationWriteBlocked()).Clicked() {
			a.savePeriod()
		}
		if spaceButton(c, "Batal membuat periode", "close", false).FillWidth().Disabled(a.organization.busy).Clicked() {
			a.organization.create = nil
		}
	})
}

func (a *App) organizationKadivView(c *ui.Context) {
	if !a.organization.loaded {
		pageEmpty(c, "users", "Data pengurus belum tersedia", "Muat ulang administrasi untuk memilih kadiv dari akun pribadi yang terverifikasi.")
		return
	}
	divisions := a.organizationDivisions()
	if len(divisions) == 0 {
		pageEmpty(c, "users", "Belum ada divisi yang dapat dikelola", "Pilih periode yang struktur divisinya sudah tersedia.")
		return
	}
	pageSection(c, "Divisi periode ini", "Penetapan berlaku untuk divisi dan periode yang dipilih.", func() {
		for _, d := range divisions {
			if spaceButton(c.Key(d.ID), d.Name, "users", a.organization.division == d.ID).FillWidth().Disabled(a.organization.busy).Clicked() {
				a.selectOrganizationDivision(d.ID)
			}
		}
	})
	o := &a.organization
	if o.division == id.Nil {
		return
	}
	if !o.membersLoaded && !o.membersLoading && o.membersErr == "" {
		a.loadOrganizationMembers(o.division)
	}
	pageSection(c, "Kepengurusan "+a.organizationDivisionName(o.division), "Anggota, undangan dan role operasional dapat dikelola dari tab Anggota pada Space divisi.", func() {
		if o.membersLoading {
			muted(c, "Memuat kepengurusan divisi…")
		}
		if o.membersErr != "" {
			pageNotice(c, "alert", "Kepengurusan belum dapat dimuat", o.membersErr)
		}
		if spaceButton(c, "Muat ulang kepengurusan", "refresh", false).FillWidth().Disabled(o.membersLoading || o.busy).Clicked() {
			a.loadOrganizationMembers(o.division)
		}
		if !o.membersLoaded {
			return
		}
		if len(o.members) == 0 {
			muted(c, "Belum ada anggota atau kadiv pada divisi ini.")
		}
		for _, member := range o.members {
			ui.Column(c.Key(member.UserID)).FillWidth().MinWidth(0).Gap(3).Padding(7, 0).Children(func() {
				ui.Text(c, member.DisplayName).FontSize(13).Bold()
				muted(c, memberRoleLabels(member.Roles)).FontSize(11)
			})
		}
	})
	if !a.organizationPeriodWritable() {
		pageNotice(c, "lock", "Kepengurusan hanya baca", "Kadiv pada periode yang telah ditutup tidak dapat diganti.")
		return
	}
	pageSection(c, "Tetapkan kadiv", "Pilih akun pribadi aktif dengan email terverifikasi. Akun yang belum menjadi anggota akan ditambahkan pada divisi ini.", func() {
		blocked := a.organizationWriteBlocked() || !o.membersLoaded || o.membersLoading
		label := "Pilih akun kadiv"
		if o.kadivUser != id.Nil {
			label = a.organizationUserName(o.kadivUser)
		}
		b := spaceButton(c, label, "user", o.kadivUser != id.Nil).FillWidth().Disabled(blocked)
		if b.Clicked() {
			o.userOpen = !o.userOpen
		}
		popover(c, b, &o.userOpen, 340, func() {
			ui.Text(c, "Akun calon kadiv").Bold()
			count := 0
			ui.Scroll(c).FillWidth().MaxHeight(240).Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
					for _, u := range o.users {
						if u.Kind != api.AccountPerson || !u.EmailVerified || u.DisabledAt != nil {
							continue
						}
						count++
						choice := ui.ButtonBase(c.Key(u.ID)).Label("Pilih kadiv " + u.DisplayName).Column().AlignItems(ui.Start).FillWidth().MinWidth(0).Padding(10).Gap(4).Radius(7).Disabled(blocked)
						if choice.Hovered() {
							choice.Background(c.Theme().SurfaceHover)
						}
						choice.FocusRing(choice.FocusVisible()).Children(func() {
							ui.Text(c, u.DisplayName).FontSize(13)
							muted(c, u.Email).FontSize(11)
						})
						if choice.Clicked() {
							a.cancelOrganizationConfirmation()
							o.kadivUser, o.userOpen = u.ID, false
						}
					}
				})
			})
			if count == 0 {
				muted(c, "Belum ada akun aktif dan terverifikasi yang dapat dipilih.")
			}
		})
		muted(c, "Kadiv lama tetap menjadi anggota. Akses privat tidak berpindah; jika masih ada 45 aktif, kadiv asli perlu mencopotnya terlebih dahulu.")
		if spaceButton(c, "Tinjau penetapan kadiv", "shield", true).FillWidth().Disabled(blocked || o.kadivUser == id.Nil).Clicked() {
			a.requestOrganizationAction("kadiv", o.kadivUser)
		}
	})
}

func (a *App) organizationHandoverView(c *ui.Context) {
	o := &a.organization
	if !o.loaded || o.handoverDraft == nil {
		pageEmpty(c, "archive", "Checklist belum dapat dimuat", "Muat ulang administrasi untuk membaca atau mengisi serah terima periode ini.")
		return
	}
	d := o.handoverDraft
	blocked := a.organizationWriteBlocked() || !a.organizationPeriodWritable()
	if !a.organizationPeriodWritable() {
		pageNotice(c, "lock", "Checklist hanya baca", "Periode telah ditutup. Catatan serah terima yang tersimpan tetap dapat dibaca.")
	}
	pageSection(c, "Checklist serah terima", "Catat penanggung jawab dan tindak lanjut. Menyimpan checklist tidak menutup periode atau menyalin tugas secara otomatis.", func() {
		if o.handover.UpdatedAt != nil {
			zone := "Asia/Jakarta"
			if p := a.period(); p != nil && p.TimeZone != "" {
				zone = p.TimeZone
			}
			muted(c, "Terakhir disimpan "+taskTimestamp(*o.handover.UpdatedAt, zone)).FontSize(11)
		} else {
			muted(c, "Checklist belum pernah disimpan.").FontSize(11)
		}
		label := "Tanpa periode penerus"
		if d.successor != id.Nil {
			label = a.organizationPeriodName(d.successor)
		}
		ui.Text(c, "Periode penerus (opsional)").FontSize(12).Bold()
		b := spaceButton(c, label, "calendar", d.successor != id.Nil).FillWidth().Disabled(blocked)
		if b.Clicked() {
			o.successorOpen = !o.successorOpen
		}
		popover(c, b, &o.successorOpen, 310, func() {
			if spaceButton(c, "Tanpa periode penerus", "close", d.successor == id.Nil).FillWidth().Disabled(blocked).Clicked() {
				d.successor, o.successorOpen = id.Nil, false
			}
			for _, p := range a.periods {
				if p.ID == o.period || p.Status == api.PeriodClosed {
					continue
				}
				if spaceButton(c.Key(p.ID), p.Label, "calendar", p.ID == d.successor).FillWidth().Disabled(blocked).Clicked() {
					d.successor, o.successorOpen = p.ID, false
				}
			}
		})
		ui.Column(c).FillWidth().MinWidth(0).Gap(14).Disabled(blocked).Children(func() {
			taskLongInput(c, "handover-incoming", "Pengurus dan kadiv baru", &d.incoming)
			taskLongInput(c, "handover-custodian", "Pengelola akun admin organisasi", &d.custodian)
			taskLongInput(c, "handover-bot", "Pengelolaan bot", &d.bot)
			taskLongInput(c, "handover-archive", "Rencana izin arsip", &d.archive)
			taskLongInput(c, "handover-remaining", "Pekerjaan tersisa / alasan tidak ada *", &d.remaining)
		})
		if spaceButton(c, "Simpan checklist serah terima", "check", true).FillWidth().Disabled(blocked || strings.TrimSpace(d.remaining) == "").Clicked() {
			a.saveHandover()
		}
	})
	pageSection(c, "Serah terima akun admin", "Perubahan kata sandi, email dan pengelola akun bersama dilakukan melalui pengaturan akun, terpisah dari checklist periode.", func() {
		if spaceButton(c, "Buka serah terima akun admin", "user", false).FillWidth().Disabled(o.busy).Clicked() {
			a.chrome.settingsTab = "Akun"
			a.navigate(PathPengaturan)
		}
	})
}

func (a *App) organizationConfirmationView(c *ui.Context) {
	confirmation := a.organization.confirm
	if confirmation == nil {
		return
	}
	title, detail, label := "Tinjau perubahan", "", "Konfirmasi perubahan"
	switch confirmation.action {
	case "activate":
		title = "Aktifkan periode " + a.organizationPeriodName(confirmation.period) + "?"
		detail = "Periode ini menjadi aktif. Periode lain tetap mengikuti statusnya sampai ditutup secara terpisah."
		label = "Konfirmasi aktivasi"
	case "close":
		title = "Tutup periode " + a.organizationPeriodName(confirmation.period) + "?"
		detail = "Seluruh ruang periode ini menjadi hanya baca. Data dan riwayat tetap tersimpan. Periode yang ditutup tidak dapat dibuka kembali."
		label = "Konfirmasi penutupan"
	case "kadiv":
		title = "Tetapkan " + a.organizationUserName(confirmation.user) + " sebagai kadiv?"
		detail = "Divisi " + a.organizationDivisionName(confirmation.division) + " · periode " + a.organizationPeriodName(confirmation.period) + ". Kadiv lama tetap menjadi anggota; reminder dan objek privatnya tidak diwariskan."
		label = "Konfirmasi kadiv"
	}
	section := ui.Column(c.Key("organization-confirmation")).FillWidth().MinWidth(0).Label("Konfirmasi administrasi")
	section.Children(func() {
		pageSection(c, title, detail, func() {
			if spaceButton(c, label, "check", true).FillWidth().Disabled(a.organizationWriteBlocked()).Clicked() {
				a.applyOrganizationConfirmation()
			}
			if spaceButton(c, "Batal konfirmasi administrasi", "close", false).FillWidth().Disabled(a.organization.busy).Clicked() {
				a.cancelOrganizationConfirmation()
			}
		})
	})
	if confirmation.reveal {
		section.ScrollIntoView()
		confirmation.reveal = false
	}
}
