//go:build !js

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/island"
)

// restFrame is one frame of the presenter's shape at rest or open: what
// view does before it draws, without a window.
func (p *islandPresenter) restFrame(now time.Time, inbox int, expanded, reduced bool, prefs devicePreferences) {
	scene := island.SceneIdle
	rest := !expanded
	geometry, room := island.Geometry(scene, expanded), 0.
	if rest && inbox > 0 {
		geometry[0] += island.BadgeRoom
		room = island.BadgeRoom
	}
	pose, _ := p.tired.Frame(now, inbox, prefs.aceTiredThresholds(), prefs.AceTired, rest, true, reduced)
	p.pose = pose
	if target := islandShape(geometry, 0); target != p.target || room != p.roomTarget || !p.shown {
		p.start, p.target, p.changed = p.geometry, target, now
		p.roomFrom, p.roomTarget = p.room, room
		if !p.shown {
			p.geometry, p.velocity, p.room = target, [4]float64{}, room
		}
		p.last = now
	}
	p.step(now, reduced)
	p.shown = true
	p.last = now
}

// The "+N" coming and going while Ace is hovered open and closed, tired
// or not, at each place along the top: every frame the region (shape)
// stays inside the window, its left edge never jumps, and Ace's eyes stay
// put on screen as the badge widens the pill (rc builds glitched while
// the island resized).
func TestIslandBadgeResizeStaysInTheWindow(t *testing.T) {
	prefs, _ := loadDevicePreferences(nil)
	for _, reduced := range []bool{false, true} {
		for _, anchor := range []float64{0, .5, 1} {
			p := &islandPresenter{anchor: anchor}
			p.forget()
			now := time.Unix(1000, 0)
			// inbox, expanded, for how long
			script := []struct {
				inbox    int
				expanded bool
				ms       int
			}{
				{0, false, 300}, {1, false, 120}, {4, false, 600}, {4, true, 150}, {12, true, 400},
				{12, false, 90}, {16, false, 900}, {16, true, 60}, {0, true, 200}, {0, false, 1800},
				{3, false, 80}, {0, false, 60}, {7, false, 1600},
			}
			prevLeft, prevEyes := math.NaN(), math.NaN()
			var opened time.Time
			for _, step := range script {
				for ms := 0; ms < step.ms; ms += 16 {
					now = now.Add(16 * time.Millisecond)
					p.restFrame(now, step.inbox, step.expanded, reduced, prefs)
					if step.expanded {
						opened = now
					}
					g, ox, oy := p.shape()
					where := fmt.Sprintf("reduced %v anchor %v inbox %d open %v", reduced, anchor, step.inbox, step.expanded)
					if ox < 0 || oy < 0 || ox+islandGroupWidth(g) > float64(islandEnvelope[0])+.01 || oy+g[1] > float64(islandEnvelope[1])+.01 {
						t.Fatalf("%s: shape %v at %v,%v leaves the window", where, g, ox, oy)
					}
					if g[0] < 100 || g[1] < 20 { // the spring overshoots a little as the details close
						t.Fatalf("%s: shape collapsed to %v", where, g)
					}
					left := islandOffset(p.geometry, p.room, anchor)
					if !math.IsNaN(prevLeft) && anchor < 1 && math.Abs(left-prevLeft) > 60 {
						t.Fatalf("%s: the pill's left edge jumped %v → %v", where, prevLeft, left)
					}
					prevLeft = left
					if now.Sub(opened) > 800*time.Millisecond && anchor < 1 {
						// At rest, the details long closed, the eyes stay
						// where Ace at rest had them as the "+N" comes
						// and goes.
						eyes := left + (p.geometry[0]-p.room)/2
						if !math.IsNaN(prevEyes) && math.Abs(eyes-prevEyes) > .5 {
							t.Fatalf("%s: the eyes jumped %v → %v", where, prevEyes, eyes)
						}
						prevEyes = eyes
					} else {
						prevEyes = math.NaN()
					}
				}
			}
			// Settled at rest with 7 unread: 148 wide, eyes where they were.
			if p.geometry[0] != 148 || p.room != island.BadgeRoom {
				t.Fatalf("anchor %v: settled at %v (room %v)", anchor, p.geometry, p.room)
			}
			if anchor < 1 {
				if got, want := islandOffset(p.geometry, p.room, anchor)+60, islandOffset(islandFirst, 0, anchor)+60; got != want {
					t.Errorf("anchor %v: eyes at %v with +N, %v without", anchor, got, want)
				}
			}
		}
	}
}

// Ace ikut capek off: the "+N" still widens Ace, but Ace stays fresh.
func TestIslandTiredOffStillWidens(t *testing.T) {
	prefs, _ := loadDevicePreferences(nil)
	prefs.AceTired = false
	p := &islandPresenter{anchor: .5}
	p.forget()
	now := time.Unix(1000, 0)
	for range 120 {
		now = now.Add(16 * time.Millisecond)
		p.restFrame(now, 20, false, false, prefs)
	}
	if p.geometry[0] != 148 || p.pose != island.RestPose() {
		t.Fatalf("off: %v, %+v", p.geometry, p.pose)
	}
}

func TestDevicePreferencesAceTired(t *testing.T) {
	p, _ := loadDevicePreferences(nil)
	if !p.AceTired || p.aceTiredThresholds() != [4]int{1, 2, 6, 15} {
		t.Fatalf("defaults: %+v", p)
	}
	// Older files (no aceTired) keep it on; out of range numbers come back.
	p, _ = loadDevicePreferences([]byte(`{"islandEnabled":true}`))
	if !p.AceTired {
		t.Fatal("an older file turned Ace ikut capek off")
	}
	p, _ = loadDevicePreferences([]byte(`{"aceTired":false,"aceTiredRatio":"9","aceTiredTop":40,"aceTiredHeavy":30,"aceTiredSleepy":2}`))
	if p.AceTired || p.AceTiredRatio != island.RatioDefault || p.AceTiredTop != 15 || p.AceTiredHeavy != 13 || p.AceTiredSleepy != 14 {
		t.Fatalf("normalized: %+v", p)
	}
	p.AceTiredRatio, p.AceTiredTop = island.RatioCustom, 9
	if got := p.aceTiredThresholds(); got != [4]int{1, 7, 8, 9} {
		t.Fatalf("custom under a lower top: %v", got)
	}
	if aceTiredRange([4]int{1, 2, 6, 15}, island.StageNews) != "1" || aceTiredRange([4]int{1, 2, 6, 15}, island.StageHeavy) != "2–5" || aceTiredRange([4]int{1, 2, 6, 15}, island.StageGrumpy) != "15+" {
		t.Fatal("ranges")
	}
}

var shotBackground = ui.RGB(0x15, 0x14, 0x11)

// aceShotTheme is Space's dark theme, as the canvas.
func aceShotTheme(c *ui.Context) {
	th := *c.Theme()
	th.Dark = true
	th.Background, th.Surface, th.SurfaceHover, th.SurfacePressed = ui.RGB(0x1b, 0x1a, 0x16), ui.RGB(0x1d, 0x1c, 0x17), ui.RGB(0x2b, 0x28, 0x21), ui.RGB(0x36, 0x32, 0x29)
	th.Border, th.Text, th.TextMuted = ui.RGB(0x36, 0x32, 0x29), ui.RGB(0xe9, 0xe5, 0xdc), ui.RGB(0xaa, 0xa2, 0x94)
	th.Accent, th.AccentText = ui.RGB(0xd7, 0xc3, 0xa0), ui.RGB(0x24, 0x25, 0x1f)
	c.SetTheme(&th)
}

// TestAceTiredScreenshots writes Ace at each stage (black and Pasir), the
// recovery's frames, a day of notices piling up and cleared as a GIF, and
// Pengaturan › Ace, to SPACE_SHOTS.
func TestAceTiredScreenshots(t *testing.T) {
	dir := os.Getenv("SPACE_SHOTS")
	if dir == "" {
		t.Skip("SPACE_SHOTS not set")
	}
	defer islandPaletteFor(devicePreferences{IslandTheme: islandThemeBlack}, nil).use()
	prefs, _ := loadDevicePreferences(nil)
	th := prefs.aceTiredThresholds()
	pals := []struct {
		name string
		pal  islandPalette
	}{
		{"hitam", islandPaletteFor(devicePreferences{IslandTheme: islandThemeBlack}, nil)},
		{"pasir", islandPaletteFor(devicePreferences{IslandTheme: islandThemeCustom, IslandColor: "#e8dcc4"}, nil)},
	}
	// Each stage still, and at a moment of its motions (sweat, zZ, huff).
	moments := [5]time.Duration{0, 3500 * time.Millisecond, 2900 * time.Millisecond, 3000 * time.Millisecond, 2950 * time.Millisecond}
	for _, pl := range pals {
		for s := range 5 {
			n := 0
			if s > 0 {
				n = th[s-1]
			}
			if s == island.StageSleepy {
				n = 12
			}
			for _, moving := range []bool{false, true} {
				pose := island.TiredPose(s, moments[s], !moving)
				tt := ui.NewTester(func(c *ui.Context) {
					aceShotTheme(c)
					ui.Column(c).Fill().Background(shotBackground).Center().Gap(8).Children(func() {
						aceStill(c, pl.pal, pose, n, 1)
						ui.Text(c, fmt.Sprintf("%s · +%d", island.StageNames[s], n)).FontSize(11).TextColor(ui.RGB(0xaa, 0xa2, 0x94))
					})
				}, 200, 80)
				tt.SetScale(3)
				tt.Frame()
				name := fmt.Sprintf("ace-capek-%s-%d", pl.name, s)
				if moving {
					name += "-gerak"
				}
				writePNG(t, filepath.Join(dir, name+".png"), tt.Image())
			}
		}
	}
	// One strip of all stages, black, for the review.
	tt := ui.NewTester(func(c *ui.Context) {
		aceShotTheme(c)
		ui.Column(c).Fill().Background(shotBackground).Padding(16).Gap(16).Children(func() {
			for _, pl := range pals {
				ui.Row(c.Key(pl.name)).FillWidth().Justify(ui.SpaceAround).Children(func() {
					for s := range 5 {
						n := 0
						if s > 0 {
							n = th[s-1]
						}
						ui.Column(c.Key(s)).Gap(6).AlignItems(ui.Center).Children(func() {
							aceStill(c, pl.pal, island.TiredPose(s, moments[s], false), n, 1)
							ui.Text(c, island.StageNames[s]+" · +"+aceTiredRange(th, s)).FontSize(11).TextColor(ui.RGB(0xaa, 0xa2, 0x94))
						})
					}
				})
			}
		})
	}, 900, 150)
	tt.SetScale(2)
	tt.Frame()
	writePNG(t, filepath.Join(dir, "ace-capek-tahap.png"), tt.Image())

	// The day: notices pile up 0 → 16, then the inbox is cleared; a frame
	// every 100 ms through island.Tired, as the island runs it.
	var tr island.Tired
	t0 := time.Unix(1000, 0)
	story := []struct {
		at time.Duration
		n  int
	}{{0, 0}, {1200 * time.Millisecond, 1}, {3000 * time.Millisecond, 4}, {6500 * time.Millisecond, 9}, {11000 * time.Millisecond, 16}, {16500 * time.Millisecond, 0}}
	var shots []image.Image
	var delays []int
	var recovery []image.Image
	room := 0.
	for at := time.Duration(0); at < 19*time.Second; at += 100 * time.Millisecond {
		n := 0
		for _, s := range story {
			if at >= s.at {
				n = s.n
			}
		}
		pose, badge := tr.Frame(t0.Add(at), n, th, true, true, true, false)
		target := 0.
		if n > 0 {
			target = island.BadgeRoom
		}
		room += (target - room) * .35
		tt := ui.NewTester(func(c *ui.Context) {
			aceShotTheme(c)
			ui.Column(c).Fill().Background(shotBackground).Center().Gap(6).Children(func() {
				aceStillRoom(c, pals[0].pal, pose, badge, room, 1)
				label := island.StageNames[tr.Stage()]
				if n == 0 && pose != island.RestPose() {
					label = "Bersih-bersih…"
				}
				ui.Text(c, fmt.Sprintf("%s · %d belum dibaca", label, n)).FontSize(11).TextColor(ui.RGB(0xaa, 0xa2, 0x94))
			})
		}, 220, 70)
		tt.SetScale(1.5)
		tt.Frame()
		img := tt.Image()
		shots = append(shots, cloneImage(img))
		delays = append(delays, 10)
		if at >= 16500*time.Millisecond && at < 18000*time.Millisecond && len(recovery) < 10 && int(at/(100*time.Millisecond))%2 == 0 {
			recovery = append(recovery, cloneImage(img))
		}
	}
	// One palette for the whole GIF, of the colors the frames use most,
	// so the warm browns stay brown.
	pal := gifPalette(shots)
	var frames []*image.Paletted
	for _, m := range shots {
		pm := image.NewPaletted(m.Bounds(), pal)
		draw.Draw(pm, m.Bounds(), m, image.Point{}, draw.Src)
		frames = append(frames, pm)
	}
	f, err := os.Create(filepath.Join(dir, "ace-capek-hari.gif"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, &gif.GIF{Image: frames, Delay: delays}); err != nil {
		t.Fatal(err)
	}
	f.Close()
	writePNG(t, filepath.Join(dir, "ace-capek-pulih.png"), joinImages(recovery, false))

	// Pengaturan › Ace, with 2,5× and with Atur sendiri.
	d := &desktopDevice{prefs: prefs, root: t.TempDir()}
	d.island = &islandPresenter{d: d}
	settings := ui.NewTester(func(c *ui.Context) {
		aceShotTheme(c)
		ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
			appui.SettingsLayout(c, "Notifikasi dan Dynamic Island", "Laptop ini · Space untuk Windows.", []appui.SettingsNavItem{{Title: "Dynamic Island", Icon: "spark", Status: d.prefs.islandSummary()}, {Title: "Ace", Icon: "eye", Status: d.prefs.aceSummary()}}, nil, func() { d.aceSettings(c) })
		})
	}, 1440, 640)
	settings.Frame()
	writePNG(t, filepath.Join(dir, "pengaturan-ace.png"), settings.Image())
	if err := settings.Click("Atur sendiri"); err != nil {
		t.Fatal(err)
	}
	settings.Advance(300 * time.Millisecond)
	settings.Frame()
	if d.prefs.AceTiredRatio != island.RatioCustom {
		t.Fatalf("Atur sendiri not chosen: %q", d.prefs.AceTiredRatio)
	}
	writePNG(t, filepath.Join(dir, "pengaturan-ace-atur-sendiri.png"), settings.Image())
}

// gifPalette is the 256 colors the images use most.
func gifPalette(ms []image.Image) color.Palette {
	count := map[color.RGBA]int{}
	for _, m := range ms {
		b := m.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := m.At(x, y).RGBA()
				count[color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}]++
			}
		}
	}
	cs := make([]color.RGBA, 0, len(count))
	for c := range count {
		cs = append(cs, c)
	}
	slices.SortFunc(cs, func(a, b color.RGBA) int { return count[b] - count[a] })
	pal := make(color.Palette, 0, 256)
	for _, c := range cs[:min(256, len(cs))] {
		pal = append(pal, c)
	}
	return pal
}
