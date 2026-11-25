package server

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func workerTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil)), now: time.Now}
	s.initWorkers()
	first, maxDelay := workerRestartFirst, workerRestartMax
	workerRestartFirst, workerRestartMax = 5*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { workerRestartFirst, workerRestartMax = first, maxDelay })
	return s
}

func (s *Server) workerState(name string) workerSignal {
	s.workers.mu.Lock()
	defer s.workers.mu.Unlock()
	return s.workers.signals[name]
}

// A panicking worker does not end the process: it is logged, marked
// failed, and started again after a delay (SEC-07).
func TestSupervisedWorkerSurvivesPanicsAndRestarts(t *testing.T) {
	s := workerTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	var starts []time.Time
	var mu sync.Mutex
	failedSeen := make(chan workerSignal, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Supervise(ctx, "sop", func(ctx context.Context) {
			defer s.workerStop("sop")
			n := runs.Add(1)
			mu.Lock()
			starts = append(starts, time.Now())
			mu.Unlock()
			if n == 2 {
				// The previous run's panic is recorded before the restart.
				select {
				case failedSeen <- s.workerState("sop"):
				default:
				}
			}
			s.workerStart("sop")
			if n <= 3 {
				var m map[string]int
				m["boom"]++ // a real runtime panic: nil map write
			}
			<-ctx.Done()
		})
	}()
	select {
	case sig := <-failedSeen:
		if sig.State != "failed" || sig.Failed == nil {
			t.Fatalf("after a panic the worker is %q (failed at %v)", sig.State, sig.Failed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker was not restarted after its panic")
	}
	deadline := time.Now().Add(5 * time.Second)
	for runs.Load() < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("worker ran %d times, want a fourth healthy run", runs.Load())
		}
		time.Sleep(time.Millisecond)
	}
	if st := s.workerState("sop").State; st != "running" {
		t.Fatalf("restarted worker state = %q", st)
	}
	mu.Lock()
	for i := 1; i < len(starts); i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < workerRestartFirst {
			t.Errorf("restart %d after %v, before the delay", i, gap)
		}
	}
	mu.Unlock()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("supervised worker did not stop with its context")
	}
	if st := s.workerState("sop").State; st != "stopped" {
		t.Fatalf("worker state after stop = %q", st)
	}
}

// A worker that returns (disabled, or its context ended) is not restarted.
func TestSupervisedWorkerThatReturnsIsNotRestarted(t *testing.T) {
	s := workerTestServer(t)
	runs := 0
	s.Supervise(context.Background(), "scheduler", func(context.Context) { runs++ })
	if runs != 1 {
		t.Fatalf("runs = %d", runs)
	}
	// A panic while stopping ends Supervise too, without waiting a delay.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Supervise(ctx, "scheduler", func(context.Context) { panic("while stopping") })
}

// Stopping during the restart delay ends Supervise at once, and workers
// without a worker_health row (realtime) only log their panic.
func TestSupervisedWorkerStopsDuringRestartDelay(t *testing.T) {
	s := workerTestServer(t)
	workerRestartFirst, workerRestartMax = time.Hour, time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Supervise(ctx, "realtime", func(context.Context) { panic("boom") })
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Supervise kept waiting its delay after the context ended")
	}
	if _, ok := s.workers.signals["realtime"]; ok {
		t.Fatal("a worker_health row was made for an unlisted worker")
	}
}

// One-off goroutines (one connection's poll, a recovery email) recover
// their panic and mark their worker's cycle failed; no goroutine is left.
func TestGoSafeRecoversAndLeavesNoGoroutine(t *testing.T) {
	s := workerTestServer(t)
	before := runtime.NumGoroutine()
	var wg sync.WaitGroup
	for range 20 {
		s.goSafe(&wg, "wa.poll", func() { panic("one connection") })
	}
	wg.Wait()
	sig := s.workerState("wa.poll")
	if !sig.CycleFailed || sig.Failed == nil {
		t.Fatalf("wa.poll after a panic: %+v", sig)
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines %d, before %d", runtime.NumGoroutine(), before)
		}
		time.Sleep(time.Millisecond)
	}
}
