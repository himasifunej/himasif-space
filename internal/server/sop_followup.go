package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func (s *Server) scheduleSOPFollowup(ctx context.Context, ref sopPolicyRef, cid id.ID) error {
	return s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, ref.Period, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return nil
		}
		var locked bool
		if err = q.QueryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1,0))`, "sop-followup:"+cid.String()).Scan(&locked); err != nil {
			return err
		}
		if !locked {
			return nil
		}
		c, err := s.store.Medhum(ctx, q, cid, store.LockShare)
		if err != nil {
			return err
		}
		if c.HumasDivisionID != ref.Division || c.PublishedAt != nil {
			return nil
		}
		policy, err := s.store.SOPPolicy(ctx, q, ref.Period, ref.Division, store.LockNone)
		if err != nil {
			return err
		}
		if !policy.Enabled || policy.Settings.FollowupClock == "" {
			return nil
		}
		loc, err := timeutil.LoadZone(period.TimeZone)
		if err != nil {
			return err
		}
		post := c.PostingDate
		if post == nil {
			day := timeutil.DateOf(c.PostingAt.In(loc))
			post = &day
		}
		clock, err := time.Parse("15:04", policy.Settings.FollowupClock)
		if err != nil {
			return err
		}
		slot := post.AddDays(-4).At(clock.Hour(), clock.Minute(), loc)
		today := timeutil.DateOf(s.now().In(loc))
		if timeutil.DateOf(slot).Compare(today) != 0 || slot.After(s.now()) {
			return nil
		}
		key := sopCalendarKey(c)
		var sent bool
		if err = q.QueryRow(ctx, `select exists(select 1 from sop_followups f join wa_outbox o on o.id=f.outbox_id where f.content_id=$1 and f.calendar_key=$2 and (f.scheduled_at at time zone $3)::date=$4 and o.status in ('sending','unknown','accepted','delivered','read'))`, cid, key, period.TimeZone, today).Scan(&sent); err != nil {
			return err
		}
		if sent {
			return nil
		}
		if c.DesignPJID == nil || c.ConceptURL == "" {
			return nil
		}
		if err = s.lockTaskAccounts(ctx, q, []id.ID{c.HumasPJID, *c.DesignPJID}); err != nil {
			return err
		}
		humas, err := s.taskAssignee(ctx, q, period.ID, c.HumasDivisionID, c.HumasPJID)
		if err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				return nil
			}
			return err
		}
		design, err := s.taskAssignee(ctx, q, period.ID, c.DesignDivisionID, *c.DesignPJID)
		if err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				return nil
			}
			return err
		}
		var cidWA, gid id.ID
		var jid, mention string
		err = q.QueryRow(ctx, `select g.connection_id,g.id,g.jid from wa_groups g join wa_connections w on w.id=g.connection_id join spaces sp on sp.id=g.space_id where g.space_id=$1 and sp.code='medhum' and w.period_id=$2 and w.division_id=$3 and w.enabled and exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$2 and m.division_id=$3 and m.role='kadiv' and m.removed_at is null and u.disabled_at is null)`, c.SpaceID, period.ID, c.HumasDivisionID).Scan(&cidWA, &gid, &jid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		err = q.QueryRow(ctx, `select jid from wa_identities where period_id=$1 and user_id=$2 order by verified_at desc,jid limit 1`, period.ID, *c.DesignPJID).Scan(&mention)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		conn, err := s.store.WAConnection(ctx, q, cidWA, store.LockShare)
		if err != nil {
			return err
		}
		snapshot := sopFollowupSnapshot{CalendarKey: key, Title: c.Title, ConceptURL: c.ConceptURL, HumasPJ: c.HumasPJID, DesignPJ: *c.DesignPJID, BotCountsRequested: policy.Settings.BotFollowupCountsRequested}
		if snapshot.BotCountsRequested && policy.BotFollowupCountsApproved {
			var agreement id.ID
			err = q.QueryRow(ctx, `select id from sop_bot_agreements where policy_id=$1 and policy_version=$2 and approved order by created_at desc,id desc limit 1`, policy.ID, policy.Version).Scan(&agreement)
			if err != nil {
				return err
			}
			snapshot.AgreementID = &agreement
		}
		// A policy save with no material message change must not replay an already
		// delivered reminder. Queued stale generations are cancelled before reuse.
		if _, err = q.Exec(ctx, `update wa_outbox o set status='cancelled',last_error='Kebijakan follow-up berubah sebelum pengiriman.' from sop_followups f where f.outbox_id=o.id and f.content_id=$1 and o.status in ('queued','failed') and (f.policy_version<>$2 or f.calendar_key<>$3)`, cid, policy.Version, key); err != nil {
			return err
		}
		var fid id.ID
		var oldOutbox *id.ID
		err = q.QueryRow(ctx, `select id,outbox_id from sop_followups where content_id=$1 and scheduled_at=$2 and policy_version=$3 and calendar_key=$4 for update`, cid, slot, policy.Version, key).Scan(&fid, &oldOutbox)
		if err == nil && oldOutbox != nil {
			return nil
		}
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !exists {
			fid = id.New()
		}
		text := waTextLanguage(conn.Language, "followup", humas.DisplayName, c.Title, design.DisplayName, strings.Split(mention, "@")[0], post.String(), c.ConceptURL)
		oid := id.New()
		payload, err := json.Marshal(whatsapp.SendRequest{ID: oid, GroupJID: jid, Text: renderWAText(conn, "followup", c.Title, text), Mentions: []string{mention}})
		if err != nil {
			return err
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if !exists {
			if _, err = q.Exec(ctx, `insert into sop_followups(id,content_id,policy_id,policy_version,calendar_version,calendar_key,scheduled_at,snapshot,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, fid, cid, policy.ID, policy.Version, c.Version, key, slot, data, s.now()); err != nil {
				return err
			}
		}
		if _, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,payload,available_at,created_at,updated_at) values($1,$2,$3,$4,'sop_followup',$5,$6,$7,$8,$8)`, oid, "sop-followup:"+fid.String(), cidWA, gid, fid, payload, slot, s.now()); err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `update sop_followups set outbox_id=$2,snapshot=$3 where id=$1`, fid, oid, data); err != nil {
			return err
		}
		r := (&http.Request{}).WithContext(ctx)
		return s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Source: "sop", Action: "sop.followup.queue", TargetType: "sop_followup", TargetID: &fid, PeriodID: &period.ID, ScopeType: "division", ScopeID: &c.HumasDivisionID, After: snapshot})
	})
}

// Dispatch validates the exact source generation and the Humas-owned Medhum
// mapping again. A reply/summary pathway cannot route a finding to Medhum.
func (s *Server) validSOPFollowup(ctx context.Context, q store.Querier, fid *id.ID, conn api.WAConnection, spaceID id.ID) (bool, error) {
	if fid == nil {
		return false, nil
	}
	var contentID, policyID id.ID
	var version int64
	var key string
	var slot time.Time
	err := q.QueryRow(ctx, `select content_id,policy_id,policy_version,calendar_key,scheduled_at from sop_followups where id=$1`, *fid).Scan(&contentID, &policyID, &version, &key, &slot)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	c, err := s.store.Medhum(ctx, q, contentID, store.LockNone)
	if err != nil {
		return false, err
	}
	if c.SpaceID != spaceID || c.HumasDivisionID != conn.DivisionID || c.PublishedAt != nil || key != sopCalendarKey(c) {
		return false, nil
	}
	policy, err := s.store.SOPPolicy(ctx, q, c.PeriodID, c.HumasDivisionID, store.LockNone)
	if err != nil {
		return false, err
	}
	var peopleActive bool
	if c.DesignPJID == nil {
		return false, nil
	}
	if err = q.QueryRow(ctx, `select exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and m.user_id=$3 and m.removed_at is null and u.disabled_at is null) and exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$4 and m.user_id=$5 and m.removed_at is null and u.disabled_at is null)`, c.PeriodID, c.HumasDivisionID, c.HumasPJID, c.DesignDivisionID, *c.DesignPJID).Scan(&peopleActive); err != nil {
		return false, err
	}
	if !peopleActive {
		return false, nil
	}
	if !policy.Enabled || policy.ID != policyID || policy.Version != version || policy.Settings.FollowupClock == "" {
		return false, nil
	}
	period, err := s.store.Period(ctx, q, c.PeriodID, store.LockNone)
	if err != nil {
		return false, err
	}
	loc, err := timeutil.LoadZone(period.TimeZone)
	if err != nil {
		return false, err
	}
	if timeutil.DateOf(slot.In(loc)).Compare(timeutil.DateOf(s.now().In(loc))) != 0 {
		return false, nil
	}
	return true, nil
}
