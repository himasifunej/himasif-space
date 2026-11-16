package appui

import (
	"context"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// PathPemantauan is the monitoring dashboard of BPH (OPEN-15).
const PathPemantauan = "/pemantauan"

// MonitorAPI is a monitor's side of org.monitor: the dashboard, and the
// spaces it opens. The server decides what they show.
type MonitorAPI interface {
	MonitorOverview(context.Context, id.ID) (api.MonitorOverview, error)
	SpaceView(context.Context, id.ID) (api.Space, error)
}

var _ MonitorAPI = (*client.Client)(nil)

type monitoringUIState struct {
	seq             uint64
	period          id.ID
	loaded, loading bool
	err             string
	overview        api.MonitorOverview
	opening         id.ID
	// opened are the spaces a monitor opened; they stay in the space list
	// across its refreshes until the period changes.
	opened []api.Space
	// tab is the page's tab: "" for the summary, "Rapat" for the
	// cross-division meeting timeline (canvas "Rapat lintas divisi").
	tab                             string
	meetings                        []api.MonitorMeeting
	meetingsLoaded, meetingsLoading bool
	meetingsErr, meetingFilter      string
	// pendingMeeting is the meeting a timeline row opens once its space
	// (pendingSpace) is selected.
	pendingMeeting, pendingSpace id.ID
}

func (a *App) monitorService() (MonitorAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(MonitorAPI)
	return s, ok
}

// isMonitor reports whether the account holds org.monitor in the period.
func (a *App) isMonitor() bool {
	p := a.period()
	return p != nil && p.Monitor && !a.isAdmin()
}

func (a *App) loadMonitoring() {
	s, ok := a.monitorService()
	if !ok {
		a.monitoring.err = "Layanan pemantauan belum tersedia."
		return
	}
	if !a.isMonitor() || a.monitoring.loading {
		return
	}
	a.monitoring.seq++
	gen, seq, p := a.gen, a.monitoring.seq, a.periodID
	a.monitoring.period, a.monitoring.loading, a.monitoring.err = p, true, ""
	a.async(func() func() {
		o, err := s.MonitorOverview(context.Background(), p)
		return func() {
			if gen != a.gen || seq != a.monitoring.seq || p != a.periodID {
				return
			}
			a.monitoring.loading = false
			if err != nil {
				a.monitoring.err = a.failed(err)
				return
			}
			a.conn = connOnline
			a.monitoring.overview, a.monitoring.loaded = o, true
		}
	})
}

// openMonitoredSpace opens a division read-mostly: the server's view of the
// space with the monitor's access joins the space list, and is selected.
func (a *App) openMonitoredSpace(sid id.ID) {
	if slices.ContainsFunc(a.access.spaces, func(s api.Space) bool { return s.ID == sid }) {
		a.selectSpace(sid)
		a.chrome.tab = "Beranda"
		a.applyPendingMeeting()
		a.navigate(PathSpaces)
		return
	}
	s, ok := a.monitorService()
	if !ok || a.monitoring.opening != id.Nil {
		return
	}
	gen, p := a.gen, a.periodID
	a.monitoring.opening, a.monitoring.err = sid, ""
	a.async(func() func() {
		sp, err := s.SpaceView(context.Background(), sid)
		return func() {
			if gen != a.gen || p != a.periodID {
				return
			}
			a.monitoring.opening = id.Nil
			if err != nil {
				a.monitoring.pendingMeeting, a.monitoring.pendingSpace = id.Nil, id.Nil
				a.monitoring.err = a.failed(err)
				return
			}
			a.conn = connOnline
			a.monitoring.opened = append(slices.DeleteFunc(a.monitoring.opened, func(s api.Space) bool { return s.ID == sp.ID }), sp)
			a.access.spaces = append(a.access.spaces, sp)
			a.selectSpace(sp.ID)
			a.chrome.tab = "Beranda"
			a.applyPendingMeeting()
			a.navigate(PathSpaces)
		}
	})
}

// withMonitoredSpaces keeps the spaces a monitor opened in a fresh list of
// the caller's own spaces, while the period still grants org.monitor.
func (a *App) withMonitoredSpaces(spaces []api.Space) []api.Space {
	if !a.isMonitor() || a.monitoring.period != a.periodID {
		a.monitoring.opened = nil
		return spaces
	}
	for _, sp := range a.monitoring.opened {
		if sp.PeriodID == a.periodID && !slices.ContainsFunc(spaces, func(s api.Space) bool { return s.ID == sp.ID }) {
			spaces = append(spaces, sp)
		}
	}
	return spaces
}

func monitorDate(at api.MonitorSpaceSummary) string {
	if at.LastMeetingAt == nil {
		return "Belum ada rapat"
	}
	t := at.LastMeetingAt.Local()
	return fmt.Sprintf("%s · %d %s", at.LastMeeting, t.Day(), monthName(t.Month())[:3])
}

func monitorSOP(s api.MonitorSpaceSummary) string {
	switch {
	case s.SOPFindings == 0:
		return "Belum ada"
	case s.SOPNeedsReview > 0:
		return fmt.Sprintf("%d temuan, %d perlu tinjau", s.SOPFindings, s.SOPNeedsReview)
	}
	return fmt.Sprintf("%d temuan", s.SOPFindings)
}

func monitorStatus(s api.MonitorSpaceSummary) string {
	switch {
	case s.Overdue > 0:
		return fmt.Sprintf("%d terlambat", s.Overdue)
	case s.Running == 0 && s.Done == 0:
		return "Belum ada tugas"
	}
	return "tepat waktu"
}

// monitoringPage is the canvas "Pemantauan organisasi": every division of
// the period, read-mostly, for holders of org.monitor.
func (a *App) monitoringPage(c *ui.Context, r *ui.Route) {
	r.Title("Pemantauan organisasi")
	if !a.isMonitor() {
		pageBody(c, "Pemantauan organisasi", func() {
			pageNotice(c, "lock", "Khusus pemegang izin pemantauan", "Admin organisasi memberi izin pemantauan kepada anggota BPH per akun, untuk satu periode.")
		})
		return
	}
	if a.monitoring.period != a.periodID {
		a.monitoring = monitoringUIState{seq: a.monitoring.seq + 1}
	}
	if !a.monitoring.loaded && !a.monitoring.loading && a.monitoring.err == "" {
		a.loadMonitoring()
	}
	t := c.Theme()
	pageBody(c, "Pemantauan organisasi", func() {
		label := "Semua divisi"
		if p := a.period(); p != nil {
			label += ", periode " + periodLabel(*p)
		}
		muted(c, label+". Tugas, rapat dan SOP dibaca lengkap; catatan privat dan reminder pribadi tetap tertutup. Divisi tidak diberi tahu saat dibuka, tetapi setiap pembukaan tercatat untuk admin.")
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).Padding(4, 10).Radius(999).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("eye")).Size(14, 14)
				ui.Text(c, "Hanya-baca, kecuali ping kadiv dan tinjauan bukti").FontSize(12)
			})
			if spaceButton(c, "Muat ulang pemantauan", "refresh", false).Disabled(a.monitoring.loading).Clicked() {
				a.monitoring.loaded = false
				a.loadMonitoring()
			}
		})
		ui.Row(c.Key("monitor-tabs")).Gap(6).Wrap().Children(func() {
			for _, tab := range []struct{ key, label, icon string }{{"", "Ringkasan divisi", "grid"}, {"Rapat", "Rapat lintas divisi", "meeting"}} {
				if spaceButton(c.Key("tab-"+tab.key), tab.label, tab.icon, a.monitoring.tab == tab.key).Clicked() {
					a.monitoring.tab = tab.key
				}
			}
		})
		if a.monitoring.err != "" {
			pageNotice(c, "alert", "Pemantauan belum dapat dimuat", a.monitoring.err)
		}
		if a.monitoring.tab == "Rapat" {
			a.monitorMeetingsView(c)
			return
		}
		if !a.monitoring.loaded {
			if a.monitoring.loading {
				muted(c, "Memuat ringkasan semua divisi…")
			}
			return
		}
		o := a.monitoring.overview
		phone := isMobile(c)
		cols := 4
		if phone || viewportMode(c) == layoutTablet {
			cols = 2
		}
		ui.Grid(c.Key("monitor-kpis")).FillWidth().Columns(cols).Gap(16).Children(func() {
			for _, k := range []struct {
				label, value, detail string
				color                ui.Color
			}{
				{"Tugas berjalan", fmt.Sprint(o.Totals.Running), fmt.Sprintf("di %d ruang", o.Totals.RunningSpaces), homeDoing},
				{"Terlambat", fmt.Sprint(o.Totals.Overdue), "lewat tenggat, belum diserahkan", t.Danger},
				{"Menunggu tinjauan", fmt.Sprint(o.Totals.AwaitingReview), "bukti diserahkan, belum dicocokkan", t.Warning},
				{"Rapat minggu ini", fmt.Sprint(o.Totals.MeetingsThisWeek), fmt.Sprintf("%d notulen belum ditautkan", o.Totals.MeetingsNoNotes), t.Success},
			} {
				ui.Column(c.Key(k.label)).MinWidth(0).Padding(16).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(4).Background(k.color)
						muted(c, k.label).FontSize(12)
					})
					ui.Text(c, k.value).FontSize(28).FontWeight(600)
					ui.Text(c, k.detail).FontSize(12).TextColor(faint(t)).MaxLines(2)
				})
			}
		})
		homePanel(c, "Divisi", fmt.Sprintf("beban berat: %d tugas berjalan atau lebih", o.HeavyLoad), 12, false, func() {
			for _, s := range o.Spaces {
				s := s
				row := ui.Column(c.Key(s.SpaceID)).FillWidth().MinWidth(0).Gap(8).Padding(10, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
				wide := row.Bounds().W >= 760
				if wide {
					row.Row().AlignItems(ui.Center).Gap(14)
				}
				row.Children(func() {
					name := s.Name
					if s.Kind == api.SpaceShared {
						name += " (bersama)"
					}
					head := ui.Column(c).MinWidth(0).Gap(3)
					if wide {
						head.Width(170).Shrink(0)
					}
					head.Children(func() {
						ui.Text(c, name).FontSize(13).Bold().MaxLines(1)
						muted(c, fmt.Sprintf("%d berjalan · %s", s.Running, monitorStatus(s))).FontSize(12)
					})
					ui.Row(c).Grow(1).MinWidth(80).Height(8).Radius(4).Background(t.SurfaceHover).Clip().Children(func() {
						if s.Progress > 0 {
							ui.Box(c).WidthPercent(float32(s.Progress)).FillHeight().Background(homeDoing)
						}
					})
					muted(c, fmt.Sprintf("%d%% · beban berat %d", s.Progress, s.Heavy)).FontSize(12).MaxLines(1)
					muted(c, monitorDate(s)).FontSize(12).MaxLines(1).MinWidth(0)
					muted(c, "SOP: "+monitorSOP(s)).FontSize(12).MaxLines(1).MinWidth(0)
					open := "Lihat " + s.Name
					if a.monitoring.opening == s.SpaceID {
						open = "Membuka…"
					}
					if spaceButton(c, open, "arrow", false).Disabled(a.monitoring.opening != id.Nil).Clicked() {
						a.openMonitoredSpace(s.SpaceID)
					}
				})
			}
		})
		homePanel(c, "Beban kerja per orang", "tugas berjalan, semua divisi", 12, false, func() {
			if len(o.Workload) == 0 {
				muted(c, "Belum ada tugas berjalan.")
			}
			most := 1
			for _, w := range o.Workload {
				most = max(most, w.Running)
			}
			for _, w := range o.Workload {
				ui.Row(c.Key(w.UserID)).FillWidth().MinWidth(0).Gap(12).Padding(6, 0).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Width(200).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
						avatar(c, w.DisplayName, 20)
						ui.Column(c).MinWidth(0).Gap(1).Children(func() {
							ui.Text(c, w.DisplayName).FontSize(13).MaxLines(1)
							muted(c, joinNames(w.Divisions)).FontSize(11).MaxLines(1)
						})
					})
					color := homeDoing
					if w.Running >= o.HeavyLoad {
						color = t.Warning
					}
					ui.Row(c).Grow(1).MinWidth(0).Height(12).Radius(6).Background(t.SurfaceHover).Clip().Children(func() {
						ui.Box(c).WidthPercent(100 * float32(w.Running) / float32(most)).FillHeight().Background(color)
					})
					ui.Text(c, fmt.Sprintf("%d tugas", w.Running)).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Width(60).Shrink(0)
				})
			}
		})
	})
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
