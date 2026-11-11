//go:build !js

package island

import (
	"math"
	"time"
)

// Ace moving along the top of the screen (0.4.5 rc4 feedback): picking
// Kiri, Tengah or Kanan slides it there on a spring instead of jumping,
// and while it slides or is dragged it wobbles like jelly, stretched along
// the motion as fast as it goes, squashed a little as it stops. Reduced
// motion: a short eased move and no wobble. Both settle to exact values
// and then ask for no more frames.

// SlideReduced is the eased move of reduced motion.
const SlideReduced = 160 * time.Millisecond

// Slide moves Ace's anchor (0 the left edge, 1 the right) to a target.
// The zero value is at rest at 0; Jump puts it somewhere at once.
type Slide struct {
	Anchor, Velocity float64 // anchor, and its change per second
	target, from     float64
	started          time.Time
}

// Jump puts Ace at anchor at once, at rest.
func (s *Slide) Jump(anchor float64) {
	*s = Slide{Anchor: anchor, target: anchor, from: anchor}
}

// Settled tells whether Ace rests at its target.
func (s *Slide) Settled() bool { return s.Anchor == s.target && s.Velocity == 0 }

// Step moves toward target by dt seconds, at now; it reports whether Ace
// still moves. The spring is Spring's: about 400 ms, overshooting by a few
// percent; it never leaves 0–1, where it stops at the edge.
func (s *Slide) Step(target float64, dt float64, now time.Time, reduced bool) bool {
	if target != s.target {
		s.target, s.from, s.started = target, s.Anchor, now
	}
	if s.Settled() {
		return false
	}
	if reduced {
		x := float64(now.Sub(s.started)) / float64(SlideReduced)
		s.Anchor, s.Velocity = s.from+(s.target-s.from)*Ease(math.Min(x, 1)), 0
		if x >= 1 {
			s.Anchor = s.target
			return false
		}
		return true
	}
	s.Anchor, s.Velocity = Spring(s.Anchor, s.Velocity, s.target, dt)
	if s.Anchor < 0 || s.Anchor > 1 {
		s.Anchor, s.Velocity = math.Max(0, math.Min(1, s.Anchor)), 0
	}
	if math.Abs(s.Anchor-s.target) < 1e-4 && math.Abs(s.Velocity) < 1e-3 {
		s.Anchor, s.Velocity = s.target, 0
		return false
	}
	return true
}

// Jelly limits: the most Ace stretches, and the speed (DIP a second) that
// stretches it that far.
const (
	JellyMax   = .16
	JellySpeed = 2400.
)

// Jelly is Ace's squash and stretch along its motion. It follows the
// speed on a springy, underdamped spring, so stopping sends it a little
// past rest: a squash, a small bounce, then still.
type Jelly struct {
	Stretch, velocity float64
}

// Step follows speed (DIP a second, either way) for dt seconds; it
// reports whether Ace still wobbles.
func (j *Jelly) Step(speed, dt float64) bool {
	target := math.Min(math.Abs(speed)/JellySpeed, 1) * JellyMax
	if dt > 0 {
		// x'' = -k(x-target) - c x': about 300 ms, overshooting ~10%.
		const k, c = 600., 28.
		for t := dt; t > 0; t -= 1. / 240 {
			h := math.Min(t, 1./240)
			a := -k*(j.Stretch-target) - c*j.velocity
			j.velocity += a * h
			j.Stretch += j.velocity * h
		}
		j.Stretch = math.Max(-JellyMax, math.Min(JellyMax, j.Stretch))
	}
	if target == 0 && math.Abs(j.Stretch) < .002 && math.Abs(j.velocity) < .02 {
		j.Stretch, j.velocity = 0, 0
		return false
	}
	return true
}

// Scale is the stretch as the pill's width and height factors: it keeps
// its area, wider and flatter while moving, narrower and taller as it
// bounces.
func (j Jelly) Scale() (sx, sy float64) {
	sx = 1 + j.Stretch
	return sx, 1 / sx
}

// EyeLag is how far the eyes trail the motion, in DIP: against the
// direction of travel (dir: -1 left, 1 right) while stretched, a little
// ahead as Ace bounces back.
func (j Jelly) EyeLag(dir float64) float64 {
	if dir == 0 {
		return 0
	}
	return -math.Copysign(j.Stretch*14, dir)
}
