package appui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// The Arsip page as on canvas A (#page-arsip): a card per closed period,
// opened read-only, and for the organization admin a "Serah terima" card
// leading to the period's handover checklist.
func (a *App) archivePage(c *ui.Context, r *ui.Route) {
	r.Title("Arsip periode")
	t := c.Theme()
	phone := isMobile(c)
	ui.Scroll(c).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(24).Gap(16)
		if phone {
			body.Padding(16).Gap(12)
		}
		body.Children(func() {
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			wide := head.Bounds().W >= 640
			if wide {
				head.Row().AlignItems(ui.Center)
			}
			head.Children(func() {
				titles := ui.Column(c).MinWidth(0).Gap(4)
				if wide {
					titles.Grow(1)
				}
				titles.Children(func() {
					ui.Text(c, "Arsip periode").FontSize(20).Bold()
					ui.Text(c, "Periode yang ditutup menjadi hanya-baca. Pengurus baru perlu izin kadiv periode aktif.").FontSize(13).TextColor(t.TextMuted)
				})
				if len(a.archiveManagedDivisions()) > 0 {
					if spaceButton(c, "Kelola izin arsip", "shield", false).Border(1, t.Border).Clicked() {
						a.navigate(PathArchiveAccess)
					}
				}
			})
			if a.periodsErr != "" {
				pageNotice(c, "alert", "Periode belum dapat dimuat", a.periodsErr)
				if spaceButton(c, "Muat ulang periode", "refresh", false).Disabled(a.periodsLoading).Clicked() {
					a.loadPeriods()
				}
			}
			var closed []api.Period
			for _, p := range a.periods {
				if p.Status == api.PeriodClosed {
					closed = append(closed, p)
				}
			}
			cards := len(closed)
			if a.isAdmin() {
				cards++
			}
			if len(closed) == 0 {
				cards++
			}
			grid := ui.Column(c).FillWidth().MinWidth(0).Gap(16)
			cols := 3
			switch w := grid.Bounds().W; {
			case w > 0 && w < 520:
				cols = 1
			case w > 0 && w < 820:
				cols = 2
			}
			grid.Children(func() {
				for row := 0; row*cols < cards; row++ {
					ui.Row(c.Key(row)).FillWidth().Gap(16).AlignItems(ui.Stretch).Children(func() {
						for k := 0; k < cols; k++ {
							cell := ui.Column(c.Key(k)).Grow(1).Basis(0).MinWidth(0)
							n := row*cols + k
							cell.Children(func() {
								switch {
								case n < len(closed):
									a.archivePeriodCard(c, closed[n])
								case n == len(closed) && len(closed) == 0 && n < cards:
									archiveCard(c, func() {
										title := "Belum ada periode ditutup"
										text := "Periode yang telah ditutup akan tercantum di sini."
										if a.periodsLoading {
											title, text = "Memuat periode…", "Mengambil daftar kepengurusan."
										}
										ui.Text(c, title).FontSize(14).Bold()
										ui.Text(c, text).FontSize(12).TextColor(t.TextMuted)
									})
								case n < cards && a.isAdmin():
									a.archiveHandoverCard(c)
								}
							})
						}
					})
				}
			})
		})
	})
}

func archiveCard(c *ui.Context, fn func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().FillHeight().MinWidth(0).Padding(16).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(fn)
}

func (a *App) archivePeriodCard(c *ui.Context, p api.Period) {
	t := c.Theme()
	archiveCard(c.Key(p.ID), func() {
		ui.Text(c, p.Label).FontSize(14).Bold()
		ui.Text(c, fmt.Sprintf("Ditutup %d %s %d · %s", p.EndsOn.Day, monthName(p.EndsOn.Month)[:3], p.EndsOn.Year, p.TimeZone)).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(7, 7).Radius(4).Background(faint(t))
			ui.Text(c, "Hanya baca · akses mengikuti izin arsip").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Box(c).Height(4)
		if spaceButton(c, "Buka (hanya baca)", "", false).Border(1, t.Border).Padding(6, 10).MinHeight(32).Label("Lihat ruang " + p.Label).Clicked() {
			a.changePeriod(p.ID)
			a.navigate(PathSpaces)
		}
	})
}

// archiveHandoverCard leads the organization admin to the handover
// checklist (Pengaturan organisasi → Serah terima), which already exists.
func (a *App) archiveHandoverCard(c *ui.Context) {
	t := c.Theme()
	archiveCard(c.Key("handover"), func() {
		ui.Text(c, "Serah terima").FontSize(14).Bold()
		ui.Text(c, "Catat pengurus baru, pengelola akun, bot, rencana izin arsip, dan pekerjaan tersisa sebelum periode ditutup.").FontSize(12).TextColor(t.TextMuted)
		ui.Box(c).Height(4)
		if spaceButton(c, "Siapkan serah terima", "", false).Border(1, t.Border).Padding(6, 10).MinHeight(32).Clicked() {
			a.cancelOrganizationConfirmation()
			a.organization.tab = "Serah terima"
			a.navigate(PathOrganisasi)
		}
	})
}
