package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Store) SOPPolicy(ctx context.Context, q Querier, pid, did id.ID, lock string) (api.SOPPolicy, error) {
	var out api.SOPPolicy
	var settings []byte
	suffix := ""
	if lock == LockUpdate {
		suffix = " for update of p"
	}
	err := q.QueryRow(ctx, `select p.id,p.period_id,p.division_id,p.space_id,p.document_version,p.enabled,p.settings,p.version,p.updated_at,coalesce((select a.approved from sop_bot_agreements a where a.policy_id=p.id and a.policy_version=p.version order by a.created_at desc,a.id desc limit 1),false) from sop_policies p where p.period_id=$1 and p.division_id=$2`+suffix, pid, did).Scan(&out.ID, &out.PeriodID, &out.DivisionID, &out.SpaceID, &out.DocumentVersion, &out.Enabled, &settings, &out.Version, &out.UpdatedAt, &out.BotFollowupCountsApproved)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(settings, &out.Settings)
	return out, err
}
func (s *Store) SaveSOPPolicy(ctx context.Context, q Querier, out api.SOPPolicy, actor id.ID, create bool) error {
	settings, err := json.Marshal(out.Settings)
	if err != nil {
		return err
	}
	if create {
		_, err = q.Exec(ctx, `insert into sop_policies(id,period_id,division_id,space_id,document_version,enabled,settings,version,updated_by,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, out.ID, out.PeriodID, out.DivisionID, out.SpaceID, out.DocumentVersion, out.Enabled, settings, out.Version, actor, out.UpdatedAt)
		return err
	}
	_, err = q.Exec(ctx, `update sop_policies set space_id=$2,document_version=$3,enabled=$4,settings=$5,version=$6,updated_by=$7,updated_at=$8 where id=$1`, out.ID, out.SpaceID, out.DocumentVersion, out.Enabled, settings, out.Version, actor, out.UpdatedAt)
	return err
}
func (s *Store) SOPContext(ctx context.Context, q Querier, cid, did id.ID) (api.SOPContext, error) {
	out := api.SOPContext{ContentID: cid, DivisionID: did}
	var data []byte
	var version int64
	err := q.QueryRow(ctx, `select data,version from sop_contexts where content_id=$1 and division_id=$2`, cid, did).Scan(&data, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	out.Version = version
	out.ContentID = cid
	out.DivisionID = did
	return out, nil
}

const sopFindingColumns = `f.id,f.period_id,f.division_id,f.content_id,c.title,f.rule,f.event_on::text,f.person_id,coalesce(u.display_name,''),f.state,f.needs_recheck,f.decision_observation_id,f.version,f.created_at,f.updated_at,o.id,o.document_version,o.policy_version,o.data,o.occurred_at,decobs.id,decobs.document_version,decobs.policy_version,decobs.data,decobs.occurred_at,coalesce(d.reason,''),coalesce(du.display_name,''),d.created_at`
const sopFindingJoin = ` from sop_findings f join medhum_contents c on c.id=f.content_id left join users u on u.id=f.person_id join sop_observations o on o.id=f.latest_observation_id left join sop_observations decobs on decobs.id=f.decision_observation_id left join lateral (select actor_id,reason,created_at from sop_decisions where finding_id=f.id order by created_at desc,id desc limit 1) d on true left join users du on du.id=d.actor_id`

func scanSOPFinding(row pgx.Row) (api.SOPFinding, error) {
	var out api.SOPFinding
	var day string
	var data, decisionData []byte
	var decisionID *id.ID
	var decisionVersion *int64
	var decisionDoc *string
	var decisionRecorded *time.Time
	err := row.Scan(&out.ID, &out.PeriodID, &out.DivisionID, &out.ContentID, &out.ContentTitle, &out.Rule, &day, &out.PersonID, &out.PersonName, &out.State, &out.NeedsRecheck, &out.DecisionObservationID, &out.Version, &out.CreatedAt, &out.UpdatedAt, &out.Latest.ID, &out.Latest.DocumentVersion, &out.Latest.PolicyVersion, &data, &out.Latest.RecordedAt, &decisionID, &decisionDoc, &decisionVersion, &decisionData, &decisionRecorded, &out.DecisionReason, &out.DecidedBy, &out.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.EventOn, err = timeutil.ParseDate(day)
	if err != nil {
		return out, err
	}
	if err = decodeSOPObservation(data, &out.Latest); err != nil {
		return out, err
	}
	if decisionID != nil {
		out.Decision = &api.SOPObservation{ID: *decisionID, DocumentVersion: *decisionDoc, PolicyVersion: *decisionVersion, RecordedAt: *decisionRecorded}
		if err = decodeSOPObservation(decisionData, out.Decision); err != nil {
			return out, err
		}
	}
	return out, nil
}
func decodeSOPObservation(data []byte, out *api.SOPObservation) error {
	var snap struct {
		EngineVersion    string          `json:"engineVersion"`
		Input            json.RawMessage `json:"input"`
		Observation      json.RawMessage `json:"observation"`
		EventOn          timeutil.Date   `json:"eventOn"`
		ContentTitle     string          `json:"contentTitle"`
		ProposedPersonID *id.ID          `json:"proposedPersonId,omitempty"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	out.EngineVersion = snap.EngineVersion
	out.EventOn = snap.EventOn
	out.ContentTitle = snap.ContentTitle
	out.ProposedPersonID = snap.ProposedPersonID
	if err := json.Unmarshal(snap.Input, &out.Input); err != nil {
		return err
	}
	return json.Unmarshal(snap.Observation, &out.Observation)
}

func (s *Store) SOPFinding(ctx context.Context, q Querier, fid id.ID, lock string) (api.SOPFinding, error) {
	suffix := ""
	if lock == LockUpdate {
		suffix = " for update of f"
	}
	return scanSOPFinding(q.QueryRow(ctx, `select `+sopFindingColumns+sopFindingJoin+` where f.id=$1`+suffix, fid))
}

type SOPFilter struct {
	AfterOn     *timeutil.Date
	AfterID     *id.ID
	Limit       int
	From, To    *timeutil.Date
	Person      *id.ID
	Rule, State string
}

func (s *Store) SOPFindings(ctx context.Context, q Querier, pid, did id.ID, f SOPFilter) ([]api.SOPFinding, error) {
	return s.sopFindings(ctx, q, pid, did, f, true)
}
func (s *Store) SOPReportFindings(ctx context.Context, q Querier, pid, did id.ID, f SOPFilter) ([]api.SOPFinding, error) {
	return s.sopFindings(ctx, q, pid, did, f, false)
}
func (s *Store) sopFindings(ctx context.Context, q Querier, pid, did id.ID, f SOPFilter, limited bool) ([]api.SOPFinding, error) {
	whereTail := ""
	limitTail := ""
	args := []any{pid, did, f.From, f.To, f.Person, f.Rule, f.State}
	if limited {
		whereTail = " and ($8::date is null or (f.event_on,f.id)<($8::date,$9::uuid))"
		limitTail = " limit $10"
		limit := f.Limit
		if limit < 1 {
			limit = 101
		}
		limit = min(limit, 251)
		args = append(args, f.AfterOn, f.AfterID, limit)
	}
	rows, err := q.Query(ctx, `select `+sopFindingColumns+sopFindingJoin+` where f.period_id=$1 and f.division_id=$2 and ($3::date is null or f.event_on>=$3) and ($4::date is null or f.event_on<=$4) and ($5::uuid is null or f.person_id=$5) and ($6='' or f.rule=$6) and ($7='' or f.state=$7)`+whereTail+` order by f.event_on desc,f.id desc`+limitTail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.SOPFinding{}
	for rows.Next() {
		finding, err := scanSOPFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, finding)
	}
	return out, rows.Err()
}
