package server

import (
	"errors"
	"net/http"
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func (s *Server) waBroadcastPreview(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req struct {
		Body string `json:"body"`
	}
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if len(req.Body) > 32000 {
		return badRequest("Contoh broadcast maksimal 32.000 byte.")
	}
	var items []broadcast.Item
	err := s.withWAContentSpace(r, p, false, func(_ store.Querier, _ api.Period, _ api.Space) error { items = broadcast.Parse(req.Body); return nil })
	if err != nil {
		return err
	}
	if items == nil {
		items = []broadcast.Item{}
	}
	writeJSON(w, 200, api.List[broadcast.Item]{Items: items})
	return nil
}

func (s *Server) withWAContentSpace(r *http.Request, p *principal, write bool, fn func(store.Querier, api.Period, api.Space) error) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	peek, err := s.store.SpaceByID(r.Context(), s.store.DB(), sid)
	if err != nil {
		return err
	}
	if peek.Code != "medhum" {
		return badRequest("Inbox bukti ini hanya untuk ruang Medhum.")
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		lock := store.LockShare
		if write {
			lock = store.LockUpdate
		}
		period, err := s.store.Period(r.Context(), q, peek.PeriodID, lock)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
			return errForbidden
		}
		if write {
			if period.Status == api.PeriodClosed {
				return errPeriodClosed
			}
			if _, err = s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		return fn(q, period, view)
	})
}

func contentDivisionCap(view api.Space, slug, cap string) bool {
	var did id.ID
	for _, d := range view.Divisions {
		if d.Slug == slug {
			did = d.ID
		}
	}
	return slices.ContainsFunc(view.Roles, func(role api.Access) bool {
		return role.DivisionID == did && slices.Contains(role.Capabilities, cap) && !role.ReadOnly
	})
}

func (s *Server) waParserPolicy(w http.ResponseWriter, r *http.Request, p *principal) error {
	write := r.Method == http.MethodPut
	var req api.WAParserPolicy
	if write {
		if err := readJSON(r, &req); err != nil {
			return err
		}
	}
	var out api.WAParserPolicy
	err := s.withWAContentSpace(r, p, write, func(q store.Querier, period api.Period, view api.Space) error {
		out.SpaceID = view.ID
		err := q.QueryRow(r.Context(), `select broadcast_enabled,drive_enabled,template_confirmed,version from wa_parser_policies where space_id=$1`, view.ID).Scan(&out.BroadcastEnabled, &out.DriveEnabled, &out.TemplateConfirmed, &out.Version)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !write {
			return nil
		}
		if req.SpaceID != id.Nil && req.SpaceID != view.ID {
			return badRequest("Space konfigurasi berbeda.")
		}
		if req.Version != out.Version {
			return conflict("Kebijakan parser berubah; muat ulang sebelum menyimpan.")
		}
		humas := contentDivisionCap(view, "humas", api.CapTasksManage)
		design := contentDivisionCap(view, "mediatek", api.CapTasksManage)
		if !humas && !design {
			return errForbidden
		}
		if (req.BroadcastEnabled != out.BroadcastEnabled || req.TemplateConfirmed != out.TemplateConfirmed) && !humas {
			return errForbidden
		}
		if req.DriveEnabled != out.DriveEnabled && !design {
			return errForbidden
		}
		if req.BroadcastEnabled && !req.TemplateConfirmed {
			return badRequest("Konfirmasikan contoh format broadcast sebelum mengaktifkan parser.")
		}
		before := out
		out.BroadcastEnabled = req.BroadcastEnabled
		out.DriveEnabled = req.DriveEnabled
		out.TemplateConfirmed = req.TemplateConfirmed
		out.Version++
		if _, err = q.Exec(r.Context(), `insert into wa_parser_policies(space_id,broadcast_enabled,drive_enabled,template_confirmed,version,updated_by,updated_at) values($1,$2,$3,$4,$5,$6,$7) on conflict(space_id) do update set broadcast_enabled=excluded.broadcast_enabled,drive_enabled=excluded.drive_enabled,template_confirmed=excluded.template_confirmed,version=excluded.version,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, view.ID, out.BroadcastEnabled, out.DriveEnabled, out.TemplateConfirmed, out.Version, p.User.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "whatsapp.parser.configure", PeriodID: &period.ID, TargetType: "space", TargetID: &view.ID, Before: before, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) waContentInbox(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.WAContentItem{}
	err := s.withWAContentSpace(r, p, false, func(q store.Querier, _ api.Period, view api.Space) error {
		var err error
		out, err = s.store.WAContentItems(r.Context(), q, view.ID)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.WAContentItem]{Items: out})
	return nil
}

func (s *Server) waContentDecision(w http.ResponseWriter, r *http.Request, p *principal) error {
	iid, err := pathID(r, "itemId")
	if err != nil {
		return err
	}
	var req api.WAContentDecision
	if err = readJSON(r, &req); err != nil {
		return err
	}
	reason, err := text(req.Reason, "alasan keputusan", 1, 2000)
	if err != nil {
		return err
	}
	if !slices.Contains([]string{"apply", "ignore", "reassign", "retract", "acknowledge"}, req.Action) {
		return badRequest("Keputusan inbox tidak valid.")
	}
	var out api.WAContentItem
	err = s.withWAContentSpace(r, p, true, func(q store.Querier, period api.Period, view api.Space) error {
		out, err = s.store.WAContentItem(r.Context(), q, iid, store.LockUpdate)
		if err != nil {
			return err
		}
		if out.SpaceID != view.ID {
			return errNotFound
		}
		if out.Version != req.ExpectedVersion {
			return conflict("Item berubah; muat ulang sebelum mengambil keputusan.")
		}
		authority := contentDivisionCap(view, "humas", api.CapTasksManage)
		if out.Kind == "drive" {
			authority = contentDivisionCap(view, "mediatek", api.CapResultsReview)
		}
		if out.Kind == "correction" {
			authority = authority || contentDivisionCap(view, "mediatek", api.CapResultsReview)
		}
		if !authority {
			return errForbidden
		}
		before := out
		switch req.Action {
		case "ignore":
			if out.State != "pending" {
				return conflict("Gunakan ralat untuk menarik bukti yang sudah diterapkan.")
			}
			out.State = "ignored"
			out.Issue = reason
		case "acknowledge":
			if out.Kind != "correction" {
				return badRequest("Tindakan ini khusus perubahan pesan sumber.")
			}
			out.State = "corrected"
			out.Issue = "Perubahan pesan ditinjau; fakta/bukti sebelumnya tetap dipertahankan. " + reason
		case "retract":
			if !slices.Contains([]string{"applied", "corrected"}, out.State) {
				return conflict("Item tidak sedang diterapkan.")
			}
			if out.ContentID == nil {
				return conflict("Item belum diterapkan ke konten.")
			}
			if err = s.retractWAContent(r, q, p, period, &out, reason); err != nil {
				return err
			}
			out.State = "retracted"
			out.Issue = reason
		default:
			if out.Kind == "correction" {
				return badRequest("Ralat pasangan dilakukan pada item konsep/Drive aslinya; perubahan pesan ini tidak menghapus riwayat.")
			}
			if req.ContentID == nil {
				return badRequest("Pilih konten tujuan.")
			}
			if req.Action == "apply" && !slices.Contains([]string{"pending", "ignored", "retracted"}, out.State) {
				return conflict("Item sudah diputuskan. Gunakan ralat pasangan bila diperlukan.")
			}
			if req.Action == "reassign" {
				if !slices.Contains([]string{"applied", "corrected", "retracted"}, out.State) {
					return conflict("Item belum mempunyai pasangan yang dapat diralat.")
				}
				if out.ContentID == nil || *out.ContentID == *req.ContentID {
					return badRequest("Pilih konten tujuan yang berbeda untuk ralat pasangan.")
				}
				if out.State != "retracted" {
					if err = s.retractWAContent(r, q, p, period, &out, reason); err != nil {
						return err
					}
				}
			}
			// A previously unlinked sender may have verified their identity since
			// staging. Resolve the original envelope again; never take a typed name.
			source, sourceGroup, err := s.waContentSource(r, q, out)
			if err != nil {
				return err
			}
			sourceActor, err := s.waSender(r, q, period, sourceGroup, source)
			if err != nil {
				return err
			}
			sourceID := sourceActor.User.ID
			out.SenderUserID = &sourceID
			out.SenderName = sourceActor.User.DisplayName
			if out.Kind == "concept" {
				err = s.applyWAConcept(r, q, p, period, &out, *req.ContentID, true)
			} else {
				err = s.applyWADrive(r, q, p, period, &out, *req.ContentID, true, req.ConfirmedAccess)
			}
			if err != nil {
				return err
			}
			if req.Action == "reassign" {
				out.State = "corrected"
			}
		}
		now := s.now()
		out.ReviewedAt = &now
		uid := p.User.ID
		out.ReviewedBy = &uid
		out.Version++
		if err = s.store.SaveWAContentItem(r.Context(), q, out, false); err != nil {
			return err
		}
		return s.waContentReview(r, q, p, out, "whatsapp.content."+req.Action, reason, before, out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) waContentSource(r *http.Request, q store.Querier, x api.WAContentItem) (waInbound, api.WAGroup, error) {
	var in waInbound
	group := api.WAGroup{SpaceID: x.SpaceID}
	err := q.QueryRow(r.Context(), `select id,connection_id,group_jid,sender_jid,sender_alt_jid,body,occurred_at,received_at,message_id,quoted_message_id from wa_inbound where id=$1`, x.InboundID).Scan(&in.ID, &in.ConnectionID, &in.GroupJID, &in.SenderJID, &in.SenderAltJID, &in.Body, &in.OccurredAt, &in.ReceivedAt, &in.MessageID, &in.QuotedMessageID)
	return in, group, err
}

func (s *Server) retractWAContent(r *http.Request, q store.Querier, p *principal, period api.Period, x *api.WAContentItem, reason string) error {
	if x.ContentID == nil {
		return conflict("Pasangan bukti belum tersedia.")
	}
	c, err := s.store.Medhum(r.Context(), q, *x.ContentID, store.LockUpdate)
	if err != nil {
		return err
	}
	if c.SpaceID != x.SpaceID {
		return errForbidden
	}
	if x.Kind == "drive" {
		if x.EvidenceID == nil || c.DesignTaskID == nil {
			return conflict("Bukti desain belum tersedia.")
		}
		t, err := s.store.Task(r.Context(), q, *c.DesignTaskID, store.LockUpdate)
		if err != nil {
			return err
		}
		if err = s.taskAccess(r.Context(), q, p, period, &t); err != nil {
			return err
		}
		var evidence api.TaskEvidence
		if err = s.applyTaskEvidenceReview(r, q, p, &t, *x.EvidenceID, api.TaskEvidenceReviewRequest{Matched: false, ExpectedVersion: t.Version}, reason, &evidence); err != nil {
			return err
		}
		return s.retractWAContentAliases(r, q, p, *x, id.Nil, reason)
	}
	if x.Kind != "concept" {
		return badRequest("Perubahan pesan ditinjau melalui item bukti aslinya.")
	}
	var receipt id.ID
	if err = q.QueryRow(r.Context(), `select receipt_id from wa_concept_item_receipts where item_id=$1`, x.ID).Scan(&receipt); err != nil {
		return err
	}
	if _, err = q.Exec(r.Context(), `update wa_concept_receipts set active=false,invalidated_by=$2,invalidated_at=$3,invalidation_reason=$4 where id=$1 and active`, receipt, p.User.ID, s.now(), reason); err != nil {
		return err
	}
	if err = s.retractWAContentAliases(r, q, p, *x, receipt, reason); err != nil {
		return err
	}
	before := c
	if err = q.QueryRow(r.Context(), `select min(first_at) from wa_concept_receipts where content_id=$1 and active`, c.ID).Scan(&c.ConceptSubmittedAt); err != nil {
		return err
	}
	c.Version++
	c.UpdatedAt = s.now()
	if err = s.store.SaveMedhum(r.Context(), q, c, false); err != nil {
		return err
	}
	return s.waContentReview(r, q, p, *x, "whatsapp.concept.retract", reason, before, map[string]any{"content": c, "receiptId": receipt})
}

func (s *Server) waContentHistory(w http.ResponseWriter, r *http.Request, p *principal) error {
	iid, err := pathID(r, "itemId")
	if err != nil {
		return err
	}
	out := []api.WAContentReview{}
	err = s.withWAContentSpace(r, p, false, func(q store.Querier, _ api.Period, view api.Space) error {
		item, err := s.store.WAContentItem(r.Context(), q, iid, store.LockNone)
		if err != nil {
			return err
		}
		if item.SpaceID != view.ID {
			return errNotFound
		}
		rows, err := q.Query(r.Context(), `select h.id,coalesce(u.display_name,'Sistem'),h.action,h.reason,h.before_data,h.after_data,h.occurred_at from wa_content_reviews h left join users u on u.id=h.actor_id where h.item_id=$1 order by h.occurred_at,h.id`, iid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var review api.WAContentReview
			if err := rows.Scan(&review.ID, &review.ActorName, &review.Action, &review.Reason, &review.Before, &review.After, &review.OccurredAt); err != nil {
				return err
			}
			out = append(out, review)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.WAContentReview]{Items: out})
	return nil
}

// Reposts share one receipt. Propagate an explicit retraction with a decision
// history on each affected item, rather than changing their state silently.
func (s *Server) retractWAContentAliases(r *http.Request, q store.Querier, p *principal, source api.WAContentItem, receipt id.ID, reason string) error {
	query := `select x.id from wa_content_items x where x.evidence_id=$1 and x.id<>$2 and x.state in ('applied','corrected') order by x.id`
	var key any = source.EvidenceID
	if source.Kind == "concept" {
		query = `select x.id from wa_content_items x join wa_concept_item_receipts a on a.item_id=x.id where a.receipt_id=$1 and x.id<>$2 and x.state in ('applied','corrected') order by x.id`
		key = receipt
	}
	rows, err := q.Query(r.Context(), query, key, source.ID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	if err != nil {
		return err
	}
	for _, iid := range ids {
		x, err := s.store.WAContentItem(r.Context(), q, iid, store.LockUpdate)
		if err != nil {
			return err
		}
		before := x
		x.State = "retracted"
		x.Issue = reason
		x.Version++
		now := s.now()
		uid := p.User.ID
		x.ReviewedAt = &now
		x.ReviewedBy = &uid
		if err = s.store.SaveWAContentItem(r.Context(), q, x, false); err != nil {
			return err
		}
		if err = s.waContentReview(r, q, p, x, "whatsapp.content.retract-linked", reason, before, x); err != nil {
			return err
		}
	}
	return nil
}
