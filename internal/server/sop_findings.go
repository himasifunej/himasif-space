package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var sopRuleNumber = regexp.MustCompile(`^7\.(?:[1-9]|1[0-9]|2[0-5])$`)

func (s *Server) sopFindingByContent(r *http.Request, q store.Querier, cid, did id.ID, rule string) (api.SOPFinding, error) {
	var fid id.ID
	err := q.QueryRow(r.Context(), `select id from sop_findings where content_id=$1 and division_id=$2 and rule=$3`, cid, did, rule).Scan(&fid)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.SOPFinding{}, store.ErrNotFound
	}
	if err != nil {
		return api.SOPFinding{}, err
	}
	return s.store.SOPFinding(r.Context(), q, fid, store.LockUpdate)
}
func validateManualSOPEvidence(e *sop.Evidence, now time.Time, zone string) error {
	if !e.Verified {
		return badRequest("Konfirmasikan bukti yang sudah diperiksa sebelum mencatat temuan manual.")
	}
	if len(e.Note) > 4000 || strings.TrimSpace(e.Note) == "" {
		return badRequest("Catat bukti yang diamati, bukan hanya dugaan tanpa penjelasan.")
	}
	if len(e.URL) > 4096 || e.URL == "" {
		return badRequest("Temuan manual memerlukan tautan bukti yang dapat diperiksa.")
	}
	u, err := url.Parse(e.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || strings.ContainsAny(e.URL, "\r\n") {
		return badRequest("Tautan bukti manual tidak valid.")
	}
	if (e.At == nil) == (e.Date == nil) {
		return badRequest("Pilih tanggal kejadian atau waktu persis bukti manual.")
	}
	loc, err := timeutil.LoadZone(zone)
	if err != nil {
		return err
	}
	if (e.At != nil && e.At.After(now.Add(5*time.Minute))) || (e.Date != nil && e.Date.Compare(timeutil.DateOf(now.In(loc))) > 0) {
		return badRequest("Waktu kejadian bukti tidak boleh berada di masa depan.")
	}
	e.Verified = true
	e.Source = "manual_sop"
	return nil
}

func parseSOPFilter(r *http.Request) (store.SOPFilter, error) {
	f := store.SOPFilter{Rule: r.URL.Query().Get("rule"), State: r.URL.Query().Get("state")}
	for _, field := range []struct {
		key   string
		value **timeutil.Date
	}{{"from", &f.From}, {"to", &f.To}} {
		if raw := r.URL.Query().Get(field.key); raw != "" {
			date, err := timeutil.ParseDate(raw)
			if err != nil {
				return f, badRequest("Rentang tanggal laporan tidak valid.")
			}
			*field.value = &date
		}
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return f, badRequest("Akhir rentang mendahului awalnya.")
	}
	if raw := r.URL.Query().Get("personId"); raw != "" {
		uid, err := id.Parse(raw)
		if err != nil {
			return f, badRequest("Filter penanggung jawab tidak valid.")
		}
		f.Person = &uid
	}
	if f.State != "" && !slices.Contains([]string{"needs_review", "confirmed", "dismissed", "corrected"}, f.State) {
		return f, badRequest("Filter status temuan tidak valid.")
	}
	if len(f.Rule) > 32 {
		return f, badRequest("Filter aturan tidak valid.")
	}
	return f, nil
}
func (s *Server) sopFindings(w http.ResponseWriter, r *http.Request, p *principal) error {
	filter, err := parseSOPFilter(r)
	if err != nil {
		return err
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 250 {
			return badRequest("Ukuran halaman harus 1 sampai 250.")
		}
	}
	filter.Limit = limit + 1
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		if len(raw) > 200 {
			return badRequest("Cursor tidak valid.")
		}
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return badRequest("Cursor tidak valid.")
		}
		parts := strings.Split(string(data), "|")
		if len(parts) != 2 {
			return badRequest("Cursor tidak valid.")
		}
		day, err := timeutil.ParseDate(parts[0])
		if err != nil {
			return badRequest("Cursor tanggal tidak valid.")
		}
		uid, err := id.Parse(parts[1])
		if err != nil {
			return badRequest("Cursor identitas tidak valid.")
		}
		filter.AfterOn = &day
		filter.AfterID = &uid
	}
	out := api.SOPFindingPage{Items: []api.SOPFinding{}, SummaryOnly: true}
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		var err error
		out.Items, err = s.store.SOPFindings(r.Context(), q, ac.scope.period.ID, ac.scope.division.ID, filter)
		if err != nil {
			return err
		}
		out.HasMore = len(out.Items) > limit
		if out.HasMore {
			out.Items = out.Items[:limit]
			last := out.Items[len(out.Items)-1]
			out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.EventOn.String() + "|" + last.ID.String()))
		}
		for index := range out.Items {
			out.Items[index].Latest.Input = sop.Input{}
			out.Items[index].Latest.Observation.Evidence = nil
			if out.Items[index].Decision != nil {
				out.Items[index].Decision.Input = sop.Input{}
				out.Items[index].Decision.Observation.Evidence = nil
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) sopFinding(w http.ResponseWriter, r *http.Request, p *principal) error {
	fid, err := pathID(r, "findingId")
	if err != nil {
		return err
	}
	var out api.SOPFinding
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		var err error
		out, err = s.store.SOPFinding(r.Context(), q, fid, store.LockNone)
		if err != nil {
			return err
		}
		if out.PeriodID != ac.scope.period.ID || out.DivisionID != ac.scope.division.ID {
			return errNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) sopDecision(w http.ResponseWriter, r *http.Request, p *principal) error {
	fid, err := pathID(r, "findingId")
	if err != nil {
		return err
	}
	var req api.SOPDecisionRequest
	if err = readJSON(r, &req); err != nil {
		return err
	}
	req.Reason, err = text(req.Reason, "alasan keputusan SOP", 1, 4000)
	if err != nil {
		return err
	}
	if !slices.Contains([]string{"confirm", "dismiss", "reopen", "correct"}, req.Action) {
		return badRequest("Keputusan SOP tidak valid.")
	}
	outcome := map[string]string{"confirm": "confirmed", "dismiss": "dismissed", "reopen": "needs_review"}[req.Action]
	if req.Action == "correct" {
		if !slices.Contains([]string{"confirmed", "dismissed", "needs_review"}, req.Outcome) {
			return badRequest("Ralat harus menetapkan hasil keputusan yang berlaku.")
		}
		outcome = req.Outcome
	}
	var extra []id.ID
	if req.PersonID != nil {
		extra = append(extra, *req.PersonID)
	}
	var out api.SOPFinding
	err = s.withSOPScope(r, p, "review", extra, func(q store.Querier, ac *sopAccess) error {
		var err error
		out, err = s.store.SOPFinding(r.Context(), q, fid, store.LockUpdate)
		if err != nil {
			return err
		}
		sc := ac.scope
		if out.PeriodID != sc.period.ID || out.DivisionID != sc.division.ID {
			return errNotFound
		}
		if req.ExpectedVersion != out.Version || req.ObservationID != out.Latest.ID {
			return conflict("Observasi/keputusan berubah. Periksa bukti terbaru sebelum memutuskan.")
		}
		if outcome == "confirmed" {
			if !slices.Contains([]string{"late", "exceeds_limit", "manual"}, out.Latest.Observation.Measurement) {
				return conflict("Bukti saat ini belum mendukung pengesahan. Lengkapi konteks/bukti manusia atau batalkan dugaan.")
			}
			policy, err := s.store.SOPPolicy(r.Context(), q, sc.period.ID, sc.division.ID, store.LockNone)
			if err != nil {
				return err
			}
			if policy.Version != out.Latest.PolicyVersion {
				return conflict("Kebijakan telah berubah. Observasi versi terbaru diperlukan sebelum pengesahan.")
			}
			content, err := s.store.Medhum(r.Context(), q, out.ContentID, store.LockShare)
			if err != nil {
				return err
			}
			input, pending, err := s.sopInput(r.Context(), q, sc.period, policy, content)
			if err != nil {
				return err
			}
			var sourceData []byte
			if err = q.QueryRow(r.Context(), `select data from sop_observations where id=$1`, out.Latest.ID).Scan(&sourceData); err != nil {
				return err
			}
			var snapshot sopSnapshot
			if err = json.Unmarshal(sourceData, &snapshot); err != nil {
				return err
			}
			if snapshot.SourceFingerprint != sopSourceFingerprint(policy, content, input, pending, out.Rule) {
				return conflict("Sumber berubah sesudah observasi. Muat observasi terbaru sebelum mengesahkan temuan.")
			}
			if req.PersonID == nil && !sameOptionalID(out.PersonID, out.Latest.ProposedPersonID) {
				return badRequest("Penanggung jawab sumber berubah; pilih atribusi secara eksplisit dengan alasan.")
			}
		}
		before := out
		if req.PersonID != nil {
			if *req.PersonID == id.Nil {
				out.PersonID = nil
				out.PersonName = ""
			} else {
				var name string
				err = q.QueryRow(r.Context(), `select u.display_name from users u where u.id=$1 and u.kind='person' and exists(select 1 from memberships m where m.user_id=u.id and m.period_id=$2 and m.division_id=$3)`, *req.PersonID, sc.period.ID, sc.division.ID).Scan(&name)
				if err != nil {
					return badRequest("Atribusi harus akun pribadi yang memiliki riwayat keanggotaan pada divisi/periode ini.")
				}
				uid := *req.PersonID
				out.PersonID = &uid
				out.PersonName = name
			}
		}
		decisionSnapshot := out.Latest
		out.Decision = &decisionSnapshot
		out.DecisionReason = req.Reason
		out.DecidedBy = p.User.DisplayName
		decidedAt := s.now()
		out.DecidedAt = &decidedAt
		out.State = outcome
		out.NeedsRecheck = false
		oid := out.Latest.ID
		out.DecisionObservationID = &oid
		out.EventOn = out.Latest.EventOn
		out.Version++
		out.UpdatedAt = s.now()
		if _, err = q.Exec(r.Context(), `update sop_findings set state=$2,person_id=$3,decision_observation_id=$4,needs_recheck=false,event_on=$5,version=$6,updated_at=$7 where id=$1`, fid, out.State, out.PersonID, oid, out.EventOn, out.Version, out.UpdatedAt); err != nil {
			return err
		}
		b, err := json.Marshal(before)
		if err != nil {
			return err
		}
		a, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into sop_decisions(id,finding_id,observation_id,actor_id,action,reason,before_data,after_data,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id.New(), fid, oid, p.User.ID, req.Action, req.Reason, b, a, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.finding." + req.Action, TargetType: "sop_finding", TargetID: &fid, PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, Before: before, After: map[string]any{"finding": out, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func sameOptionalID(a, b *id.ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Server) sopFindingHistory(w http.ResponseWriter, r *http.Request, p *principal) error {
	fid, err := pathID(r, "findingId")
	if err != nil {
		return err
	}
	decisions := []api.SOPDecision{}
	observations := []api.SOPObservation{}
	changes := []api.SOPSourceChange{}
	transfers := []api.SOPTransfer{}
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		finding, err := s.store.SOPFinding(r.Context(), q, fid, store.LockNone)
		if err != nil {
			return err
		}
		if finding.PeriodID != ac.scope.period.ID || finding.DivisionID != ac.scope.division.ID {
			return errNotFound
		}
		transferRows, err := q.Query(r.Context(), `select t.id,t.source_finding_id,t.target_finding_id,src.division_id,dst.division_id,src.rule,dst.rule,u.display_name,t.reason,t.created_at from sop_transfers t join sop_findings src on src.id=t.source_finding_id join sop_findings dst on dst.id=t.target_finding_id join users u on u.id=t.actor_id where t.source_finding_id=$1 or t.target_finding_id=$1 order by t.created_at,t.id`, fid)
		if err != nil {
			return err
		}
		for transferRows.Next() {
			var item api.SOPTransfer
			if err = transferRows.Scan(&item.ID, &item.SourceFindingID, &item.TargetFindingID, &item.SourceDivisionID, &item.TargetDivisionID, &item.SourceRule, &item.TargetRule, &item.ActorName, &item.Reason, &item.CreatedAt); err != nil {
				transferRows.Close()
				return err
			}
			transfers = append(transfers, item)
		}
		err = transferRows.Err()
		transferRows.Close()
		if err != nil {
			return err
		}
		rows, err := q.Query(r.Context(), `select d.id,u.display_name,d.action,d.reason,d.before_data,d.after_data,d.created_at from sop_decisions d join users u on u.id=d.actor_id where d.finding_id=$1 order by d.created_at,d.id`, fid)
		if err != nil {
			return err
		}
		for rows.Next() {
			var d api.SOPDecision
			if err := rows.Scan(&d.ID, &d.ActorName, &d.Action, &d.Reason, &d.Before, &d.After, &d.CreatedAt); err != nil {
				rows.Close()
				return err
			}
			decisions = append(decisions, d)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = q.Query(r.Context(), `select sequence,source_type,source_id,occurred_at from sop_source_changes where finding_id=$1 order by sequence`, fid)
		if err != nil {
			return err
		}
		for rows.Next() {
			var change api.SOPSourceChange
			if err := rows.Scan(&change.Sequence, &change.SourceType, &change.SourceID, &change.RecordedAt); err != nil {
				rows.Close()
				return err
			}
			changes = append(changes, change)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = q.Query(r.Context(), `select id,document_version,policy_version,data,occurred_at from sop_observations where finding_id=$1 order by occurred_at,id`, fid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var o api.SOPObservation
			var data []byte
			if err := rows.Scan(&o.ID, &o.DocumentVersion, &o.PolicyVersion, &data, &o.RecordedAt); err != nil {
				return err
			}
			var snap sopSnapshot
			if err = json.Unmarshal(data, &snap); err != nil {
				return err
			}
			o.EngineVersion = snap.EngineVersion
			o.Input = snap.Input
			o.Observation = snap.Observation
			o.EventOn = snap.EventOn
			o.ContentTitle = snap.ContentTitle
			o.ProposedPersonID = snap.ProposedPersonID
			observations = append(observations, o)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.SOPHistory{Transfers: transfers, Decisions: decisions, Observations: observations, SourceChanges: changes})
	return nil
}

// Evidence can establish a manual SOP case without pretending the time
// evaluator can judge visual quality. It is still only a proposal to review.
func (s *Server) manualSOPFinding(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.SOPManualFindingRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var err error
	req.Reason, err = text(req.Reason, "alasan temuan manual", 1, 4000)
	if err != nil {
		return err
	}
	if !sopRuleNumber.MatchString(req.Rule) {
		return badRequest("Nomor aturan harus merujuk poin 7.1 sampai 7.25 dokumen SOP.")
	}
	var extra []id.ID
	if req.PersonID != nil {
		extra = append(extra, *req.PersonID)
	}
	var fid id.ID
	err = s.withSOPScope(r, p, "review", extra, func(q store.Querier, ac *sopAccess) error {
		sc := ac.scope
		policy, err := s.store.SOPPolicy(r.Context(), q, sc.period.ID, sc.division.ID, store.LockNone)
		if err != nil {
			return err
		}
		if policy.Version != req.ExpectedPolicyVersion {
			return conflict("Kebijakan berubah; gunakan versi dokumen terbaru.")
		}
		c, err := s.store.Medhum(r.Context(), q, req.ContentID, store.LockShare)
		if err != nil {
			return err
		}
		if c.PeriodID != sc.period.ID || (c.HumasDivisionID != sc.division.ID && c.DesignDivisionID != sc.division.ID) {
			return errNotFound
		}
		e := req.Evidence
		e.SourceID = p.User.ID.String() + ":" + id.New().String()
		if err = validateManualSOPEvidence(&e, s.now(), sc.period.TimeZone); err != nil {
			return err
		}
		if req.PersonID != nil && *req.PersonID != id.Nil {
			var member bool
			if err = q.QueryRow(r.Context(), `select exists(select 1 from memberships where period_id=$1 and division_id=$2 and user_id=$3)`, sc.period.ID, sc.division.ID, *req.PersonID).Scan(&member); err != nil {
				return err
			}
			if !member {
				return badRequest("Penanggung jawab tidak memiliki riwayat pada divisi ini.")
			}
		}
		current, err := s.sopFindingByContent(r, q, c.ID, sc.division.ID, req.Rule)
		exists := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if exists && current.Version != req.ExpectedFindingVersion {
			return conflict("Temuan sudah ada/berubah; periksa sebelum menambah bukti manual.")
		}
		if !exists && req.ExpectedFindingVersion != 0 {
			return errNotFound
		}
		in, pending, err := s.sopInput(r.Context(), q, sc.period, policy, c)
		if err != nil {
			return err
		}
		loc, err := timeutil.LoadZone(sc.period.TimeZone)
		if err != nil {
			return err
		}
		proposed := req.PersonID
		if exists && req.PersonID == nil {
			proposed = current.PersonID
		}
		if proposed != nil && *proposed == id.Nil {
			proposed = nil
		}
		source := sopSourceFingerprint(policy, c, in, pending, req.Rule)
		o := sop.Observation{Rule: req.Rule, Measurement: "manual", Evidence: []sop.Evidence{e}, Reason: req.Reason}
		day := sopEventOn(o, c, loc)
		if !exists {
			fid = id.New()
			if _, err = q.Exec(r.Context(), `insert into sop_findings(id,period_id,division_id,content_id,rule,event_on,policy_id,person_id,created_at,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, fid, sc.period.ID, sc.division.ID, c.ID, req.Rule, day, policy.ID, proposed, s.now()); err != nil {
				return err
			}
		} else {
			fid = current.ID
		}
		snap := sopSnapshot{EngineVersion: sop.EngineVersion, SourceFingerprint: source, ProposedPersonID: proposed, Input: in, Observation: o, PendingSourceChanges: pending, EventOn: day, ContentTitle: c.Title}
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		oid := id.New()
		if _, err = q.Exec(r.Context(), `insert into sop_observations(id,finding_id,policy_version,document_version,fingerprint,measurement,data,occurred_at) values($1,$2,$3,$4,$5,'manual',$6,$7)`, oid, fid, policy.Version, policy.DocumentVersion, integrations.Hash(snap), data, s.now()); err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `update sop_findings set latest_observation_id=$2,event_on=case when decision_observation_id is null then $3 else event_on end,person_id=case when decision_observation_id is null then $6 else person_id end,needs_recheck=decision_observation_id is not null,version=version+case when $4 then 1 else 0 end,updated_at=$5 where id=$1`, fid, oid, day, exists, s.now(), proposed); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.evidence.manual", TargetType: "sop_finding", TargetID: &fid, PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, After: snap})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, map[string]id.ID{"id": fid})
	return nil
}
