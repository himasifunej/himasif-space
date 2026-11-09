package server

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"slices"
	"time"
)

// Delivery visibility is recomputed when consumed, so a cached assignment
// never grants access after membership or account revocation.
func (s *Server) deliveryVisible(ctx context.Context, q store.Querier, p *principal, d api.Delivery) bool {
	switch d.SourceKind {
	case "reminder":
		var own bool
		err := q.QueryRow(ctx, `select exists(select 1 from reminders where id=$1 and owner_id=$2 and completed_at is null)`, d.SourceID, p.User.ID).Scan(&own)
		return err == nil && own
	case "task":
		t, err := s.store.Task(ctx, q, d.SourceID, store.LockNone)
		if err != nil {
			return false
		}
		period, err := s.store.Period(ctx, q, t.PeriodID, store.LockNone)
		if err != nil || period.Status == api.PeriodClosed || t.Progress != api.TaskTodo && t.Progress != api.TaskDoing {
			return false
		}
		return s.taskAccess(ctx, q, p, period, &t) == nil && (t.Assignee.ID == p.User.ID || slices.ContainsFunc(t.Participants, func(x api.PersonRef) bool { return x.ID == p.User.ID }))
	case "meeting":
		m, err := s.store.Meeting(ctx, q, d.SourceID, store.LockNone)
		if err != nil || m.Cancelled || !m.StartsAt.After(s.now()) {
			return false
		}
		if view, err := s.spaceView(ctx, q, p, m.SpaceID); err != nil || view.ReadOnly {
			return false
		}
		var participant bool
		err = q.QueryRow(ctx, `select exists(select 1 from meeting_participants where meeting_id=$1 and user_id=$2)`, m.ID, p.User.ID).Scan(&participant)
		return err == nil && participant
	case "ping":
		if d.RelatedKind == "task" && d.RelatedID != nil {
			t, err := s.store.Task(ctx, q, *d.RelatedID, store.LockNone)
			if err != nil {
				return false
			}
			period, err := s.store.Period(ctx, q, t.PeriodID, store.LockNone)
			return err == nil && s.taskAccess(ctx, q, p, period, &t) == nil
		}
		if d.SpaceID == nil {
			return false
		}
		v, err := s.spaceView(ctx, q, p, *d.SpaceID)
		return err == nil && !v.ReadOnly
	}
	return false
}
func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() || !s.cfg.OutboundEnabled() {
		writeJSON(w, http.StatusOK, api.List[api.Delivery]{Items: []api.Delivery{}})
		return nil
	}
	out := []api.Delivery{}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		rows, err := q.Query(r.Context(), `select d.id,e.id,e.source_kind,e.source_id,e.space_id,e.title,e.body,e.time_zone,e.scheduled_at,d.available_at,d.channel,d.status,d.seen_at,d.snoozed_until,d.last_error,e.related_kind,e.related_id from delivery_recipients d join delivery_events e on e.id=d.event_id where d.recipient_id=$1 and e.cancelled_at is null and e.suppressed_at is null and d.status<>'cancelled' and d.available_at<=$2 and e.scheduled_at>=$3 order by d.available_at desc,d.id desc limit 200`, p.User.ID, s.now().Add(48*time.Hour), s.now().AddDate(0, 0, -30))
		if err != nil {
			return err
		}
		candidates := []api.Delivery{}
		for rows.Next() {
			var d api.Delivery
			if err := rows.Scan(&d.ID, &d.EventID, &d.SourceKind, &d.SourceID, &d.SpaceID, &d.Title, &d.Body, &d.TimeZone, &d.ScheduledAt, &d.AvailableAt, &d.Channel, &d.Status, &d.SeenAt, &d.SnoozedUntil, &d.LastError, &d.RelatedKind, &d.RelatedID); err != nil {
				rows.Close()
				return err
			}
			candidates = append(candidates, d)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, d := range candidates {
			if s.deliveryVisible(r.Context(), q, p, d) {
				out = append(out, d)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.Delivery]{Items: out})
	return nil
}
func (s *Server) respondDelivery(w http.ResponseWriter, r *http.Request, p *principal) error {
	did, err := pathID(r, "deliveryId")
	if err != nil {
		return err
	}
	var req api.DeliveryResponse
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains([]string{"accepted", "seen", "snooze", "unknown", "failed"}, req.Action) {
		return badRequest("Respons notifikasi tidak valid.")
	}
	if req.Action == "snooze" && (req.SnoozeMinutes < 1 || req.SnoozeMinutes > 10080) {
		return badRequest("Snooze harus 1–10080 menit.")
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		var d api.Delivery
		err := q.QueryRow(r.Context(), `select d.id,e.source_kind,e.source_id,e.space_id,d.channel,e.related_kind,e.related_id,e.scheduled_at from delivery_recipients d join delivery_events e on e.id=d.event_id where d.id=$1 and d.recipient_id=$2 and e.cancelled_at is null and e.suppressed_at is null and d.status<>'cancelled' for update of d`, did, p.User.ID).Scan(&d.ID, &d.SourceKind, &d.SourceID, &d.SpaceID, &d.Channel, &d.RelatedKind, &d.RelatedID, &d.ScheduledAt)
		if err != nil {
			return errNotFound
		}
		if !s.deliveryVisible(r.Context(), q, p, d) {
			return errNotFound
		}
		if d.Channel != "device" {
			return badRequest("Status kanal WhatsApp hanya dapat diubah adapter pengiriman.")
		}
		switch req.Action {
		case "accepted":
			_, err = q.Exec(r.Context(), `update delivery_recipients set status='accepted',accepted_at=coalesce(accepted_at,$2),last_error='' where id=$1`, did, s.now())
		case "seen":
			_, err = q.Exec(r.Context(), `update delivery_recipients set seen_at=coalesce(seen_at,$2) where id=$1`, did, s.now())
		case "snooze":
			_, err = q.Exec(r.Context(), `update delivery_recipients set status='queued',available_at=$2,snoozed_until=$2,seen_at=null,accepted_at=null where id=$1`, did, laterDeliveryTime(d.ScheduledAt, s.now().Add(time.Duration(req.SnoozeMinutes)*time.Minute)))
		default:
			_, err = q.Exec(r.Context(), `update delivery_recipients set status=$2,last_error=$3 where id=$1`, did, req.Action, "Perangkat belum mengonfirmasi penerimaan kanal.")
		}
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
func (s *Server) sendPing(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var req api.PingRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	req.Message, err = text(req.Message, "pesan ping", 0, 2000)
	if err != nil {
		return err
	}
	if req.Message == "" {
		req.Message = "Mohon periksa pekerjaan atau agenda di Space."
	}
	if len(req.Recipients) == 0 || len(req.Recipients) > 500 {
		return badRequest("Pilih 1–500 penerima ping.")
	}
	if len(req.Channels) == 0 {
		req.Channels = []string{"device"}
	}
	for _, ch := range req.Channels {
		if ch != "device" && ch != "whatsapp" {
			return badRequest("Kanal ping tidak valid.")
		}
	}
	if req.SourceKind != "" && req.SourceKind != "task" && req.SourceKind != "meeting" {
		return badRequest("Sumber ping tidak valid.")
	}
	pid := id.New()
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
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		if p.admin() || !slices.Contains(view.Capabilities, api.CapPingSend) {
			return errForbidden
		}
		if req.SourceKind != "" {
			if req.SourceID == nil {
				return badRequest("Pilih objek sumber ping.")
			}
			if req.SourceKind == "task" {
				t, err := s.store.Task(r.Context(), q, *req.SourceID, store.LockNone)
				if err != nil {
					return errNotFound
				}
				if !slices.Contains(t.SpaceIDs, sid) {
					return errNotFound
				}
				if err := s.taskAccess(r.Context(), q, p, period, &t); err != nil {
					return err
				}
				if !t.Permissions.Manage {
					return errForbidden
				}
			} else {
				m, err := s.store.Meeting(r.Context(), q, *req.SourceID, store.LockNone)
				if err != nil || m.SpaceID != sid {
					return errNotFound
				}
				if err := s.meetingManage(r, q, p, m); err != nil {
					return err
				}
			}
		}
		for _, uid := range req.Recipients {
			var allowed bool
			err := q.QueryRow(r.Context(), `select exists(select 1 from memberships m join space_divisions sd on sd.period_id=m.period_id and sd.division_id=m.division_id join users u on u.id=m.user_id where sd.space_id=$1 and m.user_id=$2 and m.removed_at is null and u.disabled_at is null)`, sid, uid).Scan(&allowed)
			if err != nil {
				return err
			}
			if !allowed {
				return badRequest("Penerima tidak lagi menjadi anggota ruang ini.")
			}
		}
		if err := s.enqueue(r.Context(), q, "ping:"+pid.String(), "ping", pid, 1, &sid, "Ping · "+view.Name, req.Message, period.TimeZone, s.now(), req.Recipients, req.Channels); err != nil {
			return err
		}
		if req.SourceID != nil {
			if _, err := q.Exec(r.Context(), `update delivery_events set related_kind=$2,related_id=$3 where source_kind='ping' and source_id=$1`, pid, req.SourceKind, *req.SourceID); err != nil {
				return err
			}
		}
		return s.record(r, q, p, store.AuditEvent{Action: "ping.created", TargetType: "ping", TargetID: &pid, PeriodID: &period.ID, After: req})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": pid, "status": "queued"})
	return nil
}

func laterDeliveryTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
