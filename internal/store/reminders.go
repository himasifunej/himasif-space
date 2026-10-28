package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Reminder is a personal reminder row.
type Reminder struct {
	ID          id.ID
	OwnerID     id.ID
	Title       string
	Notes       string
	DueKind     string
	DueOn       *timeutil.Date
	DueAt       *time.Time
	TimeZone    string
	Shared      bool
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const reminderColumns = `id, owner_id, title, notes, due_kind, due_on, due_at, time_zone, shared, completed_at, created_at, updated_at`

func scanReminder(row pgx.Row) (Reminder, error) {
	var r Reminder
	var due timeutil.Date
	var hasDue bool
	// due_on is scanned into a nullable date through a pointer below.
	err := row.Scan(&r.ID, &r.OwnerID, &r.Title, &r.Notes, &r.DueKind, &nullDate{&due, &hasDue}, &r.DueAt,
		&r.TimeZone, &r.Shared, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reminder{}, ErrNotFound
	}
	if hasDue {
		d := due
		r.DueOn = &d
	}
	return r, err
}

// nullDate scans a nullable SQL date into a timeutil.Date, setting ok.
type nullDate struct {
	date *timeutil.Date
	ok   *bool
}

func (n nullDate) Scan(src any) error {
	if src == nil {
		*n.ok = false
		return nil
	}
	*n.ok = true
	return n.date.Scan(src)
}

func collectReminders(rows pgx.Rows, err error) ([]Reminder, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reminder
	for rows.Next() {
		var r Reminder
		var due timeutil.Date
		var hasDue bool
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.Title, &r.Notes, &r.DueKind, &nullDate{&due, &hasDue}, &r.DueAt,
			&r.TimeZone, &r.Shared, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if hasDue {
			d := due
			r.DueOn = &d
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OwnedReminders returns a user's own reminders, the soonest due first,
// not-done before done.
func (s *Store) OwnedReminders(ctx context.Context, q Querier, owner id.ID) ([]Reminder, error) {
	return collectReminders(q.Query(ctx, `select `+reminderColumns+` from reminders where owner_id = $1
		order by completed_at is not null, coalesce(due_at, (due_on::timestamptz)), created_at, id`, owner))
}

// SharedWithReminders returns the reminders shared with a user, by another
// account, that are currently shared. It never returns the user's own.
func (s *Store) SharedWithReminders(ctx context.Context, q Querier, recipient id.ID) ([]Reminder, error) {
	return collectReminders(q.Query(ctx, `select `+columnsPrefixed("r")+` from reminders r
		join reminder_shares sh on sh.reminder_id = r.id
		where sh.recipient_id = $1 and r.shared and r.owner_id <> $1
		order by r.completed_at is not null, coalesce(r.due_at, (r.due_on::timestamptz)), r.created_at, r.id`, recipient))
}

func columnsPrefixed(alias string) string {
	return alias + `.id, ` + alias + `.owner_id, ` + alias + `.title, ` + alias + `.notes, ` + alias + `.due_kind, ` +
		alias + `.due_on, ` + alias + `.due_at, ` + alias + `.time_zone, ` + alias + `.shared, ` +
		alias + `.completed_at, ` + alias + `.created_at, ` + alias + `.updated_at`
}

// Reminder returns a reminder, locked as lock says.
func (s *Store) Reminder(ctx context.Context, q Querier, rid id.ID, lock string) (Reminder, error) {
	return scanReminder(q.QueryRow(ctx, `select `+reminderColumns+` from reminders where id = $1`+lock, rid))
}

// ReminderVisibleTo returns a reminder the user may see: their own, or one
// currently shared with them. ErrNotFound hides the rest, so a stranger
// cannot tell a reminder apart from one that does not exist (PRI-08).
func (s *Store) ReminderVisibleTo(ctx context.Context, q Querier, rid, user id.ID) (Reminder, error) {
	return scanReminder(q.QueryRow(ctx, `select `+columnsPrefixed("r")+` from reminders r
		where r.id = $1 and (r.owner_id = $2 or (r.shared and exists (
			select 1 from reminder_shares sh where sh.reminder_id = r.id and sh.recipient_id = $2)))`, rid, user))
}

// CreateReminder inserts a reminder.
func (s *Store) CreateReminder(ctx context.Context, q Querier, r Reminder) error {
	_, err := q.Exec(ctx, `insert into reminders
		(id, owner_id, title, notes, due_kind, due_on, due_at, time_zone, shared, completed_at, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`,
		r.ID, r.OwnerID, r.Title, r.Notes, r.DueKind, r.DueOn, r.DueAt, r.TimeZone, r.Shared, r.CompletedAt, r.CreatedAt)
	return err
}

// UpdateReminderContent changes a reminder's title, notes and due time.
func (s *Store) UpdateReminderContent(ctx context.Context, q Querier, r Reminder, now time.Time) error {
	_, err := q.Exec(ctx, `update reminders set title = $2, notes = $3, due_kind = $4, due_on = $5, due_at = $6,
		time_zone = $7, updated_at = $8 where id = $1`,
		r.ID, r.Title, r.Notes, r.DueKind, r.DueOn, r.DueAt, r.TimeZone, now)
	return err
}

// SetReminderCompleted marks a reminder done at now, or not done.
func (s *Store) SetReminderCompleted(ctx context.Context, q Querier, rid id.ID, completed bool, now time.Time) error {
	var at *time.Time
	if completed {
		at = &now
	}
	_, err := q.Exec(ctx, `update reminders set completed_at = $2, updated_at = $3 where id = $1`, rid, at, now)
	return err
}

// SetReminderShared turns a reminder's sharing toggle on or off.
func (s *Store) SetReminderShared(ctx context.Context, q Querier, rid id.ID, shared bool, now time.Time) error {
	_, err := q.Exec(ctx, `update reminders set shared = $2, updated_at = $3 where id = $1`, rid, shared, now)
	return err
}

// TouchReminder bumps a reminder's updated_at, for a change to its shares.
func (s *Store) TouchReminder(ctx context.Context, q Querier, rid id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update reminders set updated_at = $2 where id = $1`, rid, now)
	return err
}

// DeleteReminder removes a reminder and its shares (the shares cascade).
func (s *Store) DeleteReminder(ctx context.Context, q Querier, rid id.ID) error {
	_, err := q.Exec(ctx, `delete from reminders where id = $1`, rid)
	return err
}

// ReminderRecipients returns the recipients of a reminder, by name.
func (s *Store) ReminderRecipients(ctx context.Context, q Querier, rid id.ID) ([]api.ReminderShare, error) {
	rows, err := q.Query(ctx, `select sh.recipient_id, u.display_name, sh.created_at from reminder_shares sh
		join users u on u.id = sh.recipient_id where sh.reminder_id = $1 order by lower(u.display_name), sh.recipient_id`, rid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.ReminderShare, error) {
		var sh api.ReminderShare
		err := r.Scan(&sh.RecipientID, &sh.DisplayName, &sh.CreatedAt)
		return sh, err
	})
}

// AddReminderShare records a read-only recipient; ErrConflict when the
// reminder already has that recipient.
func (s *Store) AddReminderShare(ctx context.Context, q Querier, rid, recipient, by id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `insert into reminder_shares (reminder_id, recipient_id, shared_by, created_at)
		values ($1, $2, $3, $4)`, rid, recipient, by, now)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// RemoveReminderShare removes a recipient, reporting whether there was one.
func (s *Store) RemoveReminderShare(ctx context.Context, q Querier, rid, recipient id.ID) (bool, error) {
	tag, err := q.Exec(ctx, `delete from reminder_shares where reminder_id = $1 and recipient_id = $2`, rid, recipient)
	return tag.RowsAffected() > 0, err
}

// ReminderDefaultShare returns a user's default for new reminders; false
// when they have set none.
func (s *Store) ReminderDefaultShare(ctx context.Context, q Querier, user id.ID) (bool, error) {
	var def bool
	err := q.QueryRow(ctx, `select default_share from reminder_preferences where user_id = $1`, user).Scan(&def)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return def, err
}

// SetReminderDefaultShare sets a user's default for new reminders. It
// touches no existing reminder (PRI-05).
func (s *Store) SetReminderDefaultShare(ctx context.Context, q Querier, user id.ID, def bool, now time.Time) error {
	_, err := q.Exec(ctx, `insert into reminder_preferences (user_id, default_share, updated_at) values ($1, $2, $3)
		on conflict (user_id) do update set default_share = excluded.default_share, updated_at = excluded.updated_at`,
		user, def, now)
	return err
}

// ReminderAudit returns the recorded changes to a reminder, the latest
// first, for its owner (AC-09).
func (s *Store) ReminderAudit(ctx context.Context, q Querier, rid id.ID) ([]api.ReminderAuditEntry, error) {
	rows, err := q.Query(ctx, `select action, actor_user_id, occurred_at, before, after from audit_events
		where target_type = 'reminder' and target_id = $1 order by occurred_at desc, id desc`, rid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.ReminderAuditEntry, error) {
		var e api.ReminderAuditEntry
		var before, after []byte
		if err := r.Scan(&e.Action, &e.ActorUserID, &e.OccurredAt, &before, &after); err != nil {
			return e, err
		}
		e.Before = rawJSON(before)
		e.After = rawJSON(after)
		return e, nil
	})
}

func rawJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
