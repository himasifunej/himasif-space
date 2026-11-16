package appui

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// monitorMeetingsAPI is the cross-division timeline, apart from MonitorAPI
// so test fakes of the dashboard need not serve it.
type monitorMeetingsAPI interface {
	MonitorMeetings(context.Context, id.ID) ([]api.MonitorMeeting, error)
}

var _ monitorMeetingsAPI = (*client.Client)(nil)

func (a *App) loadMonitorMeetings() {
	var s monitorMeetingsAPI
	if c, ok := a.api.(*client.Client); ok {
		s = c.AccountSession()
	} else if f, ok := a.api.(monitorMeetingsAPI); ok {
		s = f
	}
	if s == nil {
		a.monitoring.meetingsErr = "Timeline rapat belum tersedia."
		return
	}
	if !a.isMonitor() || a.monitoring.meetingsLoading {
		return
	}
	gen, seq, p := a.gen, a.monitoring.seq, a.periodID
	a.monitoring.meetingsLoading, a.monitoring.meetingsErr = true, ""
	a.async(func() func() {
		items, err := s.MonitorMeetings(context.Background(), p)
		return func() {
			if gen != a.gen || seq != a.monitoring.seq || p != a.periodID {
				return
			}
			a.monitoring.meetingsLoading = false
			if err != nil {
				a.monitoring.meetingsErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.monitoring.meetings, a.monitoring.meetingsLoaded = items, true
		}
	})
}

// meetingKindLabel names a kind of meeting (MTG-06).
func meetingKindLabel(kind string) string {
	switch kind {
	case api.MeetingBPH:
		return "Rapat BPH"
	case api.MeetingPengurus:
		return "Rapat pengurus"
	case api.MeetingMedhum:
		return "Rapat Medhum"
	}
	return "Rapat divisi"
}

// monitorMeetingMatches applies the timeline's filter.
func monitorMeetingMatches(m api.MonitorMeeting, filter string) bool {
	switch filter {
	case "bph":
		return m.Kind == api.MeetingBPH
	case "pengurus":
		return m.Kind == api.MeetingPengurus
	case "divisi":
		return m.Kind == api.MeetingDivision || m.Kind == api.MeetingMedhum || m.Kind == ""
	}
	return true
}

func isBPHSpace(s *api.Space) bool {
	return s != nil && s.Kind == api.SpaceDivision && slices.ContainsFunc(s.Divisions, func(d api.Division) bool { return d.Slug == "bph" })
}

// bphSpace returns the caller's own BPH space when they may hold a meeting
// there, nil otherwise.
func (a *App) bphSpace() *api.Space {
	for i := range a.access.spaces {
		s := &a.access.spaces[i]
		if s.Access == api.AccessMember && !s.ReadOnly && isBPHSpace(s) && slices.Contains(s.Capabilities, api.CapTasksManage) {
			return s
		}
	}
	return nil
}

// beginBPHMeeting opens a new Rapat BPH in BPH's own space.
func (a *App) beginBPHMeeting() {
	s := a.bphSpace()
	if s == nil {
		return
	}
	sid := s.ID
	a.selectSpace(sid)
	a.chrome.tab = "Rapat"
	a.meetings = meetingsState{space: sid}
	a.beginMeeting(nil)
	if a.meetings.draft != nil {
		a.meetings.draft.kind = api.MeetingBPH
	}
	a.navigate(PathSpaces)
}

// openMonitorMeeting opens a meeting of any division on its Notulen tab,
// through the space it belongs to.
func (a *App) openMonitorMeeting(m api.MonitorMeeting) {
	a.monitoring.pendingMeeting = m.ID
	a.monitoring.pendingSpace = m.SpaceID
	a.openMonitoredSpace(m.SpaceID)
}

// applyPendingMeeting selects the meeting a timeline row asked for, once
// its space is the selected one.
func (a *App) applyPendingMeeting() {
	if a.monitoring.pendingMeeting == id.Nil || a.selectedSpaceID() != a.monitoring.pendingSpace {
		return
	}
	mid, sid := a.monitoring.pendingMeeting, a.monitoring.pendingSpace
	a.monitoring.pendingMeeting, a.monitoring.pendingSpace = id.Nil, id.Nil
	a.chrome.tab = "Rapat"
	a.meetings = meetingsState{space: sid, selected: mid, tab: 2}
}

func dayShort(d time.Weekday) string {
	return [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}[d]
}

func timeZoneOrLocal(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return time.Local
}

// monitorMeetingsView is the canvas "Rapat lintas divisi".
func (a *App) monitorMeetingsView(c *ui.Context) {
	t := c.Theme()
	if !a.monitoring.meetingsLoaded && !a.monitoring.meetingsLoading && a.monitoring.meetingsErr == "" {
		a.loadMonitorMeetings()
	}
	muted(c, "Jadwal rapat semua divisi dalam satu garis waktu, empat minggu ke belakang sampai delapan minggu ke depan. Notulensi divisi lain dapat dibuka; pembukaannya tercatat di log admin.")
	ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		for _, f := range []struct{ key, label string }{{"", "Semua"}, {"bph", "Rapat BPH"}, {"pengurus", "Rapat pengurus"}, {"divisi", "Divisi"}} {
			if spaceButton(c.Key("filter-"+f.key), f.label, "filter", a.monitoring.meetingFilter == f.key).Clicked() {
				a.monitoring.meetingFilter = f.key
			}
		}
		ui.Box(c).Grow(1)
		if a.bphSpace() != nil && spaceButton(c, "Buat rapat BPH", "plus", true).Clicked() {
			a.beginBPHMeeting()
		}
		if spaceButton(c, "Muat ulang rapat", "refresh", false).Disabled(a.monitoring.meetingsLoading).Clicked() {
			a.monitoring.meetingsLoaded = false
			a.loadMonitorMeetings()
		}
	})
	if a.monitoring.meetingsErr != "" {
		pageNotice(c, "alert", "Timeline rapat belum dapat dimuat", a.monitoring.meetingsErr)
	}
	if !a.monitoring.meetingsLoaded {
		if a.monitoring.meetingsLoading {
			muted(c, "Memuat rapat semua divisi…")
		}
		return
	}
	now := c.Now()
	homePanel(c, "Rapat lintas divisi", fmt.Sprintf("%d rapat", len(a.monitoring.meetings)), 12, false, func() {
		shown := 0
		for _, m := range a.monitoring.meetings {
			if !monitorMeetingMatches(m, a.monitoring.meetingFilter) {
				continue
			}
			shown++
			at := m.StartsAt.In(timeZoneOrLocal(m.TimeZone))
			ui.Row(c.Key(m.ID)).FillWidth().MinWidth(0).Gap(14).Padding(10, 0).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Column(c).Width(96).Shrink(0).Gap(2).Children(func() {
					ui.Text(c, fmt.Sprintf("%s %d %s", dayShort(at.Weekday()), at.Day(), monthName(at.Month())[:3])).FontSize(12).Bold()
					muted(c, fmt.Sprintf("%02d.%02d", at.Hour(), at.Minute())).FontSize(12)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					title := m.Title
					if m.Cancelled {
						title += " (dibatalkan)"
					}
					ui.Text(c, title).FontSize(13).Bold().MaxLines(1)
					muted(c, meetingKindLabel(m.Kind)+" · "+m.SpaceName).FontSize(12).MaxLines(1)
				})
				status := "Belum ada notulen"
				switch {
				case m.NoteID != nil:
					status = "Notulen: " + m.NoteTitle
				case m.StartsAt.After(now):
					status = "Akan datang"
				}
				muted(c, status).FontSize(12).MaxLines(1).MinWidth(0)
				label := "Buka rapat " + m.Title
				if m.NoteID != nil {
					label = "Buka notulen " + m.Title
				}
				if spaceButton(c, label, "arrow", false).Disabled(a.monitoring.opening != id.Nil).Clicked() {
					a.openMonitorMeeting(m)
				}
			})
		}
		if shown == 0 {
			muted(c, "Tidak ada rapat untuk filter ini.")
		}
	})
}

// meetingKindField lets a meeting of BPH's space be a Rapat BPH or a Rapat
// pengurus (MTG-06); elsewhere the kind follows the space.
func (a *App) meetingKindField(c *ui.Context, d *meetingDraft) {
	if !isBPHSpace(a.selectedSpace()) {
		return
	}
	if d.kind == "" && d.id == id.Nil {
		d.kind = api.MeetingBPH
	}
	ui.Text(c, "Jenis rapat").Bold()
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, k := range []string{api.MeetingBPH, api.MeetingPengurus, api.MeetingDivision} {
			chosen := d.kind == k || d.kind == "" && k == api.MeetingDivision
			if spaceButton(c.Key("kind-"+k), meetingKindLabel(k), "meeting", chosen).Clicked() {
				d.kind = k
			}
		}
	})
	if d.kind == api.MeetingPengurus {
		muted(c, "Rapat pengurus dapat mengundang kadiv semua divisi. Undangan tidak membuka catatan BPH bagi mereka.")
	}
}
