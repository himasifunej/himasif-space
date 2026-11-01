package server

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"slices"
	"time"
)

func (s *Server) meetingManage(r *http.Request, q store.Querier, p *principal, m api.Meeting) error {
	if p.admin() {
		return errForbidden
	}
	rr := r.Clone(r.Context())
	rr.SetPathValue("periodId", m.PeriodID.String())
	rr.SetPathValue("divisionId", m.OwnerDivisionID.String())
	sc, err := s.loadScope(r.Context(), q, rr, p, store.LockNone)
	if err != nil {
		return err
	}
	if sc.closed() {
		return errPeriodClosed
	}
	if m.OrganizerID == p.User.ID {
		return sc.require(api.CapTasksOwn)
	}
	return sc.require(api.CapTasksManage)
}
func (s *Server) meetingRead(r *http.Request, q store.Querier, p *principal, m *api.Meeting) error {
	view, err := s.spaceView(r.Context(), q, p, m.SpaceID)
	if err != nil {
		return err
	}
	if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
		return errNotFound
	}
	m.CanManage = s.meetingManage(r, q, p, *m) == nil
	m.Participants, err = s.store.MeetingParticipants(r.Context(), q, m.ID)
	return err
}
func (s *Server) withMeeting(r *http.Request, p *principal, write bool, fn func(store.Querier, *api.Meeting) error) error {
	return s.withMeetingAccounts(r, p, write, nil, fn)
}
func (s *Server) withMeetingAccounts(r *http.Request, p *principal, write bool, extra []id.ID, fn func(store.Querier, *api.Meeting) error) error {
	mid, err := pathID(r, "meetingId")
	if err != nil {
		return err
	}
	peek, err := s.store.Meeting(r.Context(), s.store.DB(), mid, store.LockNone)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		lock := store.LockShare
		ml := store.LockNone
		if write {
			lock = store.LockUpdate
			ml = store.LockUpdate
		}
		period, err := s.store.Period(r.Context(), q, peek.PeriodID, lock)
		if err != nil {
			return err
		}
		m, err := s.store.Meeting(r.Context(), q, mid, ml)
		if err != nil {
			return err
		}
		if err := s.meetingRead(r, q, p, &m); err != nil {
			return err
		}
		if write {
			if period.Status == api.PeriodClosed {
				return errPeriodClosed
			}
			accounts := append(slices.Clone(extra), p.User.ID)
			for _, participant := range m.Participants {
				accounts = append(accounts, participant.UserID)
			}
			if err := s.lockTaskAccounts(r.Context(), q, accounts); err != nil {
				return err
			}
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
			if err := s.meetingManage(r, q, p, m); err != nil {
				return err
			}
		}
		return fn(q, &m)
	})
}
func (s *Server) listMeetings(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	out := []api.Meeting{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, err := s.store.SpaceByID(r.Context(), q, sid)
		if err != nil {
			return err
		}
		if _, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockShare); err != nil {
			return err
		}
		if _, err := s.spaceView(r.Context(), q, p, sid); err != nil {
			return err
		}
		ms, err := s.store.Meetings(r.Context(), q, sid)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if err := s.meetingRead(r, q, p, &m); err != nil {
				return err
			}
			out = append(out, m)
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.Meeting]{Items: out})
	return nil
}
func (s *Server) getMeeting(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.Meeting
	err := s.withMeeting(r, p, false, func(q store.Querier, m *api.Meeting) error { out = *m; return nil })
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
func validateMeeting(req *api.MeetingRequest) error {
	var err error
	req.Title, err = text(req.Title, "judul rapat", 1, 240)
	if err != nil {
		return err
	}
	req.Description, err = text(req.Description, "tujuan rapat", 0, 10000)
	if err != nil {
		return err
	}
	req.Location, err = text(req.Location, "lokasi", 0, 1000)
	if err != nil {
		return err
	}
	if req.JoinURL != "" {
		u, e := url.Parse(req.JoinURL)
		if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || len(req.JoinURL) > 2000 {
			return badRequest("Tautan rapat harus berupa URL HTTP/HTTPS.")
		}
	}
	if req.StartsAt.IsZero() || (req.EndsAt != nil && !req.EndsAt.After(req.StartsAt)) {
		return badRequest("Isi waktu mulai dan selesai rapat yang valid.")
	}
	if req.TimeZone == "" {
		req.TimeZone = "Asia/Jakarta"
	}
	if _, err := time.LoadLocation(req.TimeZone); err != nil {
		return badRequest("Zona waktu tidak valid.")
	}
	if len(req.Participants) > 500 {
		return badRequest("Maksimal 500 peserta.")
	}
	if req.ReminderMinutes == nil {
		req.ReminderMinutes = []int{1440, 60, 15}
	}
	if len(req.ReminderMinutes) > 12 {
		return badRequest("Maksimal 12 slot pengingat.")
	}
	for _, v := range req.ReminderMinutes {
		if v < 0 || v > 43200 {
			return badRequest("Slot pengingat harus antara 0 dan 43200 menit.")
		}
	}
	slices.Sort(req.ReminderMinutes)
	req.ReminderMinutes = slices.Compact(req.ReminderMinutes)
	return nil
}
func (s *Server) fillMeeting(r *http.Request, q store.Querier, p *principal, m *api.Meeting, req api.MeetingRequest, create bool) error {
	if !create && m.Version != req.ExpectedVersion {
		return conflict("Rapat telah berubah. Muat ulang sebelum menyimpan.")
	}
	// Every participant must currently be able to open this space. No invitation
	// to a meeting grants membership or access to its notes.
	participants := []id.ID{}
	for _, uid := range req.Participants {
		if slices.Contains(participants, uid) {
			continue
		}
		var valid bool
		err := q.QueryRow(r.Context(), `select exists(select 1 from memberships mm join space_divisions sd on sd.period_id=mm.period_id and sd.division_id=mm.division_id join users u on u.id=mm.user_id where sd.space_id=$1 and mm.user_id=$2 and mm.removed_at is null and u.disabled_at is null and u.kind='person')`, m.SpaceID, uid).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return badRequest("Peserta harus merupakan anggota aktif ruang rapat.")
		}
		participants = append(participants, uid)
	}
	if req.NoteID != nil {
		n, err := s.store.Note(r.Context(), q, *req.NoteID, store.LockNone)
		if err != nil {
			return errNotFound
		}
		if n.SpaceID != m.SpaceID {
			return badRequest("Notulensi harus berada pada Space rapat yang sama.")
		}
		var linked bool
		if err := q.QueryRow(r.Context(), `select exists(select 1 from meetings where note_id=$1 and id<>$2)`, *req.NoteID, m.ID).Scan(&linked); err != nil {
			return err
		}
		if linked {
			return conflict("Catatan ini sudah menjadi notulensi rapat lain.")
		}
		period, err := s.store.Period(r.Context(), q, m.PeriodID, store.LockNone)
		if err != nil {
			return err
		}
		if err := s.noteAccess(r.Context(), q, p, period, &n); err != nil {
			return err
		}
	}
	before := *m
	oldParticipants := slices.Clone(m.Participants)
	m.Title = req.Title
	m.Description = req.Description
	m.Location = req.Location
	m.JoinURL = req.JoinURL
	m.StartsAt = req.StartsAt.UTC()
	m.EndsAt = nil
	if req.EndsAt != nil {
		end := req.EndsAt.UTC()
		m.EndsAt = &end
	}
	m.TimeZone = req.TimeZone
	m.NoteID = req.NoteID
	m.ReminderMinutes = req.ReminderMinutes
	m.Cancelled = req.Cancelled
	m.UpdatedAt = s.now().UTC()
	m.Version++
	if err := s.store.SaveMeeting(r.Context(), q, *m, participants, create); err != nil {
		return err
	}
	if !create {
		if _, err := q.Exec(r.Context(), `update delivery_events set cancelled_at=$2 where source_kind='meeting' and source_id=$1 and cancelled_at is null`, m.ID, s.now()); err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update delivery_recipients d set status='cancelled' from delivery_events e where d.event_id=e.id and e.source_kind='meeting' and e.source_id=$1 and d.status='queued'`, m.ID); err != nil {
			return err
		}
	}
	if err := s.record(r, q, p, store.AuditEvent{Action: "meeting.saved", TargetType: "meeting", TargetID: &m.ID, PeriodID: &m.PeriodID, Before: before, After: *m}); err != nil {
		return err
	}
	notify := slices.Clone(participants)
	for _, person := range oldParticipants {
		if !slices.Contains(notify, person.UserID) {
			notify = append(notify, person.UserID)
		}
	}
	messageKey := "meeting.created"
	title := "Rapat baru: " + m.Title
	if !create {
		title = "Perubahan rapat: " + m.Title
		messageKey = "meeting.changed"
	}
	if m.Cancelled {
		title = "Rapat dibatalkan: " + m.Title
		messageKey = "meeting.cancelled"
	}
	eventID := id.New()
	if err := s.enqueue(r.Context(), q, "meeting-change:"+eventID.String(), "ping", eventID, m.Version, &m.SpaceID, title, m.StartsAt.In(timeZoneOrUTC(m.TimeZone)).Format("02 Jan 2006 15:04 MST"), m.TimeZone, s.now(), notify, []string{"device", "whatsapp"}, deliveryMessage{Key: messageKey, Args: []string{m.Title, m.StartsAt.In(timeZoneOrUTC(m.TimeZone)).Format("02 Jan 2006 15:04 MST")}}); err != nil {
		return err
	}
	if _, err := q.Exec(r.Context(), `update delivery_events set related_kind='meeting',related_id=$2 where source_kind='ping' and source_id=$1`, eventID, m.ID); err != nil {
		return err
	}
	return s.meetingRead(r, q, p, m)
}
func (s *Server) createMeeting(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.MeetingRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateMeeting(&req); err != nil {
		return err
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var out api.Meeting
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, err := s.store.SpaceByID(r.Context(), q, sid)
		if err != nil {
			return err
		}
		period, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if err := s.lockTaskAccounts(r.Context(), q, append(slices.Clone(req.Participants), p.User.ID)); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		for _, role := range view.Roles {
			if slices.Contains(role.Capabilities, api.CapTasksManage) {
				out.OwnerDivisionID = role.DivisionID
				break
			}
		}
		if out.OwnerDivisionID == id.Nil {
			return errForbidden
		}
		out.ID = id.New()
		out.SpaceID = sid
		out.PeriodID = sp.PeriodID
		out.OrganizerID = p.User.ID
		out.CreatedAt = s.now().UTC()
		return s.fillMeeting(r, q, p, &out, req, true)
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}
func (s *Server) updateMeeting(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.MeetingRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateMeeting(&req); err != nil {
		return err
	}
	var out api.Meeting
	err := s.withMeetingAccounts(r, p, true, req.Participants, func(q store.Querier, m *api.Meeting) error {
		if err := s.fillMeeting(r, q, p, m, req, false); err != nil {
			return err
		}
		out = *m
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) recordAttendance(w http.ResponseWriter, r *http.Request, p *principal) error {
	uid, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	var req api.AttendanceRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains([]string{"unrecorded", "present", "excused", "absent"}, req.Status) {
		return badRequest("Status kehadiran tidak valid.")
	}
	if req.Status != "present" && req.ArrivedAt != nil {
		return badRequest("Waktu datang hanya untuk peserta hadir.")
	}
	if req.Status == "present" && (req.ArrivedAt == nil || req.ArrivedAt.After(s.now())) {
		return badRequest("Isi waktu datang aktual yang sudah terjadi.")
	}
	req.Note, err = text(req.Note, "catatan kehadiran", 0, 2000)
	if err != nil {
		return err
	}
	var out api.Meeting
	err = s.withMeeting(r, p, true, func(q store.Querier, m *api.Meeting) error {
		if m.OrganizerID != p.User.ID {
			return errForbidden
		}
		if m.Version != req.ExpectedVersion {
			return conflict("Rapat telah berubah. Muat ulang sebelum mencatat kehadiran.")
		}
		before := api.MeetingParticipant{}
		for _, person := range m.Participants {
			if person.UserID == uid {
				before = person
			}
		}
		tag, err := q.Exec(r.Context(), `update meeting_participants set arrived_at=$3,attendance_note=$4,attendance_status=$5,confirmed_by=$6 where meeting_id=$1 and user_id=$2`, m.ID, uid, req.ArrivedAt, req.Note, req.Status, p.User.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		// Attendance does not change schedule version or invalidate reminder slots.
		if err := s.record(r, q, p, store.AuditEvent{Action: "meeting.attendance", TargetType: "meeting", TargetID: &m.ID, PeriodID: &m.PeriodID, Before: before, After: map[string]any{"userId": uid, "attendance": req}}); err != nil {
			return err
		}
		if err := s.meetingRead(r, q, p, m); err != nil {
			return err
		}
		out = *m
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func timeZoneOrUTC(zone string) *time.Location {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// MeetingCandidates exposes only names and stable account IDs within this
// shared space, without opening another division's private member directory.
func (s *Server) meetingCandidates(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	out := []api.PersonRef{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, err := s.store.SpaceByID(r.Context(), q, sid)
		if err != nil {
			return err
		}
		if _, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockShare); err != nil {
			return err
		}
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		if !slices.Contains(view.Capabilities, api.CapMembersRead) {
			return errForbidden
		}
		rows, err := q.Query(r.Context(), `select distinct u.id,u.display_name from space_divisions sd join memberships m on m.period_id=sd.period_id and m.division_id=sd.division_id join users u on u.id=m.user_id where sd.space_id=$1 and m.removed_at is null and u.disabled_at is null and u.kind='person' order by u.display_name,u.id`, sid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var person api.PersonRef
			if err := rows.Scan(&person.ID, &person.DisplayName); err != nil {
				return err
			}
			out = append(out, person)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.PersonRef]{Items: out})
	return nil
}
