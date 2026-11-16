package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestMonitorRowLabels(t *testing.T) {
	if got := monitorSOP(api.MonitorSpaceSummary{}); got != "Belum ada" {
		t.Errorf("no findings: %q", got)
	}
	if got := monitorSOP(api.MonitorSpaceSummary{SOPFindings: 4, SOPNeedsReview: 1}); got != "4 temuan, 1 perlu tinjau" {
		t.Errorf("findings: %q", got)
	}
	if got := monitorStatus(api.MonitorSpaceSummary{Running: 3, Overdue: 2}); got != "2 terlambat" {
		t.Errorf("late: %q", got)
	}
	if got := monitorStatus(api.MonitorSpaceSummary{Running: 3}); got != "tepat waktu" {
		t.Errorf("on time: %q", got)
	}
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local)
	if got := monitorDate(api.MonitorSpaceSummary{LastMeeting: "Rapat divisi", LastMeetingAt: &at}); got != "Rapat divisi · 10 Nov" {
		t.Errorf("meeting: %q", got)
	}
}

// Spaces a monitor opened stay in the list across a refresh of the own
// spaces, and are dropped once the period no longer grants org.monitor.
func TestMonitoredSpacesKeptAcrossRefresh(t *testing.T) {
	p := id.New()
	a := &App{periodID: p, periods: []api.Period{{ID: p, Status: api.PeriodActive, Monitor: true}}}
	a.monitoring.period = p
	opened := api.Space{ID: id.New(), PeriodID: p, Name: "Mediatek", Access: api.AccessMonitor}
	a.monitoring.opened = []api.Space{opened}
	own := []api.Space{{ID: id.New(), PeriodID: p, Name: "BPH", Access: api.AccessMember}}
	if got := a.withMonitoredSpaces(own); len(got) != 2 || got[1].ID != opened.ID {
		t.Fatalf("kept %+v", got)
	}
	a.periods[0].Monitor = false
	if got := a.withMonitoredSpaces(own); len(got) != 1 || a.monitoring.opened != nil {
		t.Fatalf("revoked monitor still keeps %+v", got)
	}
}

func TestMonitorMeetingFilter(t *testing.T) {
	bph := api.MonitorMeeting{Kind: api.MeetingBPH}
	div := api.MonitorMeeting{Kind: api.MeetingDivision}
	medhum := api.MonitorMeeting{Kind: api.MeetingMedhum}
	if !monitorMeetingMatches(bph, "") || !monitorMeetingMatches(bph, "bph") || monitorMeetingMatches(bph, "divisi") {
		t.Error("bph filter")
	}
	if !monitorMeetingMatches(div, "divisi") || !monitorMeetingMatches(medhum, "divisi") || monitorMeetingMatches(div, "pengurus") {
		t.Error("divisi filter")
	}
	if meetingKindLabel(api.MeetingPengurus) != "Rapat pengurus" || meetingKindLabel("") != "Rapat divisi" {
		t.Error("labels")
	}
}
