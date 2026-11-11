//go:build !js

package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/island"
)

// A change of state starts the spring from where the shape is: the gap
// since the last frame (2 s while a banner rests, minutes after the island
// hid) must not land it at once.
func TestIslandFrameStepKeepsTheMotion(t *testing.T) {
	now := time.Now()
	if got := islandFrameStep(time.Time{}, now); got != 0 {
		t.Fatalf("first frame advanced %v", got)
	}
	if got := islandFrameStep(now.Add(-16*time.Millisecond), now); got < .015 || got > .017 {
		t.Fatalf("a 60 Hz frame advanced %v", got)
	}
	if got := islandFrameStep(now.Add(-2*time.Second), now); got != .05 {
		t.Fatalf("a late frame advanced %v", got)
	}
	// From the pill to the banner, one late frame leaves it on its way.
	g, _ := island.Spring(200, 0, 380, islandFrameStep(now.Add(-10*time.Minute), now))
	if g > 300 {
		t.Fatalf("the spring jumped to %v", g)
	}
}

func TestIslandGlyphColorsFollowTheCanvas(t *testing.T) {
	for _, tc := range []struct {
		style, kind string
		want        ui.Color
	}{
		{"A", "deadline", ui.RGB(255, 69, 58)},
		{"A", "task", ui.RGB(10, 132, 255)},
		{"B", "ping", ui.RGB(227, 160, 127)},
		{"B", "meeting", ui.RGB(168, 194, 172)},
		{"C", "meeting", ui.RGB(52, 199, 89)},
		{"C", "deadline", ui.RGB(255, 159, 10)},
	} {
		if got := islandGlyphColor(tc.style, tc.kind); got != tc.want {
			t.Errorf("%s %s: %v, want %v", tc.style, tc.kind, got, tc.want)
		}
	}
	if c := opaque(ui.RGBA(30, 30, 32, .42)); c.A != 255 {
		t.Fatalf("label color stays translucent: %v", c)
	}
}

// Fullscreen hides the island only once it has lasted its grace: the next
// frame is brought forward to then, and never past it.
func TestIslandObstructionGrace(t *testing.T) {
	now := time.Now()
	p := &islandPresenter{}
	if got := p.obstructionDeadline(now, 2*time.Second); got != 2*time.Second {
		t.Fatalf("unobstructed deadline moved to %v", got)
	}
	p.obstructedSince = now.Add(-time.Second)
	if got := p.obstructionDeadline(now, 2*time.Second); got != islandObstructionGrace-time.Second {
		t.Fatalf("obstructed deadline %v", got)
	}
	p.obstructedSince = now.Add(-time.Hour)
	if got := p.obstructionDeadline(now, 2*time.Second); got <= 0 {
		t.Fatalf("deadline must stay positive, got %v", got)
	}
	// The spike's environment override also answers for obstruction.
	p.environmentCheck = func() bool { return true }
	if p.obstructed() {
		t.Fatal("an allowed environment obstructs")
	}
}
