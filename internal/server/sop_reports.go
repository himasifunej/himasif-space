package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func summarizeSOP(findings []api.SOPFinding) api.SOPReportSummary {
	out := api.SOPReportSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.State {
		case "needs_review":
			out.NeedsReview++
		case "confirmed":
			out.Confirmed++
		case "dismissed":
			out.Dismissed++
		}
		if f.NeedsRecheck {
			out.NeedsRecheck++
		}
		if f.Latest.Observation.Measurement == "insufficient" {
			out.InsufficientEvidence++
		}
	}
	return out
}
func (s *Server) makeSOPReport(ctx context.Context, q store.Querier, sc *scope, policy api.SOPPolicy, label string, filter store.SOPFilter) (api.SOPReport, error) {
	out := api.SOPReport{PeriodID: sc.period.ID, DivisionID: sc.division.ID, DivisionName: sc.division.Name, Label: label, StartsOn: *filter.From, EndsOn: *filter.To, GeneratedAt: s.now(), PolicyVersion: policy.Version, DocumentVersion: policy.DocumentVersion}
	var err error
	out.Findings, err = s.store.SOPReportFindings(ctx, q, sc.period.ID, sc.division.ID, filter)
	if err != nil {
		return out, err
	}
	out.Summary = summarizeSOP(out.Findings)
	return out, nil
}
func sopReportText(report api.SOPReport) string { return sopReportTextLanguage(report, "id") }
func sopReportTextLanguage(report api.SOPReport, language string) string {
	summary := report.Summary
	return waTextLanguage(language, "report.body", report.DivisionName, report.Label, report.StartsOn.String(), report.EndsOn.String(), summary.NeedsReview, summary.Confirmed, summary.Dismissed, summary.NeedsRecheck, summary.InsufficientEvidence)
}
func (s *Server) sopReport(w http.ResponseWriter, r *http.Request, p *principal) error {
	filter, err := parseSOPFilter(r)
	if err != nil {
		return err
	}
	var report api.SOPReport
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		sc := ac.scope
		policy, err := s.store.SOPPolicy(r.Context(), q, sc.period.ID, sc.division.ID, store.LockNone)
		if err != nil {
			return err
		}
		label := "Rentang pilihan"
		if quarter := r.URL.Query().Get("quarter"); quarter != "" {
			found := false
			for _, rangeQ := range policy.Settings.Quarters {
				if strings.EqualFold(rangeQ.Label, quarter) {
					from, to := rangeQ.StartsOn, rangeQ.EndsOn
					filter.From = &from
					filter.To = &to
					label = rangeQ.Label
					found = true
					break
				}
			}
			if !found {
				return badRequest("Quarter belum dikonfigurasi; tidak memakai batas kalender asumsi.")
			}
		}
		if filter.From == nil || filter.To == nil {
			return badRequest("Tentukan awal/akhir rentang atau label quarter yang dikonfigurasi.")
		}
		if filter.From.Before(sc.period.StartsOn) || filter.To.Compare(sc.period.EndsOn) > 0 {
			return badRequest("Rentang laporan berada di luar periode ini.")
		}
		report, err = s.makeSOPReport(r.Context(), q, sc, policy, label, filter)
		return err
	})
	if err != nil {
		return err
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="space-sop.csv"`)
		writer := csv.NewWriter(w)
		if err = writer.Write([]string{"finding_id", "event_on", "division", "content", "rule", "person", "state", "needs_recheck", "current_measurement", "decision_measurement", "decision_sop_version", "decision_reason", "decided_by", "source_urls"}); err != nil {
			return err
		}
		for _, f := range report.Findings {
			measurement, document := "", ""
			if f.Decision != nil {
				measurement = f.Decision.Observation.Measurement
				document = f.Decision.DocumentVersion
			}
			urls := []string{}
			source := f.Latest
			if f.Decision != nil {
				source = *f.Decision
			}
			for _, e := range source.Observation.Evidence {
				if e.URL != "" {
					urls = append(urls, e.URL)
				}
			}
			row := []string{f.ID.String(), f.EventOn.String(), report.DivisionName, f.ContentTitle, f.Rule, f.PersonName, f.State, fmt.Sprint(f.NeedsRecheck), f.Latest.Observation.Measurement, measurement, document, f.DecisionReason, f.DecidedBy, strings.Join(urls, "\n")}
			for index := range row {
				row[index] = spreadsheetCell(row[index])
			}
			if err = writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	}
	if r.URL.Query().Get("summaryOnly") == "true" {
		report.Findings = nil
	}
	writeJSON(w, 200, report)
	return nil
}
func spreadsheetCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

type sopReportReference struct {
	FindingID           id.ID         `json:"findingId"`
	LatestObservation   id.ID         `json:"latestObservationId"`
	DecisionObservation *id.ID        `json:"decisionObservationId,omitempty"`
	State               string        `json:"state"`
	PersonID            *id.ID        `json:"personId,omitempty"`
	EventOn             timeutil.Date `json:"eventOn"`
	NeedsRecheck        bool          `json:"needsRecheck"`
	DecisionReason      string        `json:"decisionReason"`
	DecidedAt           *time.Time    `json:"decidedAt,omitempty"`
	DecidedBy           string        `json:"decidedBy"`
}
type sopStoredReport struct {
	Report     api.SOPReport        `json:"report"`
	References []sopReportReference `json:"references"`
}

func compactSOPReport(report api.SOPReport) sopStoredReport {
	out := sopStoredReport{Report: report, References: []sopReportReference{}}
	for _, f := range report.Findings {
		out.References = append(out.References, sopReportReference{FindingID: f.ID, LatestObservation: f.Latest.ID, DecisionObservation: f.DecisionObservationID, State: f.State, PersonID: f.PersonID, EventOn: f.EventOn, NeedsRecheck: f.NeedsRecheck, DecisionReason: f.DecisionReason, DecidedAt: f.DecidedAt, DecidedBy: f.DecidedBy})
	}
	out.Report.Findings = nil
	return out
}

func (s *Server) scheduleSOPReport(ctx context.Context, ref sopPolicyRef) error {
	return s.store.Tx(ctx, func(q store.Querier) error {
		var acquired bool
		if err := q.QueryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1,0))`, "sop-weekly:"+ref.Period.String()+":"+ref.Division.String()).Scan(&acquired); err != nil {
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
		if !policy.Enabled || policy.Settings.WeeklyWeekday == nil || policy.Settings.WeeklyClock == "" {
			return nil
		}
		loc, err := timeutil.LoadZone(period.TimeZone)
		if err != nil {
			return err
		}
		local := s.now().In(loc)
		today := timeutil.DateOf(local)
		if int(local.Weekday()) != *policy.Settings.WeeklyWeekday {
			return nil
		}
		clock, err := time.Parse("15:04", policy.Settings.WeeklyClock)
		if err != nil {
			return err
		}
		slot := today.At(clock.Hour(), clock.Minute(), loc)
		if slot.After(s.now()) {
			return nil
		}
		var existing bool
		if err = q.QueryRow(ctx, `select exists(select 1 from sop_report_runs r join wa_outbox o on o.id=r.outbox_id where r.period_id=$1 and r.division_id=$2 and (r.scheduled_at at time zone $3)::date=$4 and (r.policy_version=$5 or o.status in ('sending','unknown','accepted','delivered','read')))`, ref.Period, ref.Division, period.TimeZone, today, policy.Version).Scan(&existing); err != nil {
			return err
		}
		if existing {
			return nil
		}
		var connID, gid, sid id.ID
		var jid string
		err = q.QueryRow(ctx, `select w.id,g.id,g.space_id,g.jid from wa_connections w join wa_groups g on g.connection_id=w.id join spaces sp on sp.id=g.space_id join space_divisions sd on sd.space_id=sp.id and sd.division_id=w.division_id where w.period_id=$1 and w.division_id=$2 and w.enabled and sp.kind='division' order by g.id limit 1`, ref.Period, ref.Division).Scan(&connID, &gid, &sid, &jid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		division, err := s.store.PeriodDivision(ctx, q, ref.Period, ref.Division)
		if err != nil {
			return err
		}
		sc := &scope{period: period, division: division}
		from, to := today.AddDays(-7), today.AddDays(-1)
		// Only the portion inside this period is reported on its first week.
		if from.Before(period.StartsOn) {
			from = period.StartsOn
		}
		if to.Compare(period.EndsOn) > 0 {
			to = period.EndsOn
		}
		if to.Before(from) {
			return nil
		}
		report, err := s.makeSOPReport(ctx, q, sc, policy, "Mingguan", store.SOPFilter{From: &from, To: &to})
		if err != nil {
			return err
		}
		conn, err := s.store.WAConnection(ctx, q, connID, store.LockShare)
		if err != nil {
			return err
		}
		report.Label = waTextLanguage(conn.Language, "report.weekly")
		rid, oid := id.New(), id.New()
		snapshot, err := json.Marshal(compactSOPReport(report))
		if err != nil {
			return err
		}
		body, err := json.Marshal(whatsapp.SendRequest{ID: oid, GroupJID: jid, Text: renderWAText(conn, "weekly", waTextLanguage(conn.Language, "report.title"), sopReportTextLanguage(report, conn.Language)), Mentions: []string{}})
		if err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `update wa_outbox o set status='cancelled',last_error='Kebijakan rekap berubah sebelum pengiriman.' from sop_report_runs r where r.outbox_id=o.id and r.period_id=$1 and r.division_id=$2 and o.status in ('queued','failed') and r.policy_version<>$3`, ref.Period, ref.Division, policy.Version); err != nil {
			return err
		}
		key := fmt.Sprintf("sop-weekly:%s:%s:%s:v%d", ref.Period, ref.Division, today.String(), policy.Version)
		if _, err = q.Exec(ctx, `insert into sop_report_runs(id,period_id,division_id,policy_id,policy_version,scheduled_at,event_key,range_start,range_end,snapshot,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, rid, ref.Period, ref.Division, policy.ID, policy.Version, slot, key, from, to, snapshot, s.now()); err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,payload,available_at,created_at,updated_at) values($1,$2,$3,$4,'sop_report',$5,$6,$7,$8,$8)`, oid, key, connID, gid, rid, body, slot, s.now()); err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `update sop_report_runs set outbox_id=$2 where id=$1`, rid, oid); err != nil {
			return err
		}
		r := (&http.Request{}).WithContext(ctx)
		return s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Source: "sop", Action: "sop.report.queue", TargetType: "sop_report", TargetID: &rid, PeriodID: &ref.Period, ScopeType: "division", ScopeID: &ref.Division, After: compactSOPReport(report)})
	})
}

func (s *Server) validSOPReport(ctx context.Context, q store.Querier, rid *id.ID, conn api.WAConnection, spaceID id.ID) (bool, error) {
	if rid == nil {
		return false, nil
	}
	var policyID, pid, did id.ID
	var version int64
	var slot time.Time
	err := q.QueryRow(ctx, `select policy_id,period_id,division_id,policy_version,scheduled_at from sop_report_runs where id=$1`, *rid).Scan(&policyID, &pid, &did, &version, &slot)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if conn.PeriodID != pid || conn.DivisionID != did {
		return false, nil
	}
	var ownDivision bool
	if err = q.QueryRow(ctx, `select exists(select 1 from spaces sp join space_divisions sd on sd.space_id=sp.id where sp.id=$1 and sp.period_id=$2 and sp.kind='division' and sd.division_id=$3)`, spaceID, pid, did).Scan(&ownDivision); err != nil {
		return false, err
	}
	if !ownDivision {
		return false, nil
	}
	policy, err := s.store.SOPPolicy(ctx, q, pid, did, store.LockNone)
	if err != nil {
		return false, err
	}
	if !policy.Enabled || policy.ID != policyID || policy.Version != version {
		return false, nil
	}
	period, err := s.store.Period(ctx, q, pid, store.LockNone)
	if err != nil {
		return false, err
	}
	loc, err := timeutil.LoadZone(period.TimeZone)
	if err != nil {
		return false, err
	}
	return timeutil.DateOf(slot.In(loc)).Compare(timeutil.DateOf(s.now().In(loc))) == 0, nil
}
