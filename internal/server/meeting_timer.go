package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

// Stage T2: the shared meeting timer. Who may manage the meeting, or edit
// (manage) its note, may run the timer; everyone who reads the meeting
// sees it. The state is api.MeetingTimer on the meeting row; the meetings
// realtime trigger tells every client of a change.

// meetingNotes are the notes of a meeting: the one it points at and those
// that point at it.
func (s *Server) meetingNotes(ctx context.Context, q store.Querier, m api.Meeting) ([]id.ID, error) {
	rows, err := q.Query(ctx, `select id from notes where meeting_id=$1 or id=$2`, m.ID, m.NoteID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[id.ID])
}

// canRunMeetingTimer: the meeting's manager, or an editor of its note.
func (s *Server) canRunMeetingTimer(r *http.Request, q store.Querier, p *principal, m api.Meeting) (bool, error) {
	if p.admin() || m.Cancelled {
		return false, nil
	}
	if m.CanManage {
		return true, nil
	}
	notes, err := s.meetingNotes(r.Context(), q, m)
	if err != nil || len(notes) == 0 {
		return false, err
	}
	period, err := s.store.Period(r.Context(), q, m.PeriodID, store.LockNone)
	if err != nil {
		return false, err
	}
	for _, nid := range notes {
		n, err := s.store.Note(r.Context(), q, nid, store.LockNone)
		if err != nil {
			return false, err
		}
		if err := s.noteAccess(r.Context(), q, p, period, &n); err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				continue
			}
			return false, err
		}
		if n.Permissions.Manage {
			return true, nil
		}
	}
	return false, nil
}

func timerActionRefused(action string) error {
	switch action {
	case api.TimerStart:
		return conflict("Timer rapat sudah berjalan.")
	case api.TimerPause:
		return conflict("Timer sudah dijeda.")
	case api.TimerResume:
		return conflict("Timer sedang berjalan.")
	case api.TimerNext, api.TimerExtend, api.TimerStop:
		return conflict("Timer rapat belum dimulai.")
	}
	return badRequest("Aksi timer tidak dikenal.")
}

// meetingTimerAction is POST /api/meetings/{meetingId}/timer: one action
// (start, pause, resume, next, extend, stop) with the TimerVersion the
// caller saw. The server's clock stamps it.
func (s *Server) meetingTimerAction(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.MeetingTimerRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	switch req.Action {
	case api.TimerStart, api.TimerPause, api.TimerResume, api.TimerNext, api.TimerExtend, api.TimerStop:
	default:
		return badRequest("Aksi timer tidak dikenal.")
	}
	if p.admin() {
		return errForbidden
	}
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
	var out api.Meeting
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		period, err := s.store.Period(r.Context(), q, peek.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		m, err := s.store.Meeting(r.Context(), q, mid, store.LockUpdate)
		if err != nil {
			return err
		}
		if err := s.meetingRead(r, q, p, &m); err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		if m.Cancelled {
			return conflict("Rapat ini dibatalkan.")
		}
		if !m.CanRunTimer {
			return errForbidden
		}
		if m.TimerVersion != req.ExpectedVersion {
			who := "orang lain"
			if m.Timer != nil && m.Timer.UpdatedBy.DisplayName != "" {
				who = m.Timer.UpdatedBy.DisplayName
			}
			return conflict("Timer baru saja diubah oleh " + who + ". Tampilan dimuat ulang; coba lagi bila masih perlu.")
		}
		next, err := api.ApplyTimer(m.Timer, req.Action, s.now())
		if errors.Is(err, api.ErrTimerAction) {
			return timerActionRefused(req.Action)
		}
		if err != nil {
			return err
		}
		if next != nil {
			next.UpdatedBy = api.PersonRef{ID: p.User.ID, DisplayName: p.User.DisplayName}
		}
		if err := s.store.SetMeetingTimer(r.Context(), q, m.ID, next, m.TimerVersion+1); err != nil {
			return err
		}
		if err := s.record(r, q, p, store.AuditEvent{Action: "meeting.timer." + req.Action, TargetType: "meeting", TargetID: &m.ID, PeriodID: &m.PeriodID, Before: m.Timer, After: next}); err != nil {
			return err
		}
		if out, err = s.store.Meeting(r.Context(), q, mid, store.LockNone); err != nil {
			return err
		}
		return s.meetingRead(r, q, p, &out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
