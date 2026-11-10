//go:build !js

package island

import (
	"math"
	"testing"
	"time"
)

func TestPriorityFIFOReplacementAndOverflow(t *testing.T) {
	now := time.Unix(100, 0)
	m := Machine{}
	if !m.Push(Item{Tag: "first"}, now) || m.State != Banner {
		t.Fatal("first must immediately become banner")
	}
	m.Push(Item{Tag: "info", Priority: 0}, now)
	m.Push(Item{Tag: "ping1", Priority: 3}, now)
	m.Push(Item{Tag: "ping2", Priority: 3}, now)
	m.Push(Item{Tag: "info", Priority: 2, Value: "replacement"}, now)
	for _, want := range []string{"ping1", "ping2", "info"} {
		m.Dismiss(now)
		now = now.Add(120 * time.Millisecond)
		m.Tick(now, false, false, false)
		if m.Current == nil || m.Current.Tag != want {
			t.Fatalf("want %s, got %+v", want, m.Current)
		}
	}
	if m.Current.Value != "replacement" {
		t.Fatal("tag did not replace")
	}
	m.Push(Item{Tag: "info", Value: "current replacement"}, now)
	if m.Current.Value != "current replacement" || len(m.Queue) != 0 {
		t.Fatal("current replacement enqueued twice")
	}
	for i := 0; i < Capacity; i++ {
		if !m.Push(Item{}, now) {
			t.Fatal("premature overflow")
		}
	}
	if m.Push(Item{}, now) {
		t.Fatal("overflow must return false for OS fallback")
	}
}

func TestHoverClickLeaveEscapeAndExpiry(t *testing.T) {
	now := time.Unix(100, 0)
	m := Machine{}
	m.Push(Item{}, now)
	m.Tick(now, true, false, false)
	m.Tick(now.Add(299*time.Millisecond), true, false, false)
	if m.State != Banner {
		t.Fatal("expanded before dwell")
	}
	m.Tick(now.Add(300*time.Millisecond), true, false, false)
	if m.State != Expanded {
		t.Fatal("did not expand")
	}
	m.Tick(now.Add(time.Second), false, false, false)
	m.Tick(now.Add(2499*time.Millisecond), false, false, false)
	if !m.Closing.IsZero() {
		t.Fatal("closed before leave grace")
	}
	m.Tick(now.Add(2500*time.Millisecond), false, false, false)
	if m.Closing.IsZero() {
		t.Fatal("did not close after leave")
	}
	m.Tick(now.Add(2620*time.Millisecond), false, false, false)
	if m.State != Hidden || m.Current != nil {
		t.Fatal("did not hide after exit")
	}
	m.Push(Item{}, now)
	m.Tick(now, true, true, false)
	if m.State != Expanded {
		t.Fatal("click should expand immediately")
	}
	m.Tick(now, false, false, true)
	m.Tick(now.Add(120*time.Millisecond), false, false, false)
	if m.State != Hidden {
		t.Fatal("escape did not hide")
	}
	m.Push(Item{}, now)
	m.Tick(now.Add(BannerDuration), false, false, false)
	if m.Closing.IsZero() {
		t.Fatal("banner did not expire")
	}
}

func TestHoverPausesExpiry(t *testing.T) {
	now := time.Unix(100, 0)
	m := Machine{}
	m.Push(Item{}, now)
	m.Tick(now.Add(10*time.Second), true, false, false)
	if !m.Closing.IsZero() {
		t.Fatal("hover must pause expiry")
	}
}

func TestSpringContinuityAndSettling(t *testing.T) {
	p, v := 200., 0.
	for i := 0; i < 120; i++ {
		p, v = Spring(p, v, 380, 1./120)
	}
	if math.Abs(p-380) > .1 || math.Abs(v) > .2 {
		t.Fatalf("spring did not settle %v %v", p, v)
	}
	p, v = Spring(240, 123, 440, 0)
	if p != 240 || v != 123 {
		t.Fatal("target change must retain position and velocity")
	}
	a, b := Spring(200, 0, 440, .1)
	a, b = Spring(a, b, 440, .1)
	c, d := Spring(200, 0, 440, .2)
	if math.Abs(a-c) > 1e-9 || math.Abs(b-d) > 1e-9 {
		t.Fatal("spring depends on frame rate")
	}
	for _, x := range []float64{0, .1, .5, 1} {
		if y := Ease(x); y < 0 || y > 1 {
			t.Fatalf("invalid ease %v", y)
		}
	}
	if math.Abs(Ease(0)) > 1e-6 || math.Abs(Ease(1)-1) > 1e-6 {
		t.Fatal("ease endpoints")
	}
}
