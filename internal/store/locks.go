package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// WithSessionLock uses a dedicated physical connection, not a transaction or
// the request pool. Closing it on every exit releases the advisory lock even
// if the caller was cancelled or the result of acquisition was interrupted.
func (s *Store) WithSessionLock(ctx context.Context, key string, fn func() error) (bool, error) {
	conn, err := pgx.ConnectConfig(ctx, s.pool.Config().ConnConfig.Copy())
	if err != nil {
		return false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = conn.Close(cleanup)
	}()
	var acquired bool
	if err = conn.QueryRow(ctx, `select pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&acquired); err != nil {
		return false, err
	}
	if !acquired {
		return false, nil
	}
	return true, fn()
}
