package store

import (
	"context"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

const waColumns = `id,period_id,division_id,display_name,language,greeting,templates,enabled,status,account_jid,last_event_at,last_error,version,updated_at`

func scanWA(row pgx.Row) (api.WAConnection, error) {
	var c api.WAConnection
	var templates []byte
	err := row.Scan(&c.ID, &c.PeriodID, &c.DivisionID, &c.DisplayName, &c.Language, &c.Greeting, &templates, &c.Enabled, &c.Status, &c.AccountJID, &c.LastEventAt, &c.LastError, &c.Version, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(templates, &c.Templates)
	return c, err
}
func (s *Store) WAConnection(ctx context.Context, q Querier, cid id.ID, lock string) (api.WAConnection, error) {
	return scanWA(q.QueryRow(ctx, `select `+waColumns+` from wa_connections where id=$1`+lock, cid))
}
func (s *Store) WAConnections(ctx context.Context, q Querier, pid, did id.ID) ([]api.WAConnection, error) {
	rows, err := q.Query(ctx, `select `+waColumns+` from wa_connections where period_id=$1 and division_id=$2 order by id`, pid, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.WAConnection{}
	for rows.Next() {
		c, err := scanWA(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) WAGroups(ctx context.Context, q Querier, cid id.ID) ([]api.WAGroup, error) {
	rows, err := q.Query(ctx, `select id,space_id,jid,name,commands_enabled from wa_groups where connection_id=$1 order by name,id`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.WAGroup{}
	for rows.Next() {
		var g api.WAGroup
		if err := rows.Scan(&g.ID, &g.SpaceID, &g.JID, &g.Name, &g.CommandsEnabled); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
