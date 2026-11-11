package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Stage C2: the note as one document (canvas 8eef2f092aab, designs 1 and
// 2): a document bar, a big title, the meeting's properties and a 720 px
// column. On a phone the properties fold into one line.

const noteColumnWidth = 720

var noteMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func noteZone(name string) *time.Location {
	if loc, err := timeutil.LoadZone(name); err == nil && loc != nil {
		return loc
	}
	return time.UTC
}

// noteRelative is the document bar's time: "baru saja", "12 menit lalu",
// "pukul 19.42", "kemarin 19.42", "9 Nov 19.42".
func noteRelative(t, now time.Time, zone string) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d/time.Minute))
	}
	loc := noteZone(zone)
	lt, ln := t.In(loc), now.In(loc)
	clock := lt.Format("15.04")
	y1, m1, d1 := lt.Date()
	y2, m2, d2 := ln.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return "pukul " + clock
	}
	if y, m, dd := ln.AddDate(0, 0, -1).Date(); y == y1 && m == m1 && dd == d1 {
		return "kemarin " + clock
	}
	return fmt.Sprintf("%d %s %s", d1, noteMonths[m1-1], clock)
}

// noteSaveState is the document bar's status: what autosave is doing for
// the notulis, or when the official copy last changed for a reader.
func (a *App) noteSaveState(now time.Time) (label, icon string, warn bool) {
	n := a.notes.note
	if n == nil {
		return "", "", false
	}
	zone := a.noteViewZone()
	if a.notes.editing || a.notes.dirty {
		switch {
		case a.notes.saving:
			return "Menyimpan…", "refresh", false
		case a.notes.saveErr != "":
			return "Belum tersimpan — coba lagi", "alert", true
		case a.notes.dirty && !a.noteAutosaveActive():
			return "Perubahan belum tersimpan", "alert", true
		case a.notes.dirty:
			return "Menyimpan otomatis…", "clock", false
		case !a.notes.savedAt.IsZero():
			return "Tersimpan · " + noteRelative(a.notes.savedAt, now, zone), "check", false
		}
	}
	if !a.notes.remoteAt.IsZero() && now.Sub(a.notes.remoteAt) < 10*time.Second {
		label = "Diperbarui " + noteZoneClock(a.notes.remoteAt, zone)
		if a.notes.remoteBy != "" {
			label += " oleh " + firstName(a.notes.remoteBy)
		}
		return label, "refresh", false
	}
	return "Diedit " + noteRelative(n.UpdatedAt, now, zone), "clock", false
}

func noteZoneClock(t time.Time, zone string) string { return t.In(noteZone(zone)).Format("15.04") }

// noteLiveEditor is the person holding the editor right now, if any.
func (a *App) noteLiveEditor(now time.Time) string {
	n := a.notes.note
	if n == nil || n.Lock == nil || !now.Before(n.Lock.ExpiresAt) {
		return ""
	}
	return n.Lock.Editor.DisplayName
}

// noteMeeting is the meeting a note belongs to, from the Agenda list the
// app already keeps; loading reports that list is still on its way.
func (a *App) noteMeeting() (m *api.Meeting, loading bool) {
	n := a.notes.note
	if n == nil || n.MeetingID == nil {
		return nil, false
	}
	a.ensureMeetings()
	if a.meetings.space != a.notes.space || !a.meetings.loaded {
		return nil, a.meetings.loading || a.meetings.err == ""
	}
	for i := range a.meetings.items {
		if a.meetings.items[i].ID == *n.MeetingID {
			return &a.meetings.items[i], false
		}
	}
	return nil, false
}

type noteAttendance struct{ present, excused, absent, unrecorded int }

func meetingAttendance(m *api.Meeting) noteAttendance {
	var out noteAttendance
	for _, p := range m.Participants {
		switch p.AttendanceStatus {
		case "present":
			out.present++
		case "excused":
			out.excused++
		case "absent":
			out.absent++
		default:
			out.unrecorded++
		}
	}
	return out
}

func (n noteAttendance) parts() []string {
	var out []string
	for _, p := range []struct {
		n     int
		label string
	}{{n.present, "hadir"}, {n.excused, "izin"}, {n.absent, "tidak hadir"}, {n.unrecorded, "belum dicatat"}} {
		if p.n > 0 {
			out = append(out, fmt.Sprintf("%d %s", p.n, p.label))
		}
	}
	return out
}

// noteMeetingWhen: "Senin, 9 Nov 2026 · 19.30–20.45" (long) or
// "Sen 9 Nov · 19.30" (short, the folded phone line).
func noteMeetingWhen(m *api.Meeting, short bool) string {
	loc := noteZone(m.TimeZone)
	s := m.StartsAt.In(loc)
	if short {
		return fmt.Sprintf("%s %d %s · %s", shortDays[s.Weekday()], s.Day(), noteMonths[s.Month()-1], s.Format("15.04"))
	}
	out := fmt.Sprintf("%s, %d %s %d · %s", longDays[s.Weekday()], s.Day(), noteMonths[s.Month()-1], s.Year(), s.Format("15.04"))
	if m.EndsAt != nil {
		e := m.EndsAt.In(loc)
		if e.YearDay() == s.YearDay() && e.Year() == s.Year() {
			out += "–" + e.Format("15.04")
		} else {
			out += fmt.Sprintf(" – %d %s %s", e.Day(), noteMonths[e.Month()-1], e.Format("15.04"))
		}
	}
	return out
}

func noteMeetingPlace(m *api.Meeting) string {
	place := strings.TrimSpace(m.Location)
	if strings.TrimSpace(m.JoinURL) != "" {
		if place == "" {
			return "Rapat daring"
		}
		place += " + rapat daring"
	}
	return place
}

func noteKindTag(kind string) string {
	switch kind {
	case api.TemplateMeeting:
		return "Notulen"
	case api.TemplateSchedule:
		return "Jadwal"
	}
	return ""
}

func (a *App) openNoteMeeting() {
	n := a.notes.note
	if n == nil || n.MeetingID == nil {
		return
	}
	a.chrome.tab = "Rapat"
	a.ensureMeetings()
	a.selectMeeting(*n.MeetingID)
}

// noteTitleView is the big title: an input without a frame for the
// notulis, plain text for readers.
func (a *App) noteTitleView(c *ui.Context, phone bool) {
	if a.notes.note == nil { // closed earlier in this frame
		return
	}
	size := float32(30)
	if phone {
		size = 24
	}
	if a.notesCanEdit() {
		// A text area as high as the wrapped title (1 to 6 lines), so a long
		// title wraps on a phone instead of being cut. Enter adds no line:
		// a title is one paragraph.
		in := ui.TextAreaBase(c.Key("note-title"), &a.notes.title).Lines(1, 6).Label("Judul catatan").Placeholder("Tanpa judul").FillWidth().MinWidth(0).FontSize(size).Bold().
			OnTextKey(func(k ui.TextKey) bool { return k.Key == ui.KeyEnter })
		if in.Changed() {
			a.markNoteChanged()
		}
		return
	}
	ui.Text(c.Key("note-title-text"), a.notes.title).Label("Judul catatan").FontSize(size).Bold().FillWidth().MinWidth(0).Selectable()
}

// noteHistoryEntry is one line of the history: autosaves by the same person
// within noteHistoryMergeGap of each other show as one entry. The server's
// note_events stay one row per save (append-only).
type noteHistoryEntry struct {
	api.NoteEvent
	count int
	first time.Time
}

const noteHistoryMergeGap = 5 * time.Minute

// groupNoteHistory merges runs of note.update by one actor; events come
// newest first, as the server sends them.
func groupNoteHistory(events []api.NoteEvent) []noteHistoryEntry {
	var out []noteHistoryEntry
	for _, e := range events {
		if k := len(out) - 1; k >= 0 && e.Action == "note.update" && out[k].Action == "note.update" && out[k].Actor.ID == e.Actor.ID && out[k].first.Sub(e.CreatedAt) <= noteHistoryMergeGap {
			out[k].count++
			out[k].first = e.CreatedAt
			continue
		}
		out = append(out, noteHistoryEntry{NoteEvent: e, count: 1, first: e.CreatedAt})
	}
	return out
}

// noteTitleText is the title as saved: one line (a pasted line break
// becomes a space), trimmed.
func noteTitleText(s string) string {
	return strings.TrimSpace(strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }), " "))
}

// noteAttendanceLines fills a "Kehadiran" block from the note's meeting:
// "Demo Mediatek — hadir", "Laras Foto — izin (kuliah pengganti)".
func (a *App) noteAttendanceLines() []string {
	m, _ := a.noteMeeting()
	if m == nil {
		return nil
	}
	out := []string{}
	for _, p := range m.Participants {
		status := map[string]string{"present": "hadir", "excused": "izin", "absent": "tidak hadir"}[p.AttendanceStatus]
		if status == "" {
			status = "belum dicatat"
		}
		if note := strings.TrimSpace(p.AttendanceNote); note != "" {
			status += " (" + note + ")"
		}
		out = append(out, p.DisplayName+" — "+status)
	}
	return out
}
