//go:build !js

package main

import (
	"testing"
	"time"
)

// islandMediaFake plays a session as a browser shows it to Windows: what
// each read of the media controls returns, at the watcher's poll.
type islandMediaFake struct {
	t     *testing.T
	w     *islandMediaWatcher
	now   time.Time
	shown float64 // the bar at the last frame
}

func newIslandMediaFake(t *testing.T) *islandMediaFake {
	return &islandMediaFake{t: t, w: &islandMediaWatcher{}, now: time.Date(2026, 11, 16, 15, 41, 0, 0, time.UTC)}
}

// read is one poll returning title, playing and the timeline (pos as of
// updated; zero updated: the player leaves it out).
func (f *islandMediaFake) read(title string, playing bool, pos, dur time.Duration, updated time.Time) {
	f.w.take(islandMedia{Title: title, Artist: "Contoh", App: "Chrome", Playing: playing, At: f.now, tl: islandTimeline{Position: pos, Duration: dur, Updated: updated}}, true, f.now)
}

// frames draws the bar every 100 ms for d, and fails when it goes back.
func (f *islandMediaFake) frames(d time.Duration, what string) {
	f.t.Helper()
	for end := f.now.Add(d); f.now.Before(end); f.now = f.now.Add(100 * time.Millisecond) {
		m, _ := f.w.Current()
		q := m.Progress(f.now)
		if q < f.shown-1e-9 {
			f.t.Fatalf("%s: the bar went back from %.4f to %.4f at %s", what, f.shown, q, f.now.Format("15:04:05.0"))
		}
		f.shown = q
	}
}

// at is the bar's place, in the track, at now.
func (f *islandMediaFake) at() time.Duration {
	m, _ := f.w.Current()
	return m.positionAt(f.now)
}

func near(a, b time.Duration) bool { return absDuration(a-b) <= 200*time.Millisecond }

// A timeline read again unchanged, its time of update left out (or
// refused), must not pull the bar back at every poll: the bug Felix saw,
// the bar running then jumping back, again and again.
func TestIslandMediaStaleTimelineDoesNotSawtooth(t *testing.T) {
	f := newIslandMediaFake(t)
	for i := 0; i < 8; i++ {
		f.read("Avalanche", true, 30*time.Second, 4*time.Minute, time.Time{})
		f.frames(islandMediaPoll, "stale timeline")
	}
	if want := 30*time.Second + 8*islandMediaPoll; !near(f.at(), want) {
		t.Fatalf("after 12 s from 0:30 the bar is at %v, want %v", f.at(), want)
	}
}

// Next from the island, a browser sending the new title first and the
// new track's timeline only later: the bar starts the new track from 0
// and runs on smoothly, then follows the new timeline.
func TestIslandMediaNextBeforeTimeline(t *testing.T) {
	f := newIslandMediaFake(t)
	sent := f.now.Add(-2 * time.Minute)
	for i := 0; i < 3; i++ {
		f.read("Lagu lama", true, 0, 4*time.Minute, sent)
		f.frames(islandMediaPoll, "old track")
	}
	if !near(f.at(), 2*time.Minute+3*islandMediaPoll) {
		t.Fatalf("old track at %v", f.at())
	}
	// Next: 150 ms after, the title is the new one, the timeline the old.
	f.now = f.now.Add(150 * time.Millisecond)
	f.read("Avalanche", true, 0, 4*time.Minute, sent)
	f.shown = 0
	if f.at() != 0 {
		t.Fatalf("the new track starts at %v, want 0", f.at())
	}
	for i := 0; i < 3; i++ {
		f.frames(islandMediaPoll, "new track, old timeline")
		f.read("Avalanche", true, 0, 4*time.Minute, sent)
	}
	// Then its own timeline: 4.4 s in, of 3:10.
	upd := f.now
	f.read("Avalanche", true, 4400*time.Millisecond, 3*time.Minute+10*time.Second, upd)
	if !near(f.at(), 4400*time.Millisecond) {
		t.Fatalf("new timeline: the bar at %v, want 4.4 s", f.at())
	}
	f.shown = 0 // the length changed: the share of it may too
	f.frames(time.Second, "new timeline")
	for i := 0; i < 4; i++ {
		f.read("Avalanche", true, 4400*time.Millisecond, 3*time.Minute+10*time.Second, upd)
		f.frames(islandMediaPoll, "new timeline, read again")
	}
}

// The timeline first, the title after: the restarted timeline already is
// the new track's.
func TestIslandMediaNextTimelineFirst(t *testing.T) {
	f := newIslandMediaFake(t)
	f.read("Lagu lama", true, 2*time.Minute, 4*time.Minute, f.now)
	f.frames(islandMediaPoll, "old track")
	sent := f.now
	f.read("Lagu lama", true, 0, 3*time.Minute, sent)
	if !near(f.at(), 0) {
		t.Fatalf("restarted timeline: %v", f.at())
	}
	f.shown = 0
	f.frames(islandMediaPoll, "restarted")
	f.read("Avalanche", true, 0, 3*time.Minute, sent)
	if !near(f.at(), islandMediaPoll) {
		t.Fatalf("the new title moved the bar to %v, want %v", f.at(), islandMediaPoll)
	}
	f.frames(3*islandMediaPoll, "new track")
}

func TestIslandMediaSeekAndPause(t *testing.T) {
	f := newIslandMediaFake(t)
	f.read("Avalanche", true, time.Minute, 4*time.Minute, f.now)
	f.frames(islandMediaPoll, "playing")
	// A seek to 3:00: the bar follows at once.
	f.read("Avalanche", true, 3*time.Minute, 4*time.Minute, f.now)
	if !near(f.at(), 3*time.Minute) {
		t.Fatalf("seek: %v", f.at())
	}
	f.shown = f.w.cur.Progress(f.now)
	f.frames(3*time.Second, "after seek")
	// Pause, the timeline not yet sent: the bar stays where it is.
	here := f.at()
	f.read("Avalanche", false, 3*time.Minute, 4*time.Minute, f.now.Add(-3*time.Second))
	if !near(f.at(), here) {
		t.Fatalf("pause moved the bar from %v to %v", here, f.at())
	}
	f.frames(5*time.Second, "paused")
	if !near(f.at(), here) {
		t.Fatalf("paused bar ran to %v", f.at())
	}
	// The timeline at the pause, agreeing: nothing moves.
	f.read("Avalanche", false, here, 4*time.Minute, f.now)
	if !near(f.at(), here) {
		t.Fatalf("paused timeline moved the bar to %v", f.at())
	}
	// A seek while paused moves it.
	f.read("Avalanche", false, 30*time.Second, 4*time.Minute, f.now)
	if f.at() != 30*time.Second {
		t.Fatalf("paused seek: %v", f.at())
	}
	// Play again: on from there.
	f.shown = f.w.cur.Progress(f.now)
	f.read("Avalanche", true, 30*time.Second, 4*time.Minute, f.now)
	f.frames(2*time.Second, "played again")
	if !near(f.at(), 32*time.Second) {
		t.Fatalf("played again: %v, want 0:32", f.at())
	}
}

// What changed tells the island to draw: not every poll of a track that
// plays on.
func TestIslandMediaTakeChanged(t *testing.T) {
	w := &islandMediaWatcher{}
	now := time.Now()
	read := func(pos time.Duration, upd time.Time) bool {
		return w.take(islandMedia{Title: "x", Playing: true, At: now, tl: islandTimeline{Position: pos, Duration: time.Minute, Updated: upd}}, true, now)
	}
	if !read(0, now) {
		t.Fatal("first read unchanged")
	}
	now = now.Add(islandMediaPoll)
	if read(0, now.Add(-islandMediaPoll)) {
		t.Fatal("the same timeline read again changed the island")
	}
	if !read(40*time.Second, now) {
		t.Fatal("a seek did not change the island")
	}
}
