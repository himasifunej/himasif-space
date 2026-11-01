package server

import (
	"context"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"strings"
)

// queueWADelivery creates a single group job for an event, regardless of its
// number of recipients. Private reminders never enter the group transport.
func (s *Server) queueWADelivery(ctx context.Context, q store.Querier, eid id.ID) error {
	var cid, gid id.ID
	var jid, body string
	rows, err := q.Query(ctx, `select c.id,g.id,g.jid,e.body from delivery_events e join wa_groups g on g.space_id=e.space_id join wa_connections c on c.id=g.connection_id join periods p on p.id=c.period_id where e.id=$1 and e.source_kind<>'reminder' and e.cancelled_at is null and e.suppressed_at is null and c.enabled and p.status<>'closed'`, eid)
	if err != nil {
		return err
	}
	found := rows.Next()
	if found {
		err = rows.Scan(&cid, &gid, &jid, &body)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		_, err = q.Exec(ctx, `update delivery_recipients set status='failed',last_error='Grup WhatsApp aktif belum dikonfigurasi; pesan belum dikirim.' where event_id=$1 and channel='whatsapp' and status='queued'`, eid)
		return err
	}
	connection, err := s.store.WAConnection(ctx, q, cid, store.LockNone)
	if err != nil {
		return err
	}
	var kind, title, messageKey string
	var rawArgs []byte
	if err = q.QueryRow(ctx, `select source_kind,title,message_key,message_args from delivery_events where id=$1`, eid).Scan(&kind, &title, &messageKey, &rawArgs); err != nil {
		return err
	}
	templateKind := "reminder"
	if kind == "ping" {
		templateKind = "ping"
	}
	var args []string
	if err = json.Unmarshal(rawArgs, &args); err != nil {
		return err
	}
	title, body = waDeliveryText(connection.Language, messageKey, title, body, args)
	body = renderWAText(connection, templateKind, title, body)
	// Membership is checked again when projecting mentions. A display name or
	// an imported phone string never constitutes a verified WhatsApp identity.
	rows, err = q.Query(ctx, `select distinct on (r.recipient_id) i.jid from delivery_recipients r join delivery_events e on e.id=r.event_id join spaces sp on sp.id=e.space_id join users u on u.id=r.recipient_id and u.disabled_at is null join wa_identities i on i.period_id=sp.period_id and i.user_id=r.recipient_id where r.event_id=$1 and r.channel='whatsapp' and exists(select 1 from memberships m join space_divisions sd on sd.period_id=m.period_id and sd.division_id=m.division_id where sd.space_id=sp.id and m.user_id=u.id and m.removed_at is null) order by r.recipient_id,i.verified_at desc,i.jid`, eid)
	if err != nil {
		return err
	}
	mentions := []string{}
	for rows.Next() {
		var jid string
		if err = rows.Scan(&jid); err != nil {
			rows.Close()
			return err
		}
		mentions = append(mentions, jid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// The human-readable token accompanies the actual WhatsApp mention metadata.
	for _, jid := range mentions {
		body += "\n@" + strings.Split(jid, "@")[0]
	}
	oid := id.New()
	payload, err := json.Marshal(whatsapp.SendRequest{ID: oid, GroupJID: jid, Text: body, Mentions: mentions})
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,payload,available_at,created_at,updated_at) select $1,$2,$3,$4,'delivery',e.id,$5,e.scheduled_at,$6,$6 from delivery_events e where e.id=$7 and exists(select 1 from delivery_recipients r where r.event_id=e.id and r.channel='whatsapp') on conflict(event_key) do nothing`, oid, "delivery:"+eid.String(), cid, gid, payload, s.now(), eid)
	return err
}
