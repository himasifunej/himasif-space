package server

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// listPings is a Space's Riwayat ping (canvas A, Tahap 5b): the pings sent
// in it, newest first, with each recipient's delivery status. Whoever may
// send pings in the Space sees them all; any other reader of the Space sees
// only the pings sent to them, and only their own delivery rows. A closed
// period's history stays readable.
func (s *Server) listPings(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			return badRequest("Batas riwayat ping harus 1–200.")
		}
		limit = n
	}
	before := s.now().Add(time.Hour)
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return badRequest("Waktu riwayat ping tidak valid.")
		}
		before = t
	}
	ctx := r.Context()
	q := s.store.DB()
	view, err := s.spaceView(ctx, q, p, sid)
	if err != nil {
		return err
	}
	all := slices.Contains(view.Capabilities, api.CapPingSend)
	rows, err := q.Query(ctx, `select e.id,e.source_id,e.body,e.created_at,e.time_zone,e.related_kind,e.related_id,a.actor_user_id,coalesce(u.display_name,'')
		from delivery_events e
		left join lateral (select actor_user_id from audit_events where target_type='ping' and target_id=e.source_id and action='ping.created' order by occurred_at limit 1) a on true
		left join users u on u.id=a.actor_user_id
		where e.source_kind='ping' and e.space_id=$1 and e.created_at<$2 and e.suppressed_at is null
		and ($3 or exists(select 1 from delivery_recipients d where d.event_id=e.id and d.recipient_id=$4))
		order by e.created_at desc,e.id desc limit $5`, sid, before, all, p.User.ID, limit)
	if err != nil {
		return err
	}
	out := []api.PingRecord{}
	var events []id.ID
	index := map[id.ID]int{}
	for rows.Next() {
		var eid id.ID
		var rec api.PingRecord
		var actor *id.ID
		var actorName string
		if err := rows.Scan(&eid, &rec.ID, &rec.Message, &rec.SentAt, &rec.TimeZone, &rec.RelatedKind, &rec.RelatedID, &actor, &actorName); err != nil {
			rows.Close()
			return err
		}
		if actor != nil {
			rec.Sender = &api.PersonRef{ID: *actor, DisplayName: actorName}
		}
		rec.Recipients = []api.PingRecipientStatus{}
		index[eid] = len(out)
		events = append(events, eid)
		out = append(out, rec)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(events) > 0 {
		if err := s.pingRecipients(r, q, events, index, out, all, p.User.ID); err != nil {
			return err
		}
	}
	writeJSON(w, http.StatusOK, api.List[api.PingRecord]{Items: out})
	return nil
}

func (s *Server) pingRecipients(r *http.Request, q store.Querier, events []id.ID, index map[id.ID]int, out []api.PingRecord, all bool, self id.ID) error {
	rows, err := q.Query(r.Context(), `select d.event_id,d.recipient_id,u.display_name,d.channel,d.status,d.seen_at
		from delivery_recipients d join users u on u.id=d.recipient_id
		where d.event_id=any($1::uuid[]) and ($2 or d.recipient_id=$3)
		order by u.display_name,d.recipient_id,d.channel`, events, all, self)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eid id.ID
		var st api.PingRecipientStatus
		if err := rows.Scan(&eid, &st.Recipient.ID, &st.Recipient.DisplayName, &st.Channel, &st.Status, &st.SeenAt); err != nil {
			return err
		}
		i := index[eid]
		out[i].Recipients = append(out[i].Recipients, st)
	}
	return rows.Err()
}
