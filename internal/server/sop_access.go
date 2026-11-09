package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"slices"
)

type sopAccess struct {
	scope                                               *scope
	read, review, configure, evaluator, evaluatorReview bool
}

func (s *Server) loadSOPAccess(r *http.Request, q store.Querier, p *principal, lock string) (*sopAccess, error) {
	sc, err := s.loadScope(r.Context(), q, r, p, lock)
	if err != nil {
		return nil, err
	}
	ac := &sopAccess{scope: sc}
	if p.admin() {
		return ac, nil
	}
	ac.configure = sc.member != nil && sc.member.Role == api.RoleKadiv
	ac.review = slices.Contains(sc.caps, api.CapSOPReview)
	ac.read = slices.Contains(sc.caps, api.CapSOPRead) || ac.review
	var grant, canReview bool
	if err = q.QueryRow(r.Context(), `select exists(select 1 from sop_evaluator_grants g where g.period_id=$1 and g.division_id=$2 and g.user_id=$3 and g.revoked_at is null),exists(select 1 from sop_evaluator_grants g where g.period_id=$1 and g.division_id=$2 and g.user_id=$3 and g.revoked_at is null and g.can_review)`, sc.period.ID, sc.division.ID, p.User.ID).Scan(&grant, &canReview); err != nil {
		return nil, err
	}
	if grant {
		var litbang, bph bool
		if err = q.QueryRow(r.Context(), `select exists(select 1 from memberships m join divisions d on d.id=m.division_id where m.period_id=$1 and m.user_id=$2 and m.removed_at is null and d.slug='litbang'),exists(select 1 from memberships m join divisions d on d.id=m.division_id where m.period_id=$1 and m.user_id=$2 and m.removed_at is null and d.slug='bph')`, sc.period.ID, p.User.ID).Scan(&litbang, &bph); err != nil {
			return nil, err
		}
		if bph {
			ac.read = true
		}
		if litbang {
			ac.evaluator = true
			ac.evaluatorReview = canReview
			ac.read = true
			ac.review = ac.review || canReview
		}
	}
	return ac, nil
}
func (s *Server) withSOPScope(r *http.Request, p *principal, mode string, extra []id.ID, fn func(store.Querier, *sopAccess) error) error {
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		lock := store.LockShare
		if mode != "read" {
			lock = store.LockUpdate
		}
		ac, err := s.loadSOPAccess(r, q, p, lock)
		if err != nil {
			return err
		}
		if !ac.read {
			return errForbidden
		}
		if mode == "configure" && !ac.configure {
			return errForbidden
		}
		if mode == "review" && !ac.review {
			return errForbidden
		}
		if mode != "read" {
			if ac.scope.closed() {
				return errPeriodClosed
			}
			accounts := append(slices.Clone(extra), p.User.ID)
			if err = s.lockTaskAccounts(r.Context(), q, accounts); err != nil {
				return err
			}
			if _, err = s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		return fn(q, ac)
	})
}
func (s *Server) sopScopes(w http.ResponseWriter, r *http.Request, p *principal) error {
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	out := []api.SOPAccess{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		if _, err := s.store.Period(r.Context(), q, pid, store.LockShare); err != nil {
			return err
		}
		rows, err := q.Query(r.Context(), `select d.id from divisions d join period_divisions pd on pd.division_id=d.id where pd.period_id=$1 and d.slug in ('humas','mediatek') order by d.slug`, pid)
		if err != nil {
			return err
		}
		var dids []id.ID
		for rows.Next() {
			var did id.ID
			if err := rows.Scan(&did); err != nil {
				rows.Close()
				return err
			}
			dids = append(dids, did)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, did := range dids {
			rr := r.Clone(r.Context())
			rr.SetPathValue("divisionId", did.String())
			ac, err := s.loadSOPAccess(rr, q, p, store.LockNone)
			if err != nil {
				return err
			}
			if ac.read {
				out = append(out, api.SOPAccess{DivisionSlug: ac.scope.division.Slug, CanApproveBot: ac.evaluatorReview && ac.scope.division.Slug == "humas" && !ac.scope.closed(), PeriodID: pid, DivisionID: did, DivisionName: ac.scope.division.Name, CanReview: ac.review && !ac.scope.closed(), CanConfigure: ac.configure && !ac.scope.closed(), ReadOnly: ac.scope.closed()})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.SOPAccess]{Items: out})
	return nil
}
