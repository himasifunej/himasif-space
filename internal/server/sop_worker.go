package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
	"net/http"
	"slices"
	"time"
)

type sopPolicyRef struct{ Period, Division, Space id.ID }
type sopSnapshot struct {
	EngineVersion        string          `json:"engineVersion"`
	EventOn              timeutil.Date   `json:"eventOn"`
	ContentTitle         string          `json:"contentTitle"`
	SourceFingerprint    string          `json:"sourceFingerprint"`
	ProposedPersonID     *id.ID          `json:"proposedPersonId,omitempty"`
	Input                sop.Input       `json:"input"`
	Observation          sop.Observation `json:"observation"`
	PendingSourceChanges bool            `json:"pendingSourceChanges"`
}
type sopFollowupSnapshot struct {
	CalendarKey        string `json:"calendarKey"`
	Title              string `json:"title"`
	ConceptURL         string `json:"conceptUrl"`
	HumasPJ            id.ID  `json:"humasPjId"`
	DesignPJ           id.ID  `json:"designPjId"`
	BotCountsRequested bool   `json:"botCountsRequested"`
	AgreementID        *id.ID `json:"agreementId,omitempty"`
}

func sopCalendarKey(c api.MedhumContent) string {
	return integrations.Hash([]any{c.ID, broadcast.Normalize(c.Title), c.PostingDate, c.PostingAt, c.HumasPJID, c.DesignPJID, c.ConceptURL})
}
func sopSourceFingerprint(policy api.SOPPolicy, c api.MedhumContent, in sop.Input, pending bool, rule string) string {
	proposed := &c.HumasPJID
	if policy.DivisionID == c.DesignDivisionID {
		proposed = c.DesignPJID
	}
	return integrations.Hash([]any{sop.EngineVersion, policy.Version, policy.DocumentVersion, rule, proposed, in, pending, sopCalendarKey(c)})
}

func (s *Server) RunSOP(ctx context.Context) {
	if !s.cfg.OutboundEnabled() {
		return
	}
	defer s.workerStop("sop")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.workerStart("sop")
		refs, err := s.sopPolicyRefs(ctx)
		if err != nil && ctx.Err() == nil {
			s.workerProgress("sop", true)
			s.log.Info("sop policy scan deferred")
		}
		for _, ref := range refs {
			rows, err := s.store.DB().Query(ctx, `select id from medhum_contents where period_id=$1 and space_id=$2 order by id`, ref.Period, ref.Space)
			if err != nil {
				s.workerProgress("sop", true)
				continue
			}
			contents, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
			if err != nil {
				s.workerProgress("sop", true)
				continue
			}
			for _, cid := range contents {
				if ctx.Err() != nil {
					return
				}
				work, cancel := context.WithTimeout(ctx, 15*time.Second)
				err := s.assessSOPContent(work, ref, cid)
				if err == nil {
					err = s.scheduleSOPFollowup(work, ref, cid)
				}
				cancel()
				s.workerProgress("sop", err != nil)
				if err != nil && ctx.Err() == nil {
					s.log.Info("sop content assessment deferred", "content_id", cid)
				}
			}
		}
		for _, ref := range refs {
			if ctx.Err() != nil {
				return
			}
			work, cancel := context.WithTimeout(ctx, 25*time.Second)
			err := s.scheduleSOPReport(work, ref)
			cancel()
			s.workerProgress("sop", err != nil)
			if err != nil && ctx.Err() == nil {
				s.log.Info("sop weekly report deferred", "division_id", ref.Division)
			}
		}
		s.workerFinish("sop")
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) sopPolicyRefs(ctx context.Context) ([]sopPolicyRef, error) {
	rows, err := s.store.DB().Query(ctx, `select s.period_id,s.division_id,s.space_id from sop_policies s join periods p on p.id=s.period_id where (s.enabled or exists(select 1 from sop_findings f where f.policy_id=s.id)) and p.status<>'closed' order by s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []sopPolicyRef{}
	for rows.Next() {
		var ref sopPolicyRef
		if err := rows.Scan(&ref.Period, &ref.Division, &ref.Space); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (s *Server) assessSOPContent(ctx context.Context, ref sopPolicyRef, cid id.ID) error {
	return s.store.Tx(ctx, func(q store.Querier) error {
		var acquired bool
		if err := q.QueryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1,0))`, "sop:"+ref.Division.String()+":"+cid.String()).Scan(&acquired); err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		period, err := s.store.Period(ctx, q, ref.Period, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return nil
		}
		policy, err := s.store.SOPPolicy(ctx, q, ref.Period, ref.Division, store.LockNone)
		if err != nil {
			return err
		}
		c, err := s.store.Medhum(ctx, q, cid, store.LockShare)
		if err != nil {
			return err
		}
		if c.SpaceID != policy.SpaceID || c.PeriodID != policy.PeriodID {
			return nil
		}
		in, pending, err := s.sopInput(ctx, q, period, policy, c)
		if err != nil {
			return err
		}
		proposed := &c.HumasPJID
		if policy.DivisionID == c.DesignDivisionID {
			proposed = c.DesignPJID
		}
		loc, err := timeutil.LoadZone(period.TimeZone)
		if err != nil {
			return err
		}
		rules := slices.Clone(policy.Settings.Rules)
		rows, err := q.Query(ctx, `select rule from sop_findings where content_id=$1 and division_id=$2 order by rule`, cid, policy.DivisionID)
		if err != nil {
			return err
		}
		existingRules, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, rule := range existingRules {
			if !slices.Contains(rules, rule) {
				rules = append(rules, rule)
			}
		}
		for _, rule := range rules {
			var fid id.ID
			var oldSource, state string
			var decision *id.ID
			var previousData []byte
			err = q.QueryRow(ctx, `select f.id,f.state,f.decision_observation_id,coalesce(o.data->>'sourceFingerprint',''),o.data from sop_findings f left join sop_observations o on o.id=f.latest_observation_id where f.content_id=$1 and f.division_id=$2 and f.rule=$3 for update of f`, cid, policy.DivisionID, rule).Scan(&fid, &state, &decision, &oldSource, &previousData)
			exists := err == nil
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			var observation sop.Observation
			if !slices.Contains(sop.Rules, rule) {
				if !exists {
					continue
				}
				var previous sopSnapshot
				if err = json.Unmarshal(previousData, &previous); err != nil {
					return err
				}
				observation = previous.Observation
				observation.Reason = "Sumber kalender/konteks berubah; bukti manual tetap memerlukan pemeriksaan manusia."
			} else if !policy.Enabled || !slices.Contains(policy.Settings.Rules, rule) {
				observation = sop.Observation{Rule: rule, Measurement: "not_applicable", Evidence: []sop.Evidence{}, Reason: "Aturan otomatis dinonaktifkan pada kebijakan terbaru; keputusan sebelumnya tetap tercatat."}
			} else {
				observation, err = sop.Evaluate(rule, in)
				if err != nil {
					return err
				}
			}
			if pending && observation.Measurement != "manual" {
				observation.Measurement = "insufficient"
				observation.Reason = "Perubahan pesan sumber masih perlu ditinjau. Fakta lama dipertahankan; atribusi belum dapat dipastikan."
			}
			if !exists {
				if !policy.Enabled {
					continue
				}
				interesting := observation.Measurement == "late" || observation.Measurement == "exceeds_limit" || observation.Measurement == "insufficient"
				if !interesting || !s.sopDeadlineElapsed(observation.Deadline, c, loc) {
					continue
				}
			}
			snapshot := sopSnapshot{EngineVersion: sop.EngineVersion, ProposedPersonID: proposed, Input: in, Observation: observation, PendingSourceChanges: pending}
			snapshot.SourceFingerprint = sopSourceFingerprint(policy, c, in, pending, rule)
			if exists && oldSource == snapshot.SourceFingerprint {
				continue
			}
			eventOn := sopEventOn(observation, c, loc)
			snapshot.EventOn = eventOn
			snapshot.ContentTitle = c.Title
			if !exists {
				fid = id.New()
				state = "needs_review"
				if _, err = q.Exec(ctx, `insert into sop_findings(id,period_id,division_id,content_id,rule,event_on,policy_id,person_id,created_at,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, fid, policy.PeriodID, policy.DivisionID, c.ID, rule, eventOn, policy.ID, proposed, s.now()); err != nil {
					return err
				}
			}
			oid := id.New()
			data, err := json.Marshal(snapshot)
			if err != nil {
				return err
			}
			if _, err = q.Exec(ctx, `insert into sop_observations(id,finding_id,policy_version,document_version,fingerprint,measurement,data,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, oid, fid, policy.Version, policy.DocumentVersion, snapshot.SourceFingerprint, observation.Measurement, data, s.now()); err != nil {
				return err
			}
			if _, err = q.Exec(ctx, `update sop_findings set latest_observation_id=$2,event_on=case when decision_observation_id is null then $3 else event_on end,person_id=case when decision_observation_id is null then $4 else person_id end,needs_recheck=decision_observation_id is not null,version=version+case when $5 then 1 else 0 end,updated_at=$6 where id=$1`, fid, oid, eventOn, proposed, exists, s.now()); err != nil {
				return err
			}
			r := (&http.Request{}).WithContext(ctx)
			if err = s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Source: "sop", Action: "sop.observation.record", TargetType: "sop_finding", TargetID: &fid, PeriodID: &period.ID, ScopeType: "division", ScopeID: &policy.DivisionID, After: map[string]any{"observationId": oid, "snapshot": snapshot, "humanStatePreserved": state, "decisionObservationId": decision}}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Server) sopDeadlineElapsed(d sop.Deadline, c api.MedhumContent, loc *time.Location) bool {
	if d.At != nil {
		return !s.now().Before(*d.At)
	}
	if d.Date != nil {
		return timeutil.DateOf(s.now().In(loc)).Compare(*d.Date) > 0
	}
	if c.PostingAt != nil {
		return !s.now().Before(*c.PostingAt)
	}
	return c.PostingDate != nil && timeutil.DateOf(s.now().In(loc)).Compare(*c.PostingDate) >= 0
}
func sopEventOn(o sop.Observation, c api.MedhumContent, loc *time.Location) timeutil.Date {
	primary := 0
	if o.Rule == sop.ConceptH7 && len(o.Evidence) > 1 {
		primary = 1
	}
	if len(o.Evidence) > primary {
		e := o.Evidence[primary]
		if e.At != nil {
			return timeutil.DateOf(e.At.In(loc))
		}
		if e.Date != nil {
			return *e.Date
		}
	}
	if o.Deadline.At != nil {
		return timeutil.DateOf(o.Deadline.At.In(loc))
	}
	if o.Deadline.Date != nil {
		return *o.Deadline.Date
	}
	if c.PostingAt != nil {
		return timeutil.DateOf(c.PostingAt.In(loc))
	}
	return *c.PostingDate
}

func (s *Server) sopInput(ctx context.Context, q store.Querier, period api.Period, policy api.SOPPolicy, c api.MedhumContent) (sop.Input, bool, error) {
	humas, err := s.store.SOPContext(ctx, q, c.ID, c.HumasDivisionID)
	if err != nil {
		return sop.Input{}, false, err
	}
	design, err := s.store.SOPContext(ctx, q, c.ID, c.DesignDivisionID)
	if err != nil {
		return sop.Input{}, false, err
	}
	own := humas
	if policy.DivisionID == c.DesignDivisionID {
		own = design
	}
	in := sop.Input{Zone: period.TimeZone, Posting: sop.Deadline{At: c.PostingAt, Date: c.PostingDate}, DesignDeadline: sop.Deadline{At: c.DesignAt, Date: c.DesignDate}, ConceptMature: humas.ConceptMature, Followup: humas.Followup, FinalApproved: design.FinalApproved, UrgentConfirmed: humas.UrgentConfirmed, Urgent: humas.Urgent, UrgentException: humas.UrgentException, SpecialAgreement: own.SpecialAgreement, SpecialAgreementRules: own.SpecialAgreementRules}
	if humas.UrgentExceptionCategory != "" {
		in.UrgentException.Note = "Kategori " + humas.UrgentExceptionCategory + ": " + in.UrgentException.Note
	}
	err = q.QueryRow(ctx, `select posting_clock,design_clock from medhum_policies where space_id=$1`, c.SpaceID).Scan(&in.PostingClock, &in.DesignClock)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return in, false, err
	}
	var conceptAt time.Time
	var itemID id.ID
	err = q.QueryRow(ctx, `select first_at,first_item_id from wa_concept_receipts where content_id=$1 and active order by first_at,id limit 1`, c.ID).Scan(&conceptAt, &itemID)
	if err == nil {
		in.ConceptDelivered = sop.Evidence{At: &conceptAt, Verified: true, Source: "whatsapp_concept", SourceID: itemID.String()}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return in, false, err
	}
	if c.DesignTaskID != nil {
		var proof api.TaskEvidence
		err = q.QueryRow(ctx, `select id,submitted_at,match_status,url,note from task_evidence where task_id=$1 order by recorded_at desc,id desc limit 1`, *c.DesignTaskID).Scan(&proof.ID, &proof.SubmittedAt, &proof.MatchStatus, &proof.URL, &proof.Note)
		if err == nil {
			in.DesignDelivered = sop.Evidence{At: &proof.SubmittedAt, Verified: proof.MatchStatus == api.MatchVerified, Source: "task_evidence", SourceID: proof.ID.String(), URL: proof.URL, Note: proof.Note}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return in, false, err
		}
	}
	if !in.Followup.Verified {
		rows, err := q.Query(ctx, `select f.id,o.accepted_at,f.snapshot from sop_followups f join wa_outbox o on o.id=f.outbox_id where f.content_id=$1 and f.calendar_key=$2 and o.status in ('accepted','delivered','read') and o.accepted_at is not null and coalesce((select a.approved from sop_bot_agreements a where a.policy_id=f.policy_id and a.policy_version=f.policy_version and a.created_at<=o.accepted_at order by a.created_at desc,a.id desc limit 1),false) order by o.accepted_at,f.id`, c.ID, sopCalendarKey(c))
		if err != nil {
			return in, false, err
		}
		for rows.Next() {
			var fid id.ID
			var at time.Time
			var data []byte
			if err = rows.Scan(&fid, &at, &data); err != nil {
				rows.Close()
				return in, false, err
			}
			var snap sopFollowupSnapshot
			if err = json.Unmarshal(data, &snap); err != nil {
				rows.Close()
				return in, false, err
			}
			if snap.BotCountsRequested && snap.AgreementID != nil && snap.Title != "" && snap.ConceptURL != "" && snap.DesignPJ != id.Nil {
				in.Followup = sop.Evidence{At: &at, Verified: true, Source: "whatsapp_followup", SourceID: fid.String(), URL: snap.ConceptURL, Note: "Follow-up bot diterima bridge; kesepakatan versi terkait berlaku pada waktu kirim."}
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return in, false, err
		}
	}
	if policy.Settings.UrgentWindowStart != nil && policy.Settings.UrgentWindowEnd != nil && policy.Settings.UrgentLimit != nil {
		loc, err := timeutil.LoadZone(period.TimeZone)
		if err != nil {
			return in, false, err
		}
		day := c.PostingDate
		if day == nil {
			d := timeutil.DateOf(c.PostingAt.In(loc))
			day = &d
		}
		if day.Compare(*policy.Settings.UrgentWindowStart) >= 0 && day.Compare(*policy.Settings.UrgentWindowEnd) <= 0 {
			rows, err := q.Query(ctx, `select c.id,c.posting_date::text,c.posting_at from medhum_contents c join sop_contexts x on x.content_id=c.id and x.division_id=c.humas_division_id where c.space_id=$1 and coalesce(c.posting_date,(c.posting_at at time zone $2)::date) between $3 and $4 and x.data->>'urgent'='true' and coalesce(x.data->'urgentException'->>'verified','false')<>'true' order by c.id`, c.SpaceID, period.TimeZone, policy.Settings.UrgentWindowStart, policy.Settings.UrgentWindowEnd)
			if err != nil {
				return in, false, err
			}
			count := 0
			ambiguous := false
			for rows.Next() {
				var other id.ID
				var dateText *string
				var at *time.Time
				if err = rows.Scan(&other, &dateText, &at); err != nil {
					rows.Close()
					return in, false, err
				}
				var date timeutil.Date
				if dateText != nil {
					date, err = timeutil.ParseDate(*dateText)
					if err != nil {
						rows.Close()
						return in, false, err
					}
				} else if at != nil {
					date = timeutil.DateOf(at.In(loc))
				} else {
					ambiguous = true
					continue
				}
				if date.Before(*day) {
					count++
					continue
				}
				if date.Compare(*day) > 0 {
					continue
				}
				if other == c.ID {
					count++
					continue
				}
				if at == nil || c.PostingAt == nil || at.Equal(*c.PostingAt) {
					ambiguous = true
					continue
				}
				if at.Before(*c.PostingAt) {
					count++
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return in, false, err
			}
			if !ambiguous {
				in.UrgentCount = &count
			} else {
				in.UrgentCountIssue = "Urutan beberapa konten dadakan pada tanggal/waktu yang sama belum dapat dipastikan; tidak mengatribusikan kelebihan kuota secara otomatis."
			}
			in.UrgentLimit = policy.Settings.UrgentLimit
		}
	}
	var pending bool
	err = q.QueryRow(ctx, `select exists(select 1 from wa_content_items where space_id=$1 and kind='correction' and state='pending' and $2=any(candidates))`, c.SpaceID, c.ID).Scan(&pending)
	return in, pending, err
}
