package appui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The opening animation, option C of canvas 9b55464a544c (the owner's
// pick, "Ace jadi garis progres"): Ace appears in the middle, blinks, then
// stretches into the progress track with its eyes riding the tip; once
// full it shrinks and settles under the Space logo, which rises in with
// its name, and the credit comes last. 4.8 s.
//
// It shows at launch, also the launch after an update (its line says so),
// and never holds back an app that is ready: the app shows as soon as it
// is ready and the animation has run its natural length. While the app is
// still loading after that, Ace runs the track again, a loop, until it is.
// While the desktop asks for less motion, it is the last frame, still,
// and only for as long as the app is loading.

// splashDur is the animation's natural length; splashLoop, one run of the
// track while the app is still loading after it.
const (
	splashDur  = 4800 * time.Millisecond
	splashLoop = 2800 * time.Millisecond
)

// splashState is the animation's: when it began, and whether it is over.
type splashState struct {
	started bool
	at      time.Time
	done    bool
	update  bool // the launch after Space updated itself
}

// splashing reports whether the window shows the opening animation now,
// and draws it then.
func (a *App) splashing(c *ui.Context) bool {
	s := &a.splash
	if !a.Splash || s.done {
		return false
	}
	now := c.Now()
	if !s.started {
		s.started, s.at, s.update = true, now, a.WhatsNew != nil
	}
	still := c.Preferences().ReduceMotion
	elapsed := now.Sub(s.at)
	if a.screen != screenStarting && (still || elapsed >= splashDur) {
		s.done = true
		return false
	}
	t := splashDur
	if !still {
		t = elapsed
		c.AnimationFrame()
	}
	a.splashView(c, splashAt(t))
	return true
}

// splashFrame is where everything of the animation is at one moment.
type splashFrame struct {
	groupY      float32 // Ace and its line, from their place under the logo
	ace         float32 // Ace's opacity
	width       float32 // Ace's, 120 as a pill, 320 as the track
	fill        float32 // the progress inside it
	fillOpacity float32
	eyesX       float32 // the eyes' left, inside Ace
	eyesY       float32 // their height, 1 open, 0.1 shut
	status      float32 // the line under Ace
	logo        float32
	word, tag   float32 // the name and the tagline, rising in
	credit      float32
}

func splashClamp(x float64) float64 { return max(0, min(1, x)) }

// splashP is how far t (ms) is through the part starting at a lasting d.
func splashP(t, a, d float64) float64 { return splashClamp((t - a) / d) }

func splashOut(x float64) float64 { return 1 - math.Pow(1-x, 3) }

func splashInOut(x float64) float64 {
	if x < .5 {
		return 4 * x * x * x
	}
	return 1 - math.Pow(-2*x+2, 3)/2
}

func splashLerp(a, b, x float64) float64 { return a + (b-a)*x }

// splashBlink is an eye's height through a blink starting at a (ms):
// shut in 80 ms, open in 80.
func splashBlink(t, a float64) (float64, bool) {
	if t <= a || t >= a+160 {
		return 1, false
	}
	if t < a+80 {
		return splashLerp(1, .1, splashInOut(splashP(t, a, 80))), true
	}
	return splashLerp(.1, 1, splashInOut(splashP(t, a+80, 80))), true
}

// splashTrack is Ace as the track: stretched from 120 to 320 from ms
// grow, the progress running from fillFrom for fillFor, shrunk back from
// shrink.
func splashTrack(t, grow, fillFrom, fillFor, shrink float64) (w, fw, fillOpacity, ex float64) {
	w = splashLerp(120, 320, splashInOut(splashP(t, grow, 500)))
	w = splashLerp(w, 120, splashInOut(splashP(t, shrink, 400)))
	fw = w * splashP(t, fillFrom, fillFor)
	fillOpacity = 1 - splashOut(splashP(t, shrink, 300))
	centre, ride := (w-28)/2, max(14, fw-28-10)
	ex = splashLerp(centre, ride, splashInOut(splashP(t, grow, 500)))
	ex = splashLerp(ex, centre, splashInOut(splashP(t, shrink, 400)))
	return w, fw, fillOpacity, ex
}

// splashAt is the frame at elapsed, past splashDur the loop.
func splashAt(elapsed time.Duration) splashFrame {
	t := float64(elapsed) / float64(time.Millisecond)
	f := splashFrame{}
	if elapsed >= splashDur {
		// The loop: everything in place, Ace runs the track again and
		// blinks.
		u := float64((elapsed-splashDur)%splashLoop) / float64(time.Millisecond)
		w, fw, fo, ex := splashTrack(u, 0, 500, 1400, 2000)
		sy, _ := splashBlink(u, 2500)
		return splashFrame{ace: 1, width: float32(w), fill: float32(fw), fillOpacity: float32(fo), eyesX: float32(ex), eyesY: float32(sy),
			status: 1, logo: 1, word: 1, tag: 1, credit: 1}
	}
	// Ace starts in the middle of the window, then settles under the logo.
	f.groupY = float32(splashLerp(-56, 0, splashInOut(splashP(t, 3700, 600))))
	f.ace = float32(splashOut(splashP(t, 0, 400)))
	w, fw, fo, ex := splashTrack(t, 1100, 1600, 2000, 3600)
	f.width, f.fill, f.fillOpacity, f.eyesX = float32(w), float32(fw), float32(fo), float32(ex)
	sy := splashOut(splashP(t, 500, 250))
	if b, on := splashBlink(t, 900); on {
		sy = b
	}
	if b, on := splashBlink(t, 4500); on {
		sy = b
	}
	f.eyesY = float32(sy)
	f.status = float32(splashOut(splashP(t, 600, 400)))
	f.logo = float32(splashOut(splashP(t, 3900, 500)))
	f.word = float32(splashOut(splashP(t, 4000, 500)))
	f.tag = float32(splashOut(splashP(t, 4080, 500)))
	f.credit = float32(splashOut(splashP(t, 4200, 600)))
	return f
}

// rise fades e in from 12 DIPs below.
func rise(e ui.Element, x float32) ui.Element {
	return e.Opacity(x).Translate(0, 12*(1-x))
}

// splashView draws frame f.
func (a *App) splashView(c *ui.Context, f splashFrame) {
	t := c.Theme()
	ui.Column(c.Key("splash")).Fill().AlignItems(ui.Center).Background(t.Background).Label("Space sedang dibuka").Children(func() {
		ui.Column(c.Key("stage")).Grow(1).FillWidth().MinHeight(0).Center().Children(func() {
			ui.Column(c.Key("brand")).AlignItems(ui.Center).Margin(0, 0, 48, 0).Children(func() {
				ui.Image(c, spaceBrand()).Size(64, 64).Fit(ui.Contain).Opacity(f.logo).Scale(.94+.06*f.logo, .94+.06*f.logo)
				rise(ui.Text(c, "Space").FontSize(28).FontWeight(600).Margin(20, 0, 0, 0), f.word)
				rise(ui.Text(c, "Ruang Kerja HIMASIF").FontSize(13).TextColor(t.TextMuted).Margin(4, 0, 0, 0), f.tag)
			})
			ui.Column(c.Key("ace-group")).AlignItems(ui.Center).Gap(16).Translate(0, f.groupY).Children(func() {
				ui.Row(c.Key("ace-wrap")).Size(320, 32).Justify(ui.Center).Children(func() {
					ui.Box(c.Key("ace")).Size(f.width, 32).Radius(16).Background(ui.Hex("#000000")).Border(1, ui.Hex("#2b2821")).Clip().Opacity(f.ace).Label("Ace").Children(func() {
						ui.Box(c.Key("fill")).Absolute().Left(0).Top(0).Size(f.fill, 32).Radius(16).Background(ui.RGBA(215, 195, 160, .2*f.fillOpacity))
						ui.Row(c.Key("eyes")).Absolute().Left(f.eyesX).Top(9).Gap(10).Children(func() {
							for i := range 2 {
								ui.Box(c.Key(i)).Size(9, 14).Radius(5).Background(ui.Hex("#ffffff")).Scale(1, f.eyesY)
							}
						})
					})
				})
				ui.Row(c.Key("status")).Gap(0).Opacity(f.status).Children(func() {
					if a.splash.update {
						version := a.Version
						if a.WhatsNew != nil && a.WhatsNew.Version != "" {
							version = a.WhatsNew.Version
						}
						muted(c, "Memperbarui ke ")
						muted(c, version).Font(FontMono)
						muted(c, " · memulai ulang")
					} else {
						muted(c, "Menyambungkan…")
					}
				})
			})
		})
		ui.Text(c, "by HIMASIF ASTRA 25/26").FontSize(12).TextColor(faint(t)).Margin(0, 0, 32, 0).Opacity(f.credit)
	})
}
