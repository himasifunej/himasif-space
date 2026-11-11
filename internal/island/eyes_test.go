//go:build !js

package island

import (
	"testing"
	"time"
)

func TestEyesBlinkAndRestWithoutFrames(t *testing.T) {
	var e Eyes
	now := time.Unix(1000, 0)
	open, gaze, next := e.Frame(now, false)
	if open != 1 || gaze != 0 {
		t.Fatalf("idle eyes start open and centered, got %v %v", open, gaze)
	}
	if next.Sub(now) < 2*time.Second {
		t.Fatalf("resting eyes must not ask for frames, next in %v", next.Sub(now))
	}
	// Run a minute at the requested deadlines: every blink shuts the eyes
	// and every frame between motions is skipped.
	frames, blinks, shut := 0, 0, false
	for end := now.Add(time.Minute); now.Before(end); frames++ {
		now = next
		open, gaze, next = e.Frame(now, false)
		if open < .2 && !shut {
			blinks++
		}
		shut = open < .2
		if gaze < -1 || gaze > 1 || open < 0 || open > 1 {
			t.Fatalf("out of range: open %v gaze %v", open, gaze)
		}
		if !next.After(now) {
			t.Fatal("next frame must be in the future")
		}
	}
	if blinks < 8 || blinks > 30 {
		t.Fatalf("want a blink every few seconds, got %d a minute", blinks)
	}
	// Each blink takes about 11 frames and each glance about 35.
	if frames > 900 {
		t.Fatalf("idle island drew %d frames a minute", frames)
	}
}

func TestEyesReducedMotionNeverGlance(t *testing.T) {
	var e Eyes
	now := time.Unix(1000, 0)
	for end := now.Add(time.Minute); now.Before(end); now = now.Add(10 * time.Millisecond) {
		if _, gaze, _ := e.Frame(now, true); gaze != 0 {
			t.Fatal("reduced motion must keep the gaze still")
		}
	}
}

func TestEyesRestartAfterPause(t *testing.T) {
	var e Eyes
	now := time.Unix(1000, 0)
	e.Frame(now, false)
	now = now.Add(time.Hour)
	if open, _, next := e.Frame(now, false); open != 1 || next.Sub(now) < time.Second {
		t.Fatalf("after a pause the eyes rest first, got open %v next %v", open, next.Sub(now))
	}
}
