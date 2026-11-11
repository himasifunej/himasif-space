//go:build !js

package island

import (
	"math"
	"testing"
	"time"
)

func TestGeometryFollowsTheCanvas(t *testing.T) {
	for _, tc := range []struct {
		s        Scene
		expanded bool
		want     [3]float64
	}{
		{SceneIdle, false, [3]float64{120, 32, 16}},
		{SceneNotice, false, [3]float64{360, 56, 28}},
		{SceneMedia, false, [3]float64{300, 40, 20}},
		{SceneTimer, false, [3]float64{240, 40, 20}},
		{SceneTimer, true, [3]float64{440, 168, 32}},
		{SceneIdle, true, [3]float64{440, 168, 32}},
	} {
		if got := Geometry(tc.s, tc.expanded); got != tc.want {
			t.Errorf("%v %v: %v, want %v", tc.s, tc.expanded, got, tc.want)
		}
	}
	if x, _ := EyesAt(SceneIdle, false, 120, 32); x != 60 {
		t.Fatalf("idle eyes not centred: %v", x)
	}
	if x, _ := EyesAt(SceneMedia, false, 300, 40); x != 34 {
		t.Fatalf("activity eyes not in the leading slot: %v", x)
	}
}

func TestBreathBopAndWaveStayInRange(t *testing.T) {
	for ms := 0; ms < 10000; ms += 7 {
		d := time.Duration(ms) * time.Millisecond
		if b := Breath(d); b < 1 || b > 1.04+1e-9 {
			t.Fatalf("breath %v at %v", b, d)
		}
		if b := Bop(d); math.Abs(b) > 1.5+1e-9 {
			t.Fatalf("bop %v at %v", b, d)
		}
		for i := range 5 {
			if w := Wave(d, i); w < .3-1e-9 || w > 1+1e-9 {
				t.Fatalf("wave %v at %v", w, d)
			}
		}
	}
}

func TestGazeFollowsThenRests(t *testing.T) {
	var g Gaze
	now := time.Unix(100, 0)
	g.Pointer(now, 480, 0) // far right
	for range 30 {
		now = now.Add(16 * time.Millisecond)
		g.Step(now)
	}
	if math.Abs(g.X-GazeMax) > .05 || g.Y != 0 {
		t.Fatalf("eyes look at %v,%v, want %v,0", g.X, g.Y, GazeMax)
	}
	if g.Step(now.Add(16 * time.Millisecond)) {
		t.Fatal("settled eyes still move")
	}
	// Near the pointer the eyes turn less.
	var n Gaze
	n.Pointer(now, 24, 0)
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		n.Step(now)
	}
	if n.X > .4 {
		t.Fatalf("close pointer turned the eyes %v", n.X)
	}
	// The pointer rests: back to the middle after 3 s.
	now = now.Add(3 * time.Second)
	g.Pointer(now, 480, 0)
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		g.Step(now)
	}
	if math.Abs(g.X) > .05 || g.Following(now) {
		t.Fatalf("resting pointer kept the gaze at %v", g.X)
	}
}

func TestExpanderHoverClickLeave(t *testing.T) {
	var e Expander
	now := time.Unix(100, 0)
	e.Tick(now, true, false)
	e.Tick(now.Add(299*time.Millisecond), true, false)
	if e.Expanded {
		t.Fatal("opened before 300 ms")
	}
	e.Tick(now.Add(300*time.Millisecond), true, false)
	if !e.Expanded {
		t.Fatal("hover did not open")
	}
	// A click under the pointer closes, and hover does not reopen.
	now = now.Add(time.Second)
	e.Tick(now, true, true)
	e.Tick(now.Add(time.Second), true, false)
	if e.Expanded {
		t.Fatal("hover reopened what a click closed")
	}
	// Leave, come back: opens again; leave 600 ms: closes.
	now = now.Add(2 * time.Second)
	e.Tick(now, false, false)
	e.Tick(now.Add(10*time.Millisecond), true, false)
	e.Tick(now.Add(310*time.Millisecond), true, false)
	if !e.Expanded {
		t.Fatal("did not reopen")
	}
	e.Tick(now.Add(400*time.Millisecond), false, false)
	e.Tick(now.Add(999*time.Millisecond), false, false)
	if !e.Expanded {
		t.Fatal("closed before 600 ms away")
	}
	e.Tick(now.Add(1000*time.Millisecond), false, false)
	if e.Expanded {
		t.Fatal("stayed open after the pointer left")
	}
}

func TestGazeAwayReturnsToTheMiddle(t *testing.T) {
	var g Gaze
	now := time.Unix(100, 0)
	g.Pointer(now, 100, 100)
	for range 30 {
		now = now.Add(16 * time.Millisecond)
		g.Step(now)
	}
	g.Away()
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		g.Step(now)
	}
	if g.X != 0 || g.Y != 0 || g.Following(now) {
		t.Fatalf("away pointer kept the gaze at %v,%v", g.X, g.Y)
	}
}

// Ace breathes while someone is about and stops at the end of a breath,
// at rest, once they are gone: no jump, and no frames while it sleeps.
func TestBreatherSleeps(t *testing.T) {
	var b Breather
	t0 := time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC)
	if s, on := b.Scale(t0); on || s != 1 {
		t.Fatalf("asleep at start: %v %v", s, on)
	}
	b.Wake(t0)
	if s, on := b.Scale(t0.Add(time.Second)); !on || s <= 1 {
		t.Fatalf("awake: %v %v", s, on)
	}
	// Still breathing just after awakeUntil, until the breath's end.
	if _, on := b.Scale(t0.Add(BreathAwake + time.Second)); !on {
		t.Fatal("stopped mid-breath")
	}
	end := t0.Add(BreathAwake + breathCycle)
	if s, on := b.Scale(end); on || s != 1 {
		t.Fatalf("still breathing at the end of the cycle: %v %v", s, on)
	}
	// Scale just before the stop is at rest within a hair.
	var c Breather
	c.Wake(t0)
	if s, _ := c.Scale(end.Add(-10 * time.Millisecond)); s-1 > .0005 {
		t.Fatalf("stopped away from rest: %v", s)
	}
	// Waking again starts a breath from rest.
	b.Wake(end.Add(time.Minute))
	if s, on := b.Scale(end.Add(time.Minute)); !on || s != 1 {
		t.Fatalf("woke with a jump: %v %v", s, on)
	}
}
