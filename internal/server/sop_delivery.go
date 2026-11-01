package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// These projections deliberately omit message payloads, phone numbers, QR,
// credentials and transport controls. A SOP reader sees only its division's
// scheduled SOP messages, even if the bot also serves a shared Space.
const sopDeliverySources = `with runs as (
 select f.id,'followup'::text kind,p.period_id,p.division_id,f.policy_version,
 f.scheduled_at,f.created_at,f.outbox_id,f.snapshot,
 coalesce(f.snapshot->>'title','') title
 from sop_followups f join sop_policies p on p.id=f.policy_id
 union all
 select r.id,'weekly'::text,r.period_id,r.division_id,r.policy_version,
 r.scheduled_at,r.created_at,r.outbox_id,r.snapshot,
 coalesce(r.snapshot->'report'->>'label','Rekap mingguan') title
 from sop_report_runs r
) `
const sopDeliveryColumns = `r.id,r.kind,r.title,r.policy_version,r.scheduled_at,r.created_at,
 coalesce(o.status,'unavailable'),coalesce(o.target_name,''),coalesce(o.attempts,0),
 coalesce(o.last_error,''),o.accepted_at,o.updated_at,o.suppressed_at,coalesce(o.suppression_reason,'')`

func (s *Server) sopDeliveries(w http.ResponseWriter, r *http.Request, p *principal) error {
	var after time.Time
	afterID := id.Nil
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		if len(raw) > 200 {
			return badRequest("Cursor pengiriman tidak valid.")
		}
		b, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return badRequest("Cursor pengiriman tidak valid.")
		}
		parts := strings.Split(string(b), "|")
		if len(parts) != 2 {
			return badRequest("Cursor pengiriman tidak valid.")
		}
		after, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return badRequest("Cursor waktu tidak valid.")
		}
		afterID, err = id.Parse(parts[1])
		if err != nil {
			return badRequest("Cursor identitas tidak valid.")
		}
	}
	out := api.SOPDeliveryPage{Items: []api.SOPDelivery{}}
	err := s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		rows, err := q.Query(r.Context(), sopDeliverySources+`select `+sopDeliveryColumns+` from runs r left join wa_outbox o on o.id=r.outbox_id where r.period_id=$1 and r.division_id=$2 and ($3::timestamptz is null or (r.created_at,r.id)<($3,$4)) order by r.created_at desc,r.id desc limit 101`, ac.scope.period.ID, ac.scope.division.ID, optionalSOPCursorTime(after), afterID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item api.SOPDelivery
			if err = rows.Scan(&item.ID, &item.Kind, &item.Title, &item.PolicyVersion, &item.ScheduledAt, &item.CreatedAt, &item.Status, &item.TargetName, &item.Attempts, &item.Error, &item.AcceptedAt, &item.UpdatedAt, &item.SuppressedAt, &item.SuppressionReason); err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		if len(out.Items) > 100 {
			out.HasMore = true
			out.Items = out.Items[:100]
			last := out.Items[99]
			out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.ID.String()))
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func optionalSOPCursorTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *Server) sopDelivery(w http.ResponseWriter, r *http.Request, p *principal) error {
	rid, err := pathID(r, "runId")
	if err != nil {
		return err
	}
	var out api.SOPDelivery
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		var snapshot []byte
		err := q.QueryRow(r.Context(), sopDeliverySources+`select `+sopDeliveryColumns+`,r.snapshot from runs r left join wa_outbox o on o.id=r.outbox_id where r.id=$1 and r.period_id=$2 and r.division_id=$3`, rid, ac.scope.period.ID, ac.scope.division.ID).Scan(&out.ID, &out.Kind, &out.Title, &out.PolicyVersion, &out.ScheduledAt, &out.CreatedAt, &out.Status, &out.TargetName, &out.Attempts, &out.Error, &out.AcceptedAt, &out.UpdatedAt, &out.SuppressedAt, &out.SuppressionReason, &snapshot)
		if err != nil {
			return err
		}
		// Only the SOP snapshot is exposed; transport payloads are not read here.
		out.Snapshot = json.RawMessage(snapshot)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
