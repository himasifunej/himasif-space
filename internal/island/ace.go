//go:build !js

package island

import (
	"math"
	"time"
)

// Ace is the island (direction A, "Pil Mata", canvas dde22eb04942): a
// black pill with two capsule eyes. At rest the eyes sit in the middle;
// with an activity they slide to the leading edge and are its icon.

// Scene is what the island shows.
type Scene uint8

const (
	SceneIdle Scene = iota
	SceneNotice
	SceneTimer
	SceneMedia
)

// Geometry is the pill's width, height and radius, in DIP, as in the
// canvas: idle 120×32, ping 360×56, media 300×40, timer 240×40, expanded
// 440×168 (a notice's details, with its body and actions, 440×184).
func Geometry(s Scene, expanded bool) [3]float64 {
	switch {
	case expanded && s == SceneNotice:
		return [3]float64{440, 184, 32}
	case expanded:
		return [3]float64{440, 168, 32}
	case s == SceneNotice:
		return [3]float64{360, 56, 28}
	case s == SceneMedia:
		return [3]float64{300, 40, 20}
	case s == SceneTimer:
		return [3]float64{240, 40, 20}
	}
	return [3]float64{120, 32, 16}
}

// Eyes' pair: two capsules 9×14, 28 wide together.
const (
	EyeWidth  = 9
	EyeHeight = 14
	EyePair   = 28
	// BubbleSize is the second activity's circle beside the pill, 8 apart.
	BubbleSize = 36
	BubbleGap  = 8
)

// EyesAt is the centre of the eyes' pair in a pill of width w and height
// h: the middle at rest, the leading slot with an activity, and up in the
// corner of the details.
func EyesAt(s Scene, expanded bool, w, h float64) (x, y float64) {
	switch {
	case expanded:
		return 28 + EyePair/2, 40
	case s == SceneIdle:
		return w / 2, h / 2
	case s == SceneNotice:
		return 24 + EyePair/2, h / 2
	}
	return 20 + EyePair/2, h / 2
}

// ContentLeft is where an activity's text starts, right of the eyes.
func ContentLeft(s Scene, expanded bool) float64 {
	if expanded || s == SceneNotice {
		return 64
	}
	return 60
}

// Breath is the eyes' scale as Ace breathes: 1 to 1.04 and back over 4 s.
func Breath(t time.Duration) float64 {
	return 1 + .02*(1-math.Cos(2*math.Pi*t.Seconds()/4))
}

// Bop is the eyes' nod to music, ±1.5 DIP every 0.9 s.
func Bop(t time.Duration) float64 {
	return 1.5 * math.Sin(2*math.Pi*t.Seconds()/.9)
}

// Wave is the height, 0.3 to 1, of bar i of the five of the music's
// waveform, each a little apart, as the canvas's .wave.
func Wave(t time.Duration, i int) float64 {
	offsets := [5]float64{0, .18, .42, .09, .30}
	x := math.Mod(t.Seconds()+offsets[i%5], 1.8) / .9 // alternate: up, then down
	if x > 1 {
		x = 2 - x
	}
	return .3 + .7*Ease(x)
}

// Gaze turns the eyes toward the pointer, up to Max DIP, nearer the
// pointer less, and back to the middle once it rests for 3 s. The motion
// eases out over about 0.2 s, as the canvas's transition.
type Gaze struct {
	X, Y       float64
	tx, ty     float64
	px, py     float64
	movedAt    time.Time
	last       time.Time
	hasPointer bool
}

const (
	GazeMax  = 3
	gazeRest = 3 * time.Second
	gazeTau  = .07
)

// Pointer gives where the pointer is, relative to the eyes' centre.
func (g *Gaze) Pointer(now time.Time, dx, dy float64) {
	if !g.hasPointer || math.Abs(dx-g.px) > .5 || math.Abs(dy-g.py) > .5 {
		g.movedAt = now
	}
	g.px, g.py, g.hasPointer = dx, dy, true
	g.tx, g.ty = 0, 0
	if now.Sub(g.movedAt) < gazeRest {
		if d := math.Hypot(dx, dy); d > 0 {
			k := math.Min(1, d/240)
			g.tx, g.ty = dx/d*GazeMax*k, dy/d*GazeMax*k
		}
	}
}

// GazeReach is how near the pointer must be for the eyes to follow it:
// farther, the user works elsewhere, and Ace does not redraw at every
// move of the mouse.
const GazeReach = 600

// Away tells that the pointer is out of reach: the eyes go back to the
// middle.
func (g *Gaze) Away() {
	g.tx, g.ty, g.hasPointer = 0, 0, false
}

// Following tells whether the eyes follow the pointer now.
func (g *Gaze) Following(now time.Time) bool {
	return g.hasPointer && now.Sub(g.movedAt) < gazeRest
}

// RestsAt is when the eyes go back to the middle, if the pointer stays.
func (g *Gaze) RestsAt() time.Time { return g.movedAt.Add(gazeRest) }

// Step moves the eyes toward where they look and tells whether they still
// move.
func (g *Gaze) Step(now time.Time) bool {
	dt := 0.
	if !g.last.IsZero() {
		dt = math.Min(now.Sub(g.last).Seconds(), .1)
	}
	g.last = now
	k := 1 - math.Exp(-dt/gazeTau)
	g.X += (g.tx - g.X) * k
	g.Y += (g.ty - g.Y) * k
	if math.Abs(g.tx-g.X) < .02 && math.Abs(g.ty-g.Y) < .02 {
		g.X, g.Y = g.tx, g.ty
		return false
	}
	return true
}

// Expander opens an activity's details as a notice's: after 300 ms under
// the pointer or at a click, and closes them once the pointer has left for
// 600 ms, or at a second click.
type Expander struct {
	Expanded               bool
	hoverSince, leaveSince time.Time
	held                   bool // closed by a click: not again until the pointer leaves
}

func (e *Expander) Tick(now time.Time, hover, click bool) {
	if click {
		e.Expanded = !e.Expanded
		e.hoverSince, e.leaveSince = time.Time{}, time.Time{}
		e.held = !e.Expanded
		return
	}
	if hover {
		e.leaveSince = time.Time{}
		if e.held {
			return
		}
		if e.hoverSince.IsZero() {
			e.hoverSince = now
		}
		if now.Sub(e.hoverSince) >= 300*time.Millisecond {
			e.Expanded = true
		}
		return
	}
	e.hoverSince, e.held = time.Time{}, false
	if e.Expanded {
		if e.leaveSince.IsZero() {
			e.leaveSince = now
		}
		if now.Sub(e.leaveSince) >= 600*time.Millisecond {
			e.Expanded = false
			e.leaveSince = time.Time{}
		}
	}
}

// Reset closes the details, as the scene changes.
func (e *Expander) Reset() { *e = Expander{} }

// BreathAwake is how long Ace keeps breathing after the last sign of
// someone about: the pointer near, a hover, a click, a notice, an
// activity. Then the breath ends at its next rest and Ace sits still but
// for its blinks and glances, which ask for a few frames every few
// seconds instead of 12 a second (idle CPU, pilot 0.4.5).
const BreathAwake = 12 * time.Second

// breathCycle is Breath's period.
const breathCycle = 4 * time.Second

// Breather keeps Ace's breath going while it is awake. Its zero value is
// asleep.
type Breather struct {
	start, awakeUntil time.Time
	on                bool
}

// Wake keeps Ace breathing for BreathAwake from now; asleep, a breath
// starts from its rest, so nothing jumps.
func (b *Breather) Wake(now time.Time) {
	b.awakeUntil = now.Add(BreathAwake)
	if !b.on {
		b.on, b.start = true, now
	}
}

// Scale is the eyes' scale now, and whether Ace still breathes and so
// wants frames. Past awakeUntil, the breath under way ends at its rest
// (scale 1), where it stops.
func (b *Breather) Scale(now time.Time) (float64, bool) {
	if !b.on {
		return 1, false
	}
	t := now.Sub(b.start)
	if now.After(b.awakeUntil) {
		awake := b.awakeUntil.Sub(b.start)
		stop := (awake/breathCycle + 1) * breathCycle
		if t >= stop {
			b.on = false
			return 1, false
		}
	}
	return Breath(t), true
}
