package server

import (
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
	"net/http"
	"slices"
	"strings"
	"time"
)

func (s *Server) medhumPolicy(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	write := r.Method == http.MethodPatch
	var req api.MedhumPolicyRequest
	if write {
		if err := readJSON(r, &req); err != nil {
			return err
		}
		for _, clock := range []*string{req.PostingClock, req.DesignClock} {
			if clock == nil {
				continue
			}
			*clock = strings.TrimSpace(*clock)
			if *clock != "" {
				if _, err := time.Parse("15:04", *clock); err != nil || len(*clock) != 5 {
					return badRequest("Jam harus HH:MM atau kosong untuk menghapus kebijakan.")
				}
			}
		}
	}
	out := api.MedhumPolicy{SpaceID: sid}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, humas, design, err := s.medhumDivisions(r, q, sid)
		if err != nil {
			return err
		}
		lock := store.LockShare
		if write {
			lock = store.LockUpdate
		}
		period, err := s.store.Period(r.Context(), q, sp.PeriodID, lock)
		if err != nil {
			return err
		}
		out.TimeZone = period.TimeZone
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		err = q.QueryRow(r.Context(), `select posting_clock,design_clock,version,updated_at from medhum_policies where space_id=$1`, sid).Scan(&out.PostingClock, &out.DesignClock, &out.Version, &out.UpdatedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !write {
			return nil
		}
		if p.admin() {
			return errForbidden
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		if out.Version != req.ExpectedVersion {
			return conflict("Kebijakan waktu berubah. Muat ulang sebelum menyimpan.")
		}
		can := func(did string) bool {
			for _, role := range view.Roles {
				if role.DivisionID.String() == did && slices.Contains(role.Capabilities, api.CapTasksManage) {
					return true
				}
			}
			return false
		}
		if req.PostingClock != nil && !can(humas.String()) || req.DesignClock != nil && !can(design.String()) {
			return errForbidden
		}
		before := out
		if req.PostingClock != nil {
			if *req.PostingClock == "" {
				out.PostingClock = nil
			} else {
				out.PostingClock = req.PostingClock
			}
		}
		if req.DesignClock != nil {
			if *req.DesignClock == "" {
				out.DesignClock = nil
			} else {
				out.DesignClock = req.DesignClock
			}
		}
		out.Version++
		now := s.now().UTC()
		out.UpdatedAt = &now
		_, err = q.Exec(r.Context(), `insert into medhum_policies(space_id,posting_clock,design_clock,version,updated_by,updated_at) values($1,$2,$3,$4,$5,$6) on conflict(space_id) do update set posting_clock=excluded.posting_clock,design_clock=excluded.design_clock,version=excluded.version,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, sid, out.PostingClock, out.DesignClock, out.Version, p.User.ID, now)
		if err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "medhum.time_policy", TargetType: "space", TargetID: &sid, PeriodID: &period.ID, Before: before, After: map[string]any{"policy": out, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) connectorEvents(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.ConnectorEvent{}
	err := s.withConnector(r, p, false, func(q store.Querier, sc *scope, c *api.Connector) error {
		rows, err := q.Query(r.Context(), `select id,actor_id,action,before,after,occurred_at from connector_events where connector_id=$1 order by occurred_at desc,id desc limit 100`, c.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e api.ConnectorEvent
			var b, a []byte
			if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &b, &a, &e.OccurredAt); err != nil {
				return err
			}
			e.Before = json.RawMessage(b)
			e.After = json.RawMessage(a)
			out = append(out, e)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.ConnectorEvent]{Items: out})
	return nil
}
