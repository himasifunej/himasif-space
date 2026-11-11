package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"slices"
	"strconv"
	"strings"
	"time"
)

type MeetingsAPI interface {
	SpaceMeetings(context.Context, id.ID) ([]api.Meeting, error)
	CreateMeeting(context.Context, id.ID, api.MeetingRequest) (api.Meeting, error)
	UpdateMeeting(context.Context, id.ID, api.MeetingRequest) (api.Meeting, error)
	RecordAttendance(context.Context, id.ID, id.ID, api.AttendanceRequest) (api.Meeting, error)
	MeetingCandidates(context.Context, id.ID) ([]api.PersonRef, error)
	SpaceNotes(context.Context, id.ID) ([]api.Note, error)
}
type meetingDraft struct {
	id                                                                          id.ID
	version                                                                     int64
	title, description, location, url, date, endDate, start, end, zone, minutes string
	note                                                                        string
	participants                                                                []id.ID
	cancelled                                                                   bool
}
type meetingsState struct {
	space                 id.ID
	items                 []api.Meeting
	members               []api.PersonRef
	notes                 []api.Note
	loaded, loading, busy bool
	err                   string
	draft                 *meetingDraft
	selected              id.ID
	participant           id.ID
	arrival, note         string
	// Rapat A: the detail's sub-tab (Ringkasan · Kehadiran · Notulen), the
	// "Saya diundang" filter, the … menu, the cancel dialog and, on a
	// phone, whether the detail page is open.
	tab                   int
	mine                  bool
	menuOpen, confirmOpen bool
	// confirmID is the meeting the cancel dialog asks about, kept when the
	// default selection changes under it.
	confirmID    id.ID
	mobileDetail bool
	toast        string
	// skew is how far the server's clock is ahead of this device's, from
	// the last shared timer received (stage T2).
	skew time.Duration
}

// noteSkew reads the server's clock off a meeting's timer received at
// received.
func (s *meetingsState) noteSkew(m api.Meeting, received time.Time) {
	if m.Timer != nil && !m.Timer.ServerNow.IsZero() {
		s.skew = m.Timer.ServerNow.Sub(received)
	}
}

func (a *App) meetingsService() (MeetingsAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(MeetingsAPI)
	return s, ok
}
func (a *App) ensureMeetings() {
	sid := a.selectedSpaceID()
	if a.meetings.space != sid {
		a.meetings = meetingsState{space: sid}
	}
	if sid != id.Nil && !a.meetings.loaded && !a.meetings.loading && a.meetings.err == "" {
		a.loadMeetings()
	}
}
func (a *App) loadMeetings() {
	svc, ok := a.meetingsService()
	if !ok {
		a.meetings.err = "Layanan rapat belum tersedia."
		return
	}
	sp := a.selectedSpace()
	if sp == nil || a.meetings.loading {
		return
	}
	sid, gen := sp.ID, a.gen
	a.meetings.loading = true
	a.async(func() func() {
		items, err := svc.SpaceMeetings(context.Background(), sid)
		received := time.Now()
		members := []api.PersonRef{}
		var memberErr error
		if !sp.ReadOnly && slices.Contains(sp.Capabilities, api.CapMembersRead) {
			members, memberErr = svc.MeetingCandidates(context.Background(), sid)
		}
		notes := []api.Note{}
		if slices.Contains(sp.Capabilities, api.CapNotesRead) {
			notes, _ = svc.SpaceNotes(context.Background(), sid)
		}
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.meetings.loading = false
			if err != nil {
				a.meetings.err = a.failed(err)
				return
			}
			a.meetings.items = items
			for _, m := range items {
				a.meetings.noteSkew(m, received)
			}
			a.meetings.members = members
			a.meetings.notes = notes
			a.meetings.loaded = true
			a.meetings.err = ""
			if memberErr != nil && !sp.ReadOnly {
				a.meetings.err = "Pilihan peserta belum dapat dimuat: " + a.failed(memberErr)
			}
		}
	})
}
func (a *App) beginMeeting(m *api.Meeting) {
	d := &meetingDraft{zone: "Asia/Jakarta", minutes: "1440,60,15"}
	if p := a.period(); p != nil {
		d.zone = p.TimeZone
	}
	loc, _ := time.LoadLocation(d.zone)
	if loc == nil {
		loc = time.UTC
	}
	d.date = time.Now().In(loc).Add(time.Hour).Format(time.DateOnly)
	d.start = time.Now().In(loc).Add(time.Hour).Format("15:04")
	if m != nil {
		d.id = m.ID
		d.version = m.Version
		d.title = m.Title
		d.description = m.Description
		d.location = m.Location
		d.url = m.JoinURL
		d.zone = m.TimeZone
		d.cancelled = m.Cancelled
		loc, _ = time.LoadLocation(d.zone)
		if loc == nil {
			loc = time.UTC
		}
		d.date = m.StartsAt.In(loc).Format(time.DateOnly)
		d.start = m.StartsAt.In(loc).Format("15:04")
		if m.EndsAt != nil {
			d.end = m.EndsAt.In(loc).Format("15:04")
			if endDate := m.EndsAt.In(loc).Format(time.DateOnly); endDate != d.date {
				d.endDate = endDate
			}
		}
		mins := []string{}
		for _, v := range m.ReminderMinutes {
			mins = append(mins, strconv.Itoa(v))
		}
		d.minutes = strings.Join(mins, ",")
		for _, p := range m.Participants {
			d.participants = append(d.participants, p.UserID)
		}
		if m.NoteID != nil {
			d.note = m.NoteID.String()
		}
	}
	a.meetings.draft = d
	a.chrome.appearance = false
	a.chrome.datePickers = nil
}
func (a *App) saveMeeting() {
	d := a.meetings.draft
	if d == nil || a.meetings.busy {
		return
	}
	svc, ok := a.meetingsService()
	if !ok {
		return
	}
	loc, err := time.LoadLocation(d.zone)
	if err != nil {
		a.meetings.err = "Zona waktu tidak valid."
		return
	}
	start, err := time.ParseInLocation("2006-01-02 15:04", d.date+" "+d.start, loc)
	if err != nil {
		a.meetings.err = "Pilih tanggal dan isi jam mulai yang valid (HH:MM)."
		return
	}
	var end *time.Time
	if strings.TrimSpace(d.end) != "" {
		endDate := d.endDate
		if endDate == "" {
			endDate = d.date
		}
		at, e := time.ParseInLocation("2006-01-02 15:04", endDate+" "+d.end, loc)
		if e != nil || !at.After(start) {
			a.meetings.err = "Jam selesai harus setelah jam mulai. Untuk rapat lintas hari, pilih tanggal selesai."
			return
		}
		end = &at
	}
	mins := []int{}
	if strings.TrimSpace(d.minutes) != "" {
		for _, part := range strings.Split(d.minutes, ",") {
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				a.meetings.err = "Slot pengingat diisi menit, dipisahkan koma."
				return
			}
			mins = append(mins, v)
		}
	}
	req := api.MeetingRequest{Title: d.title, Description: d.description, Location: d.location, JoinURL: d.url, StartsAt: start, EndsAt: end, TimeZone: d.zone, Participants: slices.Clone(d.participants), ReminderMinutes: mins, Cancelled: d.cancelled, ExpectedVersion: d.version}
	if strings.TrimSpace(d.note) != "" {
		nid, err := id.Parse(strings.TrimSpace(d.note))
		if err != nil {
			a.meetings.err = "ID catatan tidak valid."
			return
		}
		req.NoteID = &nid
	}
	sid, mid, gen := a.meetings.space, d.id, a.gen
	a.meetings.busy = true
	a.async(func() func() {
		var m api.Meeting
		var err error
		if mid == id.Nil {
			m, err = svc.CreateMeeting(context.Background(), sid, req)
		} else {
			m, err = svc.UpdateMeeting(context.Background(), mid, req)
		}
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.meetings.busy = false
			if err != nil {
				a.meetings.err = a.failed(err)
				return
			}
			a.meetings.selected = m.ID
			a.meetings.draft = nil
			a.loadMeetings()
		}
	})
}

// attendance records one participant: status with, for Hadir, the arrival
// typed as "YYYY-MM-DD HH:MM" in the meeting's zone. The answer replaces
// the meeting in the list (attendance does not change its version).
func (a *App) attendance(mid, uid id.ID, version int64, status, arrival, note string) {
	svc, ok := a.meetingsService()
	if !ok || a.meetings.busy {
		return
	}
	req := api.AttendanceRequest{Status: status, Note: note, ExpectedVersion: version}
	if status == "present" {
		m := slices.IndexFunc(a.meetings.items, func(m api.Meeting) bool { return m.ID == mid })
		if m < 0 {
			return
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(arrival), meetingLoc(a.meetings.items[m]))
		if err != nil {
			a.meetings.err = "Isi waktu datang: YYYY-MM-DD HH:MM."
			return
		}
		req.ArrivedAt = &at
	}
	gen, sid := a.gen, a.meetings.space
	a.meetings.busy, a.meetings.err = true, ""
	a.async(func() func() {
		m, err := svc.RecordAttendance(context.Background(), mid, uid, req)
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.meetings.busy = false
			if err != nil {
				a.meetings.err = a.failed(err)
				a.loadMeetings()
				return
			}
			a.meetings.participant = id.Nil
			a.replaceMeeting(m)
		}
	})
}

// replaceMeeting puts a meeting the server sent back in the list.
func (a *App) replaceMeeting(m api.Meeting) {
	for i := range a.meetings.items {
		if a.meetings.items[i].ID == m.ID {
			a.meetings.items[i] = m
			return
		}
	}
	a.loadMeetings()
}

// setMeetingCancelled cancels a meeting or makes it active again, the rest
// of it unchanged.
func (a *App) setMeetingCancelled(m api.Meeting, cancelled bool) {
	svc, ok := a.meetingsService()
	if !ok || a.meetings.busy {
		return
	}
	req := meetingRequestFrom(m)
	req.Cancelled = cancelled
	// The server checks every participant again: one who has left the
	// Space would block the change, and the form cannot remove them.
	if len(a.meetings.members) > 0 {
		req.Participants = slices.DeleteFunc(req.Participants, func(uid id.ID) bool {
			return !slices.ContainsFunc(a.meetings.members, func(p api.PersonRef) bool { return p.ID == uid })
		})
	}
	gen, sid := a.gen, a.meetings.space
	a.meetings.busy, a.meetings.err = true, ""
	a.async(func() func() {
		out, err := svc.UpdateMeeting(context.Background(), m.ID, req)
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.meetings.busy = false
			a.meetings.confirmOpen = false
			if err != nil {
				a.meetings.err = a.failed(err)
				a.loadMeetings()
				return
			}
			if cancelled {
				a.meetings.toast = "Rapat dibatalkan."
			} else {
				a.meetings.toast = "Rapat aktif kembali."
			}
			a.replaceMeeting(out)
		}
	})
}

func (a *App) meetingForm(c *ui.Context) {
	d := a.meetings.draft
	ui.Column(c.Key(d)).FillWidth().Padding(16).Gap(10).Border(1, c.Theme().Border).Radius(10).Children(func() {
		taskInput(c, "meeting-title", "Judul rapat *", &d.title)
		taskLongInput(c, "meeting-description", "Tujuan / agenda", &d.description)
		a.whenField(c, "meeting-when", "Mulai rapat", &d.date, &d.start, true)
		clockField(c, "meeting-end", "Jam selesai", &d.end, false)
		muted(c, "Jam selesai boleh dikosongkan: rapat berlangsung sampai selesai.")
		if d.end != "" {
			a.dateField(c, "meeting-end-date", "Tanggal selesai (jika beda hari)", &d.endDate, false, false)
		}
		taskInput(c, "meeting-location", "Lokasi", &d.location)
		taskInput(c, "meeting-url", "Tautan rapat", &d.url)
		taskInput(c, "meeting-zone", "Zona waktu", &d.zone)
		ui.Text(c, "Pengingat").Bold()
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, slot := range []struct{ label, value string }{{"1 hari", "1440"}, {"1 jam", "60"}, {"15 menit", "15"}} {
				parts := strings.Split(d.minutes, ",")
				chosen := slices.Contains(parts, slot.value)
				if spaceButton(c, slot.label, "bell", chosen).Clicked() {
					if chosen {
						parts = slices.DeleteFunc(parts, func(v string) bool { return v == slot.value })
					} else {
						parts = append(parts, slot.value)
					}
					parts = slices.DeleteFunc(parts, func(v string) bool { return v == "" })
					d.minutes = strings.Join(parts, ",")
				}
			}
		})
		ui.Text(c, "Notulensi").Bold()
		if spaceButton(c, "Tanpa notulensi terkait", "note", d.note == "").Clicked() {
			d.note = ""
		}
		for _, note := range a.meetings.notes {
			note := note
			if spaceButton(c, note.Title, "note", d.note == note.ID.String()).Clicked() {
				d.note = note.ID.String()
			}
		}
		if spaceButton(c, "Buat notulensi di Notes", "plus", false).Clicked() {
			a.chrome.tab = "Catatan"
			a.ensureNotes()
			a.beginNote()
			a.notes.templateKind = api.TemplateMeeting
			a.notes.newTitle = "Notulensi · " + d.title
		}
		ui.Text(c, "Peserta").Bold()
		for _, m := range a.meetings.members {
			m := m
			chosen := slices.Contains(d.participants, m.ID)
			label := m.DisplayName
			if chosen {
				label = "✓ " + label
			}
			if spaceButton(c, label, "users", chosen).Clicked() {
				if chosen {
					d.participants = slices.DeleteFunc(d.participants, func(u id.ID) bool { return u == m.ID })
				} else {
					d.participants = append(d.participants, m.ID)
				}
			}
		}
		if d.id != id.Nil {
			label := "Batalkan rapat"
			if d.cancelled {
				label = "Rapat dibatalkan · klik untuk aktifkan kembali"
			}
			if spaceButton(c, label, "", d.cancelled).Clicked() {
				d.cancelled = !d.cancelled
			}
		}
		ui.Row(c).Gap(8).Children(func() {
			if spaceButton(c, "Simpan rapat", "check", true).Disabled(a.meetings.busy || a.conn != connOnline).Clicked() {
				a.saveMeeting()
			}
			if spaceButton(c, "Tutup formulir", "close", false).Disabled(a.meetings.busy).Clicked() {
				a.meetings.draft = nil
			}
		})
		muted(c, fmt.Sprintf("%d peserta dipilih. Kehadiran dikonfirmasi penyelenggara.", len(d.participants)))
	})
}
