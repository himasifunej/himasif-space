//go:build !js

package island

import (
	"math"
	"time"
)

// Ace ikut capek (canvas 4033a40b94aa, option B "Seluruh badan", with the
// owner's changes): as unread notices pile up, Ace at rest goes from fresh
// through tired to grumpy, with its whole body: it sags and flattens, its
// lids drop, its ink warms toward brown and the "+N" grows bolder; a drop
// of sweat, a "zZ", a huff. Cleared, it shakes it off and is fresh again.
// Cute, never a reproach. Everything stays inside the pill, which is all
// the island's window shows.

// BadgeRoom is how much wider Ace at rest grows with a "+N" (120 to 148
// DIP), so the badge sits clear of the eyes, which stay where they were.
const BadgeRoom = 28

// Tired stages: Segar (no unread), Ada kabar, Mulai berat, Ngantuk,
// Cemberut.
const (
	StageFresh = iota
	StageNews
	StageHeavy
	StageSleepy
	StageGrumpy
)

// StageNames are the stages' names, as the canvas and the settings say.
var StageNames = [5]string{"Segar", "Ada kabar", "Mulai berat", "Ngantuk", "Cemberut"}

// Tired steps' limits: "Cemberut mulai di" goes from TopMin to TopMax
// (owner: "maksimal 15").
const (
	TopMin     = 5
	TopMax     = 15
	TopDefault = 15
)

// Ratios of "Jarak antar tahap"; RatioCustom is "Atur sendiri".
const (
	RatioCustom  = "custom"
	RatioDefault = "2.5"
)

// Ratios are the choices of "Jarak antar tahap", in order.
var Ratios = []string{"1.5", "2", "2.5", RatioCustom}

// Thresholds are where Ada kabar, Mulai berat, Ngantuk and Cemberut begin
// (1, heavy, sleepy, top). With a ratio, each stage begins ratio times
// the one before, counted back from top; custom takes heavy and sleepy as
// given. Either way 1 < heavy < sleepy < top.
func Thresholds(ratio string, top, heavy, sleepy int) [4]int {
	top = max(TopMin, min(TopMax, top))
	if ratio != RatioCustom {
		r := ratioValue(ratio)
		sleepy = int(math.Round(float64(top) / r))
		heavy = int(math.Round(float64(top) / r / r))
	}
	heavy = max(2, min(top-2, heavy))
	sleepy = max(heavy+1, min(top-1, sleepy))
	return [4]int{1, heavy, sleepy, top}
}

func ratioValue(ratio string) float64 {
	switch ratio {
	case "1.5":
		return 1.5
	case "2":
		return 2
	}
	return 2.5
}

// StageOf is the stage of n unread notices under th.
func StageOf(n int, th [4]int) int {
	switch {
	case n <= 0:
		return StageFresh
	case n < th[1]:
		return StageNews
	case n < th[2]:
		return StageHeavy
	case n < th[3]:
		return StageSleepy
	}
	return StageGrumpy
}

// Pose is Ace's body and face at a moment, relative to Ace at rest: the
// zero Pose (with SX, SY and Pop 1) changes nothing.
type Pose struct {
	// The eyes: closed (0 open, 1 shut), the lids' drop (part of the
	// eye's height), their slant in degrees (grumpy), how far down and
	// sideways they look, in DIP.
	Close, Lid, Angle, EyeY, GlanceX float64
	// The body: moved by X, Y DIP and scaled SX, SY about its bottom
	// middle; Pop scales the eyes (the recovery's pop).
	X, Y, SX, SY, Pop float64
	// Warm shifts the ink toward brown (0 to 1); Heavy weighs the "+N"
	// (bigger, bolder, warmer); Out takes it away as the inbox clears.
	Warm, Heavy, Out float64
	// Particles, each its progress 0 to 1, or -1 when there is none.
	Puff, Sweat, Zz float64
}

// RestPose is Ace at rest, unchanged.
func RestPose() Pose {
	return Pose{SX: 1, SY: 1, Pop: 1, Puff: -1, Sweat: -1, Zz: -1}
}

// TiredCycle is the loop of a stage's little motions.
const TiredCycle = 6 * time.Second

// TiredBlend is how long a stage takes to become the next.
const TiredBlend = 450 * time.Millisecond

// TiredRecovery is the shake after the inbox is cleared.
const TiredRecovery = 1400 * time.Millisecond

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// easeInOut is the canvas's cubic in-out.
func easeInOut(x float64) float64 {
	x = clamp01(x)
	if x < .5 {
		return 4 * x * x * x
	}
	return 1 - math.Pow(-2*x+2, 3)/2
}

// easeOut is the canvas's cubic out.
func easeOut(x float64) float64 {
	x = clamp01(x)
	return 1 - math.Pow(1-x, 3)
}

// blinkAt is a blink of d ms starting at ms at, 0 to 1 and back.
func blinkAt(t, at, d float64) float64 {
	x := (t - at) / d
	if x < 0 || x > 1 {
		return 0
	}
	return math.Sin(math.Pi * x)
}

// window rises over r ms after a and falls over r ms before b.
func window(t, a, b, r float64) float64 {
	if t < a || t > b {
		return 0
	}
	return math.Min(easeOut((t-a)/r), easeOut((b-t)/r))
}

// progress is where t is between a and b, 0 to 1, or -1 outside.
func progress(t, a, b float64) float64 {
	if t < a || t > b {
		return -1
	}
	return (t - a) / (b - a)
}

// TiredPose is stage's pose at t into its cycle; still, its pose without
// the cycle's motions (reduced motion, or Ace asleep).
func TiredPose(stage int, t time.Duration, still bool) Pose {
	f := RestPose()
	ms := float64(t%TiredCycle) / float64(time.Millisecond)
	if still {
		ms = -1 // before every motion of the cycle
	}
	switch stage {
	case StageNews:
		// Glances at the number now and then.
		f.GlanceX = 2.5 * window(ms, 2800, 4600, 280)
	case StageHeavy:
		f.Lid, f.EyeY = .28, 1
		f.Close = math.Max(blinkAt(ms, 1500, 280), blinkAt(ms, 4400, 280)) // slower blinks
		f.Y, f.SY, f.Warm, f.Heavy = 1.5, .97, .3, .3
		f.Sweat = progress(ms, 2000, 3800)
	case StageSleepy:
		nod := 0.
		switch {
		case ms >= 2000 && ms < 2600:
			nod = easeInOut((ms - 2000) / 600)
		case ms >= 2600 && ms < 3300:
			nod = 1
		case ms >= 3300 && ms < 3900:
			nod = 1 - easeInOut((ms-3300)/600)
		}
		sigh := window(ms, 4300, 5500, 500)
		f.Lid, f.EyeY = .5+.25*nod, 1.2+.5*nod
		f.GlanceX = 1 // leaning toward the number
		f.Y, f.SX, f.SY = 3+nod, 1.04+.02*sigh, .9-.04*sigh
		f.Warm, f.Heavy = .6, .65
		f.Zz = progress(ms, 1800, 4600)
	case StageGrumpy:
		huff := window(ms, 2400, 3100, 180)
		f.Lid, f.Angle, f.EyeY = .4+.15*huff, 20, 1.5
		f.Close = math.Max(blinkAt(ms, 4800, 120), blinkAt(ms, 4960, 120))
		f.GlanceX = 1
		f.Puff = progress(ms, 2550, 3550)
		f.Y, f.SX, f.SY = 2-1.5*huff, 1.04+.05*huff, .94+.06*huff
		f.Warm, f.Heavy = 1, 1
	}
	return f
}

// TiredActive tells whether stage moves at t into its cycle: a blink, a
// nod, a sigh, a huff or a particle. Between them Ace holds still and
// needs no frames.
func TiredActive(stage int, t time.Duration) bool {
	ms := float64(t%TiredCycle) / float64(time.Millisecond)
	in := func(a, b float64) bool { return ms >= a && ms <= b }
	switch stage {
	case StageNews:
		return in(2800, 4600)
	case StageHeavy:
		return in(1500, 3800) || in(4400, 4680)
	case StageSleepy:
		return in(1800, 5500)
	case StageGrumpy:
		return in(2400, 3550) || in(4800, 5080)
	}
	return false
}

// BlendPose is a between a and b at k (0 a, 1 b); particles switch half
// way.
func BlendPose(a, b Pose, k float64) Pose {
	l := func(x, y float64) float64 { return x + (y-x)*k }
	pick := func(x, y float64) float64 {
		if k < .5 {
			return x
		}
		return y
	}
	return Pose{
		Close: l(a.Close, b.Close), Lid: l(a.Lid, b.Lid), Angle: l(a.Angle, b.Angle), EyeY: l(a.EyeY, b.EyeY), GlanceX: l(a.GlanceX, b.GlanceX),
		X: l(a.X, b.X), Y: l(a.Y, b.Y), SX: l(a.SX, b.SX), SY: l(a.SY, b.SY), Pop: l(a.Pop, b.Pop),
		Warm: l(a.Warm, b.Warm), Heavy: l(a.Heavy, b.Heavy), Out: l(a.Out, b.Out),
		Puff: pick(a.Puff, b.Puff), Sweat: pick(a.Sweat, b.Sweat), Zz: pick(a.Zz, b.Zz),
	}
}

// RecoverPose is Ace shaking off from (its pose as the inbox cleared), p
// 0 to 1 through TiredRecovery: the number pops away, Ace stretches with
// its eyes squeezed shut, shakes three times and pops its eyes open, fresh.
func RecoverPose(from Pose, p float64) Pose {
	p = clamp01(p)
	ms := p * float64(TiredRecovery/time.Millisecond)
	k := easeInOut(p / .55)
	f := BlendPose(from, RestPose(), k)
	f.Puff, f.Sweat, f.Zz = -1, -1, -1
	stretch := window(ms, 150, 800, 250)
	shake := 0.
	if q := (p - .55) / .3; p > .55 && p < .85 {
		shake = math.Sin(q*math.Pi*3) * (1 - q)
	}
	pop := window(ms, 800, 1300, 150)
	f.Close = .85 * stretch
	f.SX *= 1 + .12*stretch
	f.SY *= 1 - .1*stretch
	f.Y -= 2 * stretch
	f.X = 2.5 * shake
	f.Pop = 1 + .12*pop
	f.Out = easeOut(p / .18)
	return f
}

// Tired follows the unread count of Ace at rest and gives its pose: a new
// stage blends in over TiredBlend, and an inbox cleared from a tired
// stage plays the recovery. Its zero value is fresh.
type Tired struct {
	stage, prev int
	changed     time.Time
	// seen is the stage of the last unread count, wherever Ace was: the
	// inbox cleared in the details or the bell while Ace was not at rest
	// plays the recovery once Ace is back.
	seen int
	// The recovery under way: from the pose then, with n unread.
	recovering bool
	recStart   time.Time
	recFrom    Pose
	recN       int
	// clock runs the stage's cycle: only while Ace is awake, and to the
	// end of a motion under way.
	clock time.Duration
	last  time.Time
	pose  Pose
	n     int
}

// Frame is Ace's pose at now with n unread, th the thresholds; rest
// tells whether Ace is at rest (no notice, no details), awake whether it
// breathes (someone is about). Off, or not at rest, Ace is as ever.
// Reduced motion holds each stage still and skips the blends and the
// recovery. It returns the pose and the "+N" to draw: n, or the count
// just cleared while its badge leaves.
func (t *Tired) Frame(now time.Time, n int, th [4]int, on, rest, awake, reduced bool) (Pose, int) {
	dt := time.Duration(0)
	if !t.last.IsZero() {
		dt = min(now.Sub(t.last), 50*time.Millisecond)
	}
	t.last = now
	switch {
	case !on:
		t.seen = StageFresh
	case n > 0:
		t.seen = StageOf(n, th)
	}
	if !rest || !on {
		// The details or a notice take the island; Ace comes back as it
		// is, blending in.
		t.stage, t.prev, t.changed, t.recovering = StageFresh, StageFresh, time.Time{}, false
		t.pose, t.n = RestPose(), n
		return t.pose, n
	}
	if n > 0 {
		t.recovering = false
	} else if t.seen > StageFresh {
		if !reduced {
			t.recovering, t.recStart, t.recFrom, t.recN = true, now, t.pose, t.n
			if t.stage == StageFresh {
				// Cleared away from rest: the shake still plays.
				t.recFrom, t.recN = TiredPose(t.seen, 0, true), 0
			}
		}
		t.seen = StageFresh
	}
	if target := StageOf(n, th); target != t.stage {
		t.prev, t.stage, t.changed = t.stage, target, now
	}
	if t.recovering {
		p := float64(now.Sub(t.recStart)) / float64(TiredRecovery)
		if p < 1 {
			t.pose, t.n = RecoverPose(t.recFrom, p), 0
			return t.pose, t.recN
		}
		t.recovering = false
		t.changed = time.Time{}
	}
	if !reduced && (awake || TiredActive(t.stage, t.clock)) {
		t.clock = (t.clock + dt) % TiredCycle
	}
	still := reduced || (!awake && !TiredActive(t.stage, t.clock))
	pose := TiredPose(t.stage, t.clock, still)
	if k := t.blendAt(now); k < 1 && !reduced {
		pose = BlendPose(TiredPose(t.prev, t.clock, still), pose, easeInOut(k))
	}
	t.pose, t.n = pose, n
	return pose, n
}

// blendAt is how far the change of stage has come, 0 to 1.
func (t *Tired) blendAt(now time.Time) float64 {
	if t.changed.IsZero() {
		return 1
	}
	return clamp01(float64(now.Sub(t.changed)) / float64(TiredBlend))
}

// Stage is the stage Ace shows (or is going to).
func (t *Tired) Stage() int { return t.stage }

// Next is when Ace next needs a frame: soon while it blends, recovers or
// moves, else at the next motion of the cycle while awake; 0 when it can
// wait for something else.
func (t *Tired) Next(now time.Time, awake, reduced bool) time.Duration {
	if reduced {
		return 0
	}
	if t.recovering || t.blendAt(now) < 1 {
		return 16 * time.Millisecond
	}
	if t.stage == StageFresh {
		return 0
	}
	if TiredActive(t.stage, t.clock) {
		return 33 * time.Millisecond
	}
	if !awake {
		return 0
	}
	// The cycle's clock moves only with frames; while awake the breath
	// already asks for some, and the next motion starts within one.
	return 83 * time.Millisecond
}
