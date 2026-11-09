package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

type waDriveSource struct {
	Link            integrations.DriveLink `json:"link"`
	ReferenceOnly   bool                   `json:"referenceOnly"`
	QuotedMessageID string                 `json:"quotedMessageId"`
}

func waSourceAt(in waInbound) (time.Time, string) {
	if in.OccurredAt == nil {
		return in.ReceivedAt, "Waktu kirim tidak tersedia; waktu penerimaan dipertahankan sebagai acuan."
	}
	if in.OccurredAt.After(in.ReceivedAt.Add(5 * time.Minute)) {
		return in.ReceivedAt, "Waktu kirim mendahului jam penerimaan secara tidak wajar; perlu tinjauan."
	}
	return *in.OccurredAt, ""
}

func (s *Server) waContentReview(r *http.Request, q store.Querier, p *principal, x api.WAContentItem, action, reason string, before, after any) error {
	var periodID id.ID
	if err := q.QueryRow(r.Context(), `select period_id from spaces where id=$1`, x.SpaceID).Scan(&periodID); err != nil {
		return err
	}
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	var actor *id.ID
	if p != nil {
		uid := p.User.ID
		actor = &uid
	}
	if _, err = q.Exec(r.Context(), `insert into wa_content_reviews(id,item_id,actor_id,action,reason,before_data,after_data,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), x.ID, actor, action, reason, b, a, s.now()); err != nil {
		return err
	}
	return s.record(r, q, p, store.AuditEvent{ActorKind: func() string {
		if p == nil {
			return "system"
		}
		return ""
	}(), Action: action, PeriodID: &periodID, TargetType: "wa_content_item", TargetID: &x.ID, ScopeType: "space", ScopeID: &x.SpaceID, After: map[string]any{"before": before, "after": after, "reason": reason}})
}

func (s *Server) waConceptCandidates(r *http.Request, q store.Querier, sid id.ID, item broadcast.Item, zone string) ([]id.ID, error) {
	contents, err := s.store.MedhumContents(r.Context(), q, sid)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	out := []id.ID{}
	for _, c := range contents {
		if broadcast.Normalize(c.Title) != broadcast.Normalize(item.Title) {
			continue
		}
		date := c.PostingDate
		if date == nil && c.PostingAt != nil {
			d := c.PostingAt.In(loc)
			if item.PostingDate != nil {
				if d.Format("2006-01-02") != item.PostingDate.String() {
					continue
				}
			} else if item.Month == 0 || int(d.Month()) != item.Month || d.Day() != item.Day {
				continue
			}
		} else if date != nil {
			if item.PostingDate != nil {
				if date.Compare(*item.PostingDate) != 0 {
					continue
				}
			} else if item.Month == 0 || int(date.Month) != item.Month || date.Day != item.Day {
				continue
			}
		} else {
			continue
		}
		out = append(out, c.ID)
	}
	return out, nil
}

func (s *Server) waNameMatches(r *http.Request, q store.Querier, pid, did, uid id.ID, name string) (bool, error) {
	var actual string
	if err := q.QueryRow(r.Context(), `select u.display_name from users u join memberships m on m.user_id=u.id where u.id=$1 and u.disabled_at is null and m.period_id=$2 and m.division_id=$3 and m.removed_at is null`, uid, pid, did).Scan(&actual); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	normalized := broadcast.Normalize(name)
	if normalized == broadcast.Normalize(actual) {
		rows, err := q.Query(r.Context(), `select u.id,u.display_name from users u join memberships m on m.user_id=u.id where m.period_id=$1 and m.division_id=$2 and m.removed_at is null and u.disabled_at is null`, pid, did)
		if err != nil {
			return false, err
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var other id.ID
			var display string
			if err := rows.Scan(&other, &display); err != nil {
				return false, err
			}
			if broadcast.Normalize(display) == normalized {
				count++
			}
		}
		return count == 1, rows.Err()
	}
	// A literal mention token can match a verified JID only. Do not interpret
	// arbitrary long digits as a phone number or resolve a display-name guess.
	var matched bool
	if err := q.QueryRow(r.Context(), `select exists(select 1 from wa_identities where period_id=$1 and user_id=$2 and split_part(jid,'@',1)=$3)`, pid, uid, strings.TrimPrefix(strings.TrimSpace(name), "@")).Scan(&matched); err != nil {
		return false, err
	}
	return matched, nil
}

func (s *Server) applyWAConcept(r *http.Request, q store.Querier, p *principal, period api.Period, x *api.WAContentItem, cid id.ID, manual bool) error {
	c, err := s.store.Medhum(r.Context(), q, cid, store.LockUpdate)
	if err != nil {
		return err
	}
	if c.SpaceID != x.SpaceID || c.PeriodID != period.ID {
		return errForbidden
	}
	rr := r.Clone(r.Context())
	rr.SetPathValue("divisionId", c.HumasDivisionID.String())
	rr.SetPathValue("periodId", period.ID.String())
	sc, err := s.loadScope(r.Context(), q, rr, p, store.LockNone)
	if err != nil {
		return err
	}
	manager := sc.require(api.CapTasksManage) == nil
	if manual && !manager {
		return errForbidden
	}
	if !manual && p.User.ID != c.HumasPJID && !manager {
		return errForbidden
	}
	if _, err = s.taskAssignee(r.Context(), q, period.ID, c.HumasDivisionID, c.HumasPJID); err != nil {
		return err
	}
	if !manual {
		var parsed broadcast.Item
		if err = json.Unmarshal(x.Parsed, &parsed); err != nil {
			return err
		}
		matched, err := s.waNameMatches(r, q, period.ID, c.HumasDivisionID, c.HumasPJID, parsed.Humas)
		if err != nil {
			return err
		}
		if !matched {
			return conflict("PJ Humas pada broadcast belum cocok dengan akun kalender.")
		}
		if parsed.Design != "" {
			if c.DesignPJID == nil {
				return conflict("PJ desain pada kalender belum ditetapkan Mediatek.")
			}
			matched, err = s.waNameMatches(r, q, period.ID, c.DesignDivisionID, *c.DesignPJID, parsed.Design)
			if err != nil {
				return err
			}
			if !matched {
				return conflict("PJ desain pada broadcast berbeda dari penetapan Mediatek.")
			}
		}
	}
	key := integrations.Hash([]any{c.ID, broadcast.Normalize(c.Title), c.PostingDate, c.PostingAt, c.HumasPJID, c.DesignPJID})
	rid := id.New()
	inserted := true
	err = q.QueryRow(r.Context(), `insert into wa_concept_receipts(id,content_id,humas_pj_id,fact_key,first_item_id,first_at) values($1,$2,$3,$4,$5,$6) on conflict(content_id,fact_key) do nothing returning id`, rid, c.ID, c.HumasPJID, key, x.ID, x.SourceAt).Scan(&rid)
	if errors.Is(err, pgx.ErrNoRows) {
		inserted = false
		var active bool
		if err = q.QueryRow(r.Context(), `select id,active from wa_concept_receipts where content_id=$1 and fact_key=$2 for update`, c.ID, key).Scan(&rid, &active); err != nil {
			return err
		}
		if !active && !manual {
			return conflict("Fakta konsep ini telah diralat; kiriman ulang tidak mengaktifkannya otomatis.")
		}
		if !active {
			if _, err = q.Exec(r.Context(), `update wa_concept_receipts set active=true where id=$1`, rid); err != nil {
				return err
			}
		}
		if _, err = q.Exec(r.Context(), `update wa_concept_receipts set first_item_id=case when $2<first_at then $3 else first_item_id end,first_at=least(first_at,$2) where id=$1`, rid, x.SourceAt, x.ID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err = q.Exec(r.Context(), `insert into wa_concept_item_receipts(item_id,receipt_id) values($1,$2) on conflict(item_id) do update set receipt_id=excluded.receipt_id`, x.ID, rid); err != nil {
		return err
	}
	if c.ConceptSubmittedAt == nil || x.SourceAt.Before(*c.ConceptSubmittedAt) {
		at := x.SourceAt
		c.ConceptSubmittedAt = &at
		c.Version++
		c.UpdatedAt = s.now()
		if err = s.store.SaveMedhum(r.Context(), q, c, false); err != nil {
			return err
		}
	}
	x.ContentID = &c.ID
	x.State = "applied"
	now := s.now()
	x.MatchedAt = &now
	x.Issue = ""
	if !inserted {
		x.Issue = "Kiriman ulang: konsep sudah tercatat; waktu pertama dipertahankan."
	}
	if inserted && c.DesignTaskID != nil {
		task, err := s.store.Task(r.Context(), q, *c.DesignTaskID, store.LockUpdate)
		if err != nil {
			return err
		}
		if err = s.taskNotify(r, q, &task, "Konsep Humas diserahkan", c.Title, taskPeople(&task)...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) waDriveCandidates(r *http.Request, q store.Querier, sid, uid id.ID, body, quoted string) ([]id.ID, error) {
	rows, err := q.Query(r.Context(), `select c.id from medhum_contents c join tasks t on t.id=c.design_task_id where c.space_id=$1 and t.progress in ('todo','doing') and (t.assignee_id=$2 or exists(select 1 from task_participants tp where tp.task_id=t.id and tp.user_id=$2)) order by c.id`, sid, uid)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	if err != nil {
		return nil, err
	}
	if quoted != "" {
		rows, err = q.Query(r.Context(), `select distinct x.content_id from wa_content_items x join wa_inbound i on i.id=x.inbound_id where x.space_id=$1 and i.message_id=$2 and x.content_id is not null and x.state in ('applied','corrected')`, sid, quoted)
		if err != nil {
			return nil, err
		}
		related, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
		if err != nil {
			return nil, err
		}
		if len(related) > 0 {
			out = slices.DeleteFunc(out, func(cid id.ID) bool { return !slices.Contains(related, cid) })
		}
	}
	for _, line := range strings.Split(body, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || broadcast.Normalize(parts[0]) != "judul" {
			continue
		}
		matched := []id.ID{}
		for _, cid := range out {
			c, err := s.store.Medhum(r.Context(), q, cid, store.LockNone)
			if err != nil {
				return nil, err
			}
			if broadcast.Normalize(c.Title) == broadcast.Normalize(parts[1]) {
				matched = append(matched, cid)
			}
		}
		out = matched
	}
	return out, nil
}

func (s *Server) applyWADrive(r *http.Request, q store.Querier, p *principal, period api.Period, x *api.WAContentItem, cid id.ID, manual, confirmedAccess bool) error {
	var source waDriveSource
	if err := json.Unmarshal(x.Parsed, &source); err != nil {
		return err
	}
	link, err := integrations.ParseDriveLink(source.Link.URL)
	if err != nil {
		return badRequest("Tautan sumber Drive tidak valid.")
	}
	if !source.Link.Accessible && !(manual && confirmedAccess) {
		return conflict("Akses berkas belum terkonfirmasi. Periksa tautan sebelum menetapkan pasangan.")
	}
	if source.ReferenceOnly && !manual {
		return conflict("Pesan menyebut bahan referensi; perlu tinjauan sebelum dianggap hasil.")
	}
	if x.SenderUserID == nil {
		return conflict("Pengirim belum tertaut ke akun Space; verifikasi identitas sebelum menetapkan hasil.")
	}
	c, err := s.store.Medhum(r.Context(), q, cid, store.LockUpdate)
	if err != nil {
		return err
	}
	if c.SpaceID != x.SpaceID || c.PeriodID != period.ID || c.DesignTaskID == nil {
		return errForbidden
	}
	task, err := s.store.Task(r.Context(), q, *c.DesignTaskID, store.LockUpdate)
	if err != nil {
		return err
	}
	if err = s.taskAccess(r.Context(), q, p, period, &task); err != nil {
		return err
	}
	if manual && !task.Permissions.Review {
		return errForbidden
	}
	relevant := task.Assignee.ID == *x.SenderUserID || slices.ContainsFunc(task.Participants, func(person api.PersonRef) bool { return person.ID == *x.SenderUserID })
	if !relevant {
		return conflict("Pengirim bukan PJ/pelaksana desain yang relevan; bukti belum diterapkan.")
	}
	person, err := s.taskAssignee(r.Context(), q, period.ID, c.DesignDivisionID, *x.SenderUserID)
	if err != nil {
		return err
	}
	var previous id.ID
	var previousStatus string
	err = q.QueryRow(r.Context(), `select d.evidence_id,e.match_status from wa_drive_receipts d join task_evidence e on e.id=d.evidence_id where d.content_id=$1 and d.resource_id=$2`, c.ID, link.ResourceID).Scan(&previous, &previousStatus)
	if err == nil {
		if previousStatus == api.MatchRejected && !manual {
			return conflict("Bukti ini pernah ditolak/diralat. Kiriman ulang tidak mengaktifkannya kembali otomatis.")
		}
		if previousStatus != api.MatchRejected {
			if _, err = q.Exec(r.Context(), `update wa_drive_receipts set first_at=least(first_at,$3) where content_id=$1 and resource_id=$2`, c.ID, link.ResourceID, x.SourceAt); err != nil {
				return err
			}
			if _, err = q.Exec(r.Context(), `update task_evidence set submitted_at=least(submitted_at,$2) where id=$1`, previous, x.SourceAt); err != nil {
				return err
			}
			x.ContentID = &c.ID
			x.EvidenceID = &previous
			x.State = "applied"
			x.Issue = ""
			x.Issue = "Kiriman ulang: tautan sudah terhubung; bukti dan waktu pertama dipertahankan."
			now := s.now()
			x.MatchedAt = &now
			return nil
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if manual {
		task.Permissions.Progress = true
	} // fenced by ResultsReview above
	req := api.TaskEvidenceRequest{URL: link.URL, Note: "Tautan Drive dari Medhum; akses/pasangan perlu ditinjau. Sumber " + x.InboundID.String(), ExpectedVersion: task.Version}
	var evidence api.TaskEvidence
	if err = s.applyAttributedTaskEvidence(r, q, p, &task, req, x.SourceAt, person, &evidence); err != nil {
		return err
	}
	if _, err = q.Exec(r.Context(), `insert into wa_drive_receipts(content_id,resource_id,item_id,evidence_id,first_at) values($1,$2,$3,$4,$5) on conflict(content_id,resource_id) do update set item_id=excluded.item_id,evidence_id=excluded.evidence_id,first_at=least(wa_drive_receipts.first_at,excluded.first_at)`, c.ID, link.ResourceID, x.ID, evidence.ID, x.SourceAt); err != nil {
		return err
	}
	x.ContentID = &c.ID
	x.EvidenceID = &evidence.ID
	x.State = "applied"
	x.Issue = ""
	now := s.now()
	x.MatchedAt = &now
	return nil
}
