package store

import (
	"context"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

const connectorColumns = `id,period_id,division_id,space_id,kind,name,config,enabled,mode,interval_seconds,next_run_at,last_success_at,last_error,version,created_by,updated_at`

func scanConnector(row pgx.Row) (api.Connector, error) {
	var c api.Connector
	err := row.Scan(&c.ID, &c.PeriodID, &c.DivisionID, &c.SpaceID, &c.Kind, &c.Name, &c.Config, &c.Enabled, &c.Mode, &c.IntervalSeconds, &c.NextRunAt, &c.LastSuccessAt, &c.LastError, &c.Version, &c.CreatedBy, &c.UpdatedAt)
	return c, err
}
func (s *Store) Connector(ctx context.Context, q Querier, cid id.ID, lock string) (api.Connector, error) {
	return scanConnector(q.QueryRow(ctx, `select `+connectorColumns+` from connector_configs where id=$1`+lock, cid))
}
func (s *Store) Connectors(ctx context.Context, q Querier, pid, did id.ID) ([]api.Connector, error) {
	rows, err := q.Query(ctx, `select `+connectorColumns+` from connector_configs where period_id=$1 and division_id=$2 order by name,id`, pid, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Connector{}
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveConnector(ctx context.Context, q Querier, c api.Connector, create bool) error {
	if create {
		_, err := q.Exec(ctx, `insert into connector_configs (`+connectorColumns+`) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, c.ID, c.PeriodID, c.DivisionID, c.SpaceID, c.Kind, c.Name, c.Config, c.Enabled, c.Mode, c.IntervalSeconds, c.NextRunAt, c.LastSuccessAt, c.LastError, c.Version, c.CreatedBy, c.UpdatedAt)
		return err
	}
	_, err := q.Exec(ctx, `update connector_configs set name=$2,config=$3,enabled=$4,mode=$5,interval_seconds=$6,next_run_at=$7,version=$8,updated_at=$9 where id=$1`, c.ID, c.Name, c.Config, c.Enabled, c.Mode, c.IntervalSeconds, c.NextRunAt, c.Version, c.UpdatedAt)
	return err
}
func (s *Store) ConnectorItems(ctx context.Context, q Querier, cid id.ID) ([]api.ConnectorItem, error) {
	rows, err := q.Query(ctx, `select id,connector_id,data,state,issue,content_id,first_seen_at,last_seen_at from connector_items where connector_id=$1 order by last_seen_at desc,id limit 10000`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.ConnectorItem{}
	for rows.Next() {
		var item api.ConnectorItem
		var data []byte
		if err := rows.Scan(&item.ID, &item.ConnectorID, &data, &item.State, &item.Issue, &item.ContentID, &item.FirstSeenAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &item.Source); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) ConnectorRuns(ctx context.Context, q Querier, cid id.ID) ([]api.ConnectorRun, error) {
	rows, err := q.Query(ctx, `select id,connector_id,started_at,finished_at,status,snapshot_hash,item_count,error from connector_runs where connector_id=$1 order by started_at desc,id desc limit 100`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.ConnectorRun{}
	for rows.Next() {
		var run api.ConnectorRun
		if err := rows.Scan(&run.ID, &run.ConnectorID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.SnapshotHash, &run.ItemCount, &run.Error); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}
