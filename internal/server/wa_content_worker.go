package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
)

var waReferenceText = regexp.MustCompile(`(?i)\b(referensi|reference|contoh|inspirasi|bahan)\b`)

func (s *Server) processWAContent(ctx context.Context) (bool, error) {
	var iid, cid, pid id.ID
	var body string
	var kind string
	var probeAllowed bool
	err := s.store.DB().QueryRow(ctx, `select i.id,i.connection_id,c.period_id,i.body,i.kind,coalesce(pp.drive_enabled,false) and c.enabled and p.status<>'closed' and exists(select 1 from wa_identities wi join users u on u.id=wi.user_id and u.disabled_at is null join memberships m on m.user_id=u.id and m.period_id=wi.period_id and m.removed_at is null join space_divisions sd on sd.period_id=m.period_id and sd.division_id=m.division_id where wi.period_id=c.period_id and wi.jid in(i.sender_jid,nullif(i.sender_alt_jid,'')) and sd.space_id=g.space_id) from wa_inbound i join wa_connections c on c.id=i.connection_id join periods p on p.id=c.period_id left join wa_groups g on g.id=i.group_id left join wa_parser_policies pp on pp.space_id=g.space_id where i.state='queued' and (i.kind<>'message' or ltrim(i.body) not like '!%') order by i.received_at,i.id limit 1`).Scan(&iid, &cid, &pid, &body, &kind, &probeAllowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	parsed := broadcast.Parse(body)
	links := integrations.DriveLinks(body)
	// Bounded metadata reads take place outside database locks. The current
	// account, group, policy and period are checked again before any application.
	if probeAllowed && kind == "message" {
		for index, link := range links {
			if index < 3 {
				probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
				links[index] = s.sources.ProbeDrive(probeCtx, link)
				cancel()
			} else {
				links[index].Issue = "Lebih dari tiga tautan dalam satu pesan; periksa akses secara manual."
			}
		}
	}
	found := false
	err = s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, pid, store.LockUpdate)
		if err != nil {
			return err
		}
		conn, err := s.store.WAConnection(ctx, q, cid, store.LockShare)
		if err != nil {
			return err
		}
		var in waInbound
		var groupID *id.ID
		var actualKind string
		err = q.QueryRow(ctx, `select id,connection_id,group_id,group_jid,sender_jid,sender_alt_jid,body,occurred_at,received_at,message_id,quoted_message_id,kind from wa_inbound where id=$1 and state='queued' for update skip locked`, iid).Scan(&in.ID, &in.ConnectionID, &groupID, &in.GroupJID, &in.SenderJID, &in.SenderAltJID, &in.Body, &in.OccurredAt, &in.ReceivedAt, &in.MessageID, &in.QuotedMessageID, &actualKind)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		ignore := func(reason string) error {
			_, err := q.Exec(ctx, `update wa_inbound set state='ignored',error=$2 where id=$1`, iid, reason)
			return err
		}
		if groupID == nil || !conn.Enabled || period.Status == api.PeriodClosed {
			return ignore("Grup/koneksi tidak aktif atau periode ditutup.")
		}
		var group api.WAGroup
		var code string
		if err = q.QueryRow(ctx, `select g.id,g.space_id,g.jid,g.name,g.commands_enabled,sp.code from wa_groups g join spaces sp on sp.id=g.space_id where g.id=$1 and g.connection_id=$2 for share of g`, *groupID, cid).Scan(&group.ID, &group.SpaceID, &group.JID, &group.Name, &group.CommandsEnabled, &code); err != nil {
			return err
		}
		if group.JID != in.GroupJID || code != "medhum" {
			return ignore("Pesan bukan dari pemetaan grup Medhum saat ini.")
		}
		in.GroupID = group.ID
		policy := api.WAParserPolicy{}
		err = q.QueryRow(ctx, `select broadcast_enabled,drive_enabled,template_confirmed from wa_parser_policies where space_id=$1`, group.SpaceID).Scan(&policy.BroadcastEnabled, &policy.DriveEnabled, &policy.TemplateConfirmed)
		if errors.Is(err, pgx.ErrNoRows) {
			return ignore("Parser Medhum belum diaktifkan.")
		}
		if err != nil {
			return err
		}
		r := (&http.Request{}).WithContext(context.WithValue(ctx, waAuditKey{}, in.ID))
		r.SetPathValue("periodId", pid.String())
		r.SetPathValue("divisionId", conn.DivisionID.String())
		p, senderErr := s.waSender(r, q, period, group, in)
		var sender *id.ID
		senderIssue := ""
		if senderErr != nil {
			var ae *apiError
			if !errors.As(senderErr, &ae) {
				return senderErr
			}
			senderIssue = ae.message
		} else {
			uid := p.User.ID
			sender = &uid
		}
		at, timeIssue := waSourceAt(in)
		loc, err := time.LoadLocation(period.TimeZone)
		if err != nil {
			return err
		}
		if at.Before(period.StartsOn.At(0, 0, loc)) {
			timeIssue = strings.TrimSpace(timeIssue + " Waktu sumber sebelum awal periode; perlu tinjauan.")
		}
		items := []api.WAContentItem{}
		if actualKind != "message" {
			rows, err := q.Query(ctx, `select distinct x.content_id from wa_content_items x join wa_inbound original on original.id=x.inbound_id where original.connection_id=$1 and original.group_jid=$2 and original.message_id=$3 and x.content_id is not null`, cid, in.GroupJID, in.MessageID)
			if err != nil {
				return err
			}
			affected, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
			if err != nil {
				return err
			}
			if len(affected) == 0 {
				return ignore("Perubahan pesan tanpa bukti konten yang sudah terhubung.")
			}
			data, _ := json.Marshal(map[string]any{"kind": actualKind, "messageId": in.MessageID, "body": in.Body})
			items = append(items, api.WAContentItem{Kind: "correction", Position: 1, Parsed: data, Candidates: affected, Issue: "Pesan sumber diedit/dihapus. Dampak pada bukti dan waktu pertama memerlukan keputusan manusia."})
		} else {
			if policy.BroadcastEnabled && policy.TemplateConfirmed {
				for _, item := range parsed {
					candidates, err := s.waConceptCandidates(r, q, group.SpaceID, item, period.TimeZone)
					if err != nil {
						return err
					}
					data, _ := json.Marshal(item)
					issue := item.Issue
					if len(candidates) != 1 {
						issue = strings.TrimSpace(issue + " Kandidat kalender belum tunggal; pilih melalui tinjauan.")
					}
					items = append(items, api.WAContentItem{Kind: "concept", Position: item.Position, Parsed: data, Candidates: candidates, Issue: issue})
				}
			}
			if policy.DriveEnabled {
				candidates := []id.ID{}
				if sender != nil {
					candidates, err = s.waDriveCandidates(r, q, group.SpaceID, *sender, in.Body, in.QuotedMessageID)
					if err != nil {
						return err
					}
				}
				for index, link := range links {
					linkCandidates := append([]id.ID{}, candidates...)
					if sender != nil && in.QuotedMessageID == "" && !waHasTitleHint(in.Body) {
						// Reposting a known file/folder must not attach that old
						// result to a different sole remaining active assignment.
						rows, err := q.Query(ctx, `select d.content_id from wa_drive_receipts d join medhum_contents c on c.id=d.content_id join tasks t on t.id=c.design_task_id join task_evidence e on e.id=d.evidence_id where c.space_id=$1 and d.resource_id=$2 and e.match_status in ('pending','verified') and t.progress<>'cancelled' and (t.assignee_id=$3 or exists(select 1 from task_participants tp where tp.task_id=t.id and tp.user_id=$3)) order by d.content_id`, group.SpaceID, link.ResourceID, *sender)
						if err != nil {
							return err
						}
						previous, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
						if err != nil {
							return err
						}
						if len(previous) > 0 {
							linkCandidates = previous
						}
					}
					source := waDriveSource{Link: link, ReferenceOnly: waReferenceText.MatchString(body), QuotedMessageID: in.QuotedMessageID}
					data, _ := json.Marshal(source)
					issue := link.Issue
					if !link.Accessible && issue == "" {
						issue = "Akses Drive belum diperiksa."
					}
					if source.ReferenceOnly {
						issue += " Pesan menyebut bahan referensi."
					}
					if len(linkCandidates) != 1 {
						issue += " Kandidat tugas desain belum tunggal."
					}
					items = append(items, api.WAContentItem{Kind: "drive", Position: index + 1, Parsed: data, Candidates: linkCandidates, Issue: strings.TrimSpace(issue)})
				}
			}
		}
		if len(items) == 0 {
			return ignore("Tidak ada item broadcast/Drive yang dikenali oleh parser aktif.")
		}
		results := []string{}
		for index := range items {
			x := &items[index]
			x.ID = id.New()
			x.InboundID = iid
			x.SpaceID = group.SpaceID
			x.SenderUserID = sender
			x.SourceAt = at
			x.ReceivedAt = in.ReceivedAt
			x.ParserVersion = broadcast.Version
			x.State = "pending"
			x.Version = 1
			if x.Candidates == nil {
				x.Candidates = []id.ID{}
			}
			x.Issue = strings.TrimSpace(x.Issue + " " + senderIssue + " " + timeIssue)
			if err = s.store.SaveWAContentItem(ctx, q, *x, true); err != nil {
				return err
			}
			before := *x
			if x.Kind != "correction" && x.Issue == "" && len(x.Candidates) == 1 && p != nil {
				if _, err = q.Exec(ctx, `savepoint wa_content_apply`); err != nil {
					return err
				}
				if x.Kind == "concept" {
					err = s.applyWAConcept(r, q, p, period, x, x.Candidates[0], false)
				} else {
					err = s.applyWADrive(r, q, p, period, x, x.Candidates[0], false, false)
				}
				if err != nil {
					if _, rollbackErr := q.Exec(ctx, `rollback to savepoint wa_content_apply`); rollbackErr != nil {
						return rollbackErr
					}
					var ae *apiError
					if !errors.As(err, &ae) {
						return err
					}
					*x = before
					x.Issue = ae.message
				}
				if _, err = q.Exec(ctx, `release savepoint wa_content_apply`); err != nil {
					return err
				}
			}
			x.Version++
			if err = s.store.SaveWAContentItem(ctx, q, *x, false); err != nil {
				return err
			}
			if err = s.waContentReview(r, q, p, *x, "whatsapp.content.process", x.Issue, before, *x); err != nil {
				return err
			}
			if x.State == "applied" && strings.HasPrefix(x.Issue, "Kiriman ulang:") {
				continue
			}
			description := waTextLanguage(conn.Language, "content.review")
			if x.State == "applied" {
				if x.Kind == "concept" {
					description = waTextLanguage(conn.Language, "content.concept")
				} else {
					description = waTextLanguage(conn.Language, "content.evidence")
				}
			}
			results = append(results, fmt.Sprintf("%s %d: %s", waTextLanguage(conn.Language, "content.kind."+x.Kind), x.Position, description))
		}
		if len(results) > 0 {
			text := strings.Join(results, "\n")
			job := whatsapp.SendRequest{ID: id.New(), GroupJID: group.JID, Text: renderWAText(conn, "reply", "", text), Mentions: []string{}}
			data, err := json.Marshal(job)
			if err != nil {
				return err
			}
			if _, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,actor_id,payload,available_at,created_at,updated_at) values($1,$2,$3,$4,'parser',$5,$6,$7,$8,$8,$8) on conflict(event_key) do nothing`, job.ID, "parser:"+iid.String(), cid, group.ID, iid, sender, data, s.now()); err != nil {
				return err
			}
		}
		state := "applied"
		for _, x := range items {
			if x.State == "pending" {
				state = "review"
				break
			}
		}
		_, err = q.Exec(ctx, `update wa_inbound set state=$2,error='' where id=$1`, iid, state)
		return err
	})
	return found, err
}

func waHasTitleHint(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && broadcast.Normalize(parts[0]) == "judul" {
			return true
		}
	}
	return false
}
