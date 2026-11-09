package store

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// ListenChanges consumes committed invalidations on one dedicated connection
// per server, shared by all browser/desktop subscribers. It never reserves a
// query-pool connection for every user. The caller owns reconnect/backoff.
func (s *Store) ListenChanges(ctx context.Context, ready func(), changed func(string)) error {
	conn, err := pgx.ConnectConfig(ctx, s.pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer conn.Close(context.WithoutCancel(ctx))
	if _, err = conn.Exec(ctx, "listen space_changes"); err != nil {
		return err
	}
	ready()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		changed(n.Payload)
	}
}
