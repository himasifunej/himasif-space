package server

import (
	"context"
	"fmt"
	"runtime/debug"
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

// Restart delays of a worker that panicked: doubling from the first to the
// last, so a worker that panics on every cycle does not spin.
var (
	workerRestartFirst = time.Second
	workerRestartMax   = time.Minute
)

// recoverWorker turns a panic of a worker goroutine into a log line with
// its stack and a failed worker_health state, instead of the end of the
// whole process (SEC-07). It must be deferred directly. It reports whether
// it recovered one.
func (s *Server) recoverWorker(name string, recovered *bool) {
	p := recover()
	if p == nil {
		return
	}
	if recovered != nil {
		*recovered = true
	}
	s.log.Error("worker panic", "worker", name, "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	// Only workers listed in workerSilence have a worker_health row; the
	// others (realtime, operations monitor) only log.
	if signal, ok := s.workers.signals[name]; ok {
		now := s.now()
		signal.State = "failed"
		signal.Progressed = now
		signal.Failed = &now
		signal.CycleFailed = true
		s.workers.signals[name] = signal
	}
}

// Supervise runs a worker until ctx ends. When run panics, the panic is
// logged, the worker is marked failed and run starts again after a delay
// that grows with consecutive panics. A run that returns by itself ends
// Supervise: workers return when ctx ends or when they are disabled.
func (s *Server) Supervise(ctx context.Context, name string, run func(context.Context)) {
	delay := workerRestartFirst
	for {
		panicked := false
		started := s.now()
		func() {
			defer s.recoverWorker(name, &panicked)
			run(ctx)
		}()
		if !panicked || ctx.Err() != nil {
			return
		}
		// A worker that ran a while before panicking starts from the
		// shortest delay again.
		if s.now().Sub(started) > 10*workerRestartMax {
			delay = workerRestartFirst
		}
		s.log.Info("worker restarting", "worker", name, "after", delay.String())
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		delay = min(2*delay, workerRestartMax)
	}
}

// goSafe runs fn in a goroutine tracked by wg, recovering its panic as a
// failure of the worker name (a one-off task, such as polling one
// connection or sending a recovery email).
func (s *Server) goSafe(wg *sync.WaitGroup, name string, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer s.recoverWorker(name, nil)
		fn()
	}()
}
