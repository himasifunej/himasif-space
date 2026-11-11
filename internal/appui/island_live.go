package appui

import (
	"fmt"
	"strings"
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

// IslandOpenTimer shows what the island's timer runs for: a shared
// timer's meeting, on its Space's Rapat tab, or the open note's.
func (a *App) IslandOpenTimer(t IslandTimer) {
	if a.screen != screenShell {
		return
	}
	if t.Meeting == id.Nil {
		// The note is open, its timer with it: show it, without reopening
		// it, which would close the timer's note first.
		if a.notes.note != nil {
			a.navigate(PathSpaces)
			a.chrome.tab = "Catatan"
		}
		return
	}
	for _, m := range a.meetings.items {
		if m.ID == t.Meeting {
			a.selectSpace(m.SpaceID)
			a.navigate(PathSpaces)
			a.chrome.tab = "Rapat"
			a.ensureMeetings()
			a.selectMeeting(m.ID)
			return
		}
	}
	// Not loaded here: as a meeting's notification opens it.
	a.OpenNotification(api.Delivery{SourceKind: "meeting", SourceID: t.Meeting})
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

// SpaceName is a Space's name as the user knows it, or "" (UI thread).
func (a *App) SpaceName(sid id.ID) string {
	for _, sp := range a.access.spaces {
		if sp.ID == sid {
			return sp.Name
		}
	}
	return ""
}

// serverDateLayout is how servers before 0.4.5 wrote a meeting's time in
// a notice: "05 Dec 2026 08:00 WIB".
const serverDateLayout = "02 Jan 2006 15:04 MST"

// NoticeDate writes a notice's body in the app's Indonesian form when it
// is only a date as older servers wrote it: "5 Des 2026 · 08.00 WIB".
// Any other body is kept as it is.
func NoticeDate(body string) string {
	t, err := time.Parse(serverDateLayout, strings.TrimSpace(body))
	if err != nil {
		return body
	}
	f := strings.Fields(body)
	return IndonesianDateTime(t, f[len(f)-1])
}

// IndonesianDateTime is "5 Des 2026 · 08.00 WIB", the zone as given.
func IndonesianDateTime(t time.Time, zone string) string {
	return fmt.Sprintf("%d %s %d · %02d.%02d %s", t.Day(), noteMonths[t.Month()-1], t.Year(), t.Hour(), t.Minute(), zone)
}

// DeliveryKind is what a delivery is about, for its label and grouping:
// a meeting's change comes as a ping related to the meeting.
func DeliveryKind(d api.Delivery) string {
	// A ping someone sent ("Ping · Mediatek") is a ping, even about a task
	// or a meeting: its icon is the bell, not the task's.
	if d.SourceKind == "ping" && strings.HasPrefix(d.Title, "Ping · ") {
		return "ping"
	}
	if d.RelatedKind != "" {
		return d.RelatedKind
	}
	return d.SourceKind
}

// DeliveryGroup is the burst a delivery belongs to, and its label: the
// same kind, the same headline ("Rapat baru: …"), the same Space. A
// delivery without a headline belongs to none.
func DeliveryGroup(d api.Delivery) (group, label string) {
	head, _, ok := strings.Cut(d.Title, ": ")
	if !ok || head == "" || len([]rune(head)) > 40 {
		return "", ""
	}
	space := ""
	if d.SpaceID != nil {
		space = d.SpaceID.String()
	}
	return DeliveryKind(d) + "|" + head + "|" + space, strings.ToLower(head)
}
