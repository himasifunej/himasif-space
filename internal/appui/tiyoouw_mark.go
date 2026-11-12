package appui

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/egoist/mygo/ui"
)

// The Tiyoouw mark, the owner's logo (A / Open, "parallax fold"), ported
// from the logo kit's tiyoouw-mark.js v1.0.0 (enowX note 61d4cbe71100):
// the same geometry on its 240-unit grid, the "intro" entry motion and
// the hover="eyes" behaviour, step for step. The kit eases the cursor
// following by a fixed share a frame; here the share is per 1/60 s, so a
// faster display does not make the eyes quicker. Brand rules: monochrome,
// never redrawn, settled at once while the desktop asks for less motion.

// markPt is a point on the 240-unit grid.
type markPt [2]float64

// markPiece is a plane: its outline and its half of the T, a hole.
type markPiece struct{ o, h []markPt }

// markFrame is the mark at one moment: the left and the right plane.
type markFrame struct{ L, R markPiece }

// Geometry, as the kit's.
var (
	markK = math.Tan(math.Pi / 12) // 15°
)

const (
	markF, markW, markH, markYT                  = 120.0, 80.0, 132.0, 54.0
	markCBTop, markCBBot, markCBHalf, markStemHf = 40.0, 70.0, 48.0, 18.0
	markStem                                     = markH
)

func markRP(x, off float64) markPt { return markPt{x, markYT + off + markK*(x-markF)} }

// markPieces is the two planes at slant k; k = 0 is the mark unfolded flat.
func markPieces(k float64) markFrame {
	r := func(x, off float64) markPt { return markPt{x, markYT + off + k*(x-markF)} }
	l := func(x, off float64) markPt { return markPt{x, markYT - markW*k + off - k*(x-markF)} }
	a, b, c0, c1 := markF-markStemHf, markF+markStemHf, markF-markCBHalf, markF+markCBHalf
	chev := r(a, markH)
	return markFrame{
		L: markPiece{
			o: []markPt{l(markF-markW, 0), l(markF, 0), r(markF, markH), chev, {markF - markW, chev[1] + k*(a-(markF-markW))}},
			h: []markPt{r(c0, markCBTop), r(markF, markCBTop), r(markF, markStem), r(a, markStem), r(a, markCBBot), r(c0, markCBBot)},
		},
		R: markPiece{
			o: []markPt{r(markF, 0), r(markF+markW, 0), r(markF+markW, markH), r(markF, markH)},
			h: []markPt{r(markF, markCBTop), r(c1, markCBTop), r(c1, markCBBot), r(b, markCBBot), r(b, markStem), r(markF, markStem)},
		},
	}
}

// markFinal is the settled mark in two pieces; markPath, the same as one
// seamless outline (no seam at the fold), drawn when it rests.
var (
	markFinal = markPieces(markK)
	markPath  = func() []markPt {
		a, b, c0, c1 := markF-markStemHf, markF+markStemHf, markF-markCBHalf, markF+markCBHalf
		L, R := markFinal.L.o, markFinal.R.o
		return []markPt{L[0], L[1], R[0], R[1], R[2],
			markRP(b, markH), markRP(b, markCBBot), markRP(c1, markCBBot), markRP(c1, markCBTop), markRP(c0, markCBTop), markRP(c0, markCBBot),
			markRP(a, markCBBot), markRP(a, markH), L[4]}
	}()
)

// markEyeState is what the eyes do: where they look (-1 left, 1 right),
// how shut they are, how far they drop and spread.
type markEyeState struct{ look, blink, drop, spread float64 }

// The eyes are the logo's own planes pulled apart (80×132), leaning 8°
// inward-down with high pupils. Point order matches the T halves, so they
// morph one to one.
var (
	markEyeTilt = math.Tan(8 * math.Pi / 180)
	markEyeY0   = 54 - markEyeTilt*40
)

const markEyeW = 80.0

func markEyePt(side int, lu, lv float64, st markEyeState) markPt {
	x0, s, u := 20.0, -1.0, lu
	if side == 1 {
		x0, s, u = 140, 1, 1-lu
	}
	top := markEyeY0 + markEyeTilt*markEyeW*u
	v := .5 + (lv-.5)*(1-.96*st.blink)
	x := x0 + lu*markEyeW + 4*st.look + s*st.spread
	return markPt{x, top + v*markH + st.drop}
}

func markEyes(st markEyeState) markFrame {
	sh := .2 * st.look
	p0, p1, pm, q0, q1, qm := .32+sh, .68+sh, .5+sh, .2, .62, .41
	E := func(side int, u, v float64) markPt { return markEyePt(side, u, v, st) }
	return markFrame{
		L: markPiece{
			o: []markPt{E(0, 0, 0), E(0, 1, 0), E(0, 1, 1), E(0, .775, 1), E(0, 0, 1)},
			h: []markPt{E(0, p0, q0), E(0, p1, q0), E(0, p1, q1), E(0, pm, q1), E(0, p0, q1), E(0, p0, qm)},
		},
		R: markPiece{
			o: []markPt{E(1, 0, 0), E(1, 1, 0), E(1, 1, 1), E(1, 0, 1)},
			h: []markPt{E(1, p0, q0), E(1, p1, q0), E(1, p1, qm), E(1, p1, q1), E(1, pm, q1), E(1, p0, q1)},
		},
	}
}

// Easing and helpers, as the kit's.
func markLerp(a, b, k float64) float64 { return a + (b-a)*k }
func markClamp(x float64) float64      { return max(0, min(1, x)) }
func markSeg(t, a, b float64) float64  { return markClamp((t - a) / (b - a)) }
func markEaseOut(x float64) float64    { return 1 - math.Pow(1-x, 3) }
func markBackOut(x float64) float64 {
	const c = 1.4
	return 1 + (c+1)*math.Pow(x-1, 3) + c*math.Pow(x-1, 2)
}

// markLerpPts moves A toward B by kx across and ky down: across never
// overshoots (planes must not cross the fold), down may spring.
func markLerpPts(A, B []markPt, kx, ky float64) []markPt {
	out := make([]markPt, len(A))
	for i, p := range A {
		out[i] = markPt{markLerp(p[0], B[i][0], kx), markLerp(p[1], B[i][1], ky)}
	}
	return out
}

func markLerpPiece(A, B markPiece, kx, ky float64) markPiece {
	return markPiece{o: markLerpPts(A.o, B.o, kx, ky), h: markLerpPts(A.h, B.h, kx, ky)}
}

func markLerpFrame(A, B markFrame, kx, ky float64) markFrame {
	return markFrame{L: markLerpPiece(A.L, B.L, kx, ky), R: markLerpPiece(A.R, B.R, kx, ky)}
}

func markShift(P markPiece, dx, dy float64) markPiece {
	mv := func(ps []markPt) []markPt {
		out := make([]markPt, len(ps))
		for i, p := range ps {
			out[i] = markPt{p[0] + dx, p[1] + dy}
		}
		return out
	}
	return markPiece{o: mv(P.o), h: mv(P.h)}
}

// markIntroDur is the kit's "intro": two eyes pop open, glance left and
// right, blink, crouch, then spring together into the logo.
const markIntroDur = 3.45

func markDart(t, a float64) float64 { return markEaseOut(markSeg(t, a, a+.14)) }

func markLookAt(t float64) float64 {
	switch {
	case t < 1:
		return -markDart(t, .5)
	case t < 1.5:
		return -1 + 2*markDart(t, 1)
	}
	return 1 - markDart(t, 1.5)
}

// markIntro returns the intro's frame at t seconds and its opacity.
func markIntro(t float64) (markFrame, float64) {
	pop := 1 - markBackOut(markSeg(t, .05, .38))
	blink := 0.0
	if t > 1.72 && t < 1.9 {
		blink = math.Sin(math.Pi * markSeg(t, 1.72, 1.9))
	}
	crouch := markEaseOut(markSeg(t, 2.02, 2.28)) * (1 - markSeg(t, 2.28, 2.36))
	e := markEyes(markEyeState{look: markLookAt(t), blink: max(pop, blink) + .1*crouch, drop: 6.6 * crouch, spread: 6 * crouch})
	// Right plane lands first, left a beat later: the parallax step.
	sR, sL := markSeg(t, 2.3, 2.66), markSeg(t, 2.38, 2.76)
	return markFrame{
		L: markLerpPiece(e.L, markFinal.L, markEaseOut(sL), markBackOut(sL)),
		R: markLerpPiece(e.R, markFinal.R, markEaseOut(sR), markBackOut(sR)),
	}, markEaseOut(markSeg(t, 0, .15))
}

// markMode is what the mark is doing.
type markMode int

const (
	markIdle   markMode = iota
	markEntry           // the intro
	markSplit           // hovered: the planes part a little
	markToEyes          // turning into eyes
	markEyesOn          // eyes following the pointer, blinking
	markToLogo          // the pointer left: crouch, spring back
)

// tiyoouwMark is the state of one mark on screen.
type tiyoouwMark struct {
	mode       markMode
	t0         time.Time
	from, cur  markFrame
	hasCur     bool
	hover      bool
	tx, ty     float64 // where the pointer is, -1..1 over the mark
	px, py     float64 // the same, eased
	look       float64
	last       time.Time // the previous step
	blinkAt    time.Time
	nextBlink  time.Time
	introAsked bool
}

// play starts the intro from the beginning at now.
func (m *tiyoouwMark) play(now time.Time) {
	m.mode, m.t0 = markEntry, now
}

// set starts mode m at now, from the frame shown.
func (m *tiyoouwMark) set(mode markMode, now time.Time) {
	m.mode, m.t0 = mode, now
	m.from = markFinal
	if m.hasCur {
		m.from = m.cur
	}
}

// wake turns the mark into eyes, as a hover does.
func (m *tiyoouwMark) wake(now time.Time) {
	m.hover = true
	switch m.mode {
	case markIdle:
		m.set(markSplit, now)
	case markToLogo:
		m.set(markToEyes, now)
	}
}

// sleep springs it back into the logo.
func (m *tiyoouwMark) sleep(now time.Time) {
	m.hover, m.tx, m.ty = false, 0, 0
	switch m.mode {
	case markSplit, markToEyes, markEyesOn:
		m.set(markToLogo, now)
	}
}

// rest settles the mark.
func (m *tiyoouwMark) rest() {
	m.mode, m.cur, m.hasCur = markIdle, markFinal, true
}

// ease moves v toward target by share per 1/60 s over dt.
func markEase(v, target, share, dt float64) float64 {
	return v + (target-v)*(1-math.Pow(1-share, dt*60))
}

func (m *tiyoouwMark) eyesNow(now time.Time, dt float64) markFrame {
	m.look = markEase(m.look, max(-1, min(1, m.tx*1.4)), .14, dt)
	if m.nextBlink.IsZero() || now.After(m.nextBlink) {
		m.blinkAt = now
		m.nextBlink = now.Add(1800*time.Millisecond + time.Duration(rand.Float64()*1600)*time.Millisecond)
	}
	bt := float64(now.Sub(m.blinkAt)) / float64(170*time.Millisecond)
	blink := 0.0
	if bt >= 0 && bt < 1 {
		blink = math.Sin(math.Pi * bt)
	}
	return markEyes(markEyeState{look: m.look, blink: blink, drop: m.ty * 3})
}

// step advances the mark to now. It returns the frame to draw (whole
// false: the settled one-piece mark), its opacity, and whether it still
// moves.
func (m *tiyoouwMark) step(now time.Time) (f markFrame, alpha float64, whole, moving bool) {
	dt := 0.0
	if !m.last.IsZero() {
		dt = min(.1, now.Sub(m.last).Seconds())
	}
	m.last = now
	t := now.Sub(m.t0).Seconds()
	m.px = markEase(m.px, m.tx, .12, dt)
	m.py = markEase(m.py, m.ty, .12, dt)
	switch m.mode {
	case markEntry:
		if t >= markIntroDur {
			m.rest()
			if m.hover {
				m.set(markSplit, now)
				return markFinal, 1, true, true
			}
			return markFinal, 1, true, false
		}
		f, alpha = markIntro(max(0, t))
		m.cur, m.hasCur = f, true
		return f, alpha, false, true
	case markSplit:
		s := markEaseOut(markSeg(t, 0, .22))
		gap := 6 * s
		f = markFrame{L: markShift(markFinal.L, -gap-3*m.px, -2*m.py-3*s), R: markShift(markFinal.R, gap+3*m.px, 2*m.py+3*s)}
		m.cur, m.hasCur = f, true
		if t > .5 {
			m.nextBlink = now.Add(1100 * time.Millisecond)
			m.set(markToEyes, now)
		}
		return f, 1, false, true
	case markToEyes:
		k := markSeg(t, 0, .42)
		f = markLerpFrame(m.from, m.eyesNow(now, dt), markEaseOut(k), markBackOut(k))
		m.cur, m.hasCur = f, true
		if k >= 1 {
			m.mode = markEyesOn
		}
		return f, 1, false, true
	case markEyesOn:
		f = m.eyesNow(now, dt)
		m.cur, m.hasCur = f, true
		return f, 1, false, true
	case markToLogo:
		// Crouch for a beat, then spring back: right plane first, left a
		// beat later.
		c := markSeg(t, 0, .12)
		cr := markFrame{L: markShift(m.from.L, -3*c, 4*c), R: markShift(m.from.R, 3*c, 4*c)}
		kR, kL := markSeg(t, .12, .5), markSeg(t, .18, .58)
		f = markFrame{L: markLerpPiece(cr.L, markFinal.L, markEaseOut(kL), markBackOut(kL)), R: markLerpPiece(cr.R, markFinal.R, markEaseOut(kR), markBackOut(kR))}
		m.cur, m.hasCur = f, true
		if kL >= 1 {
			m.rest()
			return markFinal, 1, true, false
		}
		return f, 1, false, true
	}
	return markFinal, 1, true, false
}

// markPolyPath adds the closed polygon pts to path, mapped by at; reverse
// walks it the other way round.
func markPolyPath(path *ui.Path, pts []markPt, at func(markPt) (float32, float32), reverse bool) {
	for i := range pts {
		p := pts[i]
		if reverse {
			p = pts[len(pts)-1-i]
		}
		x, y := at(p)
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Close()
}

// markArea is twice the signed area of a polygon.
func markArea(pts []markPt) float64 {
	s := 0.0
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		s += p[0]*q[1] - q[0]*p[1]
	}
	return s
}

// markPiecePath is a plane with its hole: MyGo fills by non-zero winding,
// so the hole runs against the outline (the kit's SVG is even-odd).
func markPiecePath(path *ui.Path, P markPiece, at func(markPt) (float32, float32)) {
	markPolyPath(path, P.o, at, false)
	markPolyPath(path, P.h, at, (markArea(P.o) > 0) == (markArea(P.h) > 0))
}

// drawMark paints frame f (or the settled mark, when whole) into r, whose
// box is the kit's viewBox 16 16 208 208, in color c.
func drawMark(p *ui.Painter, r ui.Rect, f markFrame, whole bool, c ui.Color) {
	s := float64(min(r.W, r.H)) / 208
	ox := float64(r.X) + (float64(r.W)-208*s)/2
	oy := float64(r.Y) + (float64(r.H)-208*s)/2
	at := func(q markPt) (float32, float32) {
		return float32(ox + (q[0]-16)*s), float32(oy + (q[1]-16)*s)
	}
	path := new(ui.Path)
	if whole {
		markPolyPath(path, markPath, at, false)
	} else {
		markPiecePath(path, f.L, at)
		markPiecePath(path, f.R, at)
	}
	p.FillPath(path, c)
}

// markView draws the mark size DIPs square, in the theme's ink. It plays
// the intro once when first shown, again on a click, and wakes into eyes
// that follow the pointer while hovered. While the desktop asks for less
// motion it stays settled.
func markView(c *ui.Context, m *tiyoouwMark, size float32, label string) ui.Element {
	t := c.Theme()
	still := c.Preferences().ReduceMotion
	now := c.Now()
	b := ui.ButtonBase(c).Label(label).Size(size, size).Radius(12).Cursor(ui.CursorPointer)
	b.FocusRing(b.FocusVisible())
	if still {
		if m.mode != markIdle {
			m.rest()
		}
	} else {
		if !m.introAsked {
			m.introAsked = true
			m.play(now)
		}
		if b.Clicked() {
			m.play(now)
		}
		x, y, over := b.PointerPosition()
		if over {
			bw, bh := b.Bounds().W, b.Bounds().H
			if bw > 0 && bh > 0 {
				m.tx, m.ty = float64(x/bw-.5)*2, float64(y/bh-.5)*2
			}
		}
		switch {
		case over && !m.hover:
			if m.mode == markEntry {
				m.hover = true
			} else {
				m.wake(now)
			}
		case !over && m.hover:
			m.sleep(now)
		}
	}
	ink := t.Text
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		if still {
			drawMark(p, r, markFinal, true, ink)
			return
		}
		f, alpha, whole, moving := m.step(p.Now())
		drawMark(p, r, f, whole, ink.Alpha(float32(alpha)))
		if moving {
			p.AnimationFrame()
		}
	})
	return b
}
