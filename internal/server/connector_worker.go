package server

import (
	"context"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"os"
	"time"
)

// RunConnectors owns only its own polling loop; a slow/unavailable source
// never blocks the reminder scheduler or the API. Configurations are opt-in.
func (s *Server) RunConnectors(ctx context.Context) {
	if !s.cfg.OutboundEnabled() {
		return
	}
	defer s.workerStop("connectors")
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		s.workerStart("connectors")
		for count := 0; count < 4 && ctx.Err() == nil; count++ {
			c, run, found, err := s.claimConnector(ctx)
			if err != nil {
				s.workerProgress("connectors", true)
				s.log.Error("connector claim failed", "error", err)
				break
			}
			if !found {
				break
			}
			work, cancel := context.WithTimeout(ctx, 90*time.Second)
			actor, authErr := s.connectorSystemActor(work, c)
			var items []api.ConnectorSourceItem
			fetchErr := authErr
			if fetchErr == nil {
				items, fetchErr = s.readConnector(work, c)
			}
			err = s.store.Tx(work, func(q store.Querier) error {
				period, err := s.store.Period(work, q, c.PeriodID, store.LockUpdate)
				if err != nil {
					return err
				}
				current, err := s.store.Connector(work, q, c.ID, store.LockUpdate)
				if err != nil {
					return err
				}
				if current.Version != c.Version || !current.Enabled || period.Status == api.PeriodClosed {
					fetchErr = errors.New("konektor dinonaktifkan/diubah atau periode ditutup; snapshot tidak diterapkan")
				}
				if fetchErr == nil {
					rr := (&http.Request{}).WithContext(work)
					rr.SetPathValue("periodId", c.PeriodID.String())
					rr.SetPathValue("divisionId", c.DivisionID.String())
					sc, err := s.loadScope(work, q, rr, actor, store.LockNone)
					if err != nil {
						return err
					}
					if err := sc.require(api.CapTasksManage); err != nil {
						fetchErr = errors.New("kewenangan pemilik sumber berubah selama fetch")
					}
				}
				return s.finishConnectorFetch(work, q, current, &run, items, fetchErr)
			})
			if err != nil {
				s.workerProgress("connectors", true)
				s.failConnectorRun(ctx, c.ID, run.ID, "Fetch tidak disimpan; konfigurasi/akses berubah atau proses terhenti.")
			} else if fetchErr == nil {
				if err := s.applyConnectorBindings(work, c, run.ID); err != nil {
					s.workerProgress("connectors", true)
					s.log.Info("connector auto-apply deferred", "connector_id", c.ID)
				}
			}
			if err == nil && fetchErr == nil && c.Mode == "mirror" && os.Getenv("SPACE_EXTERNAL_WRITES_ENABLED") == "true" {
				if _, mirrorErr := s.runConnectorMirror(work, c, actor); mirrorErr != nil {
					s.workerProgress("connectors", true)
					s.log.Info("connector mirror deferred", "connector_id", c.ID)
				}
			}
			if fetchErr != nil {
				s.workerProgress("connectors", true)
			}
			cancel()
		}
		s.workerFinish("connectors")
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (s *Server) claimConnector(ctx context.Context) (api.Connector, api.ConnectorRun, bool, error) {
	var c api.Connector
	run := api.ConnectorRun{ID: id.New(), Status: "running", StartedAt: s.now().UTC()}
	found := false
	err := s.store.Tx(ctx, func(q store.Querier) error {
		// A process death leaves a durable running receipt. Its timeout is longer
		// than the bounded fetch context, so active work is not stolen.
		if _, err := q.Exec(ctx, `update connector_runs set status='failed',finished_at=$1,error='Proses fetch sebelumnya berhenti; data baik terakhir dipertahankan.' where status='running' and started_at<$2`, s.now(), s.now().Add(-5*time.Minute)); err != nil {
			return err
		}
		rows, err := q.Query(ctx, `select c.id from connector_configs c join periods p on p.id=c.period_id where c.enabled and p.status<>'closed' and c.next_run_at<=$1 and not exists(select 1 from connector_runs rr where rr.connector_id=c.id and rr.status='running') order by c.next_run_at,c.id limit 1 for update of c skip locked`, s.now())
		if err != nil {
			return err
		}
		var cid id.ID
		if rows.Next() {
			if err := rows.Scan(&cid); err != nil {
				rows.Close()
				return err
			}
			found = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		c, err = s.store.Connector(ctx, q, cid, store.LockNone)
		if err != nil {
			return err
		}
		run.ConnectorID = c.ID
		if _, err := q.Exec(ctx, `update connector_configs set next_run_at=$2 where id=$1`, c.ID, s.now().Add(time.Duration(c.IntervalSeconds)*time.Second)); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `insert into connector_runs(id,connector_id,started_at,status) values($1,$2,$3,'running')`, run.ID, c.ID, run.StartedAt)
		return err
	})
	return c, run, found, err
}
func (s *Server) connectorSystemActor(ctx context.Context, c api.Connector) (*principal, error) {
	// The source belongs to the division, not to a former kadiv's browser
	// session. Resolve current leadership and recompute its capabilities.
	p := &principal{}
	err := s.store.DB().QueryRow(ctx, `select u.id,u.kind,u.display_name from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and m.role='kadiv' and m.removed_at is null and u.disabled_at is null`, c.PeriodID, c.DivisionID).Scan(&p.User.ID, &p.User.Kind, &p.User.DisplayName)
	if err != nil {
		return nil, errors.New("kadiv aktif pemilik sumber belum tersedia; polling ditunda")
	}
	r := (&http.Request{}).WithContext(ctx)
	r.SetPathValue("periodId", c.PeriodID.String())
	r.SetPathValue("divisionId", c.DivisionID.String())
	sc, err := s.loadScope(ctx, s.store.DB(), r, p, store.LockNone)
	if err != nil {
		return nil, err
	}
	if sc.require(api.CapTasksManage) != nil {
		return nil, errors.New("pemilik sumber tidak memiliki kewenangan operasional aktif")
	}
	return p, nil
}
func (s *Server) failConnectorRun(ctx context.Context, cid, rid id.ID, message string) {
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.store.Tx(bounded, func(q store.Querier) error {
		if _, err := q.Exec(bounded, `update connector_runs set status='failed',finished_at=$2,error=$3 where id=$1 and status='running'`, rid, s.now(), message); err != nil {
			return err
		}
		_, err := q.Exec(bounded, `update connector_configs set last_error=$2 where id=$1`, cid, message)
		return err
	})
}
