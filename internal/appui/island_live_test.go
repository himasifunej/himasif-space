package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The island shows a meeting's shared timer as the note shows it: time
// left of the agenda item, over it, or the time run without an agenda.
func TestIslandTimerClock(t *testing.T) {
	t0 := time.Date(2026, 11, 11, 14, 0, 0, 0, time.UTC)
	agenda := []noteAgendaItem{{"Evaluasi", 15}, {"Konten", 20}}
	var it IslandTimer
	islandTimerClock(&it, noteTimerState(noteTimer{started: t0}, t0.Add(4*time.Minute+30*time.Second), agenda))
	if it.Clock != "10:30" || !it.Countdown || it.Over || it.Agenda != "Evaluasi" {
		t.Fatalf("countdown: %+v", it)
	}
	it = IslandTimer{}
	islandTimerClock(&it, noteTimerState(noteTimer{started: t0}, t0.Add(17*time.Minute), agenda))
	if it.Clock != "+02:00" || !it.Over {
		t.Fatalf("over: %+v", it)
	}
	it = IslandTimer{}
	paused := t0.Add(5 * time.Minute)
	islandTimerClock(&it, noteTimerState(noteTimer{started: t0, pausedAt: paused}, t0.Add(time.Hour), nil))
	if it.Clock != "05:00" || it.Countdown || !it.Paused {
		t.Fatalf("elapsed, paused: %+v", it)
	}
}

// A loaded meeting's shared timer shows when no note is open; one left
// running for half a day, or a cancelled meeting's, does not.
func TestIslandTimerFromMeetings(t *testing.T) {
	now := time.Now()
	a := &App{screen: screenShell}
	a.meetings.items = []api.Meeting{
		{ID: id.New(), Title: "Lama", Timer: &api.MeetingTimer{StartedAt: now.Add(-13 * time.Hour)}},
		{ID: id.New(), Title: "Batal", Cancelled: true, Timer: &api.MeetingTimer{StartedAt: now.Add(-time.Minute)}},
		{ID: id.New(), Title: "Rapat Divisi", CanRunTimer: true, Timer: &api.MeetingTimer{StartedAt: now.Add(-2 * time.Minute)}},
	}
	it, ok := a.IslandTimer(now)
	if !ok || it.Title != "Rapat Divisi" || !it.Control || it.Clock != "02:00" {
		t.Fatalf("got %+v %v", it, ok)
	}
	a.screen = screenLogin
	if _, ok := a.IslandTimer(now); ok {
		t.Fatal("a signed-out app shows a timer")
	}
}
