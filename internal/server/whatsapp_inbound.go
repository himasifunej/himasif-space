package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
)

type waAuditKey struct{}
type waInbound struct {
	ID, ConnectionID, GroupID               id.ID
	GroupJID, SenderJID, SenderAltJID, Body string
	OccurredAt                              *time.Time
	ReceivedAt                              time.Time
	MessageID, QuotedMessageID              string
}

var waUserJID = regexp.MustCompile(`^[0-9]+@(s\.whatsapp\.net|lid)$`)

// Commands commit their canonical mutation, reply job and processed receipt in
// one transaction. A crash can neither apply twice nor reply before committing.
func (s *Server) processWACommand(ctx context.Context) (bool, error) {
	var peekID, cid, pid id.ID
	var peekBody string
	err := s.store.DB().QueryRow(ctx, `select i.id,i.connection_id,c.period_id,i.body from wa_inbound i join wa_connections c on c.id=i.connection_id where i.state='queued' and i.kind='message' and ltrim(i.body) like '!%' order by i.received_at,i.id limit 1`).Scan(&peekID, &cid, &pid, &peekBody)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fields := strings.Fields(peekBody)
	if len(fields) > 0 && strings.HasPrefix(fields[0], "!") {
		if kind, ok := waMediaCommand(strings.ToLower(fields[0][1:])); ok {
			prepareCtx, cancel := context.WithTimeout(ctx, kind.Timeout)
			prepared := s.prepareWAMedia(prepareCtx, kind, peekID, pid, cid)
			cancel()
			ctx = context.WithValue(ctx, waMediaKey{}, prepared)
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
		var gid *id.ID
		err = q.QueryRow(ctx, `select id,connection_id,group_id,group_jid,sender_jid,sender_alt_jid,body,occurred_at,received_at,message_id,quoted_message_id from wa_inbound where id=$1 and state='queued' for update skip locked`, peekID).Scan(&in.ID, &in.ConnectionID, &gid, &in.GroupJID, &in.SenderJID, &in.SenderAltJID, &in.Body, &in.OccurredAt, &in.ReceivedAt, &in.MessageID, &in.QuotedMessageID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		var group api.WAGroup
		if gid != nil {
			err = q.QueryRow(ctx, `select id,space_id,jid,name,commands_enabled from wa_groups where id=$1 and connection_id=$2 for share`, *gid, cid).Scan(&group.ID, &group.SpaceID, &group.JID, &group.Name, &group.CommandsEnabled)
			if err != nil {
				return err
			}
		}
		in.GroupID = group.ID
		// A direct message to the bot carries only a link code (waDirectInbound).
		direct := gid == nil && in.GroupJID == ""
		if direct && (!conn.Enabled || period.Status == api.PeriodClosed || !(strings.HasPrefix(in.Body, "!link ") || in.Body == "!pindah")) {
			_, err = q.Exec(ctx, `update wa_inbound set state='ignored',error='Bot tidak aktif atau periode ditutup.' where id=$1`, in.ID)
			return err
		}
		if !direct && (gid == nil || group.JID != in.GroupJID || !group.CommandsEnabled || !conn.Enabled || period.Status == api.PeriodClosed) {
			_, err = q.Exec(ctx, `update wa_inbound set state='ignored',error='Command dinonaktifkan, grup berubah, atau periode ditutup.' where id=$1`, in.ID)
			return err
		}
		r := (&http.Request{}).WithContext(context.WithValue(context.WithValue(ctx, waLanguageKey{}, conn.Language), waAuditKey{}, in.ID))
		r.SetPathValue("periodId", pid.String())
		r.SetPathValue("divisionId", conn.DivisionID.String())
		fields := strings.Fields(in.Body)
		command := strings.ToLower(strings.TrimPrefix(fields[0], "!"))
		if _, err = q.Exec(ctx, `savepoint wa_command`); err != nil {
			return err
		}
		var reply whatsapp.SendRequest
		var actorID *id.ID
		// Linking answers in plain text: no bot name or greeting around it.
		plain := command == "link" || command == "pindah"
		if command == "link" {
			reply.Text, err = s.consumeWALink(r, q, period, group, in, fields)
		} else if command == "pindah" {
			reply.Text, err = s.moveWALink(r, q, period, group, in)
		} else {
			var p *principal
			p, err = s.waSender(r, q, period, group, in)
			if err == nil {
				uid := p.User.ID
				actorID = &uid
				reply, err = s.runWACommand(r, q, p, period, conn, group, in, command, fields[1:])
			}
		}
		state, issue := "applied", ""
		if err != nil {
			if _, rollbackErr := q.Exec(ctx, `rollback to savepoint wa_command`); rollbackErr != nil {
				return rollbackErr
			}
			var ae *apiError
			if !errors.As(err, &ae) {
				return err
			}
			reply = whatsapp.SendRequest{Text: waFailure(conn.Language, ae)}
			state, issue = "failed", ae.code
		}
		if _, err = q.Exec(ctx, `release savepoint wa_command`); err != nil {
			return err
		}
		if reply.Text != "" || reply.AssetID != "" {
			if reply.Text != "" {
				if !plain {
					reply.Text = renderWAText(conn, "reply", "", reply.Text)
				}
			}
			parts := []whatsapp.SendRequest{reply}
			if reply.AssetID != "" && reply.MediaType == "sticker" && reply.Text != "" {
				// WhatsApp stickers have no caption. Keep text and media as two
				// independently journaled jobs rather than silently losing text.
				parts[0].AssetID, parts[0].MediaType = "", ""
				media := reply
				media.Text = ""
				parts = append(parts, media)
			}
			for index, part := range parts {
				part.ID = id.New()
				part.GroupJID = group.JID
				groupID, source := &group.ID, "command"
				if direct {
					part.GroupJID, part.ToJID, groupID, source = "", in.SenderJID, nil, "link_dm"
				}
				if part.Mentions == nil {
					part.Mentions = []string{}
				}
				payload, err := json.Marshal(part)
				if err != nil {
					return err
				}
				key := "command:" + in.ID.String()
				if index > 0 {
					key += ":media"
				}
				if _, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,actor_id,payload,available_at,created_at,updated_at) values($1,$2,$3,$4,$9,$5,$6,$7,$8,$8,$8) on conflict(event_key) do nothing`, part.ID, key, cid, groupID, in.ID, actorID, payload, s.now(), source); err != nil {
					return err
				}
			}
		}
		_, err = q.Exec(ctx, `update wa_inbound set state=$2,error=$3 where id=$1`, in.ID, state, issue)
		return err
	})
	return found, err
}

func (s *Server) waSender(r *http.Request, q store.Querier, period api.Period, group api.WAGroup, in waInbound) (*principal, error) {
	if !waUserJID.MatchString(in.SenderJID) {
		return nil, errForbidden
	}
	rows, err := q.Query(r.Context(), `select distinct user_id from wa_identities where period_id=$1 and (jid=$2 or ($3<>'' and jid=$3))`, period.ID, in.SenderJID, in.SenderAltJID)
	if err != nil {
		return nil, err
	}
	users := []id.ID{}
	for rows.Next() {
		var uid id.ID
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return nil, err
		}
		users = append(users, uid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(users) != 1 {
		return nil, badRequest("Identitas WhatsApp belum terverifikasi atau memiliki konflik. Buat kode tautan di akun Space, lalu kirim !link <kode>.")
	}
	u, err := s.store.UserForUpdate(r.Context(), q, users[0])
	if err != nil {
		return nil, err
	}
	if u.DisabledAt != nil || u.Kind == api.AccountOrganizationAdmin {
		return nil, errForbidden
	}
	p := &principal{User: u}
	view, err := s.spaceView(r.Context(), q, p, group.SpaceID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
		return nil, errForbidden
	}
	return p, nil
}

func (s *Server) runWACommand(r *http.Request, q store.Querier, p *principal, period api.Period, conn api.WAConnection, group api.WAGroup, in waInbound, command string, args []string) (whatsapp.SendRequest, error) {
	reply := whatsapp.SendRequest{}
	switch command {
	case "help", "bantuan":
		reply.Text = waText(r.Context(), "help")
	case "tugas", "hari", "pending":
		loc, err := timeutil.LoadZone(period.TimeZone)
		if err != nil {
			return reply, err
		}
		today := s.now().In(loc).Format("2006-01-02")
		rows, err := q.Query(r.Context(), `select t.id,t.title,t.progress,t.match_status,u.display_name,coalesce(t.deadline_date::text,to_char(t.deadline_at at time zone $2,'YYYY-MM-DD HH24:MI')) from tasks t join task_spaces ts on ts.task_id=t.id join users u on u.id=t.assignee_id where ts.space_id=$1 and t.progress not in ('done','cancelled') and ($3<>'pending' or t.match_status='pending') and ($3<>'hari' or coalesce(t.deadline_date,(t.deadline_at at time zone $2)::date)=$4::date) order by coalesce(t.deadline_date,(t.deadline_at at time zone $2)::date),t.id limit 31`, group.SpaceID, period.TimeZone, command, today)
		if err != nil {
			return reply, err
		}
		lines := []string{}
		for rows.Next() {
			var tid id.ID
			var title, progress, match, name, due string
			if err := rows.Scan(&tid, &title, &progress, &match, &name, &due); err != nil {
				rows.Close()
				return reply, err
			}
			lines = append(lines, fmt.Sprintf("%s\n%s · %s · %s · %s\nID %s", title, name, due, waProgress(conn.Language, progress), waMatch(conn.Language, match), tid))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return reply, err
		}
		if len(lines) > 30 {
			lines = lines[:30]
			lines = append(lines, waText(r.Context(), "tasks.limit"))
		}
		reply.Text = strings.Join(lines, "\n\n")
		if reply.Text == "" {
			reply.Text = waText(r.Context(), "tasks.empty")
		}
	case "jadwal":
		rows, err := q.Query(r.Context(), `select title,to_char(starts_at at time zone $2,'YYYY-MM-DD HH24:MI'),location from meetings where space_id=$1 and not cancelled and (ends_at>$3 or (ends_at is null and starts_at>$3 - interval '1 day')) order by starts_at,id limit 20`, group.SpaceID, period.TimeZone, s.now())
		if err != nil {
			return reply, err
		}
		lines := []string{}
		for rows.Next() {
			var title, at, location string
			if err := rows.Scan(&title, &at, &location); err != nil {
				rows.Close()
				return reply, err
			}
			lines = append(lines, title+" · "+at+" · "+location)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return reply, err
		}
		reply.Text = strings.Join(lines, "\n")
		if reply.Text == "" {
			reply.Text = waText(r.Context(), "meetings.empty")
		}
	case "summary", "rekap", "eval":
		return s.waReportCommand(r, q, p, period, group, command, args)
	case "cek", "done", "bantu", "delegasi":
		return s.waTaskCommand(r, q, p, period, group, in, command, args)
	case "sticker", "setsticker", "setgif":
		kind, _ := waMediaCommand(command)
		return s.waMediaCommand(r, q, p, conn, group, in, kind, command, args)
	default:
		err := q.QueryRow(r.Context(), `select response,asset_id,media_type from wa_custom_commands where connection_id=$1 and trigger=$2 and (cardinality(group_ids)=0 or $3=any(group_ids))`, conn.ID, command, group.ID).Scan(&reply.Text, &reply.AssetID, &reply.MediaType)
		if errors.Is(err, pgx.ErrNoRows) {
			return reply, badRequest("Command belum tersedia. Gunakan !help untuk daftar command.")
		}
		if err != nil {
			return reply, err
		}
		reply.Text = renderWACommandText(reply.Text, waCommandValues{Sender: p.User.DisplayName, Group: group.Name, Bot: conn.DisplayName, Args: strings.Join(args, " "), At: s.now(), Zone: period.TimeZone})
	}
	return reply, nil
}
