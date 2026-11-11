package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"strconv"
	"time"
)

// RunScheduler belongs to the server lifetime. PostgreSQL serializes instances;
// persistent unique event keys, never local timers, determine delivery identity.
func (s *Server) RunScheduler(ctx context.Context) {
	if !s.cfg.OutboundEnabled() {
		return
	}
	defer s.workerStop("scheduler")
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		s.workerStart("scheduler")
		c, cancel := context.WithTimeout(ctx, 25*time.Second)
		if err := s.schedule(c); err != nil && ctx.Err() == nil {
			s.workerProgress("scheduler", true)
			s.log.Error("scheduler", "err", err)
		}
		cancel()
		s.workerFinish("scheduler")
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

type scheduledSource struct {
	kind             string
	id               id.ID
	space            *id.ID
	version          int64
	title, zone      string
	created, updated time.Time
	due              time.Time
	dateOnly         bool
	recipients       []id.ID
	minutes          []int
	policy           api.NotificationPolicy
}

func (s *Server) schedule(ctx context.Context) error {
	return s.store.Tx(ctx, func(q store.Querier) error {
		var acquired bool
		if err := q.QueryRow(ctx, `select pg_try_advisory_xact_lock(3578414021)`).Scan(&acquired); err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		now := s.now().UTC()
		// A stale event is cancelled even when it was scheduled by another instance.
		_, err := q.Exec(ctx, `update delivery_events e set cancelled_at=$1 where cancelled_at is null and (
 (source_kind='meeting' and not exists(select 1 from meetings m join periods p on p.id=m.period_id where m.id=e.source_id and m.version=e.source_version and not m.cancelled and p.status<>'closed')) or
 (source_kind='task' and not exists(select 1 from tasks t join periods p on p.id=t.period_id where t.id=e.source_id and t.notification_revision=e.source_version and t.progress in ('todo','doing') and p.status<>'closed')) or
 (source_kind='reminder' and not exists(select 1 from reminders r where r.id=e.source_id and r.completed_at is null and r.notification_revision=e.source_version)))`, now)
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `update delivery_recipients d set status='cancelled' from delivery_events e where e.id=d.event_id and e.cancelled_at is not null and d.status='queued'`); err != nil {
			return err
		}
		sources, err := s.scheduleSources(ctx, q)
		if err != nil {
			return err
		}
		for _, src := range sources {
			loc, err := time.LoadLocation(src.zone)
			if err != nil {
				return err
			}
			localNow := now.In(loc)
			today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
			// Only today's relevant missed task slot is caught up. Older historical
			// slots remain unsent, and meetings created late never backfill past slots.
			slots := []time.Time{}
			switch src.kind {
			case "meeting":
				for _, mins := range src.minutes {
					slot := src.due.Add(-time.Duration(mins) * time.Minute)
					if !slot.Before(src.updated) && !slot.Before(today) && slot.Before(today.AddDate(0, 0, 2)) {
						slots = append(slots, slot)
					}
				}
			case "reminder":
				slot := src.due
				if src.dateOnly {
					d := src.due.In(loc)
					slot = time.Date(d.Year(), d.Month(), d.Day(), 6, 0, 0, 0, loc)
				}
				if !slot.Before(today) && !slot.Before(src.created) {
					slots = append(slots, slot)
				}
			case "task":
				d := src.due.In(loc)
				deadlineDay := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
				for _, days := range src.policy.TaskDays {
					slot := deadlineDay.AddDate(0, 0, -days).Add(time.Duration(src.policy.LocalHour)*time.Hour + time.Duration(src.policy.LocalMinute)*time.Minute)
					if !slot.Before(today) && (!src.dateOnly && !slot.After(src.due) || src.dateOnly) && !slot.Before(src.created) {
						slots = append(slots, slot)
					}
				}
				if today.After(deadlineDay) {
					// Calendar-day iteration preserves local-day recurrence over DST.
					days := int(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC).Sub(time.Date(deadlineDay.Year(), deadlineDay.Month(), deadlineDay.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24)
					if days%src.policy.OverdueDays == 0 {
						slots = append(slots, today.Add(time.Duration(src.policy.LocalHour)*time.Hour+time.Duration(src.policy.LocalMinute)*time.Minute))
					}
				}
			}
			for _, slot := range slots {
				if slot.After(now.Add(48 * time.Hour)) {
					continue
				}
				key := src.kind + ":" + src.id.String() + ":" + strconv.FormatInt(src.version, 10) + ":" + slot.UTC().Format(time.RFC3339)
				body := "Pengingat: " + src.title
				message := deliveryMessage{Key: "reminder", Args: []string{src.title}}
				if src.kind == "task" {
					overdue := !src.dateOnly && slot.After(src.due)
					if src.dateOnly {
						d := src.due.In(loc)
						overdue = slot.In(loc).Format("2006-01-02") > d.Format("2006-01-02")
					}
					if overdue {
						body = "Tugas melewati tenggat: " + src.title
						message.Key = "overdue"
					}
				}
				if err := s.enqueue(ctx, q, key, src.kind, src.id, src.version, src.space, src.title, body, src.zone, slot, src.recipients, scheduledChannels(src.kind), message); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Server) enqueue(ctx context.Context, q store.Querier, key, kind string, source id.ID, version int64, space *id.ID, title, body, zone string, at time.Time, recipients []id.ID, channels []string, messages ...deliveryMessage) error {
	control, err := s.store.RuntimeControl(ctx, q, store.LockNone)
	if err != nil {
		return err
	}
	if control.OutboundNotBefore != nil && at.Before(*control.OutboundNotBefore) {
		return nil
	}
	if control.ActivationID != nil {
		key = "activation:" + control.ActivationID.String() + ":" + key
	}
	eid := id.New()
	newID := eid
	message := deliveryMessage{Args: []string{}}
	if len(messages) > 1 {
		return fmt.Errorf("one delivery message descriptor is allowed")
	}
	if len(messages) == 1 {
		message = messages[0]
	}
	if message.Args == nil {
		message.Args = []string{}
	}
	messageArgs, err := json.Marshal(message.Args)
	if err != nil {
		return err
	}
	relatedKind, relatedID := "", (*id.ID)(nil)
	if message.RelatedKind != "" && message.RelatedID != id.Nil {
		relatedKind, relatedID = message.RelatedKind, &message.RelatedID
	}
	_, err = q.Exec(ctx, `insert into delivery_events(id,event_key,source_kind,source_id,source_version,space_id,title,body,time_zone,scheduled_at,created_at,message_key,message_args,related_kind,related_id) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) on conflict(event_key) do nothing`, eid, key, kind, source, version, space, title, body, zone, at, s.now(), message.Key, messageArgs, relatedKind, relatedID)
	if err != nil {
		return err
	}
	if err := q.QueryRow(ctx, `select id from delivery_events where event_key=$1`, key).Scan(&eid); err != nil {
		return err
	}
	for _, uid := range recipients {
		for _, channel := range channels {
			status, lastError := "queued", ""
			if _, err := q.Exec(ctx, `insert into delivery_recipients(id,event_id,recipient_id,channel,status,available_at,last_error) values($1,$2,$3,$4,$5,$6,$7) on conflict(event_id,recipient_id,channel) do nothing`, id.New(), eid, uid, channel, status, at, lastError); err != nil {
				return err
			}
		}
	}
	// Never backfill old failures merely because a bot was paired later.
	if eid == newID {
		return s.queueWADelivery(ctx, q, eid)
	}
	return nil
}

func (s *Server) scheduleSources(ctx context.Context, q store.Querier) ([]scheduledSource, error) {
	out := []scheduledSource{}
	rows, err := q.Query(ctx, `select m.id,m.space_id,m.version,m.title,m.time_zone,m.created_at,m.updated_at,m.starts_at,m.reminder_minutes,coalesce(array_agg(mp.user_id) filter(where mp.user_id is not null),'{}'::uuid[]) from meetings m join periods p on p.id=m.period_id left join meeting_participants mp on mp.meeting_id=m.id where not m.cancelled and p.status<>'closed' and m.starts_at>=$1 group by m.id`, s.now())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		v := scheduledSource{kind: "meeting"}
		if err := rows.Scan(&v.id, &v.space, &v.version, &v.title, &v.zone, &v.created, &v.updated, &v.due, &v.minutes, &v.recipients); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `select r.id,r.title,r.time_zone,r.created_at,r.updated_at,coalesce(r.due_at,r.due_on::timestamp at time zone r.time_zone),r.due_kind='date',r.owner_id,r.notification_revision from reminders r where completed_at is null`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		v := scheduledSource{kind: "reminder"}
		var uid id.ID
		if err := rows.Scan(&v.id, &v.title, &v.zone, &v.created, &v.updated, &v.due, &v.dateOnly, &uid, &v.version); err != nil {
			rows.Close()
			return nil, err
		}
		v.recipients = []id.ID{uid}
		out = append(out, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `select t.id,(select ts.space_id from task_spaces ts join spaces sp on sp.id=ts.space_id where ts.task_id=t.id order by (sp.code='medhum') desc,(sp.kind='division') desc,sp.id limit 1),t.notification_revision,t.title,p.time_zone,t.created_at,t.updated_at,coalesce(t.deadline_at,t.deadline_date::timestamp at time zone p.time_zone),t.deadline_date is not null,t.assignee_id,coalesce(tp.task_days,np.task_days,'{2,1,0}'::integer[]),coalesce(tp.local_hour,np.local_hour,6),coalesce(tp.local_minute,np.local_minute,0),coalesce(tp.overdue_days,np.overdue_days,1) from tasks t join periods p on p.id=t.period_id left join notification_policies np on np.period_id=t.period_id and np.division_id=t.owner_division_id left join task_notification_policies tp on tp.task_id=t.id where p.status<>'closed' and t.progress in ('todo','doing')`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		v := scheduledSource{kind: "task"}
		var uid id.ID
		if err := rows.Scan(&v.id, &v.space, &v.version, &v.title, &v.zone, &v.created, &v.updated, &v.due, &v.dateOnly, &uid, &v.policy.TaskDays, &v.policy.LocalHour, &v.policy.LocalMinute, &v.policy.OverdueDays); err != nil {
			rows.Close()
			return nil, err
		}
		v.recipients = []id.ID{uid}
		out = append(out, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("task schedule: %w", err)
	}
	return out, nil
}

func scheduledChannels(kind string) []string {
	if kind == "reminder" {
		return []string{"device"}
	}
	return []string{"device", "whatsapp"}
}

type deliveryMessage struct {
	Key  string
	Args []string
	// RelatedKind ("task", "meeting") and RelatedID name the card a ping is
	// about, for the app to open and the WhatsApp message to link.
	RelatedKind string
	RelatedID   id.ID
}
