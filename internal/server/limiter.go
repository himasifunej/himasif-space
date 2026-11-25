package server

import (
	"container/list"
	"sync"
	"time"
)

// limiter allows a number of events per key in a sliding window, in
// memory. It protects login from guessing; one server process is enough
// for Space's scale, and a restart forgiving the counts is acceptable.
//
// It holds at most maxKeys keys. A new key at capacity evicts the key used
// least recently: refusing every new key instead would let anyone lock all
// logins by filling the table with made-up emails (SEC-02). Eviction only
// ever forgets counts, the lenient direction; with client addresses taken
// from Cloudflare (clientIPs in auth.go) cycling identities to reset a limit needs
// real addresses.
type limiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	maxKeys int
	hits    map[string]*list.Element // of *limitEntry, in order
	order   *list.List               // front: used most recently
	sweep   time.Time
}

type limitEntry struct {
	key string
	ts  []time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, maxKeys: 10_000, hits: map[string]*list.Element{}, order: list.New()}
}

// allow records an event of key at now and reports whether it is within
// the limit. A refused event is not recorded.
func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	if now.Sub(l.sweep) > l.window {
		for e := l.order.Back(); e != nil; {
			prev := e.Prev()
			if ent := e.Value.(*limitEntry); len(ent.ts) == 0 || !ent.ts[len(ent.ts)-1].After(cutoff) {
				l.remove(e)
			}
			e = prev
		}
		l.sweep = now
	}
	e, exists := l.hits[key]
	if !exists {
		for len(l.hits) >= l.maxKeys && l.order.Len() > 0 {
			l.remove(l.order.Back())
		}
		e = l.order.PushFront(&limitEntry{key: key})
		l.hits[key] = e
	} else {
		l.order.MoveToFront(e)
	}
	ent := e.Value.(*limitEntry)
	i := 0
	for i < len(ent.ts) && !ent.ts[i].After(cutoff) {
		i++
	}
	ent.ts = ent.ts[i:]
	if len(ent.ts) >= l.max {
		return false
	}
	ent.ts = append(ent.ts, now)
	return true
}

// forgive takes back the event of key recorded at at, for an attempt that
// turned out legitimate: a successful login does not count as a guess.
func (l *limiter) forgive(key string, at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.hits[key]
	if !ok {
		return
	}
	ent := e.Value.(*limitEntry)
	for i := len(ent.ts) - 1; i >= 0; i-- {
		if ent.ts[i].Equal(at) {
			ent.ts = append(ent.ts[:i], ent.ts[i+1:]...)
			break
		}
	}
	if len(ent.ts) == 0 {
		l.remove(e)
	}
}

func (l *limiter) remove(e *list.Element) {
	delete(l.hits, e.Value.(*limitEntry).key)
	l.order.Remove(e)
}
