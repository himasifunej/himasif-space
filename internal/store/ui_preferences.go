package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

// UIPreferences returns an account's interface preferences; an account
// that never saved any gets an empty set.
func (s *Store) UIPreferences(ctx context.Context, q Querier, uid id.ID) (api.UIPreferences, error) {
	out := api.UIPreferences{SpaceTabs: map[string]api.SpaceTabPreference{}}
	var raw []byte
	var at time.Time
	err := q.QueryRow(ctx, `select space_tabs,updated_at from user_ui_preferences where user_id=$1`, uid).Scan(&raw, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out.SpaceTabs); err != nil {
		return out, err
	}
	out.UpdatedAt = &at
	return out, nil
}

// SetUIPreferences replaces an account's interface preferences.
func (s *Store) SetUIPreferences(ctx context.Context, q Querier, uid id.ID, p api.UIPreferences, now time.Time) error {
	tabs := p.SpaceTabs
	if tabs == nil {
		tabs = map[string]api.SpaceTabPreference{}
	}
	raw, err := json.Marshal(tabs)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into user_ui_preferences(user_id,space_tabs,updated_at) values($1,$2,$3)
		on conflict(user_id) do update set space_tabs=excluded.space_tabs,updated_at=excluded.updated_at`, uid, raw, now)
	return err
}
