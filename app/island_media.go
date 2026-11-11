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
	p := m.Position
	if m.Playing {
		p += now.Sub(m.At)
	}
	return max(0, min(1, float64(p)/float64(m.Duration)))
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
		if ok && w.ok && m.Playing == w.cur.Playing {
			m.Since = w.cur.Since
		} else {
			m.Since = time.Now()
		}
		changed := ok != w.ok || m.Title != w.cur.Title || m.Artist != w.cur.Artist || m.Playing != w.cur.Playing || m.App != w.cur.App
		w.cur, w.ok = m, ok
		w.mu.Unlock()
		if changed && w.changed != nil {
			w.changed()
		}
	}
}
