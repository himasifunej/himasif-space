package appui

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// IslandTimer is the meeting timer the desktop's island shows live (Ace
// island v2): the open note's, which knows its agenda, else a shared timer
// of a meeting the app has loaded. The island reads it on the UI thread.
type IslandTimer struct {
	Meeting   id.ID // id.Nil: a note's timer on this device
	Title     string
	Agenda    string // the current agenda item, if any
	Clock     string // time left of the item, "+mm:ss" over it, or time run
	Countdown bool
	Over      bool
	Paused    bool
	Control   bool // may pause and resume it
}

// islandTimerStale: a shared timer left running is not shown forever.
const islandTimerStale = 12 * time.Hour

func (a *App) IslandTimer(now time.Time) (IslandTimer, bool) {
	if a.screen != screenShell {
		return IslandTimer{}, false
	}
	if n := a.notes.note; n != nil {
		if ctl := a.noteTimerCtl(); ctl.tm != nil {
			t := IslandTimer{Title: n.Title, Control: ctl.control}
			if ctl.meeting != nil {
				t.Meeting, t.Title = ctl.meeting.ID, ctl.meeting.Title
			}
			islandTimerClock(&t, noteTimerState(*ctl.tm, now, a.noteAgendaItems()))
			return t, true
		}
	}
	for _, m := range a.meetings.items {
		if m.Timer == nil || m.Cancelled {
			continue
		}
		tm := sharedNoteTimer(*m.Timer, a.meetings.skew)
		if now.Sub(tm.started) > islandTimerStale {
			continue
		}
		t := IslandTimer{Meeting: m.ID, Title: m.Title, Control: m.CanRunTimer}
		islandTimerClock(&t, noteTimerState(tm, now, nil))
		return t, true
	}
	return IslandTimer{}, false
}

func islandTimerClock(t *IslandTimer, v noteTimerView) {
	t.Paused = v.paused
	t.Clock = noteClock(v.elapsed)
	if v.agenda < 0 {
		return
	}
	t.Agenda, t.Countdown = v.title, true
	t.Clock = noteLeft(v.left)
	if v.over > 0 {
		t.Over, t.Clock = true, "+"+noteClock(v.over)
	}
}

// IslandTimerAct pauses or resumes the timer the island shows
// (api.TimerPause, api.TimerResume), as the note's timer buttons do.
func (a *App) IslandTimerAct(meeting id.ID, action string) {
	if action != api.TimerPause && action != api.TimerResume {
		return
	}
	if meeting == id.Nil {
		if a.notes.note != nil {
			a.noteTimerAct(noteTimerCtl{tm: a.noteTimer(), control: true}, action, time.Now())
		}
		return
	}
	for _, m := range a.meetings.items {
		if m.ID == meeting && m.CanRunTimer && m.Timer != nil {
			a.runMeetingTimer(m.ID, m.TimerVersion, action)
		}
	}
}

// IslandToday is the line the island's details show at rest: the next
// meeting today, as "Rapat Divisi · 14.00", or "".
func (a *App) IslandToday(now time.Time) string {
	if a.screen != screenShell {
		return ""
	}
	var next *api.Meeting
	y, mo, d := now.Date()
	for i := range a.meetings.items {
		m := &a.meetings.items[i]
		at := m.StartsAt.In(now.Location())
		if m.Cancelled || at.Before(now.Add(-15*time.Minute)) {
			continue
		}
		if ay, am, ad := at.Date(); ay != y || am != mo || ad != d {
			continue
		}
		if next == nil || m.StartsAt.Before(next.StartsAt) {
			next = m
		}
	}
	if next == nil {
		return ""
	}
	return next.Title + " · " + next.StartsAt.In(now.Location()).Format("15.04")
}
