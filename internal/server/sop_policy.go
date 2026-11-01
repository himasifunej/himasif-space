package server

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"slices"
	"strings"
	"time"
)

func sopOwnedRules(slug string) []string {
	if slug == "humas" {
		return []string{sop.ConceptH7, sop.FollowupH4, sop.Urgent24, sop.UrgentLimit}
	}
	if slug == "mediatek" {
		return []string{sop.DesignH2, sop.FinalH12}
	}
	return nil
}
func validateSOPSettings(req *api.SOPPolicyRequest, sc *scope) error {
	var err error
	req.DocumentVersion, err = text(req.DocumentVersion, "versi dokumen SOP", 1, 120)
	if err != nil {
		return err
	}
	req.Reason, err = text(req.Reason, "alasan konfigurasi", 1, 2000)
	if err != nil {
		return err
	}
	if len(req.Settings.Rules) > len(sop.Rules) {
		return badRequest("Daftar aturan terlalu panjang.")
	}
	slices.Sort(req.Settings.Rules)
	req.Settings.Rules = slices.Compact(req.Settings.Rules)
	for _, rule := range req.Settings.Rules {
		if !slices.Contains(sopOwnedRules(sc.division.Slug), rule) {
			return badRequest("Aturan bukan kewenangan kebijakan divisi ini.")
		}
	}
	if req.Enabled && len(req.Settings.Rules) == 0 && req.Settings.FollowupClock == "" && req.Settings.WeeklyClock == "" {
		return badRequest("Pilih aturan atau jadwal follow-up/rekap sebelum mengaktifkan kebijakan.")
	}
	for _, clock := range []string{req.Settings.FollowupClock, req.Settings.WeeklyClock} {
		if clock != "" {
			if _, err := time.Parse("15:04", clock); err != nil || len(clock) != 5 {
				return badRequest("Jam konfigurasi harus HH:MM.")
			}
		}
	}
	if sc.division.Slug != "humas" && (req.Settings.FollowupClock != "" || req.Settings.BotFollowupCountsRequested) {
		return badRequest("Follow-up Medhum dikelola kebijakan Humas.")
	}
	if (req.Settings.WeeklyClock == "") != (req.Settings.WeeklyWeekday == nil) {
		return badRequest("Hari dan jam rekap mingguan harus ditetapkan bersama.")
	}
	if req.Settings.WeeklyWeekday != nil && (*req.Settings.WeeklyWeekday < 0 || *req.Settings.WeeklyWeekday > 6) {
		return badRequest("Hari rekap tidak valid.")
	}
	if len(req.Settings.Quarters) > 12 {
		return badRequest("Maksimal dua belas rentang evaluasi per periode.")
	}
	labels := map[string]bool{}
	for index, quarter := range req.Settings.Quarters {
		label := strings.TrimSpace(quarter.Label)
		if label == "" || len(label) > 80 || labels[strings.ToLower(label)] {
			return badRequest("Label quarter harus unik dan tidak kosong.")
		}
		labels[strings.ToLower(label)] = true
		req.Settings.Quarters[index].Label = label
		if quarter.StartsOn.Before(sc.period.StartsOn) || quarter.EndsOn.Compare(sc.period.EndsOn) > 0 || quarter.EndsOn.Before(quarter.StartsOn) {
			return badRequest("Rentang quarter harus berada di periode ini.")
		}
		for j := 0; j < index; j++ {
			other := req.Settings.Quarters[j]
			if quarter.StartsOn.Compare(other.EndsOn) <= 0 && quarter.EndsOn.Compare(other.StartsOn) >= 0 {
				return badRequest("Rentang quarter tidak boleh bertumpang tindih.")
			}
		}
	}
	if (req.Settings.UrgentWindowStart == nil) != (req.Settings.UrgentWindowEnd == nil) {
		return badRequest("Awal dan akhir rentang dadakan harus diisi bersama.")
	}
	if req.Settings.UrgentWindowStart != nil {
		if req.Settings.UrgentWindowStart.Before(sc.period.StartsOn) || req.Settings.UrgentWindowEnd.Compare(sc.period.EndsOn) > 0 || req.Settings.UrgentWindowEnd.Before(*req.Settings.UrgentWindowStart) {
			return badRequest("Rentang hitung dadakan tidak valid untuk periode ini.")
		}
	}
	if sc.division.Slug == "humas" && req.Settings.UrgentLimit == nil {
		limit := 5
		req.Settings.UrgentLimit = &limit
	}
	if req.Settings.UrgentLimit != nil && *req.Settings.UrgentLimit != 5 {
		return badRequest("Batas dadakan pada aturan SOP yang didukung adalah lima. Rentang penghitungannya ditetapkan secara eksplisit.")
	}
	if len(req.Settings.BotAgreementNote) > 4000 {
		return badRequest("Catatan kesepakatan terlalu panjang.")
	}
	if req.Settings.BotFollowupCountsRequested && strings.TrimSpace(req.Settings.BotAgreementNote) == "" {
		return badRequest("Catat kesepakatan yang diajukan untuk ditinjau Litbang.")
	}
	return nil
}

func (s *Server) sopPolicy(w http.ResponseWriter, r *http.Request, p *principal) error {
	write := r.Method == http.MethodPut
	mode := "read"
	if write {
		mode = "configure"
	}
	var req api.SOPPolicyRequest
	if write {
		if err := readJSON(r, &req); err != nil {
			return err
		}
	}
	var out api.SOPPolicy
	err := s.withSOPScope(r, p, mode, nil, func(q store.Querier, ac *sopAccess) error {
		sc := ac.scope
		if len(sopOwnedRules(sc.division.Slug)) == 0 {
			return badRequest("Evaluasi pilot hanya untuk Humas dan Mediatek.")
		}
		var err error
		out, err = s.store.SOPPolicy(r.Context(), q, sc.period.ID, sc.division.ID, store.LockNone)
		exists := !errors.Is(err, store.ErrNotFound)
		if err != nil && exists {
			return err
		}
		if !exists {
			out = api.SOPPolicy{PeriodID: sc.period.ID, DivisionID: sc.division.ID, Settings: api.SOPSettings{Rules: []string{}, Quarters: []api.SOPQuarter{}}}
		}
		if !write {
			return nil
		}
		if err = validateSOPSettings(&req, sc); err != nil {
			return err
		}
		if req.SpaceID == id.Nil {
			if err = q.QueryRow(r.Context(), `select sp.id from spaces sp join space_divisions sd on sd.space_id=sp.id where sp.period_id=$1 and sp.code='medhum' and sd.division_id=$2`, sc.period.ID, sc.division.ID).Scan(&req.SpaceID); err != nil {
				return err
			}
		}
		sp, err := s.store.SpaceByID(r.Context(), q, req.SpaceID)
		if err != nil {
			return err
		}
		if sp.PeriodID != sc.period.ID || sp.Code != "medhum" || !slices.ContainsFunc(sp.Divisions, func(d api.Division) bool { return d.ID == sc.division.ID }) {
			return badRequest("Kebijakan harus menunjuk Medhum periode/divisi ini.")
		}
		if req.ExpectedVersion != out.Version {
			return conflict("Kebijakan SOP berubah; muat ulang sebelum menyimpan.")
		}
		before := out
		if !exists {
			out.ID = id.New()
		}
		out.SpaceID = sp.ID
		out.DocumentVersion = req.DocumentVersion
		out.Enabled = req.Enabled
		out.Settings = req.Settings
		out.Version++
		out.UpdatedAt = s.now()
		out.BotFollowupCountsApproved = false
		if err = s.store.SaveSOPPolicy(r.Context(), q, out, p.User.ID, !exists); err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into sop_source_changes(finding_id,source_type,source_id,occurred_at) select id,'policy',$1,$2 from sop_findings where policy_id=$1 and decision_observation_id is not null`, out.ID, s.now()); err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `update sop_findings set needs_recheck=true,version=version+1,updated_at=$2 where policy_id=$1 and decision_observation_id is not null`, out.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.policy.configure", TargetType: "sop_policy", TargetID: &out.ID, PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, Before: before, After: map[string]any{"policy": out, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) sopBotAgreement(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req struct {
		Approved        bool   `json:"approved"`
		ExpectedVersion int64  `json:"expectedVersion"`
		Reason          string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		return err
	}
	reason, err := text(req.Reason, "alasan kesepakatan", 1, 2000)
	if err != nil {
		return err
	}
	err = s.withSOPScope(r, p, "review", nil, func(q store.Querier, ac *sopAccess) error {
		if !ac.evaluatorReview || ac.scope.division.Slug != "humas" {
			return errForbidden
		}
		policy, err := s.store.SOPPolicy(r.Context(), q, ac.scope.period.ID, ac.scope.division.ID, store.LockUpdate)
		if err != nil {
			return err
		}
		if policy.Version != req.ExpectedVersion {
			return conflict("Versi kebijakan berubah; periksa proposal terbaru.")
		}
		if req.Approved && !policy.Settings.BotFollowupCountsRequested {
			return badRequest("Humas belum mengajukan kesepakatan pemenuhan lewat bot.")
		}
		if _, err = q.Exec(r.Context(), `insert into sop_bot_agreements(id,policy_id,policy_version,evaluator_id,approved,reason,created_at) values($1,$2,$3,$4,$5,$6,$7)`, id.New(), policy.ID, policy.Version, p.User.ID, req.Approved, reason, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.bot-agreement", TargetType: "sop_policy", TargetID: &policy.ID, PeriodID: &policy.PeriodID, After: map[string]any{"policyVersion": policy.Version, "approved": req.Approved, "reason": reason}})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func (s *Server) sopEvaluatorGrants(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.SOPEvaluatorGrantRequest
	write := r.Method == http.MethodPost
	mode := "read"
	if write {
		mode = "configure"
		if err := readJSON(r, &req); err != nil {
			return err
		}
		var err error
		req.Reason, err = text(req.Reason, "alasan izin evaluator", 1, 2000)
		if err != nil {
			return err
		}
	}
	out := []api.SOPEvaluatorGrant{}
	var extras []id.ID
	if write {
		extras = []id.ID{req.UserID}
	}
	err := s.withSOPScope(r, p, mode, extras, func(q store.Querier, ac *sopAccess) error {
		if !ac.configure {
			return errForbidden
		}
		sc := ac.scope
		if write {
			var eligible, exists bool
			if err := q.QueryRow(r.Context(), `select exists(select 1 from users u join memberships m on m.user_id=u.id join divisions d on d.id=m.division_id where u.id=$1 and u.kind='person' and u.disabled_at is null and u.email_verified_at is not null and m.period_id=$2 and m.removed_at is null and (d.slug='litbang' or (d.slug='bph' and not $4))),exists(select 1 from sop_evaluator_grants where period_id=$2 and division_id=$3 and user_id=$1 and revoked_at is null)`, req.UserID, sc.period.ID, sc.division.ID, req.CanReview).Scan(&eligible, &exists); err != nil {
				return err
			}
			if !eligible {
				return badRequest("Izin tinjauan hanya untuk Litbang aktif; izin baca juga dapat diberikan kepada BPH aktif.")
			}
			if exists {
				return conflict("Izin evaluator masih aktif. Cabut izin lama sebelum mengganti kewenangannya.")
			}
			gid := id.New()
			if _, err := q.Exec(r.Context(), `insert into sop_evaluator_grants(id,period_id,division_id,user_id,can_review,granted_by,granted_at) values($1,$2,$3,$4,$5,$6,$7)`, gid, sc.period.ID, sc.division.ID, req.UserID, req.CanReview, p.User.ID, s.now()); err != nil {
				return err
			}
			if err := s.record(r, q, p, store.AuditEvent{Action: "sop.evaluator.grant", TargetType: "sop_evaluator_grant", TargetID: &gid, PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, After: req}); err != nil {
				return err
			}
		}
		rows, err := q.Query(r.Context(), `select g.id,g.user_id,u.display_name,g.can_review,g.granted_at,g.revoked_at from sop_evaluator_grants g join users u on u.id=g.user_id where g.period_id=$1 and g.division_id=$2 order by g.granted_at desc,g.id desc`, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var grant api.SOPEvaluatorGrant
			if err := rows.Scan(&grant.ID, &grant.UserID, &grant.DisplayName, &grant.CanReview, &grant.GrantedAt, &grant.RevokedAt); err != nil {
				return err
			}
			out = append(out, grant)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.SOPEvaluatorGrant]{Items: out})
	return nil
}

func (s *Server) revokeSOPEvaluator(w http.ResponseWriter, r *http.Request, p *principal) error {
	gid, err := pathID(r, "grantId")
	if err != nil {
		return err
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err = readJSON(r, &req); err != nil {
		return err
	}
	reason, err := text(req.Reason, "alasan pencabutan", 1, 2000)
	if err != nil {
		return err
	}
	err = s.withSOPScope(r, p, "configure", nil, func(q store.Querier, ac *sopAccess) error {
		tag, err := q.Exec(r.Context(), `update sop_evaluator_grants set revoked_at=$4 where id=$1 and period_id=$2 and division_id=$3 and revoked_at is null`, gid, ac.scope.period.ID, ac.scope.division.ID, s.now())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.evaluator.revoke", TargetType: "sop_evaluator_grant", TargetID: &gid, PeriodID: &ac.scope.period.ID, ScopeType: "division", ScopeID: &ac.scope.division.ID, After: map[string]any{"reason": reason}})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
