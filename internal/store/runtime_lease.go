package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const runtimeLeaseKey = "space-runtime-mode"

// WithRuntimeLease fences a serving process against changing its database's
// quarantine marker. An exclusive quarantine operation refuses a running
// server. Losing the dedicated lease connection shuts down this server.
func (s *Store) WithRuntimeLease(ctx context.Context, mode string, serve func(context.Context) error) error {
	connection, err := pgx.ConnectConfig(ctx, s.pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = connection.Close(cleanup)
	}()
	var acquired bool
	if err = connection.QueryRow(ctx, `select pg_try_advisory_lock_shared(hashtextextended($1,0))`, runtimeLeaseKey).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return errors.New("database runtime mode is being changed; server did not start")
	}
	if err = s.VerifyRunMode(ctx, mode); err != nil {
		return err
	}
	running, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(running) }()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			cancel()
			return <-done
		case <-ticker.C:
			check, stopCheck := context.WithTimeout(running, 2*time.Second)
			err = connection.Ping(check)
			stopCheck()
			if err != nil {
				cancel()
				<-done
				return fmt.Errorf("runtime lease connection lost; server stopped: %w", err)
			}
		}
	}
}
