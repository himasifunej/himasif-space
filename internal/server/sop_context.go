package server

import (
	"encoding/json"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

func evidenceEmpty(e sop.Evidence) bool {
	return e.At == nil && e.Date == nil && !e.Verified && e.URL == "" && e.Note == ""
}
func (s *Server) validateSOPContext(req *api.SOPContextRequest, sc *scope) error {
	context := &req.Context
	if sc.division.Slug == "humas" {
		if !evidenceEmpty(context.FinalApproved) {
			return badRequest("Konfirmasi final desain merupakan konteks Mediatek.")
		}
	} else if sc.division.Slug == "mediatek" {
		if !evidenceEmpty(context.ConceptMature) || !evidenceEmpty(context.Followup) || !evidenceEmpty(context.UrgentConfirmed) || !evidenceEmpty(context.UrgentException) || context.Urgent != nil || context.UrgentExceptionCategory != "" {
			return badRequest("Konteks permintaan/follow-up/dadakan dikelola melalui scope Humas.")
		}
	} else {
		return badRequest("Konteks pilot hanya untuk Humas dan Mediatek.")
	}
	if context.UrgentException.Verified && !slices.Contains([]string{"sempro", "semhas_magang", "berita_duka"}, context.UrgentExceptionCategory) {
		return badRequest("Pilih kategori pengecualian SOP yang sesuai. Kesepakatan lain dicatat sebagai kesepakatan khusus dengan bukti.")
	}
	if len(context.SpecialAgreementRules) > len(sop.Rules) {
		return badRequest("Cakupan kesepakatan terlalu panjang.")
	}
	slices.Sort(context.SpecialAgreementRules)
	context.SpecialAgreementRules = slices.Compact(context.SpecialAgreementRules)
	for _, rule := range context.SpecialAgreementRules {
		if !slices.Contains(sopOwnedRules(sc.division.Slug), rule) {
			return badRequest("Kesepakatan harus menunjuk aturan milik scope divisi ini.")
		}
	}
	if context.SpecialAgreement.Verified && len(context.SpecialAgreementRules) == 0 {
		return badRequest("Tentukan aturan yang dicakup kesepakatan; kesepakatan bukan pengecualian seluruh SOP.")
	}
	zone, err := timeutil.LoadZone(sc.period.TimeZone)
	if err != nil {
		return err
	}
	for _, field := range []struct {
		proof    *sop.Evidence
		noteOnly bool
	}{{&context.ConceptMature, false}, {&context.Followup, false}, {&context.FinalApproved, false}, {&context.UrgentConfirmed, false}, {&context.UrgentException, true}, {&context.SpecialAgreement, true}} {
		e := field.proof
		if len(e.URL) > 4096 || len(e.Note) > 4000 {
			return badRequest("Tautan atau catatan bukti konteks terlalu panjang.")
		}
		if e.URL != "" {
			u, err := url.Parse(e.URL)
			if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || strings.ContainsAny(e.URL, "\r\n") {
				return badRequest("Tautan bukti konteks harus URL http/https yang valid.")
			}
		}
		if e.At != nil && e.Date != nil {
			return badRequest("Bukti konteks harus memakai tanggal saja atau waktu persis.")
		}
		if e.Verified && !field.noteOnly && e.At == nil && e.Date == nil {
			return badRequest("Bukti waktu yang dikonfirmasi memerlukan tanggal atau waktu kejadian.")
		}
		if e.At != nil && e.At.After(s.now().Add(5*time.Minute)) {
			return badRequest("Waktu kejadian bukti tidak boleh berada di masa depan.")
		}
		if e.Date != nil && e.Date.Compare(timeutil.DateOf(s.now().In(zone))) > 0 {
			return badRequest("Tanggal kejadian bukti tidak boleh berada di masa depan.")
		}
		if e.Verified && strings.TrimSpace(e.Note) == "" {
			e.Note = req.Reason
		}
		if !evidenceEmpty(*e) {
			e.Source = "manual"
			e.SourceID = fmt.Sprintf("%s:%s:%d", context.ContentID, sc.division.ID, context.Version)
		} else {
			e.Source = ""
			e.SourceID = ""
		}
	}
	return nil
}

func (s *Server) sopContext(w http.ResponseWriter, r *http.Request, p *principal) error {
	cid, err := pathID(r, "contentId")
	if err != nil {
		return err
	}
	write := r.Method == http.MethodPut
	mode := "read"
	if write {
		mode = "review"
	}
	var req api.SOPContextRequest
	if write {
		if err = readJSON(r, &req); err != nil {
			return err
		}
		req.Reason, err = text(req.Reason, "alasan pencatatan konteks", 1, 2000)
		if err != nil {
			return err
		}
	}
	var out api.SOPContext
	err = s.withSOPScope(r, p, mode, nil, func(q store.Querier, ac *sopAccess) error {
		c, err := s.store.Medhum(r.Context(), q, cid, store.LockNone)
		if err != nil {
			return err
		}
		sc := ac.scope
		if c.PeriodID != sc.period.ID || (c.HumasDivisionID != sc.division.ID && c.DesignDivisionID != sc.division.ID) {
			return errNotFound
		}
		out, err = s.store.SOPContext(r.Context(), q, cid, sc.division.ID)
		if err != nil {
			return err
		}
		if !write {
			return nil
		}
		if out.Version != req.ExpectedVersion {
			return conflict("Konteks berubah; muat ulang sebelum menyimpan.")
		}
		if (req.Context.ContentID != id.Nil && req.Context.ContentID != cid) || (req.Context.DivisionID != id.Nil && req.Context.DivisionID != sc.division.ID) {
			return badRequest("Identitas konteks tidak sesuai scope.")
		}
		req.Context.ContentID = cid
		req.Context.DivisionID = sc.division.ID
		req.Context.Version = out.Version + 1
		if err = s.validateSOPContext(&req, sc); err != nil {
			return err
		}
		before := out
		out = req.Context
		data, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into sop_contexts(content_id,division_id,data,version,updated_by,updated_at) values($1,$2,$3,$4,$5,$6) on conflict(content_id,division_id) do update set data=excluded.data,version=excluded.version,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, cid, sc.division.ID, data, out.Version, p.User.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "sop.context.record", TargetType: "medhum_content", TargetID: &cid, PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, Before: before, After: map[string]any{"context": out, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
