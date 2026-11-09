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

// noteDocBar is the bar above the document: where the note sits, its kind,
// the save state, who is writing, and privacy (display only until C6).
func (a *App) noteDocBar(c *ui.Context, phone bool) {
	t := c.Theme()
	n := a.notes.note
	now := c.Now()
	label, icon, warn := a.noteSaveState(now)
	status := func() {
		color := t.TextMuted
		if warn {
			color = t.Warning
		}
		ui.Row(c.Key("note-save-state")).Gap(6).AlignItems(ui.Center).MinWidth(0).Shrink(1).Tooltip(fmt.Sprintf("Isi resmi versi %d", n.Version)).Children(func() {
			ui.Icon(c, spaceIcon(icon)).Size(14, 14).Shrink(0).TextColor(color)
			ui.Text(c, label).FontSize(12).TextColor(color).MaxLines(1).MinWidth(0)
		})
		if a.notes.saveErr != "" && a.notesCanEdit() && !a.notes.saving {
			if outlineButton(c.Key("note-retry"), "Coba simpan lagi", "refresh", false).Height(28).Clicked() {
				a.commitNote(false)
			}
		}
	}
	privacy := func() {
		chip := ui.Row(c.Key("note-privacy")).Height(28).Padding(0, 10).Gap(6).AlignItems(ui.Center).Radius(8).Border(1, t.Border).Shrink(0).
			Tooltip("Catatan hanya terlihat oleh anggota divisi ini. Pengaturan berbagi menyusul.")
		chip.Children(func() {
			ui.Icon(c, spaceIcon("lock")).Size(13, 13).TextColor(t.TextMuted)
			if !phone {
				ui.Text(c, "Hanya divisi ini").FontSize(12).TextColor(t.TextMuted).MaxLines(1)
			}
		})
	}
	bar := ui.Row(c.Key("note-docbar")).FillWidth().MinWidth(0).Height(52).Padding(0, 16).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	bar.Children(func() {
		if phone {
			if spaceIconButton(c, "Kembali ke daftar catatan", "left").Disabled(a.notes.busy).Clicked() {
				a.closeNote()
			}
			ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(status)
			privacy()
			return
		}
		ui.Row(c.Key("note-crumbs")).Grow(1).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
			if n.MeetingID != nil {
				b := ui.ButtonBase(c.Key("crumb-meeting")).Label("Buka rapat terkait").Tooltip("Buka rapat terkait").Row().Gap(6).AlignItems(ui.Center).Padding(4, 6).Radius(6).TextColor(t.TextMuted).Shrink(0)
				if b.Hovered() {
					b.Background(t.SurfaceHover).TextColor(t.Text)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Icon(c, spaceIcon("calendar")).Size(14, 14)
					ui.Text(c, "Rapat").FontSize(13)
				})
				if b.Clicked() {
					a.openNoteMeeting()
				}
			} else {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
					ui.Icon(c, spaceIcon("note")).Size(14, 14).TextColor(t.TextMuted)
					ui.Text(c, "Catatan").FontSize(13).TextColor(t.TextMuted)
				})
			}
			ui.Icon(c, spaceIcon("chevron")).Size(12, 12).TextColor(faint(t)).Shrink(0)
			ui.Text(c, n.Title).FontSize(13).FontWeight(500).MaxLines(1).MinWidth(0).Shrink(1)
			if kind := noteKindTag(n.TemplateKind); kind != "" {
				ui.Box(c).Padding(2, 8).Radius(6).Background(spaceMix(t.Surface, t.Background, .5)).Border(1, t.Border).Shrink(0).Children(func() {
					ui.Text(c, kind).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).MinWidth(0).Shrink(1).Children(status)
		if editor := a.noteLiveEditor(now); editor != "" {
			avatar(c.Key("note-editor-avatar"), editor, 24).Tooltip(editor + " sedang mencatat")
		}
		privacy()
		if spaceIconButton(c, "Tutup catatan", "close").Disabled(a.notes.busy).Clicked() {
			a.closeNote()
		}
	})
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
	a.chrome.tab = "Agenda"
	a.ensureMeetings()
	a.meetings.selected = *n.MeetingID
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
		in := ui.TextInput(c.Key("note-title"), &a.notes.title).Label("Judul catatan").Placeholder("Tanpa judul").FillWidth().FontSize(size).Bold().Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(0)
		if in.Changed() {
			a.markNoteChanged()
		}
		return
	}
	ui.Text(c, a.notes.title).FontSize(size).Bold().FillWidth().MinWidth(0).Selectable()
}

// noteProperties lists the meeting's facts under the title. Desktop: a
// label column and values. Phone: one folded line that opens the same list.
func (a *App) noteProperties(c *ui.Context, phone bool) {
	t := c.Theme()
	n := a.notes.note
	if n == nil {
		return
	}
	m, loading := a.noteMeeting()
	if phone {
		summary := []string{}
		if m != nil {
			summary = append(summary, noteMeetingWhen(m, true))
			summary = append(summary, meetingAttendance(m).parts()...)
		} else if loading {
			summary = append(summary, "Memuat data rapat…")
		} else {
			summary = append(summary, "Dibuat "+noteRelative(n.CreatedAt, c.Now(), a.noteViewZone()))
		}
		b := ui.ButtonBase(c.Key("note-props-toggle")).Label("Properti catatan").Row().FillWidth().MinWidth(0).MinHeight(44).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(8).Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.FocusRing(b.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon("calendar")).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, strings.Join(summary, " · ")).FontSize(13).TextColor(t.TextMuted).MaxLines(1).Grow(1).MinWidth(0)
			chev := "down"
			if a.notes.propsOpen {
				chev = "chevron"
			}
			ui.Icon(c, spaceIcon(chev)).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
		})
		if b.Clicked() {
			a.notes.propsOpen = !a.notes.propsOpen
		}
		if !a.notes.propsOpen {
			return
		}
	}
	labelWidth := float32(132)
	if phone {
		labelWidth = 96
	}
	row := func(key, icon, label string, value func()) {
		ui.Row(c.Key(key)).FillWidth().MinWidth(0).MinHeight(32).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Row(c).Width(labelWidth).Shrink(0).Height(32).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon(icon)).Size(14, 14).TextColor(faint(t))
				ui.Text(c, label).FontSize(13).TextColor(faint(t))
			})
			ui.Row(c).Grow(1).MinWidth(0).MinHeight(32).Wrap().Gap(6).AlignItems(ui.Center).Children(value)
		})
	}
	text := func(s string) func() {
		return func() { ui.Text(c, s).FontSize(13).FillWidth().MinWidth(0) }
	}
	ui.Column(c.Key("note-props")).FillWidth().MinWidth(0).Gap(2).Children(func() {
		switch {
		case m != nil:
			row("when", "calendar", "Tanggal", text(noteMeetingWhen(m, false)))
			if place := noteMeetingPlace(m); place != "" {
				row("where", "globe", "Tempat", text(place))
			}
			row("who", "users", "Kehadiran", func() { a.noteAttendanceChips(c, m) })
		case loading:
			row("when", "calendar", "Rapat", text("Memuat data rapat…"))
		case n.MeetingID != nil:
			row("when", "calendar", "Rapat", text("Data rapat tidak tersedia untuk akunmu."))
		default:
			by := ""
			if n.CreatedBy.DisplayName != "" {
				by = " oleh " + n.CreatedBy.DisplayName
			}
			loc := noteZone(a.noteViewZone())
			ct := n.CreatedAt.In(loc)
			row("created", "calendar", "Dibuat", text(fmt.Sprintf("%s, %d %s %d%s", longDays[ct.Weekday()], ct.Day(), noteMonths[ct.Month()-1], ct.Year(), by)))
		}
	})
}

// noteAttendanceChips: avatar and first name per participant; anyone not
// present is faint, with the status (and the reason given) beside them.
func (a *App) noteAttendanceChips(c *ui.Context, m *api.Meeting) {
	t := c.Theme()
	if len(m.Participants) == 0 {
		ui.Text(c, "Belum ada peserta.").FontSize(13).TextColor(t.TextMuted)
		return
	}
	for _, p := range m.Participants {
		status := map[string]string{"excused": "izin", "absent": "tidak hadir"}[p.AttendanceStatus]
		if p.AttendanceStatus != "present" && status == "" {
			status = "belum dicatat"
		}
		if status != "" && strings.TrimSpace(p.AttendanceNote) != "" {
			status += " · " + p.AttendanceNote
		}
		chip := ui.Row(c.Key(p.UserID)).Height(26).Padding(0, 8, 0, 2).Gap(6).AlignItems(ui.Center).Radius(13).Background(spaceMix(t.Surface, t.Background, .5)).Tooltip(p.DisplayName)
		if status != "" {
			chip.Opacity(.72)
		}
		chip.Children(func() {
			avatar(c, p.DisplayName, 22)
			ui.Text(c, firstName(p.DisplayName)).FontSize(12).MaxLines(1)
			if status != "" {
				ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
			}
		})
	}
	if parts := meetingAttendance(m).parts(); len(parts) > 0 {
		ui.Text(c, strings.Join(parts, " · ")).FontSize(12).TextColor(faint(t)).FillWidth()
	}
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
