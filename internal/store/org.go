package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Querier runs statements, on the pool or in a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB returns the pool, for statements outside a transaction.
func (s *Store) DB() Querier { return s.pool }

// Tx runs fn in a transaction, committed when fn returns nil.
func (s *Store) Tx(ctx context.Context, fn func(q Querier) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(tx) })
}

// Periods returns every period, the latest first.
func (s *Store) Periods(ctx context.Context) ([]api.Period, error) {
	rows, err := s.pool.Query(ctx, `select id, label, starts_on, ends_on, time_zone, status from periods order by starts_on desc`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.Period, error) {
		var p api.Period
		err := r.Scan(&p.ID, &p.Label, &p.StartsOn, &p.EndsOn, &p.TimeZone, &p.Status)
		return p, err
	})
}

// PeriodByLabel returns the period with a label.
func (s *Store) PeriodByLabel(ctx context.Context, label string) (api.Period, error) {
	return s.PeriodByLabelQ(ctx, s.pool, label)
}

// PeriodByLabelQ returns the period with a label, in q.
func (s *Store) PeriodByLabelQ(ctx context.Context, q Querier, label string) (api.Period, error) {
	var p api.Period
	err := q.QueryRow(ctx, `select id, label, starts_on, ends_on, time_zone, status from periods where label = $1`, label).
		Scan(&p.ID, &p.Label, &p.StartsOn, &p.EndsOn, &p.TimeZone, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// CreatePeriod adds a period.
func (s *Store) CreatePeriod(ctx context.Context, q Querier, p api.Period) (api.Period, error) {
	if p.ID == id.Nil {
		p.ID = id.New()
	}
	if p.TimeZone == "" {
		p.TimeZone = timeutil.DefaultZoneName
	}
	_, err := q.Exec(ctx, `insert into periods (id, label, starts_on, ends_on, time_zone, status) values ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.Label, p.StartsOn, p.EndsOn, p.TimeZone, p.Status)
	if isUniqueViolation(err) {
		return p, ErrConflict
	}
	return p, err
}

// Divisions returns every division, by name.
func (s *Store) Divisions(ctx context.Context) ([]api.Division, error) {
	rows, err := s.pool.Query(ctx, `select id, slug, name from divisions order by name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.Division, error) {
		var d api.Division
		err := r.Scan(&d.ID, &d.Slug, &d.Name)
		return d, err
	})
}

// EnsureDivision adds a division unless its slug exists, reporting whether
// it added one.
func (s *Store) EnsureDivision(ctx context.Context, q Querier, slug, name string) (api.Division, bool, error) {
	d := api.Division{ID: id.New(), Slug: slug, Name: name}
	tag, err := q.Exec(ctx, `insert into divisions (id, slug, name) values ($1, $2, $3) on conflict (slug) do nothing`, d.ID, slug, name)
	if err != nil {
		return d, false, err
	}
	if tag.RowsAffected() == 0 {
		err = q.QueryRow(ctx, `select id, slug, name from divisions where slug = $1`, slug).Scan(&d.ID, &d.Slug, &d.Name)
		return d, false, err
	}
	return d, true, nil
}

// AuditEvent is one entry of the audit log.
type AuditEvent struct {
	ActorUserID *id.ID
	ActorKind   string // api.AccountPerson, api.AccountOrganizationAdmin, "system" or "bot"
	Action      string // as "auth.login" or "period.create"
	Source      string // "api", "wa", "import", "sync", "system" or "cli"
	PeriodID    *id.ID
	ScopeType   string
	ScopeID     *id.ID
	TargetType  string
	TargetID    *id.ID
	Before      any
	After       any
	RequestID   string
	OccurredAt  time.Time
}

// Audit appends an event to the audit log. Call it in the transaction of
// the change it records, so the change and its record commit together.
func (s *Store) Audit(ctx context.Context, q Querier, e AuditEvent) error {
	if e.Action == "" || e.ActorKind == "" || e.Source == "" {
		return errors.New("store: audit event needs an action, an actor kind and a source")
	}
	if strings.ContainsAny(e.Action, " \n") {
		return errors.New("store: audit action must be a dotted name")
	}
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	_, err = q.Exec(ctx, `insert into audit_events
		(id, occurred_at, actor_user_id, actor_kind, action, source, period_id, scope_type, scope_id, target_type, target_id, before, after, request_id)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		id.New(), e.OccurredAt, e.ActorUserID, e.ActorKind, e.Action, e.Source, e.PeriodID,
		e.ScopeType, e.ScopeID, e.TargetType, e.TargetID, before, after, e.RequestID)
	return err
}

func jsonOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
