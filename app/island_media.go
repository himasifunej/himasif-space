//go:build !js

package main

import (
	"strings"
	"sync"
	"time"
)

// islandMedia is what another app plays, as Windows' media controls show
// it (SMTC): the island's media activity.
type islandMedia struct {
	Title, Artist, App string
	Playing            bool
	// Position at At, and Duration (0: unknown, as a stream).
	Position, Duration time.Duration
	At                 time.Time
	// Since is when it last started or stopped playing.
	Since time.Time
	// tl is the session's timeline as read, which follow turns into
	// Position at At.
	tl islandTimeline
}

// islandTimeline is a session's timeline as Windows holds it: Position as
// of Updated (zero when the player leaves it out), which a player updates
// now and then, not as it plays. Reading it again gives the same values
// until the player sends new ones.
type islandTimeline struct {
	Position, Duration time.Duration
	Updated            time.Time
}

// islandMediaPausedFor is how long paused media keeps its place in the
// island: a tab left paused does not hold it for good.
const islandMediaPausedFor = time.Minute

// Shown tells whether the island shows the media at now.
func (m islandMedia) Shown(now time.Time) bool {
	if m.Title == "" {
		return false
	}
	return m.Playing || now.Sub(m.Since) < islandMediaPausedFor
}

// Progress is the share of the track played at now, or -1 if unknown.
func (m islandMedia) Progress(now time.Time) float64 {
	if m.Duration <= 0 {
		return -1
	}
	return float64(m.positionAt(now)) / float64(m.Duration)
}

// positionAt is the place in the track at now, as the island's clock
// keeps it: Position at At, advancing while playing.
func (m islandMedia) positionAt(now time.Time) time.Duration {
	p := m.Position
	if m.Playing {
		p += now.Sub(m.At)
	}
	return max(0, min(m.Duration, p))
}

// reported is the place in the track at m.At by the timeline just read:
// its Position, advanced by the time since the player sent it, when it
// says when.
func (m islandMedia) reported() time.Duration {
	p := m.tl.Position
	if u := m.tl.Updated; m.Playing && !u.IsZero() && u.Before(m.At) && m.At.Sub(u) < time.Hour {
		p += m.At.Sub(u)
	}
	return max(0, min(m.Duration, p))
}

// Thresholds of follow.
const (
	// islandMediaDrift is how far a new timeline may differ from the
	// island's clock before the bar moves to it (a seek): within it, the
	// clock keeps the bar smooth.
	islandMediaDrift = time.Second
	// islandMediaFresh is how near its start a timeline must be to be
	// the new track's, when the track changed but the timeline did not.
	islandMediaFresh = 5 * time.Second
)

// follow sets m's Position at At, m just read, from prev, the media read
// before it (ok: there was one). The island keeps its own clock and
// moves it only on what the player says anew: a timeline read again
// unchanged is old news, not where the track is now. Moving the bar back
// to it at every read made it run and jump back again and again, as
// after Next, when a browser keeps the last track's timeline a while.
func (m islandMedia) follow(prev islandMedia, ok bool) islandMedia {
	m.Duration = m.tl.Duration
	if m.Duration <= 0 {
		m.Position = 0
		return m
	}
	if !ok || prev.Duration <= 0 {
		m.Position = m.reported()
		return m
	}
	fresh := m.tl != prev.tl
	same := m.Title == prev.Title && m.Artist == prev.Artist && m.App == prev.App
	switch {
	case !same && fresh:
		// Next, Previous or the playlist: the new track's timeline.
		m.Position = m.reported()
	case !same:
		// A new track, its timeline yet to come: the old one, unless the
		// player already restarted it, is the last track's.
		m.Position = 0
		if r := m.reported(); r < islandMediaFresh {
			m.Position = r
		}
	case fresh && absDuration(m.reported()-prev.positionAt(m.At)) > islandMediaDrift:
		// A seek, or the clock gone astray: the player knows best.
		m.Position = m.reported()
	case m.Playing != prev.Playing:
		// Played or paused, the timeline yet to say where: from where
		// the clock is.
		m.Position = prev.positionAt(m.At)
	default:
		// Nothing new, or what agrees with the clock: it runs on.
		m.Position, m.At = prev.Position, prev.At
	}
	return m
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// islandMediaApp is the player's name from its app ID: "Spotify.exe",
// "SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify", "Chrome".
func islandMediaApp(aumid string) string {
	name := aumid
	if i := strings.LastIndexByte(name, '!'); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndexAny(name, `\/`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".EXE")
	switch strings.ToLower(name) {
	case "msedge":
		return "Edge"
	case "chrome":
		return "Chrome"
	case "firefox":
		return "Firefox"
	case "app", "":
		return ""
	}
	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

// islandMediaWatcher reads the system's current media session while the
// island is on screen, and sends it commands. Reading and commands run on
// one goroutine of its own (WinRT's, on Windows); changed runs when what
// it read changes.
type islandMediaWatcher struct {
	mu      sync.Mutex
	cur     islandMedia
	ok      bool
	active  bool
	wake    chan struct{}
	cmds    chan string
	changed func()
}

// Media commands.
const (
	mediaToggle   = "toggle"
	mediaNext     = "next"
	mediaPrevious = "previous"
)

func newIslandMediaWatcher(changed func()) *islandMediaWatcher {
	w := &islandMediaWatcher{wake: make(chan struct{}, 1), cmds: make(chan string, 4), changed: changed}
	if !islandMediaSupported() {
		return w
	}
	go w.run()
	return w
}

// Current is the media last read.
func (w *islandMediaWatcher) Current() (islandMedia, bool) {
	if w == nil {
		return islandMedia{}, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cur, w.ok
}

// SetActive starts and stops reading: only while the island shows, so a
// hidden island (fullscreen, off) asks Windows nothing.
func (w *islandMediaWatcher) SetActive(on bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	was := w.active
	w.active = on
	w.mu.Unlock()
	if on && !was {
		select {
		case w.wake <- struct{}{}:
		default:
		}
	}
}

// Command asks the player to play or pause, or skip.
func (w *islandMediaWatcher) Command(cmd string) {
	if w == nil || !islandMediaSupported() {
		return
	}
	select {
	case w.cmds <- cmd:
	default:
	}
}

// islandMediaPoll is how often the session is read while the island shows:
// cheap calls, and a track's change shows within it.
const islandMediaPoll = 1500 * time.Millisecond

func (w *islandMediaWatcher) run() {
	s := newIslandMediaSession()
	defer s.close()
	tick := time.NewTicker(islandMediaPoll)
	defer tick.Stop()
	for {
		w.mu.Lock()
		active := w.active
		w.mu.Unlock()
		// Hidden, the goroutine sleeps until the island shows again.
		var tc <-chan time.Time
		if active {
			tc = tick.C
		}
		select {
		case <-tc:
		case <-w.wake:
		case cmd := <-w.cmds:
			s.command(cmd)
			time.Sleep(150 * time.Millisecond) // the player answers, then read it
		}
		w.mu.Lock()
		active = w.active
		w.mu.Unlock()
		if !active {
			continue
		}
		m, ok := s.read()
		w.mu.Lock()
		changed := w.take(m, ok, time.Now())
		w.mu.Unlock()
		if changed && w.changed != nil {
			w.changed()
		}
	}
}

// take keeps m, just read at now, and tells whether the island shows
// something else: another track, played or paused, the bar moved.
func (w *islandMediaWatcher) take(m islandMedia, ok bool, now time.Time) bool {
	if ok {
		m = m.follow(w.cur, w.ok)
	}
	if ok && w.ok && m.Playing == w.cur.Playing {
		m.Since = w.cur.Since
	} else {
		m.Since = now
	}
	changed := ok != w.ok || m.Title != w.cur.Title || m.Artist != w.cur.Artist || m.Playing != w.cur.Playing || m.App != w.cur.App ||
		m.Duration != w.cur.Duration || m.Position != w.cur.Position || !m.At.Equal(w.cur.At)
	w.cur, w.ok = m, ok
	return changed
}
