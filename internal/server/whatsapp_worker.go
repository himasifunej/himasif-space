package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
)

// RunWhatsApp is separate from scheduling and source polling. All transport
// identities and cursors survive process restarts and multiple API instances.
func (s *Server) RunWhatsApp(ctx context.Context) {
	if !s.cfg.OutboundEnabled() {
		return
	}
	var workers sync.WaitGroup
	run := func(name string, interval time.Duration, work func()) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer s.workerStop(name)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				s.workerStart(name)
				work()
				s.workerFinish(name)
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	run("wa.poll", 10*time.Second, func() {
		if !s.wa.Available() {
			return
		}
		ids, err := s.waConnectionIDs(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.workerProgress("wa.poll", true)
				s.log.Info("whatsapp connection list deferred")
			}
			return
		}
		var polling sync.WaitGroup
		slots := make(chan struct{}, 2)
		for _, cid := range ids {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				polling.Wait()
				return
			}
			polling.Add(1)
			go func(cid id.ID) {
				defer polling.Done()
				defer func() { <-slots }()
				work, cancel := context.WithTimeout(ctx, 45*time.Second)
				defer cancel()
				if err := s.pollWAConnection(work, cid); err != nil && ctx.Err() == nil {
					s.workerProgress("wa.poll", true)
					s.log.Info("whatsapp polling deferred", "connection_id", cid)
				}
			}(cid)
		}
		polling.Wait()
	})
	run("wa.commands", time.Second, func() {
		for n := 0; n < 20 && ctx.Err() == nil; n++ {
			work, cancel := context.WithTimeout(ctx, 15*time.Second)
			found, err := s.processWACommand(work)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					s.workerProgress("wa.commands", true)
					s.log.Info("whatsapp command processing deferred")
				}
				return
			}
			if !found {
				return
			}
		}
	})
	run("wa.content", 3*time.Second, func() {
		for n := 0; n < 10 && ctx.Err() == nil; n++ {
			work, cancel := context.WithTimeout(ctx, 70*time.Second)
			found, err := s.processWAContent(work)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					s.workerProgress("wa.content", true)
					s.log.Info("whatsapp content processing deferred")
				}
				return
			}
			if !found {
				return
			}
		}
	})
	run("wa.outbox", 3*time.Second, func() {
		if !s.wa.Available() {
			return
		}
		ids, err := s.waConnectionIDs(ctx)
		if err != nil {
			s.workerProgress("wa.outbox", true)
			return
		}
		var sending sync.WaitGroup
		slots := make(chan struct{}, 2)
		for _, cid := range ids {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				sending.Wait()
				return
			}
			sending.Add(1)
			go func(cid id.ID) {
				defer sending.Done()
				defer func() { <-slots }()
				for n := 0; n < 5 && ctx.Err() == nil; n++ {
					work, cancel := context.WithTimeout(ctx, 40*time.Second)
					found, err := s.deliverWAConnection(work, cid)
					cancel()
					if err != nil {
						if ctx.Err() == nil {
							s.workerProgress("wa.outbox", true)
							s.log.Info("whatsapp outbox deferred", "connection_id", cid)
						}
						return
					}
					if !found {
						return
					}
				}
			}(cid)
		}
		sending.Wait()
	})
	workers.Wait()
}

func (s *Server) waConnectionIDs(ctx context.Context) ([]id.ID, error) {
	rows, err := s.store.DB().Query(ctx, `select id from wa_connections order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []id.ID{}
	for rows.Next() {
		var cid id.ID
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		out = append(out, cid)
	}
	return out, rows.Err()
}

func (s *Server) pollWAConnection(ctx context.Context, cid id.ID) error {
	_, err := s.withWATransport(ctx, cid, func() error {
		snap, err := s.waTransportSnapshot(ctx, cid)
		if err != nil {
			return err
		}
		state, err := s.wa.Configure(ctx, cid, snap.allowedGroups())
		if err != nil {
			return err
		}
		if !snap.Connection.Enabled || snap.Closed {
			if state.Status != "disconnected" && state.Status != "needs_pairing" {
				state, err = s.wa.Action(ctx, cid, "disconnect")
			}
		} else if slices.Contains([]string{"disconnected", "backpressure"}, state.Status) {
			state, err = s.wa.Action(ctx, cid, "connect")
		}
		if err != nil {
			return err
		}
		events, err := s.wa.Events(ctx, cid, snap.Cursor)
		if err != nil {
			return err
		}
		cursor := snap.Cursor
		err = s.store.Tx(ctx, func(q store.Querier) error {
			period, err := s.store.Period(ctx, q, snap.Connection.PeriodID, store.LockShare)
			if err != nil {
				return err
			}
			c, err := s.store.WAConnection(ctx, q, cid, store.LockUpdate)
			if err != nil {
				return err
			}
			groups, err := s.store.WAGroups(ctx, q, cid)
			if err != nil {
				return err
			}
			var currentCursor int64
			if err = q.QueryRow(ctx, `select event_cursor from wa_connections where id=$1`, cid).Scan(&currentCursor); err != nil {
				return err
			}
			if currentCursor != snap.Cursor {
				return errors.New("whatsapp cursor changed during transport")
			}
			if c.Version == snap.Connection.Version {
				if _, err = q.Exec(ctx, `update wa_connections set status=$2,account_jid=$3,last_error=$4,last_polled_at=$5 where id=$1`, cid, state.Status, state.AccountJID, state.LastError, s.now()); err != nil {
					return err
				}
			}
			control, err := s.store.RuntimeControl(ctx, q, store.LockNone)
			if err != nil {
				return err
			}
			for _, event := range events {
				// A missing sequence means a bridge backup/state mismatch. Never silently
				// advance past it or clear the bridge queue in that situation.
				if event.Sequence != cursor+1 {
					return errors.New("whatsapp event sequence is not contiguous")
				}
				data, err := json.Marshal(event)
				if err != nil {
					return err
				}
				if _, err = q.Exec(ctx, `insert into wa_bridge_events(connection_id,sequence,data,created_at) values($1,$2,$3,$4) on conflict do nothing`, cid, event.Sequence, data, s.now()); err != nil {
					return err
				}
				if slices.Contains([]string{"message", "edit", "revoke"}, event.Type) {
					var gid *id.ID
					for _, g := range groups {
						if g.JID == event.GroupJID {
							v := g.ID
							gid = &v
							break
						}
					}
					state, reason := "queued", ""
					if gid == nil || !c.Enabled || period.Status == api.PeriodClosed {
						state, reason = "ignored", "Grup tidak aktif atau periode ditutup saat penerimaan."
					}
					if event.MessageID == "" || event.EnvelopeID == "" || event.Revision == "" || event.ReceivedAt.IsZero() {
						return errors.New("incomplete whatsapp envelope")
					}
					if control.OutboundNotBefore != nil && (event.ReceivedAt.Before(*control.OutboundNotBefore) || event.OccurredAt == nil || event.OccurredAt.Before(*control.OutboundNotBefore)) {
						state, reason = "ignored", "Input before activation cutoff or missing source timestamp; retained without replay."
					}
					mentions, err := json.Marshal(event.MentionedJIDs)
					if err != nil {
						return err
					}
					media, err := json.Marshal(event.Media)
					if err != nil {
						return err
					}
					if _, err = q.Exec(ctx, `insert into wa_inbound(id,connection_id,group_id,group_jid,sender_jid,sender_alt_jid,message_id,envelope_id,revision,kind,body,quoted_message_id,mentions,media,occurred_at,received_at,state,error) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) on conflict(connection_id,envelope_id,revision) do nothing`, id.New(), cid, gid, event.GroupJID, event.SenderJID, event.SenderAltJID, event.MessageID, event.EnvelopeID, event.Revision, event.Type, event.Text, event.QuotedMessageID, mentions, media, event.OccurredAt, event.ReceivedAt, state, reason); err != nil {
						return err
					}
				}
				cursor = event.Sequence
				if _, err = q.Exec(ctx, `update wa_connections set event_cursor=$2,last_event_at=$3 where id=$1`, cid, cursor, event.ReceivedAt); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return s.wa.Acknowledge(ctx, cid, cursor)
	})
	return err
}

type waClaim struct {
	ID, ConnectionID, PeriodID id.ID
	Status                     string
	Lease                      time.Time
}

func (s *Server) claimWAJob(ctx context.Context, cid id.ID) (waClaim, bool, error) {
	var claim waClaim
	found := false
	err := s.store.Tx(ctx, func(q store.Querier) error {
		err := q.QueryRow(ctx, `select o.id,o.connection_id,c.period_id,o.status from wa_outbox o join wa_connections c on c.id=o.connection_id where c.id=$3 and o.suppressed_at is null and o.available_at<=$1 and (o.lease_until is null or o.lease_until<=$1) and (o.status in ('queued','unknown') or (o.status='sending') or (o.status in ('accepted','delivered') and o.created_at>$2)) order by o.available_at,o.id limit 1 for update of o skip locked`, s.now(), s.now().AddDate(0, 0, -7), cid).Scan(&claim.ID, &claim.ConnectionID, &claim.PeriodID, &claim.Status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		claim.Lease = s.now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond)
		_, err = q.Exec(ctx, `update wa_outbox set status=case when status='queued' then 'sending' else status end,lease_until=$2,updated_at=$3 where id=$1`, claim.ID, claim.Lease, s.now())
		return err
	})
	return claim, found, err
}

func (s *Server) deliverWAConnection(ctx context.Context, cid id.ID) (bool, error) {
	found := false
	_, err := s.withWATransport(ctx, cid, func() error {
		claim, ok, err := s.claimWAJob(ctx, cid)
		if err != nil {
			return err
		}
		found = ok
		if !ok {
			return nil
		}
		return s.dispatchWAClaim(ctx, claim)
	})
	return found, err
}

func (s *Server) dispatchWAClaim(ctx context.Context, claim waClaim) error {
	var request whatsapp.SendRequest
	var current, sourceKind, status string
	var sourceID *id.ID
	var attempts int
	eligible, send, reconcile := false, false, false
	// Commit the authorization and durable attempt before entering transport.
	// Configuration changes can cancel queued work; an authorized attempt that
	// has entered sending is in flight and its receipt is reconciled afterwards.
	err := s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, claim.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		conn, err := s.store.WAConnection(ctx, q, claim.ConnectionID, store.LockShare)
		if err != nil {
			return err
		}
		// Group locks protect the authorization snapshot. They are released
		// before transport, once the durable attempt has entered sending.
		var groupID id.ID
		if err = q.QueryRow(ctx, `select group_id from wa_outbox where id=$1`, claim.ID).Scan(&groupID); err != nil {
			return err
		}
		var jid string
		var spaceID id.ID
		var commandsEnabled bool
		if err = q.QueryRow(ctx, `select jid,space_id,commands_enabled from wa_groups where id=$1 and connection_id=$2 for share`, groupID, conn.ID).Scan(&jid, &spaceID, &commandsEnabled); err != nil {
			return err
		}
		var payload []byte
		var actorID *id.ID
		var lease *time.Time
		if err = q.QueryRow(ctx, `select payload,status,source_kind,source_id,actor_id,attempts,lease_until from wa_outbox where id=$1 for update`, claim.ID).Scan(&payload, &current, &sourceKind, &sourceID, &actorID, &attempts, &lease); err != nil {
			return err
		}
		if lease == nil || !lease.Equal(claim.Lease) || !slices.Contains([]string{"sending", "unknown", "accepted", "delivered"}, current) {
			return nil
		}
		if err = json.Unmarshal(payload, &request); err != nil {
			return err
		}
		request.ID = claim.ID
		status = current
		eligible = true
		if claim.Status == "queued" {
			valid := conn.Enabled && period.Status != api.PeriodClosed && request.GroupJID == jid
			if sourceKind == "command" || sourceKind == "parser" {
				if sourceKind == "command" {
					valid = valid && commandsEnabled
				}
				if actorID != nil {
					user, err := s.store.UserForUpdate(ctx, q, *actorID)
					if err != nil {
						return err
					}
					if user.DisabledAt != nil {
						valid = false
					} else {
						view, accessErr := s.spaceView(ctx, q, &principal{User: user}, spaceID)
						if accessErr != nil {
							var ae *apiError
							if !errors.As(accessErr, &ae) {
								return accessErr
							}
							valid = false
						} else {
							valid = valid && slices.Contains(view.Capabilities, api.CapSpaceRead)
							if sourceKind == "command" && sourceID != nil {
								var body string
								if err = q.QueryRow(ctx, `select body from wa_inbound where id=$1`, *sourceID).Scan(&body); err != nil {
									return err
								}
								fields := strings.Fields(body)
								if len(fields) > 0 && (strings.EqualFold(fields[0], "!eval") || strings.EqualFold(fields[0], "!rekap")) {
									rr := (&http.Request{}).WithContext(ctx)
									rr.SetPathValue("periodId", period.ID.String())
									rr.SetPathValue("divisionId", conn.DivisionID.String())
									ac, err := s.loadSOPAccess(rr, q, &principal{User: user}, store.LockNone)
									if err != nil {
										return err
									}
									valid = valid && ac.read
									sp, err := s.store.SpaceByID(ctx, q, spaceID)
									if err != nil {
										return err
									}
									valid = valid && sp.Kind == api.SpaceDivision
								}
							}
						}
					}
				}
			}
			if len(request.Mentions) > 0 {
				var mentionsValid bool
				if err = q.QueryRow(ctx, `select not exists(select 1 from unnest($2::text[]) mentioned(jid) where not exists(select 1 from wa_identities i join users u on u.id=i.user_id and u.disabled_at is null join memberships m on m.user_id=u.id and m.period_id=i.period_id and m.removed_at is null join space_divisions sd on sd.period_id=m.period_id and sd.division_id=m.division_id where i.jid=mentioned.jid and sd.space_id=$1 and i.period_id=$3))`, spaceID, request.Mentions, period.ID).Scan(&mentionsValid); err != nil {
					return err
				}
				valid = valid && mentionsValid
			}
			if sourceKind == "sop_followup" {
				live, err := s.validSOPFollowup(ctx, q, sourceID, conn, spaceID)
				if err != nil {
					return err
				}
				valid = valid && live
			}
			if sourceKind == "sop_report" {
				live, err := s.validSOPReport(ctx, q, sourceID, conn, spaceID)
				if err != nil {
					return err
				}
				valid = valid && live
			}
			if sourceKind == "delivery" {
				var live bool
				if err = q.QueryRow(ctx, `select exists(select 1 from delivery_events e where e.id=$1 and e.cancelled_at is null and e.suppressed_at is null and e.source_kind<>'reminder' and e.scheduled_at<=$2 and (
     (e.source_kind='task' and exists(select 1 from tasks t where t.id=e.source_id and t.notification_revision=e.source_version and t.progress in ('todo','doing'))) or
     (e.source_kind='meeting' and exists(select 1 from meetings m where m.id=e.source_id and m.version=e.source_version and not m.cancelled)) or e.source_kind='ping'))`, sourceID, s.now()).Scan(&live); err != nil {
					return err
				}
				valid = valid && live
			}
			if !valid {
				status = "cancelled"
			} else {
				attempts++
				send = true
				if _, err = q.Exec(ctx, `update wa_outbox set attempts=$2 where id=$1`, claim.ID, attempts); err != nil {
					return err
				}
			}
		} else {
			reconcile = true
			if current == "sending" {
				status = "unknown"
			}
		}
		return nil
	})
	if err != nil || !eligible {
		return err
	}
	var job *whatsapp.Job
	messageID, lastError := "", ""
	if status == "cancelled" {
		lastError = "Tujuan, periode, atau sumber berubah sebelum pengiriman."
	}
	if send {
		sent, sendErr := s.wa.Send(ctx, claim.ConnectionID, request)
		if sendErr == nil {
			job = &sent
		} else {
			status, lastError = "unknown", "Hasil kirim belum pasti; menunggu pemeriksaan jurnal bridge."
			var failure *whatsapp.Error
			if errors.As(sendErr, &failure) && failure.Attempted != nil && !*failure.Attempted {
				status, lastError = "failed", "Bridge menolak sebelum mencoba mengirim."
				if failure.Status == 503 && attempts < 8 {
					status, lastError = "queued", "Koneksi belum tersedia; pengiriman belum dicoba oleh bridge."
				}
			}
		}
	} else if reconcile {
		job, err = s.wa.Job(ctx, claim.ConnectionID, claim.ID)
		if err != nil {
			job = nil
		}
		if job == nil {
			lastError = "Jurnal belum dapat mengonfirmasi hasil; tidak dikirim ulang."
		}
	}
	return s.store.Tx(ctx, func(q store.Querier) error {
		var lease *time.Time
		var rowStatus string
		if err := q.QueryRow(ctx, `select lease_until,status from wa_outbox where id=$1 for update`, claim.ID).Scan(&lease, &rowStatus); err != nil {
			return err
		}
		if lease == nil || !lease.Equal(claim.Lease) || !slices.Contains([]string{"sending", "unknown", "accepted", "delivered"}, rowStatus) {
			return nil
		}
		var acceptedAt *time.Time
		if job != nil {
			if job.ID != claim.ID {
				return errors.New("bridge returned another job identity")
			}
			messageID = job.MessageID
			if job.AcceptedAt != nil && !job.AcceptedAt.After(s.now().Add(5*time.Minute)) {
				acceptedAt = job.AcceptedAt
			}
			switch job.Status {
			case "accepted", "delivered", "read":
				status = job.Status
			case "sending", "unknown":
				if current != "accepted" && current != "delivered" {
					status = "unknown"
				}
				lastError = "Bridge belum memastikan hasil kirim."
			default:
				return fmt.Errorf("unsupported bridge job status")
			}
			// Do not regress a receipt when an earlier bridge snapshot is observed.
			if current == "delivered" && status == "accepted" {
				status = "delivered"
			}
		}
		delay := time.Minute
		if status == "queued" {
			delay = time.Duration(1<<min(attempts, 8)) * 15 * time.Second
		}
		if _, err = q.Exec(ctx, `update wa_outbox set status=$2,attempts=$3,message_id=case when $4='' then message_id else $4 end,last_error=$5,available_at=$6,lease_until=null,updated_at=$7,accepted_at=coalesce(accepted_at,$8) where id=$1`, claim.ID, status, attempts, messageID, lastError, s.now().Add(delay), s.now(), acceptedAt); err != nil {
			return err
		}
		if sourceKind == "delivery" {
			deliveryStatus := status
			if slices.Contains([]string{"delivered", "read"}, status) {
				deliveryStatus = "accepted"
			}
			if deliveryStatus == "sending" {
				deliveryStatus = "unknown"
			}
			_, err = q.Exec(ctx, `update delivery_recipients set status=$2,attempts=$3,last_error=$4,accepted_at=case when $2='accepted' then coalesce(accepted_at,$5) else accepted_at end where event_id=$1 and channel='whatsapp'`, sourceID, deliveryStatus, attempts, lastError, s.now())
		}
		return err
	})
}
