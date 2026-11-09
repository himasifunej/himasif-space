package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func (s *Server) sopTransferTarget(r *http.Request, q store.Querier, p *principal, source api.SOPFinding, did id.ID, rule string, lock string) (*sopAccess, api.SOPPolicy, *api.SOPFinding, error) {
	if did == source.DivisionID || did == id.Nil || !sopRuleNumber.MatchString(rule) {
		return nil, api.SOPPolicy{}, nil, badRequest("Pilih divisi lain dan nomor aturan tujuan yang valid.")
	}
	rr := r.Clone(r.Context())
	rr.SetPathValue("divisionId", did.String())
	ac, err := s.loadSOPAccess(rr, q, p, store.LockNone)
	if err != nil {
		return nil, api.SOPPolicy{}, nil, err
	}
	if !ac.read || !ac.review {
		return nil, api.SOPPolicy{}, nil, errForbidden
	}
	if ac.scope.closed() {
		return nil, api.SOPPolicy{}, nil, errPeriodClosed
	}
	content, err := s.store.Medhum(r.Context(), q, source.ContentID, store.LockShare)
	if err != nil {
		return nil, api.SOPPolicy{}, nil, err
	}
	if content.PeriodID != source.PeriodID || (did != content.HumasDivisionID && did != content.DesignDivisionID) {
		return nil, api.SOPPolicy{}, nil, errNotFound
	}
	policy, err := s.store.SOPPolicy(r.Context(), q, source.PeriodID, did, store.LockNone)
	if err != nil {
		return nil, policy, nil, err
	}
	if policy.Version == 0 || policy.SpaceID != content.SpaceID {
		return nil, policy, nil, conflict("Tetapkan kebijakan SOP tujuan pada ruang konten ini terlebih dahulu.")
	}
	var fid id.ID
	err = q.QueryRow(r.Context(), `select id from sop_findings where content_id=$1 and division_id=$2 and rule=$3`, source.ContentID, did, rule).Scan(&fid)
	if errors.Is(err, pgx.ErrNoRows) {
		return ac, policy, nil, nil
	}
	if err != nil {
		return nil, policy, nil, err
	}
	finding, err := s.store.SOPFinding(r.Context(), q, fid, lock)
	return ac, policy, &finding, err
}
func (s *Server) sopTransferPreview(w http.ResponseWriter, r *http.Request, p *principal) error {
	fid, err := pathID(r, "findingId")
	if err != nil {
		return err
	}
	did, err := id.Parse(r.URL.Query().Get("divisionId"))
	if err != nil {
		return badRequest("Divisi tujuan tidak valid.")
	}
	rule := r.URL.Query().Get("rule")
	out := api.SOPTransferPreview{People: []api.SOPPerson{}}
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, source *sopAccess) error {
		if !source.review {
			return errForbidden
		}
		if source.scope.closed() {
			return errPeriodClosed
		}
		finding, err := s.store.SOPFinding(r.Context(), q, fid, store.LockNone)
		if err != nil {
			return err
		}
		if finding.PeriodID != source.scope.period.ID || finding.DivisionID != source.scope.division.ID {
			return errNotFound
		}
		_, policy, target, err := s.sopTransferTarget(r, q, p, finding, did, rule, store.LockNone)
		if err != nil {
			return err
		}
		out.PolicyVersion = policy.Version
		out.DocumentVersion = policy.DocumentVersion
		out.Finding = target
		rows, err := q.Query(r.Context(), `select u.id,u.display_name,bool_or(m.removed_at is null and u.disabled_at is null) from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and u.kind='person' group by u.id,u.display_name order by u.display_name,u.id`, finding.PeriodID, did)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var person api.SOPPerson
			if err = rows.Scan(&person.ID, &person.DisplayName, &person.Active); err != nil {
				return err
			}
			out.People = append(out.People, person)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) sopTransferFinding(w http.ResponseWriter, r *http.Request, p *principal) error {
	fid, err := pathID(r, "findingId")
	if err != nil {
		return err
	}
	var req api.SOPTransferRequest
	if err = readJSON(r, &req); err != nil {
		return err
	}
	req.Reason, err = text(req.Reason, "alasan ralat lintas divisi", 1, 4000)
	if err != nil {
		return err
	}
	extra := []id.ID{}
	if req.TargetPersonID != nil && *req.TargetPersonID != id.Nil {
		extra = append(extra, *req.TargetPersonID)
	}
	var result api.SOPTransfer
	err = s.withSOPScope(r, p, "review", extra, func(q store.Querier, ac *sopAccess) error {
		source, err := s.store.SOPFinding(r.Context(), q, fid, store.LockUpdate)
		if err != nil {
			return err
		}
		if source.PeriodID != ac.scope.period.ID || source.DivisionID != ac.scope.division.ID {
			return errNotFound
		}
		if source.Version != req.ExpectedVersion || source.Latest.ID != req.ObservationID {
			return conflict("Temuan sumber berubah; periksa ulang sebelum meralat.")
		}
		_, policy, target, err := s.sopTransferTarget(r, q, p, source, req.TargetDivisionID, req.TargetRule, store.LockUpdate)
		if err != nil {
			return err
		}
		if policy.Version != req.TargetPolicyVersion {
			return conflict("Kebijakan tujuan berubah; muat ulang pratinjau ralat.")
		}
		if target == nil && req.TargetFindingVersion != 0 || target != nil && target.Version != req.TargetFindingVersion {
			return conflict("Temuan tujuan berubah; muat ulang pratinjau ralat.")
		}
		person := req.TargetPersonID
		if person != nil && *person == id.Nil {
			person = nil
		}
		if person != nil {
			var member bool
			if err = q.QueryRow(r.Context(), `select exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and m.user_id=$3 and u.kind='person')`, source.PeriodID, req.TargetDivisionID, *person).Scan(&member); err != nil {
				return err
			}
			if !member {
				return badRequest("Atribusi tujuan harus memiliki riwayat keanggotaan pada divisi/periode tujuan.")
			}
		}
		content, err := s.store.Medhum(r.Context(), q, source.ContentID, store.LockShare)
		if err != nil {
			return err
		}
		input, pending, err := s.sopInput(r.Context(), q, ac.scope.period, policy, content)
		if err != nil {
			return err
		}
		// A transferred suspicion is never proof of guilt in a different division.
		observation := sop.Observation{Rule: req.TargetRule, Measurement: "insufficient", Evidence: source.Latest.Observation.Evidence, Reason: "Ralat atribusi lintas divisi: " + req.Reason + ". Periksa aturan dan bukti tujuan sebelum membuat keputusan."}
		snap := sopSnapshot{EngineVersion: sop.EngineVersion, SourceFingerprint: sopSourceFingerprint(policy, content, input, pending, req.TargetRule), ProposedPersonID: person, Input: input, Observation: observation, PendingSourceChanges: pending, EventOn: source.EventOn, ContentTitle: content.Title}
		targetID := id.New()
		if target != nil {
			targetID = target.ID
		}
		now := s.now()
		if target == nil {
			if _, err = q.Exec(r.Context(), `insert into sop_findings(id,period_id,division_id,content_id,rule,event_on,policy_id,person_id,created_at,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, targetID, source.PeriodID, req.TargetDivisionID, source.ContentID, req.TargetRule, source.EventOn, policy.ID, person, now); err != nil {
				return err
			}
		}
		oid := id.New()
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into sop_observations(id,finding_id,policy_version,document_version,fingerprint,measurement,data,occurred_at) values($1,$2,$3,$4,$5,'insufficient',$6,$7)`, oid, targetID, policy.Version, policy.DocumentVersion, integrations.Hash(snap), data, now); err != nil {
			return err
		}
		// Existing decisions remain in history. The explicit transfer opens a new
		// review rather than silently retaining a confirmed target attribution.
		if _, err = q.Exec(r.Context(), `update sop_findings set latest_observation_id=$2,decision_observation_id=$2,state='needs_review',person_id=$3,event_on=$4,needs_recheck=false,version=version+case when $5 then 1 else 0 end,updated_at=$6 where id=$1`, targetID, oid, person, source.EventOn, target != nil, now); err != nil {
			return err
		}
		beforeTarget := any(nil)
		if target != nil {
			beforeTarget = target
		}
		targetAfter, err := s.store.SOPFinding(r.Context(), q, targetID, store.LockNone)
		if err != nil {
			return err
		}
		targetAfter.DecisionReason = req.Reason
		targetAfter.DecidedBy = p.User.DisplayName
		targetAfter.DecidedAt = &now
		if err = s.recordSOPTransferDecision(r, q, p, targetID, oid, "reopen", req.Reason, beforeTarget, targetAfter); err != nil {
			return err
		}
		beforeSource := source
		source.State = "dismissed"
		source.NeedsRecheck = false
		source.Version++
		source.UpdatedAt = now
		source.DecisionObservationID = &source.Latest.ID
		if _, err = q.Exec(r.Context(), `update sop_findings set state='dismissed',decision_observation_id=$2,needs_recheck=false,version=$3,updated_at=$4 where id=$1`, fid, source.Latest.ID, source.Version, now); err != nil {
			return err
		}
		sourceAfter, err := s.store.SOPFinding(r.Context(), q, fid, store.LockNone)
		if err != nil {
			return err
		}
		sourceAfter.DecisionReason = req.Reason
		sourceAfter.DecidedBy = p.User.DisplayName
		sourceAfter.DecidedAt = &now
		if err = s.recordSOPTransferDecision(r, q, p, fid, source.Latest.ID, "correct", req.Reason, beforeSource, sourceAfter); err != nil {
			return err
		}
		result = api.SOPTransfer{ID: id.New(), SourceFindingID: fid, TargetFindingID: targetID, SourceDivisionID: source.DivisionID, TargetDivisionID: req.TargetDivisionID, SourceRule: source.Rule, TargetRule: req.TargetRule, ActorName: p.User.DisplayName, Reason: req.Reason, CreatedAt: now}
		if _, err = q.Exec(r.Context(), `insert into sop_transfers(id,source_finding_id,target_finding_id,source_observation_id,target_observation_id,actor_id,reason,created_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, result.ID, fid, targetID, source.Latest.ID, oid, p.User.ID, req.Reason, now); err != nil {
			return err
		}
		for _, scope := range []struct{ did, fid id.ID }{{source.DivisionID, fid}, {req.TargetDivisionID, targetID}} {
			if err = s.record(r, q, p, store.AuditEvent{Action: "sop.finding.transfer", TargetType: "sop_finding", TargetID: &scope.fid, PeriodID: &source.PeriodID, ScopeType: "division", ScopeID: &scope.did, After: result}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, result)
	return nil
}
func (s *Server) recordSOPTransferDecision(r *http.Request, q store.Querier, p *principal, fid, oid id.ID, action, reason string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = q.Exec(r.Context(), `insert into sop_decisions(id,finding_id,observation_id,actor_id,action,reason,before_data,after_data,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id.New(), fid, oid, p.User.ID, action, reason, b, a, s.now())
	return err
}
