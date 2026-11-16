package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

const monitorColumns = `g.id, g.period_id, g.user_id, u.display_name, g.granted_by, g.granted_at, g.revoked_at, g.revoked_reason`

func scanMonitor(r pgx.CollectableRow) (api.MonitorGrant, error) {
	var g api.MonitorGrant
	err := r.Scan(&g.ID, &g.PeriodID, &g.UserID, &g.DisplayName, &g.GrantedBy, &g.GrantedAt, &g.RevokedAt, &g.RevokedReason)
	return g, err
}

// MonitorActive reports whether uid holds org.monitor in period pid. A
// closed period grants nothing, whatever its rows say.
func (s *Store) MonitorActive(ctx context.Context, q Querier, pid, uid id.ID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `select exists(select 1 from monitor_grants g join periods p on p.id = g.period_id
		join users u on u.id = g.user_id
		where g.period_id = $1 and g.user_id = $2 and g.revoked_at is null and p.status <> 'closed' and u.disabled_at is null)`, pid, uid).Scan(&ok)
	return ok, err
}

// MonitorPeriodIDs returns the periods in which uid holds org.monitor.
func (s *Store) MonitorPeriodIDs(ctx context.Context, q Querier, uid id.ID) (map[id.ID]bool, error) {
	rows, err := q.Query(ctx, `select g.period_id from monitor_grants g join periods p on p.id = g.period_id
		where g.user_id = $1 and g.revoked_at is null and p.status <> 'closed'`, uid)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	out := map[id.ID]bool{}
	for _, i := range ids {
		out[i] = true
	}
	return out, err
}

// MonitorGrants returns the active grants of a period, by name.
func (s *Store) MonitorGrants(ctx context.Context, q Querier, pid id.ID) ([]api.MonitorGrant, error) {
	rows, err := q.Query(ctx, `select `+monitorColumns+` from monitor_grants g join users u on u.id = g.user_id
		where g.period_id = $1 and g.revoked_at is null order by lower(u.display_name), g.id`, pid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMonitor)
}

// MonitorGrant returns an active grant of a period.
func (s *Store) MonitorGrant(ctx context.Context, q Querier, pid, gid id.ID) (api.MonitorGrant, error) {
	rows, err := q.Query(ctx, `select `+monitorColumns+` from monitor_grants g join users u on u.id = g.user_id
		where g.period_id = $1 and g.id = $2 and g.revoked_at is null`, pid, gid)
	if err != nil {
		return api.MonitorGrant{}, err
	}
	g, err := pgx.CollectExactlyOneRow(rows, scanMonitor)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

// CreateMonitorGrant records a grant; ErrConflict when the account already
// holds one in the period.
func (s *Store) CreateMonitorGrant(ctx context.Context, q Querier, g api.MonitorGrant) error {
	_, err := q.Exec(ctx, `insert into monitor_grants (id, period_id, user_id, granted_by, granted_at) values ($1, $2, $3, $4, $5)`,
		g.ID, g.PeriodID, g.UserID, g.GrantedBy, g.GrantedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// RevokeMonitorGrant ends one active grant.
func (s *Store) RevokeMonitorGrant(ctx context.Context, q Querier, gid, by id.ID, reason string, now time.Time) error {
	tag, err := q.Exec(ctx, `update monitor_grants set revoked_at = $3, revoked_by = $2, revoked_reason = $4
		where id = $1 and revoked_at is null`, gid, by, now, reason)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// RevokePeriodMonitors ends every active grant of a period, for its close,
// and returns the grants it ended. by is who closed the period.
func (s *Store) RevokePeriodMonitors(ctx context.Context, q Querier, pid, by id.ID, now time.Time) ([]api.MonitorGrant, error) {
	grants, err := s.MonitorGrants(ctx, q, pid)
	if err != nil {
		return nil, err
	}
	if _, err := q.Exec(ctx, `update monitor_grants set revoked_at = $3, revoked_by = $2, revoked_reason = 'period_closed'
		where period_id = $1 and revoked_at is null`, pid, by, now); err != nil {
		return nil, err
	}
	return grants, nil
}

// BPHMemberships returns the active members of the BPH division of a period.
func (s *Store) BPHMemberships(ctx context.Context, q Querier, pid id.ID) ([]Membership, error) {
	return s.memberships(ctx, q, `m.period_id = $1 and m.division_id = (select id from divisions where slug = 'bph')`, pid)
}

// MonitorAudit returns the latest monitoring entries of a period: grants,
// revocations, reads and actions, newest first.
func (s *Store) MonitorAudit(ctx context.Context, q Querier, pid id.ID, limit int) ([]api.MonitorAuditEvent, error) {
	rows, err := q.Query(ctx, `select e.id, e.occurred_at, e.action, e.actor_user_id, coalesce(a.display_name, ''),
			case when e.target_type = 'user' then e.target_id end, coalesce(t.display_name, ''),
			case when e.scope_type = 'division' then e.scope_id end, coalesce(d.name, ''),
			coalesce(e.after ->> 'detail', '')
		from audit_events e
		left join users a on a.id = e.actor_user_id
		left join users t on e.target_type = 'user' and t.id = e.target_id
		left join divisions d on e.scope_type = 'division' and d.id = e.scope_id
		where e.period_id = $1 and e.action like 'monitor.%'
		order by e.occurred_at desc, e.id desc limit $2`, pid, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.MonitorAuditEvent, error) {
		var e api.MonitorAuditEvent
		err := r.Scan(&e.ID, &e.OccurredAt, &e.Action, &e.ActorID, &e.ActorName, &e.SubjectID, &e.SubjectName, &e.DivisionID, &e.DivisionName, &e.Detail)
		return e, err
	})
}
