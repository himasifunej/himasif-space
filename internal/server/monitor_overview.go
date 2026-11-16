package server

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// monitorHeavyLoad is how many running tasks make one person's load heavy
// on the dashboard. An assumption: the canvas shows 4–6 tasks as heavy.
const monitorHeavyLoad = 4

const monitorRunning = `t.progress in ('todo','doing','submitted')`

// weekStart returns the Monday 00:00 of the week of now, in loc.
func weekStart(now time.Time, loc *time.Location) time.Time {
	t := now.In(loc)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

// buildMonitorOverview builds the dashboard of a period: every space with
// its running, done, overdue and awaiting-review tasks, heavy loads, last
// meeting and SOP findings, and the people with the most running tasks.
// own marks the spaces of the caller's own divisions.
func (s *Server) buildMonitorOverview(ctx context.Context, q store.Querier, period api.Period, own func(store.Space) bool) (api.MonitorOverview, error) {
	now := s.now()
	loc, err := timeutil.LoadZone(period.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	today := timeutil.DateOf(now.In(loc)).String()
	out := api.MonitorOverview{PeriodID: period.ID, GeneratedAt: now.UTC(), Spaces: []api.MonitorSpaceSummary{},
		Workload: []api.MonitorWorkload{}, HeavyLoad: monitorHeavyLoad}
	spaces, err := s.store.Spaces(ctx, q, period.ID)
	if err != nil {
		return out, err
	}
	for _, sp := range spaces {
		out.Spaces = append(out.Spaces, api.MonitorSpaceSummary{SpaceID: sp.ID, Name: sp.Name, Kind: sp.Kind,
			Divisions: sp.Divisions, Own: own != nil && own(sp)})
	}
	row := func(sid id.ID) *api.MonitorSpaceSummary {
		for i := range out.Spaces {
			if out.Spaces[i].SpaceID == sid {
				return &out.Spaces[i]
			}
		}
		return nil
	}
	overdue := `t.progress in ('todo','doing') and (t.deadline_at < $2 or t.deadline_date < $3::date)`
	review := `t.progress = 'submitted' and t.match_status = 'pending'`
	err = each(ctx, q, func(scan func(...any) error) error {
		var sid id.ID
		var run, done, late, pending int
		if err := scan(&sid, &run, &done, &late, &pending); err != nil {
			return err
		}
		if r := row(sid); r != nil {
			r.Running, r.Done, r.Overdue, r.AwaitingReview = run, done, late, pending
			if run+done > 0 {
				r.Progress = done * 100 / (run + done)
			}
		}
		return nil
	}, `select ts.space_id, count(*) filter (where `+monitorRunning+`), count(*) filter (where t.progress = 'done'),
			count(*) filter (where `+overdue+`), count(*) filter (where `+review+`)
		from tasks t join task_spaces ts on ts.task_id = t.id where t.period_id = $1 group by ts.space_id`, period.ID, now, today)
	if err != nil {
		return out, err
	}
	err = each(ctx, q, func(scan func(...any) error) error {
		var sid id.ID
		var n int
		if err := scan(&sid, &n); err != nil {
			return err
		}
		if r := row(sid); r != nil {
			r.Heavy = n
		}
		return nil
	}, `with load as (select t.assignee_id, count(*) n from tasks t where t.period_id = $1 and `+monitorRunning+` group by t.assignee_id)
		select ts.space_id, count(distinct t.assignee_id) from tasks t join task_spaces ts on ts.task_id = t.id
		join load l on l.assignee_id = t.assignee_id
		where t.period_id = $1 and `+monitorRunning+` and l.n >= $2 group by ts.space_id`, period.ID, monitorHeavyLoad)
	if err != nil {
		return out, err
	}
	err = each(ctx, q, func(scan func(...any) error) error {
		var sid id.ID
		var title string
		var at time.Time
		if err := scan(&sid, &title, &at); err != nil {
			return err
		}
		if r := row(sid); r != nil {
			r.LastMeeting, r.LastMeetingAt = title, &at
		}
		return nil
	}, `select distinct on (space_id) space_id, title, starts_at from meetings
		where period_id = $1 and not cancelled and starts_at <= $2 order by space_id, starts_at desc`, period.ID, now)
	if err != nil {
		return out, err
	}
	sop := map[id.ID][2]int{}
	err = each(ctx, q, func(scan func(...any) error) error {
		var did id.ID
		var n, pending int
		if err := scan(&did, &n, &pending); err != nil {
			return err
		}
		sop[did] = [2]int{n, pending}
		return nil
	}, `select division_id, count(*) filter (where state in ('needs_review','confirmed')), count(*) filter (where state = 'needs_review')
		from sop_findings where period_id = $1 group by division_id`, period.ID)
	if err != nil {
		return out, err
	}
	kadiv := map[id.ID][]api.PersonRef{}
	err = each(ctx, q, func(scan func(...any) error) error {
		var did id.ID
		var p api.PersonRef
		if err := scan(&did, &p.ID, &p.DisplayName); err != nil {
			return err
		}
		kadiv[did] = append(kadiv[did], p)
		return nil
	}, `select m.division_id, u.id, u.display_name from memberships m join users u on u.id = m.user_id
		where m.period_id = $1 and m.role = 'kadiv' and m.removed_at is null and u.disabled_at is null order by u.display_name`, period.ID)
	if err != nil {
		return out, err
	}
	for i := range out.Spaces {
		r := &out.Spaces[i]
		for _, d := range r.Divisions {
			r.SOPFindings += sop[d.ID][0]
			r.SOPNeedsReview += sop[d.ID][1]
			r.Kadiv = append(r.Kadiv, kadiv[d.ID]...)
		}
		if r.Running > 0 {
			out.Totals.RunningSpaces++
		}
	}
	// Totals count a task once, however many spaces show it.
	if err := q.QueryRow(ctx, `select count(*) filter (where `+monitorRunning+`), count(*) filter (where `+overdue+`), count(*) filter (where `+review+`)
		from tasks t where t.period_id = $1`, period.ID, now, today).Scan(&out.Totals.Running, &out.Totals.Overdue, &out.Totals.AwaitingReview); err != nil {
		return out, err
	}
	week := weekStart(now, loc)
	if err := q.QueryRow(ctx, `select count(*), count(*) filter (where note_id is null and starts_at <= $4)
		from meetings where period_id = $1 and not cancelled and starts_at >= $2 and starts_at < $3`, period.ID, week, week.AddDate(0, 0, 7), now).
		Scan(&out.Totals.MeetingsThisWeek, &out.Totals.MeetingsNoNotes); err != nil {
		return out, err
	}
	err = each(ctx, q, func(scan func(...any) error) error {
		var w api.MonitorWorkload
		if err := scan(&w.UserID, &w.DisplayName, &w.Running, &w.Divisions); err != nil {
			return err
		}
		out.Workload = append(out.Workload, w)
		return nil
	}, `select u.id, u.display_name, count(*), array_agg(distinct d.name order by d.name)
		from tasks t join users u on u.id = t.assignee_id join divisions d on d.id = t.owner_division_id
		where t.period_id = $1 and `+monitorRunning+`
		group by u.id, u.display_name order by count(*) desc, lower(u.display_name) limit 10`, period.ID)
	if err != nil {
		return out, err
	}
	// Division spaces first, shared ones (Medhum) after, as the canvas.
	slices.SortStableFunc(out.Spaces, func(a, b api.MonitorSpaceSummary) int {
		switch {
		case a.Kind == b.Kind:
			return 0
		case a.Kind == api.SpaceDivision:
			return -1
		}
		return 1
	})
	return out, nil
}

// each runs a query and calls fn with each row's Scan.
func each(ctx context.Context, q store.Querier, fn func(scan func(...any) error) error, sql string, args ...any) error {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

// monitorOverview is GET /api/periods/{periodId}/monitor, the dashboard,
// for holders of org.monitor only. Reading it is logged once, for the
// period: it shows counts and titles of every division.
func (s *Server) monitorOverview(w http.ResponseWriter, r *http.Request, p *principal) error {
	ctx, q := r.Context(), s.store.DB()
	period, err := s.requireMonitor(ctx, q, r, p)
	if err != nil {
		return err
	}
	ms, err := s.store.UserMemberships(ctx, q, p.User.ID, period.ID)
	if err != nil {
		return err
	}
	out, err := s.buildMonitorOverview(ctx, q, period, func(sp store.Space) bool {
		return slices.ContainsFunc(ms, func(m store.Membership) bool { return sp.HasDivision(m.DivisionID) })
	})
	if err != nil {
		return err
	}
	uid, kind := p.actor()
	pid := period.ID
	if err := s.store.Audit(ctx, q, store.AuditEvent{ActorUserID: uid, ActorKind: kind, Action: "monitor.read", Source: "api", PeriodID: &pid,
		RequestID: requestID(ctx), OccurredAt: s.now(), After: map[string]any{"detail": "Membuka dashboard pemantauan", "route": r.Pattern}}); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// monitorMeetings is GET /api/periods/{periodId}/monitor/meetings: every
// meeting of the period on one timeline, with its kind and its notes' title
// (canvas "Rapat lintas divisi"), for holders of org.monitor. from and to
// (RFC 3339) bound it; by default four weeks back and eight ahead.
// Reading it is logged once; opening a note is logged with its division.
func (s *Server) monitorMeetings(w http.ResponseWriter, r *http.Request, p *principal) error {
	ctx, q := r.Context(), s.store.DB()
	period, err := s.requireMonitor(ctx, q, r, p)
	if err != nil {
		return err
	}
	now := s.now()
	from, to := now.AddDate(0, 0, -28), now.AddDate(0, 0, 56)
	for _, b := range []struct {
		name string
		at   *time.Time
	}{{"from", &from}, {"to", &to}} {
		if v := r.URL.Query().Get(b.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return badRequest("Rentang waktu harus RFC 3339.")
			}
			*b.at = t
		}
	}
	if !to.After(from) || to.Sub(from) > 400*24*time.Hour {
		return badRequest("Rentang waktu rapat tidak valid (paling lama 400 hari).")
	}
	items, err := s.store.PeriodMeetings(ctx, q, period.ID, from, to)
	if err != nil {
		return err
	}
	uid, kind := p.actor()
	pid := period.ID
	if err := s.store.Audit(ctx, q, store.AuditEvent{ActorUserID: uid, ActorKind: kind, Action: "monitor.read", Source: "api", PeriodID: &pid,
		RequestID: requestID(ctx), OccurredAt: s.now(), After: map[string]any{"detail": "Membaca timeline rapat lintas divisi", "route": r.Pattern}}); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.MonitorMeeting]{Items: nonNil(items)})
	return nil
}
