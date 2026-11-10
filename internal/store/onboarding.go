package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Onboarding returns an account's onboarding marks; an account that never
// recorded one has none.
func (s *Store) Onboarding(ctx context.Context, q Querier, uid id.ID) (api.Onboarding, error) {
	out := api.Onboarding{Marks: map[string]time.Time{}}
	var raw []byte
	var at time.Time
	err := q.QueryRow(ctx, `select marks,updated_at from user_onboarding where user_id=$1`, uid).Scan(&raw, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out.Marks); err != nil {
		return out, err
	}
	out.UpdatedAt = &at
	return out, nil
}

// SetOnboardingMark records mark key at now, or removes it with clear. A
// mark already recorded keeps its first time.
func (s *Store) SetOnboardingMark(ctx context.Context, q Querier, uid id.ID, key string, clear bool, now time.Time) error {
	if clear {
		_, err := q.Exec(ctx, `insert into user_onboarding(user_id,marks,updated_at) values($1,'{}'::jsonb,$2)
			on conflict(user_id) do update set marks=user_onboarding.marks - $3::text, updated_at=excluded.updated_at`, uid, now, key)
		return err
	}
	mark, err := json.Marshal(map[string]time.Time{key: now.UTC()})
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into user_onboarding(user_id,marks,updated_at) values($1,$2,$3)
		on conflict(user_id) do update set marks=excluded.marks || user_onboarding.marks, updated_at=excluded.updated_at`, uid, mark, now)
	return err
}

// WhatsAppLinked reports whether the user linked a WhatsApp number in
// period pid, or in any period when pid is Nil.
func (s *Store) WhatsAppLinked(ctx context.Context, q Querier, uid, pid id.ID) (bool, error) {
	var ok bool
	var err error
	if pid == id.Nil {
		err = q.QueryRow(ctx, `select exists(select 1 from wa_identities where user_id=$1)`, uid).Scan(&ok)
	} else {
		err = q.QueryRow(ctx, `select exists(select 1 from wa_identities where user_id=$1 and period_id=$2)`, uid, pid).Scan(&ok)
	}
	return ok, err
}

// SetDisplayName changes the name a user shows to others.
func (s *Store) SetDisplayName(ctx context.Context, q Querier, uid id.ID, name string) error {
	_, err := q.Exec(ctx, `update users set display_name=$2, updated_at=now() where id=$1`, uid, name)
	return err
}
