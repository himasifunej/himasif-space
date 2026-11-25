package auth

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

func withHashLimits(t *testing.T, concurrency, queue int) {
	t.Helper()
	SetHashLimits(concurrency, queue)
	t.Cleanup(func() { SetHashLimits(DefaultHashConcurrency, DefaultHashQueue) })
}

// Many logins at once run at most the configured number of Argon2id hashes
// together, so the memory they take stays bounded (SEC-02): unbounded, ten
// verifications would hold 190 MiB at once.
func TestParallelVerificationsStayWithinTheHashLimit(t *testing.T) {
	h, err := HashPassword("kata-sandi-panjang")
	if err != nil {
		t.Fatal(err)
	}
	const limit, callers = 2, 10
	withHashLimits(t, limit, callers)
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	stop := make(chan struct{})
	var peakHeap uint64
	var sampler sync.WaitGroup
	sampler.Add(1)
	go func() {
		defer sampler.Done()
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			peakHeap = max(peakHeap, m.HeapInuse)
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- VerifyPasswordContext(context.Background(), h, "kata-sandi-panjang")
		}()
	}
	wg.Wait()
	close(stop)
	sampler.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("verification: %v", err)
		}
	}
	if _, peak := HashStats(); peak < 1 || peak > limit {
		t.Fatalf("peak hashes at once = %d, limit %d", peak, limit)
	}
	// Garbage of finished runs may wait for the collector, so allow a few
	// runs' worth over the limit; still far below all ten at once.
	const run = argonMemory * 1024
	if grown := int64(peakHeap) - int64(base.HeapInuse); grown > 6*run {
		t.Fatalf("heap grew %d MiB during %d parallel verifications (limit %d)", grown>>20, callers, limit)
	}
}

func TestBusyHashingIsRefusedNotQueuedForever(t *testing.T) {
	h, err := HashPassword("kata-sandi-panjang")
	if err != nil {
		t.Fatal(err)
	}
	withHashLimits(t, 1, 0)
	release, err := currentGate().acquire(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	// No queue: refused at once.
	if err := VerifyPasswordContext(context.Background(), h, "kata-sandi-panjang"); !errors.Is(err, ErrBusy) {
		t.Fatalf("full gate without queue = %v", err)
	}
	if err := BurnTimeContext(context.Background(), "x"); !errors.Is(err, ErrBusy) {
		t.Fatalf("burn on a full gate = %v", err)
	}
	// With a queue, a caller gives up when its context ends.
	withHashLimits(t, 1, 4)
	release()
	release, _ = currentGate().acquire(context.Background(), true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := HashPasswordContext(ctx, "kata-sandi-panjang"); !errors.Is(err, ErrBusy) {
		t.Fatalf("timed out wait = %v", err)
	}
	release()
	if err := VerifyPasswordContext(context.Background(), h, "kata-sandi-panjang"); err != nil {
		t.Fatalf("after release: %v", err)
	}
	if n, _ := HashStats(); n != 0 {
		t.Fatalf("slots leaked: %d in flight", n)
	}
}
