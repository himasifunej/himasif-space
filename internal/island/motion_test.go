//go:build !js

package island

import (
	"math"
	"testing"
	"time"
)

// Kiri → Kanan settles in about 350–500 ms, overshoots a little, and then
// asks for no frames.
func TestSlideSpring(t *testing.T) {
	var s Slide
	s.Jump(0)
	now := time.Unix(0, 0)
	dt := 1. / 60
	max, settledAt := 0., time.Duration(0)
	for i := 0; i < 120; i++ {
		now = now.Add(time.Second / 60)
		if !s.Step(1, dt, now, false) {
			settledAt = time.Duration(i+1) * time.Second / 60
			break
		}
		max = math.Max(max, s.Anchor)
		if s.Anchor > 1 {
			t.Fatalf("left the screen: %v", s.Anchor)
		}
	}
	if settledAt == 0 || s.Anchor != 1 || s.Velocity != 0 {
		t.Fatalf("never settled: %+v", s)
	}
	// 95% of the way, the eye's "arrived", within 350–500 ms.
	var r Slide
	r.Jump(0)
	now = time.Unix(0, 0)
	arrived := time.Duration(0)
	for i := 0; i < 120 && arrived == 0; i++ {
		now = now.Add(time.Second / 60)
		r.Step(1, dt, now, false)
		if r.Anchor >= .95 {
			arrived = time.Duration(i+1) * time.Second / 60
		}
	}
	if arrived < 200*time.Millisecond || arrived > 500*time.Millisecond {
		t.Fatalf("arrived after %v", arrived)
	}
	if settledAt > 900*time.Millisecond {
		t.Fatalf("settled after %v", settledAt)
	}
	if s.Step(1, dt, now, false) {
		t.Fatal("still moving at rest")
	}
}

// Kanan → Tengah overshoots past the middle a little before settling.
func TestSlideOvershoots(t *testing.T) {
	var s Slide
	s.Jump(1)
	now := time.Unix(0, 0)
	min := 1.
	for i := 0; i < 120; i++ {
		now = now.Add(time.Second / 60)
		if !s.Step(.5, 1./60, now, false) {
			break
		}
		min = math.Min(min, s.Anchor)
	}
	if min >= .5 || min < .45 || s.Anchor != .5 {
		t.Fatalf("min %v, rest %v", min, s.Anchor)
	}
}

// Reduced motion: a short eased move, no overshoot, done in SlideReduced.
func TestSlideReduced(t *testing.T) {
	var s Slide
	s.Jump(0)
	start := time.Unix(0, 0)
	s.Step(1, 0, start, true)
	for ms := 16; ms < 2000; ms += 16 {
		now := start.Add(time.Duration(ms) * time.Millisecond)
		moving := s.Step(1, .016, now, true)
		if s.Anchor > 1 || s.Velocity != 0 {
			t.Fatalf("reduced: %+v", s)
		}
		if !moving {
			if time.Duration(ms)*time.Millisecond > SlideReduced+20*time.Millisecond {
				t.Fatalf("took %d ms", ms)
			}
			break
		}
	}
	if s.Anchor != 1 {
		t.Fatalf("rest %v", s.Anchor)
	}
}

// The jelly stretches with speed, never past JellyMax, squashes past rest
// as the motion stops, and then rests exactly.
func TestJellyWobblesAndRests(t *testing.T) {
	var j Jelly
	for i := 0; i < 30; i++ {
		j.Step(4000, 1./60)
	}
	sx, sy := j.Scale()
	if sx <= 1.1 || sx > 1+JellyMax+1e-9 || math.Abs(sx*sy-1) > 1e-9 {
		t.Fatalf("moving: %v %v", sx, sy)
	}
	if lag := j.EyeLag(1); lag >= 0 || lag < -3 {
		t.Fatalf("eye lag %v", lag)
	}
	squashed := false
	frames := 0
	for ; frames < 120 && j.Step(0, 1./60); frames++ {
		if j.Stretch < -.003 {
			squashed = true
		}
	}
	if !squashed {
		t.Fatal("no bounce past rest")
	}
	if j.Stretch != 0 || frames*1000/60 > 600 {
		t.Fatalf("rest %v after %d frames", j.Stretch, frames)
	}
	if j.Step(0, 1./60) {
		t.Fatal("wobbling at rest")
	}
}
