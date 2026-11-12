//go:build !js

package main

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
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

func TestIslandMediaShownProgressAndApp(t *testing.T) {
	now := time.Now()
	m := islandMedia{Title: "Lagu", Playing: true, Position: time.Minute, Duration: 4 * time.Minute, At: now.Add(-time.Minute), Since: now.Add(-time.Hour)}
	if !m.Shown(now) || m.Progress(now) != .5 {
		t.Fatalf("playing: shown %v progress %v", m.Shown(now), m.Progress(now))
	}
	m.Playing = false
	if m.Shown(now) || m.Progress(now) != .25 {
		t.Fatal("media paused an hour ago still holds the island")
	}
	m.Since = now.Add(-10 * time.Second)
	if !m.Shown(now) {
		t.Fatal("just paused media left the island")
	}
	if (islandMedia{Playing: true}).Shown(now) {
		t.Fatal("media without a title shown")
	}
	if (islandMedia{Title: "x"}).Progress(now) != -1 {
		t.Fatal("a stream has a progress")
	}
	for in, want := range map[string]string{
		"Spotify.exe": "Spotify",
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify": "Spotify",
		"chrome":          "Chrome",
		`C:\x\msedge.exe`: "Edge",
		"":                "",
	} {
		if got := islandMediaApp(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// The window is never resized: it holds every shape, with the spring's
// overshoot past the largest, and the shape sits in it as far along as
// the island is along the screen.
func TestIslandEnvelopeHoldsEveryShape(t *testing.T) {
	scenes := []island.Scene{island.SceneIdle, island.SceneNotice, island.SceneTimer, island.SceneMedia}
	for _, s := range scenes {
		for _, open := range []bool{false, true} {
			g := island.Geometry(s, open)
			bubbles := []float64{0, island.BubbleSize}
			if open {
				bubbles = bubbles[:1] // the details take the island: no circle
			}
			for _, bubble := range bubbles {
				shape := islandShape(g, bubble)
				if w := islandGroupWidth(shape); w > float64(islandEnvelope[0]) || g[1] > float64(islandEnvelope[1]) {
					t.Errorf("scene %v open %v bubble %v: %v×%v outside the window", s, open, bubble, w, g[1])
				}
			}
			// The spring overshoots by a few percent (damping 19, stiffness 170).
			peak, v := g[0]/2, 0.
			for range 200 {
				peak, v = island.Spring(peak, v, g[0], .016)
				if peak > float64(islandEnvelope[0]) {
					t.Fatalf("scene %v open %v: the spring reaches %v", s, open, peak)
				}
			}
		}
	}
	if w := islandGroupWidth([4]float64{240, 40, 20, 36}); w != 284 {
		t.Fatalf("with circle %v", w)
	}
	idle := islandShape(island.Geometry(island.SceneIdle, false), 0)
	if x := islandOffset(idle, 0, 0); x != 0 {
		t.Errorf("at the left edge the pill starts at %v", x)
	}
	if x := islandOffset(idle, 0, 1); x+islandGroupWidth(idle) != float64(islandEnvelope[0]) {
		t.Errorf("at the right edge the pill ends at %v", x+islandGroupWidth(idle))
	}
	if x := islandOffset(idle, 0, .5); x != (float64(islandEnvelope[0])-120)/2 {
		t.Errorf("in the middle the pill starts at %v", x)
	}
	// With a "+N", Ace grows 28 DIP to the right, its eyes where they
	// were; at the right edge it grows left instead, inside the window.
	badged := idle
	badged[0] += island.BadgeRoom
	for _, anchor := range []float64{0, .25, .5, .75} {
		if x, was := islandOffset(badged, island.BadgeRoom, anchor), islandOffset(idle, 0, anchor); x != was {
			t.Errorf("anchor %v: with +N the pill starts at %v, not %v", anchor, x, was)
		}
	}
	if x := islandOffset(badged, island.BadgeRoom, 1); x+islandGroupWidth(badged) != float64(islandEnvelope[0]) {
		t.Errorf("at the right edge the wider pill ends at %v", x+islandGroupWidth(badged))
	}
	for in, want := range map[float64]float64{-1: 0, 2: 1, .25: .25, math.NaN(): .5} {
		if got := islandAnchor(in); got != want {
			t.Errorf("anchor %v: %v, want %v", in, got, want)
		}
	}
}

// Eight "Rapat baru" in one Space within seconds gather into one notice;
// another kind, another Space, or a later burst stays apart.
func TestIslandGathersABurst(t *testing.T) {
	space := id.New()
	p := &islandPresenter{d: &desktopDevice{}}
	now := time.Now()
	meeting := func(title string, sp id.ID) appui.Notice {
		d := api.Delivery{SourceKind: "ping", RelatedKind: "meeting", Title: "Rapat baru: " + title, SpaceID: &sp}
		n := appui.Notice{Kind: appui.DeliveryKind(d), Title: d.Title}
		n.Group, n.GroupLabel = appui.DeliveryGroup(d)
		n.Space = sp
		return n
	}
	push := func(n appui.Notice, at time.Time) {
		item, ok := p.gather(n, nil, at)
		if !ok {
			n.Tag = "group:" + n.Group + "#" + at.String()
			item = island.Item{Tag: n.Tag, Value: islandNotice{n: n, count: 1, first: at, latest: n.Title}}
		}
		p.m.Push(item, at)
	}
	for i := range 8 {
		push(meeting(fmt.Sprint("Debat ", i), space), now.Add(time.Duration(i)*300*time.Millisecond))
	}
	if p.m.Current == nil || len(p.m.Queue) != 0 {
		t.Fatalf("not one notice: current %v, %d queued", p.m.Current, len(p.m.Queue))
	}
	x := p.m.Current.Value.(islandNotice)
	if x.count != 8 || p.noticeTitle(x) != "8 rapat baru" || noticeBody(x) != "Terbaru: Rapat baru: Debat 7" || x.n.Kind != "meeting" {
		t.Fatalf("gathered %+v: %q / %q", x, p.noticeTitle(x), noticeBody(x))
	}
	push(meeting("Lain", id.New()), now.Add(3*time.Second))
	push(meeting("Nanti", space), now.Add(30*time.Second))
	if len(p.m.Queue) != 2 {
		t.Fatalf("other Space or later burst gathered: %d queued", len(p.m.Queue))
	}
	if islandKindLabel(x.n.Kind) != "Rapat" {
		t.Fatal("a meeting's notice reads as", islandKindLabel(x.n.Kind))
	}
}

// Styles: those who never chose follow Space's theme; Custom keeps its
// text readable on light and dark colors; a bad color falls back to black.
func TestIslandStyles(t *testing.T) {
	prefs, _ := loadDevicePreferences([]byte(`{"islandEnabled":true,"islandStyle":"C"}`))
	if prefs.IslandTheme != islandThemeFollow {
		t.Fatalf("upgrade keeps %q", prefs.IslandTheme)
	}
	light := &ui.Theme{Surface: ui.RGB(250, 248, 244), Text: ui.RGB(30, 28, 25), Dark: false}
	if p := islandPaletteFor(prefs, light); p.ink != light.Surface || p.text != light.Text || p.dark {
		t.Fatalf("follow %+v", p)
	}
	sand := islandPaletteFor(devicePreferences{IslandTheme: islandThemeCustom, IslandColor: "#e8dcc4", IslandOpacity: .7}, nil)
	if sand.text.R > 100 || sand.ink.A > 190 || sand.ink.A < 170 {
		t.Fatalf("light custom %+v", sand)
	}
	night := islandPaletteFor(devicePreferences{IslandTheme: islandThemeCustom, IslandColor: "#14213d"}, nil)
	if night.text.R < 200 || night.ink.A != 255 {
		t.Fatalf("dark custom %+v", night)
	}
	if bad := islandPaletteFor(devicePreferences{IslandTheme: islandThemeCustom, IslandColor: "biru"}, nil); bad.ink.R != 0 || bad.ink.G != 0 {
		t.Fatalf("bad color %+v", bad)
	}
	if g := islandPaletteFor(devicePreferences{IslandTheme: islandThemeGlass}, nil); g.ink.A == 255 || !g.dark {
		t.Fatalf("glass %+v", g)
	}
	if islandOpacity(.1) != .4 || islandOpacity(0) != 1 {
		t.Fatal("opacity bounds")
	}
}
