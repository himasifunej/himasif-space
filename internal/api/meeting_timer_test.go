package api

import (
	"errors"
	"testing"
	"time"
)

// The shared timer's state machine: pauses are left out of the running
// time, next closes the agenda item with its real length, stop clears it,
// and actions that do not fit the state are refused.
func TestApplyTimer(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 23, 50, 0, 0, time.UTC) // runs past midnight
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }
	must := func(tm *MeetingTimer, action string, now time.Time) *MeetingTimer {
		t.Helper()
		out, err := ApplyTimer(tm, action, now)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		return out
	}
	if _, err := ApplyTimer(nil, TimerPause, t0); !errors.Is(err, ErrTimerAction) {
		t.Fatal("pause without timer")
	}
	tm := must(nil, TimerStart, t0)
	if _, err := ApplyTimer(tm, TimerStart, t0); !errors.Is(err, ErrTimerAction) {
		t.Fatal("start twice")
	}
	tm = must(tm, TimerPause, at(5))
	if _, err := ApplyTimer(tm, TimerPause, at(6)); !errors.Is(err, ErrTimerAction) {
		t.Fatal("pause twice")
	}
	if got := tm.Running(at(30)); got != 5*time.Minute {
		t.Fatalf("paused running %v", got)
	}
	before := *tm
	tm = must(tm, TimerResume, at(8))
	if before.PausedAt == nil || tm.PausedMs != (3*time.Minute).Milliseconds() {
		t.Fatalf("resume %+v (input changed: %v)", tm, before.PausedAt == nil)
	}
	if got := tm.Running(at(20)); got != 17*time.Minute {
		t.Fatalf("running %v", got)
	}
	tm = must(tm, TimerExtend, at(20))
	tm = must(tm, TimerNext, at(20))
	if tm.Agenda != 1 || len(tm.DoneMs) != 1 || tm.DoneMs[0] != (17*time.Minute).Milliseconds() || tm.ExtraMs != 0 || tm.AgendaAtMs != (17*time.Minute).Milliseconds() {
		t.Fatalf("next %+v", tm)
	}
	if out := must(tm, TimerStop, at(25)); out != nil {
		t.Fatal("stop keeps a timer")
	}
	if _, err := ApplyTimer(tm, "skip", at(25)); !errors.Is(err, ErrTimerAction) {
		t.Fatal("unknown action")
	}
}
