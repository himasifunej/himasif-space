package appui

import (
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (a *App) archiveManagementView(c *ui.Context) {
	a.ensureArchiveAdmin()
	pageBody(c, "Izin arsip", func() {
		a.periodLine(c)
		muted(c, "Berikan anggota periode ini akses baca ke satu Space dari periode yang telah ditutup.")
		divisions := a.archiveManagedDivisions()
		if len(divisions) == 0 {
			pageNotice(c, "lock", "Pengelolaan izin tidak tersedia", "Pilih periode yang masih terbuka dan divisi yang kamu kelola sebagai kadiv asli atau admin organisasi. Role 45 tidak mengatur akses arsip.")
			return
		}
		pageSection(c, "Divisi pemberi izin", "Izin berlaku untuk anggota divisi dan periode yang dipilih.", func() {
			for _, d := range divisions {
				if spaceButton(c.Key(d.ID), d.Name, "users", a.archives.division == d.ID).FillWidth().Disabled(a.archives.busy).Clicked() {
					a.selectArchiveDivision(d.ID)
				}
			}
		})
		if a.archives.err != "" {
			pageNotice(c, "alert", "Permintaan belum berhasil", a.archives.err)
		}
		if a.archives.notice != "" {
			pageNotice(c, "check", "Perubahan tersimpan", a.archives.notice)
		}
		if a.archives.loading {
			muted(c, "Memuat anggota, izin dan pilihan Space arsip…")
		}
		if a.archives.busy {
			muted(c, "Menyimpan perubahan izin ke server…")
		}
		if a.conn != connOnline {
			pageNotice(c, "wifi", "Tidak terhubung", "Pengaturan izin memerlukan koneksi. Perubahan belum dikirim ke cloud.")
		}
		if spaceButton(c, "Muat ulang izin arsip", "refresh", false).FillWidth().Disabled(a.archives.loading || a.archives.busy).Clicked() {
			a.loadArchiveAdmin()
		}
		if !a.archives.loaded {
			pageEmpty(c, "archive", "Izin belum dapat ditampilkan", "Daftar izin ditampilkan setelah kewenangan dan data dari server berhasil dimuat.")
			return
		}
		pageNotice(c, "shield", "Akses baca per Space", "Izin arsip tidak membuka direktori divisi atau objek privat. Reminder pribadi tetap mengikuti izin pemiliknya.")
		a.archiveGrantForm(c)
		a.archiveExistingGrants(c)
	})
}

func (a *App) archiveGrantForm(c *ui.Context) {
	state := &a.archives
	pageSection(c, "Berikan akses arsip", "Pilih akun anggota dan Space arsip secara eksplisit. Daftar pilihan Space hanya metadata dan tidak memberi akses ke isinya.", func() {
		if state.targetsErr != "" {
			pageNotice(c, "alert", "Pilihan Space belum dapat dimuat", state.targetsErr)
			muted(c, "Izin yang sudah ada tetap dapat ditinjau dan dicabut di bawah.")
		}
		blocked := a.archiveWriteBlocked()
		ui.Text(c, "Anggota penerima").FontSize(12).Bold()
		userLabel := "Pilih anggota penerima"
		if state.user != id.Nil {
			userLabel = a.archiveUserName(state.user)
		}
		userButton := spaceButton(c, userLabel, "user", state.user != id.Nil).FillWidth().Disabled(blocked)
		if userButton.Clicked() {
			state.userOpen = !state.userOpen
		}
		popover(c, userButton, &state.userOpen, 320, func() {
			ui.Text(c, "Pilih akun anggota").Bold()
			if len(state.members) == 0 {
				muted(c, "Belum ada anggota aktif pada divisi ini.")
			}
			ui.Scroll(c).FillWidth().MaxHeight(230).Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
					for _, m := range state.members {
						if spaceButton(c.Key(m.UserID), m.DisplayName, "user", state.user == m.UserID).FillWidth().Label("Pilih penerima arsip " + m.DisplayName).Disabled(blocked).Clicked() {
							state.user, state.userOpen = m.UserID, false
						}
					}
				})
			})
		})
		ui.Text(c, "Space periode lama").FontSize(12).Bold()
		spaceLabel := "Pilih Space arsip"
		if state.space != id.Nil {
			spaceLabel = a.archiveSpaceName(state.space)
		}
		spaceButtonElement := spaceButton(c, spaceLabel, "archive", state.space != id.Nil).FillWidth().Disabled(blocked || !state.targetsLoaded)
		if spaceButtonElement.Clicked() {
			state.spaceOpen = !state.spaceOpen
		}
		popover(c, spaceButtonElement, &state.spaceOpen, 340, func() {
			ui.Text(c, "Pilih satu Space arsip").Bold()
			if len(state.targets) == 0 {
				muted(c, "Belum ada Space dari periode tertutup yang dapat diberi izin oleh divisi ini.")
			}
			ui.Scroll(c).FillWidth().MaxHeight(240).Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
					for _, target := range state.targets {
						choice := ui.ButtonBase(c.Key(target.SpaceID)).Label("Pilih arsip " + target.PeriodLabel + " " + target.SpaceName).Column().AlignItems(ui.Start).FillWidth().MinWidth(0).Padding(10).Gap(4).Radius(7).Disabled(blocked)
						if choice.Hovered() {
							choice.Background(c.Theme().SurfaceHover)
						}
						choice.FocusRing(choice.FocusVisible()).Children(func() {
							ui.Text(c, target.SpaceName).FontSize(13)
							muted(c, target.PeriodLabel).FontSize(11)
						})
						if choice.Clicked() {
							state.space, state.spaceOpen = target.SpaceID, false
						}
					}
				})
			})
		})
		duplicate := false
		for _, grant := range state.grants {
			if grant.UserID == state.user && grant.SpaceID == state.space {
				duplicate = true
				break
			}
		}
		if duplicate {
			muted(c, "Anggota ini sudah memiliki izin dari divisi ini untuk Space yang dipilih.")
		}
		if spaceButton(c, "Berikan izin baca arsip", "check", true).FillWidth().Disabled(blocked || !state.targetsLoaded || state.user == id.Nil || state.space == id.Nil || duplicate).Clicked() {
			a.grantArchive()
		}
	})
}

func (a *App) archiveExistingGrants(c *ui.Context) {
	state := &a.archives
	ui.Text(c, "Izin yang diberikan divisi ini").FontSize(16).Bold()
	if len(state.grants) == 0 {
		pageEmpty(c, "archive", "Belum ada izin arsip", "Anggota periode baru belum mendapat izin tambahan dari divisi ini.")
		return
	}
	for _, grant := range state.grants {
		pageSection(c.Key(grant.ID), a.archiveUserName(grant.UserID), a.archiveSpaceName(grant.SpaceID), func() {
			muted(c, "Hanya baca · izin diberikan untuk satu Space.")
			if !grant.CreatedAt.IsZero() {
				zone := "Asia/Jakarta"
				if p := a.period(); p != nil && p.TimeZone != "" {
					zone = p.TimeZone
				}
				muted(c, "Diberikan "+taskTimestamp(grant.CreatedAt, zone)).FontSize(11)
			}
			if state.confirmGrant == grant.ID {
				confirmation := ui.Column(c.Key("archive-revoke-confirmation")).FillWidth().MinWidth(0).Gap(10).Label("Konfirmasi pencabutan arsip")
				confirmation.Children(func() {
					pageNotice(c, "alert", "Cabut izin arsip ini?", "Pencabutan menghapus izin tambahan dari divisi ini. Hak baca dari keanggotaan lama atau izin lain yang masih berlaku tetap dipertahankan.")
					if spaceButton(c, "Konfirmasi cabut izin arsip", "unlink", true).FillWidth().Disabled(a.archiveWriteBlocked()).Clicked() {
						a.revokeArchive()
					}
					if spaceButton(c, "Batal cabut izin arsip", "close", false).FillWidth().Disabled(state.busy).Clicked() {
						state.confirmGrant = id.Nil
					}
				})
				if state.revealRevoke {
					confirmation.ScrollIntoView()
					state.revealRevoke = false
				}
			} else if spaceButton(c, "Cabut izin arsip", "unlink", false).FillWidth().Disabled(a.archiveWriteBlocked()).Clicked() {
				a.requestArchiveRevoke(grant.ID)
			}
		})
	}
}
