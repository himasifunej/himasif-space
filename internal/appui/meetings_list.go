package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Rapat A (pilot 0.4.4, owner's choice): the meetings of a Space as a list
// grouped Hari ini / Akan datang / Sudah lewat beside the chosen meeting's
// detail (Ringkasan · Kehadiran · Notulen). On a phone the list opens the
// detail as a page of its own. The helpers here hold the wording and the
// grouping, apart from the drawing.

// meetingGroup is where a meeting sits in the list.
type meetingGroup int

const (
	meetingToday meetingGroup = iota
	meetingUpcoming
	meetingPast
)

var meetingGroupNames = [...]string{"Hari ini", "Akan datang", "Sudah lewat"}

// meetingLoc is the meeting's time zone, the default zone when it has none
// this device knows.
func meetingLoc(m api.Meeting) *time.Location {
	loc, err := timeutil.LoadZone(m.TimeZone)
	if err != nil || loc == nil {
		loc, _ = timeutil.LoadZone("")
	}
	if loc == nil {
		loc = time.UTC
	}
	return loc
}

// meetingGroupOf puts a meeting by its day in its own zone: today's
// meetings stay under Hari ini all day, finished or not.
func meetingGroupOf(m api.Meeting, now time.Time) meetingGroup {
	loc := meetingLoc(m)
	day, today := timeutil.DateOf(m.StartsAt.In(loc)), timeutil.DateOf(now.In(loc))
	switch {
	case day == today, m.EndsAt != nil && now.Before(*m.EndsAt) && !now.Before(m.StartsAt):
		// A meeting going on past midnight stays under Hari ini.
		return meetingToday
	case m.StartsAt.After(now):
		return meetingUpcoming
	}
	return meetingPast
}

// meetingSection is one heading of the list and its meetings.
type meetingSection struct {
	group meetingGroup
	items []api.Meeting
}

// meetingSections groups the meetings: today and upcoming soonest first,
// the past newest first. Empty groups are left out.
func meetingSections(items []api.Meeting, now time.Time) []meetingSection {
	byGroup := [3][]api.Meeting{}
	for _, m := range items {
		g := meetingGroupOf(m, now)
		byGroup[g] = append(byGroup[g], m)
	}
	out := []meetingSection{}
	for g, list := range byGroup {
		if len(list) == 0 {
			continue
		}
		slices.SortStableFunc(list, func(a, b api.Meeting) int {
			if meetingGroup(g) == meetingPast {
				return b.StartsAt.Compare(a.StartsAt)
			}
			return a.StartsAt.Compare(b.StartsAt)
		})
		out = append(out, meetingSection{group: meetingGroup(g), items: list})
	}
	return out
}

// meetingDefault is the meeting the detail shows first: the next one today,
// else the next to come, else the latest that passed.
func meetingDefault(items []api.Meeting, now time.Time) id.ID {
	sections := meetingSections(items, now)
	for _, s := range sections {
		for _, m := range s.items {
			if !m.Cancelled {
				return m.ID
			}
		}
	}
	if len(sections) > 0 {
		return sections[0].items[0].ID
	}
	return id.Nil
}

// meetingClock is "19.30", the canvas' way of writing a time.
func meetingClock(t time.Time) string { return t.Format("15.04") }

// meetingDay is "Rab 11 Nov".
func meetingDay(t time.Time) string {
	return fmt.Sprintf("%s %d %s", shortDays[t.Weekday()], t.Day(), monthName(t.Month())[:3])
}

// meetingWhen is the Waktu of the detail: "Rab 11 Nov · 19.30–20.30 WIB",
// the end's day added when it is another, and without an end time
// "Rab 11 Nov · 19.30 WIB". (The old list wrote "— selesai" there, which
// read as if the meeting were over.)
func meetingWhen(m api.Meeting) string {
	loc := meetingLoc(m)
	start := m.StartsAt.In(loc)
	zone := start.Format("MST")
	if m.EndsAt == nil {
		return meetingDay(start) + " · " + meetingClock(start) + " " + zone
	}
	end := m.EndsAt.In(loc)
	if timeutil.DateOf(end) != timeutil.DateOf(start) {
		return meetingDay(start) + " · " + meetingClock(start) + " – " + meetingDay(end) + " " + meetingClock(end) + " " + zone
	}
	return meetingDay(start) + " · " + meetingClock(start) + "–" + meetingClock(end) + " " + zone
}

// meetingIn is "3 jam 10 menit lagi", "25 menit lagi", "2 hari lagi".
func meetingIn(d time.Duration) string {
	d = d.Round(time.Minute)
	switch {
	case d < time.Minute:
		return "sebentar lagi"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lagi", int(d.Minutes()))
	case d < 24*time.Hour:
		h, m := int(d.Hours()), int(d.Minutes())%60
		if m == 0 {
			return fmt.Sprintf("%d jam lagi", h)
		}
		return fmt.Sprintf("%d jam %d menit lagi", h, m)
	}
	return fmt.Sprintf("%d hari lagi", int(d.Hours()/24))
}

// meetingStatusKind colors the status line.
type meetingStatusKind int

const (
	statusSoon meetingStatusKind = iota
	statusLive
	statusDone
	statusCancelled
)

// meetingStatus is the line above the detail's title: "Hari ini · mulai
// 19.30 · 3 jam 10 menit lagi", "Berlangsung · timer bersama berjalan",
// "Sudah lewat · Kam 5 Nov", "Dibatalkan".
func meetingStatus(m api.Meeting, now time.Time) (string, meetingStatusKind) {
	loc := meetingLoc(m)
	start := m.StartsAt.In(loc)
	switch {
	case m.Cancelled:
		return "Dibatalkan · " + meetingDay(start), statusCancelled
	case m.Timer != nil:
		return "Berlangsung · timer bersama berjalan", statusLive
	}
	ended := m.EndsAt != nil && !now.Before(*m.EndsAt)
	switch meetingGroupOf(m, now) {
	case meetingToday:
		switch {
		case now.Before(m.StartsAt):
			return "Hari ini · mulai " + meetingClock(start) + " · " + meetingIn(m.StartsAt.Sub(now)), statusSoon
		case ended:
			return "Hari ini · selesai " + meetingClock(m.EndsAt.In(loc)), statusDone
		case m.EndsAt != nil:
			return "Berlangsung · sampai " + meetingClock(m.EndsAt.In(loc)), statusLive
		}
		return "Hari ini · mulai " + meetingClock(start) + " · jam selesai belum diisi", statusLive
	case meetingUpcoming:
		return "Akan datang · " + meetingDay(start) + " · " + meetingIn(m.StartsAt.Sub(now)), statusSoon
	}
	return "Sudah lewat · " + meetingDay(start), statusDone
}

// meetingReminders is the Pengingat: "H-1, 1 jam, dan 15 menit sebelum".
func meetingReminders(minutes []int) string {
	if len(minutes) == 0 {
		return "Tanpa pengingat"
	}
	sorted := slices.Clone(minutes)
	slices.SortFunc(sorted, func(a, b int) int { return b - a })
	parts := []string{}
	for _, v := range sorted {
		switch {
		case v > 0 && v%1440 == 0:
			parts = append(parts, fmt.Sprintf("H-%d", v/1440))
		case v > 0 && v%60 == 0:
			parts = append(parts, fmt.Sprintf("%d jam", v/60))
		default:
			parts = append(parts, fmt.Sprintf("%d menit", v))
		}
	}
	if len(parts) == 1 {
		return parts[0] + " sebelum"
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", dan " + parts[len(parts)-1] + " sebelum"
}

// meetingTally counts the attendance.
type meetingTally struct {
	present, excused, absent, unrecorded int
}

func meetingTallyOf(m api.Meeting) meetingTally {
	var t meetingTally
	for _, p := range m.Participants {
		switch p.AttendanceStatus {
		case "present":
			t.present++
		case "excused":
			t.excused++
		case "absent":
			t.absent++
		default:
			t.unrecorded++
		}
	}
	return t
}

// meetingStatusLabel names an attendance status.
func meetingStatusLabel(status string) string {
	switch status {
	case "present":
		return "Hadir"
	case "excused":
		return "Izin"
	case "absent":
		return "Tidak hadir"
	}
	return "Belum dicatat"
}

// meetingRowMeta is the second line of a list row: the time, the place
// and, for a meeting that passed, how many came ("Kehadiran belum dicatat"
// while nobody is recorded), otherwise how many are invited.
func meetingRowMeta(m api.Meeting, now time.Time) (meta string, warn string) {
	parts := []string{meetingClock(m.StartsAt.In(meetingLoc(m)))}
	if place := meetingPlace(m); place != "" {
		parts = append(parts, place)
	}
	n := len(m.Participants)
	t := meetingTallyOf(m)
	switch {
	case m.Cancelled:
	case meetingGroupOf(m, now) == meetingPast && n > 0 && t.unrecorded == n:
		warn = "Kehadiran belum dicatat"
	case !now.Before(m.StartsAt) && n > 0 && t.unrecorded < n:
		parts = append(parts, fmt.Sprintf("%d/%d hadir", t.present, n))
	case n > 0:
		parts = append(parts, fmt.Sprintf("%d peserta", n))
	}
	return strings.Join(parts, "  "), warn
}

// meetingPlace is where it is held: the place, or "Daring" for a meeting
// with only a link.
func meetingPlace(m api.Meeting) string {
	if s := strings.TrimSpace(m.Location); s != "" {
		return s
	}
	if strings.TrimSpace(m.JoinURL) != "" {
		return "Daring"
	}
	return ""
}

// meetingArrivalDefault is the arrival filled in when the organizer marks
// someone Hadir: the start, which the organizer can change. Before the
// start there is none: the server refuses an arrival in the future.
// now is the server's time (the device's plus meetingsState.skew).
func meetingArrivalDefault(m api.Meeting, now time.Time) (time.Time, bool) {
	if now.Before(m.StartsAt) {
		return time.Time{}, false
	}
	return m.StartsAt, true
}

// meetingRequestFrom is the meeting as an update request, for a change of
// one field (Batalkan / Aktifkan kembali).
func meetingRequestFrom(m api.Meeting) api.MeetingRequest {
	req := api.MeetingRequest{Title: m.Title, Description: m.Description, Location: m.Location, JoinURL: m.JoinURL, StartsAt: m.StartsAt, EndsAt: m.EndsAt, TimeZone: m.TimeZone, NoteID: m.NoteID, ReminderMinutes: slices.Clone(m.ReminderMinutes), Cancelled: m.Cancelled, ExpectedVersion: m.Version}
	if req.ReminderMinutes == nil {
		req.ReminderMinutes = []int{}
	}
	req.Participants = []id.ID{}
	for _, p := range m.Participants {
		req.Participants = append(req.Participants, p.UserID)
	}
	return req
}

// meetingInvites: the user organizes the meeting or is invited to it.
func meetingInvites(m api.Meeting, user id.ID) bool {
	return m.OrganizerID == user || slices.ContainsFunc(m.Participants, func(p api.MeetingParticipant) bool { return p.UserID == user })
}
