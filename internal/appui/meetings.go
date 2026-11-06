package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
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
func (a *App) attendance(mid, uid id.ID, version int64, status string) {
	svc, ok := a.meetingsService()
	if !ok || a.meetings.busy {
		return
	}
	req := api.AttendanceRequest{Status: status, Note: a.meetings.note, ExpectedVersion: version}
	if status == "present" {
		m := slices.IndexFunc(a.meetings.items, func(m api.Meeting) bool { return m.ID == mid })
		if m < 0 {
			return
		}
		loc, err := time.LoadLocation(a.meetings.items[m].TimeZone)
		if err != nil {
			return
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", a.meetings.arrival, loc)
		if err != nil {
			a.meetings.err = "Isi waktu datang aktual: YYYY-MM-DD HH:MM."
			return
		}
		req.ArrivedAt = &at
	}
	gen, sid := a.gen, a.meetings.space
	a.meetings.busy = true
	a.async(func() func() {
		_, err := svc.RecordAttendance(context.Background(), mid, uid, req)
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.meetings.busy = false
			if err != nil {
				a.meetings.err = a.failed(err)
				return
			}
			a.loadMeetings()
		}
	})
}
func (a *App) meetingsView(c *ui.Context) {
	a.ensureMeetings()
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() { a.meetingsListView(c) })
		if a.meetings.draft != nil && !a.chrome.appearance {
			workspaceui.Drawer(c.Key("meeting-drawer"), &a.chrome.drawerWidth, func() {
				ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Rapat").FontSize(18).Bold().Grow(1)
					if spaceIconButton(c, "Tutup formulir", "close").Disabled(a.meetings.busy).Clicked() {
						a.meetings.draft = nil
					}
				})
				if a.meetings.draft != nil {
					ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() { a.meetingForm(c) })
				}
			})
		}
	})
}
func (a *App) meetingsListView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(24).Gap(14).Children(func() {
			ui.Text(c, "Rapat dan agenda").FontSize(24).Bold()
			ui.Row(c).Wrap().Gap(8).Children(func() {
				if spaceButton(c, "Muat ulang", "refresh", false).Disabled(a.meetings.loading || a.meetings.busy).Clicked() {
					a.loadMeetings()
				}
				sp := a.selectedSpace()
				can := sp != nil && !sp.ReadOnly && !a.isAdmin() && slices.Contains(sp.Capabilities, api.CapTasksManage) && a.conn == connOnline
				if spaceButton(c, "Buat rapat", "plus", true).Disabled(!can || a.meetings.busy).Clicked() {
					a.beginMeeting(nil)
				}
			})
			if a.meetings.err != "" {
				ui.Text(c, a.meetings.err).TextColor(c.Theme().Danger)
			}
			if a.meetings.loaded && len(a.meetings.items) == 0 {
				muted(c, "Belum ada rapat di ruang ini.")
			}
			for _, m := range a.meetings.items {
				m := m
				ui.Column(c.Key(m.ID)).FillWidth().Padding(16).Gap(8).Border(1, c.Theme().Border).Radius(10).Children(func() {
					title := m.Title
					if m.Cancelled {
						title += " · Dibatalkan"
					}
					ui.Text(c, title).Bold().FontSize(17)
					endLabel := "selesai"
					if m.EndsAt != nil {
						endLabel = taskTimestamp(*m.EndsAt, m.TimeZone)
					}
					muted(c, taskTimestamp(m.StartsAt, m.TimeZone)+" — "+endLabel)
					ui.Text(c, m.Description)
					muted(c, m.Location+" "+m.JoinURL)
					ui.Row(c).Wrap().Gap(8).Children(func() {
						if spaceButton(c, "Peserta / kehadiran", "users", a.meetings.selected == m.ID).Clicked() {
							a.meetings.selected = m.ID
							a.meetings.participant = id.Nil
							a.meetings.arrival, a.meetings.note = "", ""
						}
						if spaceButton(c, "Ubah / batalkan", "settings", false).Disabled(!m.CanManage || a.conn != connOnline || a.meetings.busy).Clicked() {
							a.beginMeeting(&m)
						}
						if m.NoteID != nil && spaceButton(c, "Buka notulensi", "note", false).Clicked() {
							a.chrome.tab = "Notes"
							a.ensureNotes()
							a.openNote(*m.NoteID)
						}
					})
					if a.meetings.selected == m.ID {
						ui.Row(c).FillWidth().MinWidth(0).Gap(16).AlignItems(ui.Start).Children(func() {
							ui.Column(c).Width(220).Shrink(0).Gap(6).Children(func() {
								ui.Text(c, "Peserta").Bold()
								for _, participant := range m.Participants {
									p := participant
									if spaceButton(c.Key(p.UserID), p.DisplayName, "users", a.meetings.participant == p.UserID).FillWidth().Clicked() {
										a.meetings.participant = p.UserID
										a.meetings.arrival, a.meetings.note = "", p.AttendanceNote
										if p.ArrivedAt != nil {
											if loc, err := time.LoadLocation(m.TimeZone); err == nil {
												a.meetings.arrival = p.ArrivedAt.In(loc).Format("2006-01-02 15:04")
											}
										}
									}
								}
								if len(m.Participants) == 0 {
									muted(c, "Belum ada peserta.")
								}
							})
							ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
								participant := slices.IndexFunc(m.Participants, func(p api.MeetingParticipant) bool { return p.UserID == a.meetings.participant })
								if participant < 0 {
									muted(c, "Pilih peserta untuk melihat kehadiran.")
									return
								}
								p := m.Participants[participant]
								ui.Text(c, p.DisplayName).Bold()
								status := map[string]string{"present": "Hadir", "excused": "Izin", "absent": "Tidak hadir", "unrecorded": "Belum dicatat"}[p.AttendanceStatus]
								if status == "" {
									status = "Belum dicatat"
								}
								muted(c, "Status: "+status)
								if m.OrganizerID != a.user.ID {
									if p.ArrivedAt != nil {
										muted(c, "Datang: "+taskTimestamp(*p.ArrivedAt, m.TimeZone))
									}
									if p.AttendanceNote != "" {
										muted(c, p.AttendanceNote)
									}
									return
								}
								ui.Field(c, "Waktu datang aktual · wajib untuk Hadir", func() {
									ui.TextInput(c.Key("arrival"), &a.meetings.arrival).Label("Waktu datang aktual · zona rapat").Placeholder("Contoh: 2026-11-05 19:00").FillWidth()
								})
								muted(c, "Gunakan tanggal dan jam dalam zona "+m.TimeZone+".").FontSize(11)
								ui.Field(c, "Catatan kehadiran · opsional", func() {
									ui.TextInput(c.Key("attendance-note"), &a.meetings.note).Label("Catatan kehadiran").Placeholder("Contoh: terlambat karena kelas").FillWidth()
								})
								ui.Row(c).Wrap().Gap(6).Children(func() {
									for _, status := range []struct{ k, label string }{{"present", "Hadir"}, {"excused", "Izin"}, {"absent", "Tidak hadir"}, {"unrecorded", "Hapus catatan hadir"}} {
										if spaceButton(c.Key(status.k), status.label, "", false).Disabled(a.conn != connOnline || a.meetings.busy).Clicked() {
											a.attendance(m.ID, p.UserID, m.Version, status.k)
										}
									}
								})
							})
						})
					}
				})
			}
		})
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
			a.chrome.tab = "Notes"
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
