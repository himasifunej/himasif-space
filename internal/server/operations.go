package server

import (
	"errors"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func laterTime(current, candidate *time.Time) *time.Time {
	if candidate != nil && (current == nil || candidate.After(*current)) {
		return candidate
	}
	return current
}

func (s *Server) systemOperations(w http.ResponseWriter, r *http.Request, p *principal) error {
	mode := s.cfg.RunMode
	if mode == "" {
		mode = "live"
	}
	out := api.SystemOperations{Build: BuildInfo(), Status: "ok", Version: Version, RunMode: mode, GeneratedAt: s.now().UTC(), Workers: []api.WorkerHealth{}}
	names := []string{}
	states := map[string]*api.WorkerHealth{}
	bad := map[string]int{}
	for name, quiet := range workerSilence {
		names = append(names, name)
		states[name] = &api.WorkerHealth{Name: name, State: "not_observed", ExpectedSilenceSeconds: int(quiet.Seconds())}
	}
	sort.Strings(names)
	rows, err := s.store.DB().Query(r.Context(), `select worker,state,progressed_at,succeeded_at,failed_at from worker_health where runtime_mode=$1 and progressed_at>=$2 order by worker,progressed_at desc`, mode, s.now().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name, state string
		var progress time.Time
		var success, failure *time.Time
		if err = rows.Scan(&name, &state, &progress, &success, &failure); err != nil {
			return err
		}
		health := states[name]
		if health == nil {
			continue
		}
		if health.LastProgressAt == nil {
			snapshot := progress
			health.LastProgressAt = &snapshot
		}
		health.LastSuccessAt = laterTime(health.LastSuccessAt, success)
		health.LastFailureAt = laterTime(health.LastFailureAt, failure)
		age := s.now().Sub(progress)
		if age < -30*time.Second || age > workerSilence[name] || state == "stopped" || state == "disabled" || state == "not_started" {
			continue
		}
		health.ActiveInstances++
		if state == "failed" {
			bad[name]++
			if health.State == "not_observed" {
				health.State = "failed"
			}
		} else if state == "running" {
			health.State = "running"
		} else if health.State != "running" {
			health.State = "waiting"
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, name := range names {
		health := states[name]
		if !s.cfg.OutboundEnabled() {
			health.State = "disabled"
			health.ActiveInstances = 0
		} else if health.ActiveInstances == 0 {
			if health.LastProgressAt != nil {
				health.State = "stale"
			}
			out.Status = "degraded"
		} else if bad[name] > 0 {
			health.State = "degraded"
			out.Status = "degraded"
		}
		out.Workers = append(out.Workers, *health)
	}
	if !s.cfg.OutboundEnabled() {
		out.Status = "disabled"
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) divisionOperations(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	out := api.DivisionOperations{GeneratedAt: s.now().UTC(), Connectors: []api.ConnectorHealth{}}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		out.PeriodID = sc.period.ID
		out.DivisionID = sc.division.ID
		out.DivisionName = sc.division.Name
		out.CanReadSources = slices.Contains(sc.caps, api.CapTasksManage)
		out.CanReadWhatsApp = slices.Contains(sc.caps, api.CapWhatsAppManage)
		if !out.CanReadSources && !out.CanReadWhatsApp {
			return errForbidden
		}
		if out.CanReadSources {
			notifications := &api.QueueHealth{}
			// Counts are notifications to members of this division, not message
			// bodies or private reminders. EXISTS avoids multiplying shared recipients.
			err = q.QueryRow(r.Context(), `select count(*) filter(where d.status='queued' and e.cancelled_at is null and e.suppressed_at is null),count(*) filter(where d.status='queued' and e.cancelled_at is null and e.suppressed_at is null and d.available_at<=$3),count(*) filter(where d.status='unknown'),count(*) filter(where d.status='failed'),count(*) filter(where d.status='accepted'),count(*) filter(where d.status='cancelled' or (d.status='queued' and e.cancelled_at is not null)),count(*) filter(where e.suppressed_at is not null),min(d.available_at) filter(where d.status='queued' and e.cancelled_at is null and e.suppressed_at is null and d.available_at<=$3) from delivery_recipients d join delivery_events e on e.id=d.event_id join spaces sp on sp.id=e.space_id where sp.period_id=$1 and e.source_kind<>'reminder' and exists(select 1 from space_divisions sd where sd.space_id=sp.id and sd.division_id=$2) and exists(select 1 from memberships m where m.period_id=$1 and m.division_id=$2 and m.user_id=d.recipient_id) and d.channel='device'`, sc.period.ID, sc.division.ID, s.now()).Scan(&notifications.Queued, &notifications.Due, &notifications.Unknown, &notifications.Failed, &notifications.Accepted, &notifications.Cancelled, &notifications.Suppressed, &notifications.OldestDueAt)
			if err != nil {
				return err
			}
			out.MemberNotifications = notifications
			rows, err := q.Query(r.Context(), `select c.id,c.name,c.kind,c.enabled,c.last_success_at,c.next_run_at,c.last_error<>'',c.interval_seconds,(select count(*) from connector_items i where i.connector_id=c.id and i.state in ('missing','conflict','pending')) from connector_configs c where c.period_id=$1 and c.division_id=$2 order by c.name,c.id`, sc.period.ID, sc.division.ID)
			if err != nil {
				return err
			}
			for rows.Next() {
				var source api.ConnectorHealth
				var failed bool
				var interval int
				if err = rows.Scan(&source.ID, &source.Name, &source.Kind, &source.Enabled, &source.LastSuccessAt, &source.NextRunAt, &failed, &interval, &source.ReviewItems); err != nil {
					rows.Close()
					return err
				}
				source.State = "ok"
				switch {
				case !source.Enabled:
					source.State = "disabled"
				case failed:
					source.State = "failed"
				case source.LastSuccessAt == nil:
					source.State = "not_synced"
				case s.now().Sub(*source.LastSuccessAt) > max(5*time.Minute, time.Duration(interval)*2*time.Second):
					source.State = "stale"
				}
				if !s.cfg.OutboundEnabled() {
					source.State = "disabled"
				}
				out.Connectors = append(out.Connectors, source)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		if out.CanReadWhatsApp {
			whatsapp := &api.WhatsAppHealth{}
			err = q.QueryRow(r.Context(), `select c.id,c.display_name,c.enabled,c.status,c.last_event_at,c.last_polled_at,c.last_error<>'',(select count(*) from wa_groups g where g.connection_id=c.id),(select count(*) from wa_inbound i where i.connection_id=c.id and i.state='queued') from wa_connections c where c.period_id=$1 and c.division_id=$2`, sc.period.ID, sc.division.ID).Scan(&whatsapp.ID, &whatsapp.Name, &whatsapp.Enabled, &whatsapp.State, &whatsapp.LastEventAt, &whatsapp.LastPolledAt, &whatsapp.HasError, &whatsapp.MappedGroups, &whatsapp.InboundQueued)
			if err == nil {
				if !whatsapp.Enabled {
					whatsapp.State = "disabled"
				} else if whatsapp.LastPolledAt == nil {
					whatsapp.State = "not_observed"
				} else if s.now().Sub(*whatsapp.LastPolledAt) > 2*time.Minute {
					whatsapp.State = "stale"
				}
				if !s.cfg.OutboundEnabled() {
					whatsapp.State = "disabled"
				}
				queue := &whatsapp.Queue
				if err = q.QueryRow(r.Context(), `select count(*) filter(where status='queued' and suppressed_at is null),count(*) filter(where status='queued' and suppressed_at is null and available_at<=$2),count(*) filter(where status='sending'),count(*) filter(where status='unknown'),count(*) filter(where status='failed'),count(*) filter(where status='accepted'),count(*) filter(where status='delivered'),count(*) filter(where status='read'),count(*) filter(where status='cancelled'),count(*) filter(where suppressed_at is not null),min(available_at) filter(where status='queued' and suppressed_at is null and available_at<=$2) from wa_outbox where connection_id=$1`, whatsapp.ID, s.now()).Scan(&queue.Queued, &queue.Due, &queue.Sending, &queue.Unknown, &queue.Failed, &queue.Accepted, &queue.Delivered, &queue.Read, &queue.Cancelled, &queue.Suppressed, &queue.OldestDueAt); err != nil {
					return err
				}
				out.WhatsApp = whatsapp
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
