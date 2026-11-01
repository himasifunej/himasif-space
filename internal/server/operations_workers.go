package server

import (
	"context"
	"sync"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

type workerSignal struct {
	State             string
	Progressed        time.Time
	Succeeded, Failed *time.Time
	CycleFailed       bool
}
type workerRegistry struct {
	mu       sync.Mutex
	instance id.ID
	signals  map[string]workerSignal
}

var workerSilence = map[string]time.Duration{"scheduler": 90 * time.Second, "connectors": 10 * time.Minute, "wa.poll": 2 * time.Minute, "wa.commands": 6 * time.Minute, "wa.content": 15 * time.Minute, "wa.outbox": 5 * time.Minute, "sop": 2 * time.Minute}

func (s *Server) initWorkers() {
	s.workers.instance = id.New()
	s.workers.signals = map[string]workerSignal{}
	for name := range workerSilence {
		state := "not_started"
		if !s.cfg.OutboundEnabled() {
			state = "disabled"
		}
		s.workers.signals[name] = workerSignal{State: state, Progressed: s.now()}
	}
}
func (s *Server) workerStart(name string) {
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	signal := s.workers.signals[name]
	signal.State = "running"
	signal.Progressed = s.now()
	signal.CycleFailed = false
	s.workers.signals[name] = signal
}
func (s *Server) workerProgress(name string, failed bool) {
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	signal := s.workers.signals[name]
	now := s.now()
	signal.Progressed = now
	if failed {
		signal.Failed = &now
		signal.CycleFailed = true
	} else {
		signal.Succeeded = &now
	}
	s.workers.signals[name] = signal
}
func (s *Server) workerFinish(name string) {
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	signal := s.workers.signals[name]
	now := s.now()
	signal.Progressed = now
	signal.State = "waiting"
	if signal.CycleFailed {
		signal.State = "failed"
	} else {
		signal.Succeeded = &now
	}
	s.workers.signals[name] = signal
}
func (s *Server) workerStop(name string) {
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	signal := s.workers.signals[name]
	signal.State = "stopped"
	signal.Progressed = s.now()
	s.workers.signals[name] = signal
}

// The monitor persists the worker's real progress time, never the monitor's
// own tick time. A stuck/exited worker cannot appear healthy just because
// this separate goroutine is still able to write PostgreSQL.
func (s *Server) RunOperationsMonitor(ctx context.Context) {
	flush := func() {
		s.workers.mu.Lock()
		snapshot := map[string]workerSignal{}
		for name, signal := range s.workers.signals {
			snapshot[name] = signal
		}
		s.workers.mu.Unlock()
		write, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		mode := s.cfg.RunMode
		if mode == "" {
			mode = "live"
		}
		for name, signal := range snapshot {
			_, err := s.store.DB().Exec(write, `insert into worker_health(instance_id,worker,runtime_mode,state,progressed_at,succeeded_at,failed_at) values($1,$2,$3,$4,$5,$6,$7) on conflict(instance_id,worker) do update set state=excluded.state,progressed_at=excluded.progressed_at,succeeded_at=excluded.succeeded_at,failed_at=excluded.failed_at`, s.workers.instance, name, mode, signal.State, signal.Progressed, signal.Succeeded, signal.Failed)
			if err != nil {
				if ctx.Err() == nil {
					s.log.Info("worker health persistence deferred")
				}
				return
			}
		}
	}
	flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-ticker.C:
			flush()
		}
	}
}
