//go:build !js

package island

import (
	"math"
	"time"
)

// The idle island is Ace: a pill with two eyes. They blink every few
// seconds and now and then glance aside; between those moments nothing
// moves, so the island asks for no frames at all.
const (
	BlinkDuration  = 140 * time.Millisecond
	GlanceDuration = 1600 * time.Millisecond
	glanceTurn     = 280 * time.Millisecond
)

// Eyes schedules Ace's idle motion. The zero value starts on its first
// Frame; a long pause (a hidden island) restarts the schedule rather than
// replaying what was missed.
type Eyes struct {
	rnd           uint32
	blink, glance time.Time
	gaze          float64
}

// Frame returns how open the eyes are (0 shut, 1 open), where they look
// (-1 left, 1 right) and when the next frame is due: soon while they
// move, at the next blink or glance while they rest.
func (e *Eyes) Frame(now time.Time, reduced bool) (open, gaze float64, next time.Time) {
	if e.blink.IsZero() {
		e.rnd = uint32(now.UnixNano()) | 1
		e.blink = now.Add(e.span(2200, 4200))
		e.glance = now.Add(e.span(6000, 11000))
	}
	if now.Sub(e.blink) >= BlinkDuration {
		e.blink = now.Add(e.span(2800, 6200))
		// Sometimes twice, as people do.
		if e.span(0, 4) == 0 {
			e.blink = now.Add(120 * time.Millisecond)
		}
	}
	if now.Sub(e.glance) >= GlanceDuration {
		e.glance = now.Add(e.span(7000, 13000))
		e.gaze = 1
		if e.span(0, 2) == 0 {
			e.gaze = -1
		}
	}
	open = 1
	if t := now.Sub(e.blink); t >= 0 {
		open = 1 - math.Sin(math.Pi*float64(t)/float64(BlinkDuration))
	}
	if t := now.Sub(e.glance); t >= 0 && !reduced {
		in := Ease(float64(t) / float64(glanceTurn))
		out := Ease(float64(t-GlanceDuration+glanceTurn) / float64(glanceTurn))
		gaze = e.gaze * (in - out)
	}
	frame := now.Add(16 * time.Millisecond)
	next = e.blink
	if !next.After(now) {
		next = frame
	}
	if !reduced {
		g := e.glance
		// While the eyes hold their glance, nothing moves until they turn back.
		if t := now.Sub(e.glance); t >= glanceTurn && t < GlanceDuration-glanceTurn {
			g = e.glance.Add(GlanceDuration - glanceTurn)
		} else if t >= 0 {
			g = frame
		}
		if g.Before(next) {
			next = g
		}
	}
	return open, gaze, next
}

// span is a pseudo-random duration in [lo, hi) milliseconds.
func (e *Eyes) span(lo, hi int) time.Duration {
	e.rnd ^= e.rnd << 13
	e.rnd ^= e.rnd >> 17
	e.rnd ^= e.rnd << 5
	return time.Duration(lo+int(e.rnd%uint32(hi-lo))) * time.Millisecond
}
