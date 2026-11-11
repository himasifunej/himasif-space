//go:build !js

// Package island owns the desktop notification queue and deterministic timing.
// The caller serializes access; no goroutines or frame timers live here.
package island

import (
	"math"
	"sort"
	"time"
)

type State uint8

const (
	Hidden State = iota
	Banner
	Expanded
)

type Item struct {
	Tag      string
	Priority int
	Value    any
}

const Capacity = 64
const BannerDuration = 7 * time.Second

type Machine struct {
	State                         State
	Current                       *Item
	Queue                         []Item
	Until, hoverSince, leaveSince time.Time
	Closing                       time.Time
	// stayOpen: the open details were closed by the user, with more
	// queued; the next notice opens in their place, and the island folds
	// only when the queue is empty.
	stayOpen bool
}

// Push replaces a tagged entry without changing its queue position. Bounded
// queues reject overflow so the caller can hand the notice to Windows instead.
func (m *Machine) Push(n Item, now time.Time) bool {
	if n.Tag != "" {
		if m.Current != nil && m.Current.Tag == n.Tag {
			m.Current = &n
			m.Until = now.Add(BannerDuration)
			m.Closing = time.Time{}
			return true
		}
		for i := range m.Queue {
			if m.Queue[i].Tag == n.Tag {
				m.Queue[i] = n
				return true
			}
		}
	}
	if len(m.Queue) >= Capacity {
		return false
	}
	m.Queue = append(m.Queue, n)
	if m.Current == nil {
		m.next(now)
	}
	return true
}

func (m *Machine) next(now time.Time) {
	m.hoverSince, m.leaveSince, m.Closing = time.Time{}, time.Time{}, time.Time{}
	if len(m.Queue) == 0 {
		m.Current = nil
		m.State = Hidden
		return
	}
	sort.SliceStable(m.Queue, func(i, j int) bool { return m.Queue[i].Priority > m.Queue[j].Priority })
	n := m.Queue[0]
	m.Queue = m.Queue[1:]
	m.Current = &n
	m.State = Banner
	if m.stayOpen {
		m.State = Expanded
	}
	m.stayOpen = false
	m.Until = now.Add(BannerDuration)
}

// Dismiss closes the current notice, by the user's hand (Tutup, Buka,
// Tunda): the next queued one follows in the open details.
func (m *Machine) Dismiss(now time.Time) {
	if m.Current != nil && m.Closing.IsZero() {
		m.Closing = now
		m.stayOpen = m.State == Expanded && len(m.Queue) > 0
	}
}

// DismissAll closes the current notice and drops the queue.
func (m *Machine) DismissAll(now time.Time) []Item {
	dropped := m.Queue
	m.Queue = nil
	m.Dismiss(now)
	return dropped
}

// lapse closes the current notice by itself (its time ran out, or the
// pointer left the details): the island folds back.
func (m *Machine) lapse(now time.Time) {
	m.Dismiss(now)
	m.stayOpen = false
}

// Tick pauses banner expiry under the pointer. Expansion needs 300 ms dwell,
// and leaving expanded content allows 1.5 seconds to return to the actions.
func (m *Machine) Tick(now time.Time, hover, expand, escape bool) {
	if m.Current == nil {
		return
	}
	if !m.Closing.IsZero() {
		if now.Sub(m.Closing) >= 120*time.Millisecond {
			m.next(now)
		}
		return
	}
	if escape {
		m.Dismiss(now)
		return
	}
	if hover {
		m.leaveSince = time.Time{}
		if m.hoverSince.IsZero() {
			m.hoverSince = now
		}
		if expand || now.Sub(m.hoverSince) >= 300*time.Millisecond {
			m.State = Expanded
		}
		m.Until = now.Add(BannerDuration)
	} else {
		m.hoverSince = time.Time{}
		if m.State == Expanded {
			if m.leaveSince.IsZero() {
				m.leaveSince = now
			}
			if now.Sub(m.leaveSince) >= 1500*time.Millisecond {
				m.lapse(now)
			}
		} else if !now.Before(m.Until) {
			m.lapse(now)
		}
	}
}

// Spring integrates the exact underdamped solution (mass 1, k 170, c 19).
// Retaining velocity across calls keeps interrupted changes continuous.
func Spring(position, velocity, target, seconds float64) (float64, float64) {
	const a = 9.5
	w := math.Sqrt(170 - a*a)
	e, c, s := math.Exp(-a*seconds), math.Cos(w*seconds), math.Sin(w*seconds)
	d := position - target
	return target + e*(d*c+(velocity+a*d)/w*s), e * (velocity*c - (a*velocity+170*d)/w*s)
}

// Ease is cubic-bezier(.2,0,0,1), solving x by bisection, stable at x=0.
func Ease(x float64) float64 {
	return bezier(x, .2, 0)
}

// ExitEase is cubic-bezier(.4,0,1,1).
func ExitEase(x float64) float64 { return bezier(x, .4, 1) }

func bezier(x, x1, x2 float64) float64 {
	x = math.Max(0, math.Min(1, x))
	lo, hi := 0., 1.
	for i := 0; i < 24; i++ {
		t := (lo + hi) / 2
		v := 3*x1*(1-t)*(1-t)*t + 3*x2*(1-t)*t*t + t*t*t
		if v < x {
			lo = t
		} else {
			hi = t
		}
	}
	t := (lo + hi) / 2
	return 3*(1-t)*t*t + t*t*t
}
