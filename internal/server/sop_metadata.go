package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"net/http"
)

func (s *Server) sopPeople(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.SOPPerson{}
	err := s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		rows, err := q.Query(r.Context(), `select u.id,u.display_name,exists(select 1 from memberships m where m.user_id=u.id and m.period_id=$1 and m.division_id=$2 and m.removed_at is null) and u.disabled_at is null from users u where u.kind='person' and exists(select 1 from memberships m where m.user_id=u.id and m.period_id=$1 and m.division_id=$2) order by u.display_name,u.id`, ac.scope.period.ID, ac.scope.division.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var person api.SOPPerson
			if err := rows.Scan(&person.ID, &person.DisplayName, &person.Active); err != nil {
				return err
			}
			out = append(out, person)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.SOPPerson]{Items: out})
	return nil
}
func (s *Server) sopEvaluatorCandidates(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.SOPEvaluatorCandidate{}
	err := s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		if !ac.configure {
			return errForbidden
		}
		rows, err := q.Query(r.Context(), `select u.id,u.display_name,d.name,d.slug='litbang' from memberships m join users u on u.id=m.user_id join divisions d on d.id=m.division_id where m.period_id=$1 and m.removed_at is null and u.kind='person' and u.disabled_at is null and u.email_verified_at is not null and d.slug in ('litbang','bph') order by d.name,u.display_name,u.id`, ac.scope.period.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var candidate api.SOPEvaluatorCandidate
			if err := rows.Scan(&candidate.ID, &candidate.DisplayName, &candidate.DivisionName, &candidate.CanReview); err != nil {
				return err
			}
			out = append(out, candidate)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.SOPEvaluatorCandidate]{Items: out})
	return nil
}
func (s *Server) sopContents(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.SOPContentRef{}
	err := s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		rows, err := q.Query(r.Context(), `select c.id,c.title,c.posting_date::text,c.posting_at,case when c.humas_division_id=$2 then c.humas_pj_id else c.design_pj_id end,coalesce(u.display_name,'') from medhum_contents c left join users u on u.id=case when c.humas_division_id=$2 then c.humas_pj_id else c.design_pj_id end where c.period_id=$1 and (c.humas_division_id=$2 or c.design_division_id=$2) order by coalesce(c.posting_at,c.posting_date::timestamptz),c.id`, ac.scope.period.ID, ac.scope.division.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var content api.SOPContentRef
			var date *string
			if err := rows.Scan(&content.ID, &content.Title, &date, &content.PostingAt, &content.PersonID, &content.PersonName); err != nil {
				return err
			}
			if date != nil {
				d, err := timeutil.ParseDate(*date)
				if err != nil {
					return err
				}
				content.PostingDate = &d
			}
			out = append(out, content)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.SOPContentRef]{Items: out})
	return nil
}
