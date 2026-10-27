package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// createPeriod adds a period in preparation, with the divisions chosen and
// their spaces. People, roles and work are not copied (PER-02).
func (s *Server) createPeriod(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	var req api.CreatePeriodRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	label, err := text(req.Label, "label periode", 1, 40)
	if err != nil {
		return err
	}
	if req.StartsOn.IsZero() || req.EndsOn.IsZero() {
		return badRequest("Isi tanggal mulai dan selesai periode.")
	}
	if req.EndsOn.Before(req.StartsOn) {
		return badRequest("Tanggal selesai periode tidak boleh sebelum tanggal mulai.")
	}
	tz := req.TimeZone
	if tz == "" {
		tz = timeutil.DefaultZoneName
	}
	if _, err := timeutil.LoadZone(tz); err != nil {
		return badRequest("Zona waktu tidak dikenal.")
	}
	all, err := s.store.Divisions(r.Context())
	if err != nil {
		return err
	}
	known := map[id.ID]bool{}
	for _, d := range all {
		known[d.ID] = true
	}
	var divisionIDs []id.ID
	seen := map[id.ID]bool{}
	for _, d := range req.DivisionIDs {
		if !known[d] {
			return badRequest("Ada divisi yang tidak dikenal.")
		}
		if !seen[d] {
			seen[d] = true
			divisionIDs = append(divisionIDs, d)
		}
	}
	if len(divisionIDs) == 0 {
		for _, d := range all {
			divisionIDs = append(divisionIDs, d.ID)
		}
	}
	var created api.Period
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		var err error
		created, err = s.store.CreatePeriod(r.Context(), q, api.Period{Label: label, StartsOn: req.StartsOn, EndsOn: req.EndsOn, TimeZone: tz, Status: api.PeriodPreparing})
		if errors.Is(err, store.ErrConflict) {
			return conflict("Label periode sudah dipakai.")
		}
		if err != nil {
			return err
		}
		spaces, err := s.store.EnsurePeriodStructure(r.Context(), q, created.ID, divisionIDs)
		if err != nil {
			return err
		}
		if err := s.record(r, q, p, store.AuditEvent{Action: "period.create", PeriodID: &created.ID,
			TargetType: "period", TargetID: &created.ID, After: map[string]any{"period": created, "divisionIds": divisionIDs}}); err != nil {
			return err
		}
		for _, sp := range spaces {
			if err := s.record(r, q, p, store.AuditEvent{Action: "space.create", PeriodID: &created.ID,
				TargetType: "space", TargetID: &sp.ID, After: map[string]any{"code": sp.Code, "name": sp.Name, "kind": sp.Kind}}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, created)
	return nil
}

func (s *Server) activatePeriod(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.changePeriodStatus(w, r, p, "period.activate", func(cur string) (string, error) {
		switch cur {
		case api.PeriodPreparing:
			return api.PeriodActive, nil
		case api.PeriodClosed:
			return "", errPeriodClosed
		}
		return "", conflict("Periode ini sudah aktif.")
	})
}

// closePeriod closes a period for good: it becomes read-only, its data is
// kept (PER-05).
func (s *Server) closePeriod(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.changePeriodStatus(w, r, p, "period.close", func(cur string) (string, error) {
		if cur == api.PeriodClosed {
			return "", errPeriodClosed
		}
		return api.PeriodClosed, nil
	})
}

func (s *Server) changePeriodStatus(w http.ResponseWriter, r *http.Request, p *principal, action string, next func(string) (string, error)) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	var out api.Period
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		cur, err := s.store.Period(r.Context(), q, pid, store.LockUpdate)
		if err != nil {
			return notFound(err, errNotFound)
		}
		status, err := next(cur.Status)
		if err != nil {
			return err
		}
		if out, err = s.store.SetPeriodStatus(r.Context(), q, pid, status, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: action, PeriodID: &pid, TargetType: "period", TargetID: &pid,
			Before: map[string]any{"status": cur.Status}, After: map[string]any{"status": status}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) getHandover(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	if _, err := s.store.Period(r.Context(), s.store.DB(), pid, store.LockNone); err != nil {
		return notFound(err, errNotFound)
	}
	h, err := s.store.Handover(r.Context(), s.store.DB(), pid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	writeJSON(w, http.StatusOK, h)
	return nil
}

// putHandover writes the handover checklist (PER-07). It records the plan;
// it copies no work and closes nothing.
func (s *Server) putHandover(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	var req api.HandoverRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	h := api.Handover{PeriodID: pid, SuccessorPeriodID: req.SuccessorPeriodID}
	fields := []struct {
		in   string
		out  *string
		name string
		min  int
	}{
		{req.IncomingLeadership, &h.IncomingLeadership, "pengurus/kadiv baru", 0},
		{req.AdminCustodian, &h.AdminCustodian, "pengelola admin organisasi", 0},
		{req.BotManagement, &h.BotManagement, "pengelolaan bot", 0},
		{req.ArchivePlan, &h.ArchivePlan, "rencana arsip", 0},
		{req.RemainingWork, &h.RemainingWork, "pekerjaan tersisa (atau alasan tidak ada)", 1},
	}
	for _, f := range fields {
		v, err := text(f.in, f.name, f.min, 4000)
		if err != nil {
			return err
		}
		*f.out = v
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		period, err := s.store.Period(r.Context(), q, pid, store.LockShare)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if h.SuccessorPeriodID != nil {
			if *h.SuccessorPeriodID == pid {
				return badRequest("Periode penerus harus periode lain.")
			}
			succ, err := s.store.Period(r.Context(), q, *h.SuccessorPeriodID, store.LockNone)
			if err != nil {
				return notFound(err, badRequest("Periode penerus tidak ditemukan."))
			}
			if succ.Status == api.PeriodClosed {
				return badRequest("Periode penerus sudah ditutup.")
			}
		}
		before, err := s.store.Handover(r.Context(), q, pid)
		var prev any
		if err == nil {
			prev = before
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err := s.store.PutHandover(r.Context(), q, h, p.User.ID, s.now()); err != nil {
			return err
		}
		if h, err = s.store.Handover(r.Context(), q, pid); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "period.handover.update", PeriodID: &pid,
			TargetType: "period", TargetID: &pid, Before: prev, After: h})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, h)
	return nil
}

// periodSpaces lists the spaces of a period the caller may open; the
// others are left out, not shown as locked.
func (s *Server) periodSpaces(w http.ResponseWriter, r *http.Request, p *principal) error {
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	period, err := s.store.Period(ctx, q, pid, store.LockNone)
	if err != nil {
		return notFound(err, errNotFound)
	}
	spaces, err := s.store.Spaces(ctx, q, pid)
	if err != nil {
		return err
	}
	views, err := s.spaceViews(ctx, q, p, period, spaces)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.Space]{Items: views})
	return nil
}

func (s *Server) getSpace(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	v, err := s.spaceView(r.Context(), s.store.DB(), p, sid)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

// spaceView returns one space with the caller's access; errForbidden when
// the caller may not open it.
func (s *Server) spaceView(ctx context.Context, q store.Querier, p *principal, sid id.ID) (api.Space, error) {
	sp, err := s.store.SpaceByID(ctx, q, sid)
	if err != nil {
		return api.Space{}, notFound(err, errNotFound)
	}
	period, err := s.store.Period(ctx, q, sp.PeriodID, store.LockNone)
	if err != nil {
		return api.Space{}, err
	}
	views, err := s.spaceViews(ctx, q, p, period, []store.Space{sp})
	if err != nil {
		return api.Space{}, err
	}
	if len(views) == 0 {
		return api.Space{}, errForbidden
	}
	return views[0], nil
}

// divisionAccess answers what the caller may do in a division of a period.
// Having no role there is an answer, not an error.
func (s *Server) divisionAccess(w http.ResponseWriter, r *http.Request, p *principal) error {
	sc, err := s.loadScope(r.Context(), s.store.DB(), r, p, store.LockNone)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, sc.access())
	return nil
}
