package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"os"
	"time"
)

type mirrorCandidate struct {
	source  api.ConnectorSourceItem
	content id.ID
}

func (s *Server) runConnectorMirror(ctx context.Context, c api.Connector, p *principal) (int, error) {
	if os.Getenv("SPACE_EXTERNAL_WRITES_ENABLED") != "true" {
		return 0, conflict("Penulisan sumber belum diaktifkan oleh pengelola server.")
	}
	if c.Kind != "sheets" || c.Mode != "mirror" {
		return 0, badRequest("Pilih mode mirror Sheets secara eksplisit.")
	}
	var config api.SheetsConfig
	if json.Unmarshal(c.Config, &config) != nil || config.KeyColumn == 0 || len(config.MirrorColumns) == 0 {
		return 0, badRequest("Mirror memerlukan kolom ID stabil dan kolom khusus Space.")
	}
	runID := id.New()
	candidates := []mirrorCandidate{}
	err := s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, c.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		current, err := s.store.Connector(ctx, q, c.ID, store.LockUpdate)
		if err != nil {
			return err
		}
		if current.Version != c.Version || current.Mode != "mirror" {
			return conflict("Konfigurasi berubah sebelum mirror.")
		}
		r := (&http.Request{}).WithContext(ctx)
		r.SetPathValue("periodId", c.PeriodID.String())
		r.SetPathValue("divisionId", c.DivisionID.String())
		if p.Session.ID != id.Nil {
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
		} else {
			var active bool
			if err := q.QueryRow(ctx, `select exists(select 1 from users where id=$1 and disabled_at is null)`, p.User.ID).Scan(&active); err != nil {
				return err
			}
			if !active {
				return errForbidden
			}
		}
		sc, err := s.loadScope(ctx, q, r, p, store.LockNone)
		if err != nil {
			return err
		}
		if err := sc.require(api.CapTasksManage); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `update connector_mirror_runs set status='unknown',finished_at=$2,error='Proses mirror terhenti; baca ulang sumber sebelum retry.' where connector_id=$1 and status='running' and started_at<$3`, c.ID, s.now(), s.now().Add(-5*time.Minute)); err != nil {
			return err
		}
		var running bool
		if err := q.QueryRow(ctx, `select exists(select 1 from connector_mirror_runs where connector_id=$1 and status='running')`, c.ID).Scan(&running); err != nil {
			return err
		}
		if running {
			return conflict("Mirror sumber ini masih berjalan.")
		}
		rows, err := q.Query(ctx, `select i.data,b.content_id from connector_items i join connector_bindings b on b.connector_id=i.connector_id and b.external_key=i.external_key where i.connector_id=$1 and i.state='applied' and i.fingerprint=b.accepted_fingerprint order by i.id`, c.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var candidate mirrorCandidate
			var data []byte
			if err := rows.Scan(&data, &candidate.content); err != nil {
				rows.Close()
				return err
			}
			if json.Unmarshal(data, &candidate.source) != nil {
				rows.Close()
				return errors.New("snapshot mirror tidak valid")
			}
			candidates = append(candidates, candidate)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx, `insert into connector_mirror_runs(id,connector_id,started_at,status) values($1,$2,$3,'running')`, runID, c.ID, s.now())
		return err
	})
	if err != nil {
		return 0, err
	}
	count := 0
	outboundStarted := false
	var failure error
	for _, candidate := range candidates {
		// Authorization/configuration is reloaded for every outbound operation.
		var values map[string]string
		failure = s.store.Tx(ctx, func(q store.Querier) error {
			period, err := s.store.Period(ctx, q, c.PeriodID, store.LockShare)
			if err != nil {
				return err
			}
			if period.Status == api.PeriodClosed {
				return errPeriodClosed
			}
			current, err := s.store.Connector(ctx, q, c.ID, store.LockNone)
			if err != nil {
				return err
			}
			if current.Version != c.Version || current.Mode != "mirror" {
				return conflict("Konfigurasi mirror berubah.")
			}
			r := (&http.Request{}).WithContext(ctx)
			r.SetPathValue("periodId", c.PeriodID.String())
			r.SetPathValue("divisionId", c.DivisionID.String())
			if p.Session.ID != id.Nil {
				if _, err := s.lockedAccount(r, q, p); err != nil {
					return err
				}
			} else {
				var active bool
				if err := q.QueryRow(ctx, `select exists(select 1 from users where id=$1 and disabled_at is null)`, p.User.ID).Scan(&active); err != nil {
					return err
				}
				if !active {
					return errForbidden
				}
			}
			sc, err := s.loadScope(ctx, q, r, p, store.LockNone)
			if err != nil {
				return err
			}
			if err := sc.require(api.CapTasksManage); err != nil {
				return err
			}
			content, err := s.store.Medhum(ctx, q, candidate.content, store.LockNone)
			if err != nil {
				return err
			}
			if content.SpaceID != c.SpaceID {
				return errForbidden
			}
			tid := content.DesignTaskID
			if config.MirrorStage == "humas" {
				tid = content.HumasTaskID
			}
			if tid == nil {
				return conflict("Tugas tahap yang dipilih belum diterbitkan.")
			}
			task, err := s.store.Task(ctx, q, *tid, store.LockNone)
			if err != nil {
				return err
			}
			evidence := ""
			for _, ev := range task.Evidence {
				if ev.MatchStatus == api.MatchVerified {
					evidence = ev.URL
					break
				}
			}
			values = map[string]string{"progress": task.Progress, "matchStatus": task.MatchStatus, "evidenceUrl": evidence}
			return nil
		})
		if failure != nil {
			break
		}
		outboundStarted = true
		changed, err := s.sources.MirrorRow(ctx, config, c.ID.String(), candidate.source, values)
		if err != nil {
			failure = err
			break
		}
		if changed {
			count++
		}
	}
	status, message := "succeeded", ""
	if failure != nil {
		status = "failed"
		if outboundStarted {
			status = "unknown"
		}
		message = failure.Error()
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.store.Tx(finish, func(q store.Querier) error {
		if _, err := q.Exec(finish, `update connector_mirror_runs set status=$2,finished_at=$3,item_count=$4,error=$5 where id=$1`, runID, status, s.now(), count, message); err != nil {
			return err
		}
		return s.connectorEvent(finish, q, c.ID, &p.User.ID, "mirror."+status, nil, map[string]any{"runId": runID, "items": count, "error": message})
	})
	return count, failure
}
func (s *Server) mirrorConnector(w http.ResponseWriter, r *http.Request, p *principal) error {
	var c api.Connector
	err := s.withConnector(r, p, true, func(q store.Querier, sc *scope, current *api.Connector) error { c = *current; return nil })
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	count, err := s.runConnectorMirror(ctx, c, p)
	if err != nil {
		return err
	}
	writeJSON(w, 200, map[string]any{"updatedItems": count})
	return nil
}
