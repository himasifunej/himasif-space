//go:build !js

package main

import (
	"errors"
	"math"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

var errIslandUnavailable = errors.New("dynamic island unavailable; use OS toast")

// islandTopGap is the island's distance from the top of the work area, in
// DIP, as in the approved canvas.
const islandTopGap = 6

// islandIdleDestroy is how long an empty island keeps its window, when it
// does not stay on screen.
const islandIdleDestroy = 10 * time.Minute

// islandObstructionGrace is how long fullscreen must last before the island
// steps aside: a screenshot overlay or a splash screen comes and goes.
const islandObstructionGrace = 1500 * time.Millisecond

// islandFirst is the shape a window starts from, before its first frame:
// Ace at rest.
var islandFirst = islandShape(island.Geometry(island.SceneIdle, false), 0)

// islandShape is the pill (width, height, radius) and the second
// activity's circle beside it (0: none), in DIP.
func islandShape(g [3]float64, bubble float64) [4]float64 {
	return [4]float64{g[0], g[1], g[2], bubble}
}

// islandWindowSize is the window around a shape: the pill, and the circle
// 8 DIP right of it.
func islandWindowSize(g [4]float64) (int, int) {
	w, h := g[0], g[1]
	if g[3] > .5 {
		w += island.BubbleGap + g[3]
		h = math.Max(h, g[3])
	}
	return int(math.Round(w)), int(math.Round(h))
}

// The island's colors (direction A): a black pill, white eyes and text,
// the system's accents for each activity.
var (
	islandInk     = ui.RGB(0, 0, 0)
	islandText    = ui.RGB(245, 245, 247)
	islandMuted   = ui.RGBA(245, 245, 247, .72)
	islandRing    = ui.RGBA(255, 255, 255, .06)
	islandButton  = ui.RGB(44, 44, 46)
	islandPing    = ui.RGB(255, 159, 10)
	islandMeeting = ui.RGB(48, 209, 88)
	islandMusic   = ui.RGB(10, 132, 255)
	islandOver    = ui.RGB(255, 69, 58)
)

type islandNotice struct {
	n        appui.Notice
	fallback func()
}

// All machine access is serialized. Native window and animation fields belong
// only to MyGo's UI thread; delivery workers only enqueue and request Update.
type islandPresenter struct {
	// Only the isolated spike supplies an override, to measure rendering even
	// when this Windows session currently has Focus Assist or fullscreen on.
	environmentCheck func() bool
	// spikeLive stands in for the app's timer and Windows' media in the
	// spike, which has neither.
	spikeLive func(time.Time) (appui.IslandTimer, bool, islandMedia, bool)
	spikeOpen bool          // the spike holds the details open
	idleAfter time.Duration // 0: islandIdleDestroy; the spike shortens it
	d         *desktopDevice
	mu        sync.Mutex
	m         island.Machine
	win       *mygo.Window
	// The shape (islandShape) springs toward target, the eyes' centre
	// toward eyesTarget, from start when it changed.
	geometry, velocity, start, target [4]float64
	eyes, eyesVelocity                [2]float64
	last, changed, contentChanged     time.Time
	idleSince, obstructedSince        time.Time
	lastWidth, lastHeight             int
	region                            [4]float64 // the shape the window's region was cut to
	// key names what the island shows (scene, details, which notice): a
	// new one fades the content in.
	key      string
	scene    island.Scene
	expanded bool
	shown    bool // the window shows a frame of this presenter's
	watching bool // a goroutine waits for fullscreen to end
	swap     bool // the second activity was clicked: it leads
	// Ace's life: blinks and glances, the gaze after the pointer, breath.
	blink       island.Eyes
	gaze        island.Gaze
	expander    island.Expander
	breath      island.Breather
	born        time.Time
	happyUntil  time.Time
	media       *islandMediaWatcher
	mediaActive bool
}

func (p *islandPresenter) environmentAllowed() bool {
	if p.environmentCheck != nil {
		return p.environmentCheck()
	}
	return islandEnvironmentAllowed()
}

// obstructed tells whether a fullscreen app, a game or a presentation
// covers the screen: the island hides until it ends, keeping what it shows.
// It is looser than environmentAllowed, which decides whether a new notice
// may interrupt at all.
func (p *islandPresenter) obstructed() bool {
	if p.environmentCheck != nil {
		return !p.environmentCheck()
	}
	return islandObstructed()
}

type islandNotifier struct {
	appui.Notifier
	p *islandPresenter
}

func (n islandNotifier) Present(x appui.Notice) error {
	prefs := n.p.d.notificationPrefs.Load().(devicePreferences)
	return (appui.MultiNotifier{Base: n.Notifier, Extra: []appui.NoticePresenter{n.p}, ReplaceBase: prefs.IslandReplaceToast}).Present(x)
}
func (n islandNotifier) DeniedHint() string {
	if h, ok := n.Notifier.(appui.NotifierHint); ok {
		return h.DeniedHint()
	}
	return "Periksa izin notifikasi Windows."
}

func (p *islandPresenter) Present(n appui.Notice) error {
	return p.offer(n, func() { p.reportFallback(showNoticeNotification(n.Title, n.Body, "space-"+n.Tag)) })
}

func (p *islandPresenter) reportFallback(err error) {
	if err != nil {
		p.d.win.Update(func() { p.d.app.ShowNotice("Notifikasi Windows belum muncul: " + err.Error()) })
	}
}

func (p *islandPresenter) offer(n appui.Notice, fallback func()) error {
	prefs := p.d.notificationPrefs.Load().(devicePreferences)
	if !islandSupported() || !prefs.IslandEnabled || !p.environmentAllowed() {
		return errIslandUnavailable
	}
	// Hold account lock through enqueue so logout/reset cannot race a delivery.
	p.d.mu.Lock()
	if n.AccountID != "" && p.d.activeUser.String() != n.AccountID {
		p.d.mu.Unlock()
		return errIslandUnavailable
	}
	p.mu.Lock()
	if !prefs.IslandReplaceToast {
		fallback = nil
	}
	if fallback != nil && n.AccountID != "" {
		send := fallback
		fallback = func() {
			p.d.deliveryMu.Lock()
			defer p.d.deliveryMu.Unlock()
			p.d.mu.Lock()
			active := p.d.activeUser.String() == n.AccountID
			p.d.mu.Unlock()
			if active {
				send()
			}
		}
	}
	priority := 0
	switch n.Kind {
	case "ping", "meeting":
		priority = 3
	case "deadline":
		priority = 2
	case appui.NoticeDelivery, "task":
		priority = 1
	}
	ok := p.m.Push(island.Item{Tag: n.Tag, Priority: priority, Value: islandNotice{n, fallback}}, time.Now())
	p.mu.Unlock()
	p.d.mu.Unlock()
	if !ok {
		return errIslandUnavailable
	}
	p.d.win.Update(func() {
		p.mu.Lock()
		hasCurrent := p.m.Current != nil
		p.mu.Unlock()
		if !hasCurrent {
			return
		}
		if !p.d.notificationPrefs.Load().(devicePreferences).IslandEnabled {
			p.reset(true)
			return
		}
		p.ensureWindow()
		if p.watching {
			return // shown when fullscreen ends.
		}
		if !p.win.IsVisible() {
			// The banner grows out of Ace at rest.
			p.forget()
			p.show()
			p.shown = true
		}
		p.win.Update(func() {})
	})
	return nil
}

func (p *islandPresenter) ensureWindow() {
	if p.media == nil {
		p.media = newIslandMediaWatcher(func() {
			p.d.win.Update(func() {
				if p.win != nil && p.win.IsVisible() {
					p.win.Update(func() {})
				}
			})
		})
	}
	if p.win != nil {
		return
	}
	p.forget()
	w, h := islandWindowSize(p.geometry)
	p.win = mygo.NewWindow(mygo.WindowOptions{Title: "Dynamic Island · Space", Width: w, Height: h, Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, DisableResize: true, Panel: true, Content: ui.View(p.view)})
	islandPrepareWindow(p.win.NativeHandle())
	islandClearBackdrop(p.win.NativeHandle())
	p.win.OnClose(func(e *mygo.CloseEvent) {
		if !p.d.quitting {
			e.PreventDefault()
			p.reset(false)
		}
	})
}

// forget puts the shape back to Ace at rest, for a window about to show.
func (p *islandPresenter) forget() {
	p.geometry, p.velocity, p.target = islandFirst, [4]float64{}, islandFirst
	p.eyes = [2]float64{islandFirst[0] / 2, islandFirst[1] / 2}
	p.eyesVelocity = [2]float64{}
	p.key, p.scene, p.expanded, p.shown = "", island.SceneIdle, false, false
	p.contentChanged, p.last = time.Time{}, time.Time{}
	p.lastWidth, p.lastHeight = 0, 0
	p.expander.Reset()
}

// show puts the hidden window on screen, at its shape's size.
func (p *islandPresenter) show() {
	if p.win.IsVisible() {
		return
	}
	w, h := islandWindowSize(p.geometry)
	p.win.SetSize(w, h)
	p.lastWidth, p.lastHeight = w, h
	p.position(w, h)
	islandWindowShape(p.win.NativeHandle(), p.geometry, w, h)
	p.region = p.geometry
	p.win.ShowInactive()
}

// reset discards account content, or sends accepted content to the OS when
// settings make the island unavailable while it is visible.
func (p *islandPresenter) reset(fallback bool) {
	p.mu.Lock()
	items := append([]island.Item(nil), p.m.Queue...)
	if p.m.Current != nil {
		items = append(items, *p.m.Current)
	}
	p.m = island.Machine{}
	p.mu.Unlock()
	if p.win != nil {
		p.win.Hide()
	}
	p.forget()
	p.setMediaActive(false)
	if fallback {
		for _, it := range items {
			if f := it.Value.(islandNotice).fallback; f != nil {
				go f()
			}
		}
	}
	p.scheduleIdleDestroy()
}

func (p *islandPresenter) scheduleIdleDestroy() {
	if !p.idleSince.IsZero() {
		return
	}
	p.idleSince = time.Now()
	stamp := p.idleSince
	idle := p.idleAfter
	if idle == 0 {
		idle = islandIdleDestroy
	}
	time.AfterFunc(idle, func() {
		p.d.win.Update(func() {
			p.mu.Lock()
			empty := p.m.Current == nil
			p.mu.Unlock()
			if p.idleSince != stamp {
				return
			}
			p.idleSince = time.Time{}
			prefs := p.d.notificationPrefs.Load().(devicePreferences)
			if empty && p.win != nil && !p.win.IsVisible() && !(prefs.IslandEnabled && prefs.IslandAlwaysOn) {
				p.win.Destroy()
				p.win = nil
				// The renderer's buffers are garbage now; return them to
				// Windows instead of waiting for the scavenger.
				debug.FreeOSMemory()
			}
		})
	})
}

func (p *islandPresenter) position(w, h int) {
	if islandPosition(p.win.NativeHandle(), p.d.win.NativeHandle(), !p.d.win.IsVisible(), w, h) {
		return
	}
	display := mygo.Screen.DisplayMatching(p.d.win.Bounds())
	if !p.d.win.IsVisible() {
		display = mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	}
	a := display.WorkArea
	if a.Width == 0 {
		a = mygo.Screen.PrimaryDisplay().WorkArea
	}
	p.win.SetPosition(a.X+(a.Width-w)/2, a.Y+islandTopGap)
}

func (p *islandPresenter) refreshPosition() {
	if p.win == nil {
		return
	}
	w, h := islandWindowSize(p.geometry)
	p.position(w, h)
	islandWindowShape(p.win.NativeHandle(), p.geometry, w, h)
	p.win.Update(func() {})
}

func (p *islandPresenter) setMediaActive(on bool) {
	if p.mediaActive != on {
		p.mediaActive = on
		p.media.SetActive(on)
	}
}

// islandFrame is what one frame shows, read under the machine's lock and
// from the app.
type islandFrame struct {
	scene, second island.Scene
	expanded      bool
	notice        *islandNotice
	item          *island.Item
	queued        int
	closing       time.Time
	until         time.Time
	timer         appui.IslandTimer
	media         islandMedia
}

func (p *islandPresenter) view(c *ui.Context) {
	// What the pill leaves out stays clear: the desktop shows through.
	c.Root().Background(ui.Transparent)
	prefs := p.d.notificationPrefs.Load().(devicePreferences)
	if !prefs.IslandEnabled {
		p.reset(true)
		return
	}
	now := c.Now()
	if p.obstructed() {
		if p.obstructedSince.IsZero() {
			p.obstructedSince = now
		}
		if now.Sub(p.obstructedSince) >= islandObstructionGrace {
			p.suspend()
			return
		}
	} else {
		p.obstructedSince = time.Time{}
	}
	if p.born.IsZero() {
		p.born = now
	}
	reduced := prefs.IslandReduceMotion || c.Preferences().ReduceMotion
	px, py, pointerOK := p.pointer()
	overPill := pointerOK && insideIsland(px, py, [3]float64{p.geometry[0], p.geometry[1], p.geometry[2]})
	f := p.frame(c, now, prefs, overPill)
	if f.scene == island.SceneIdle && !prefs.IslandAlwaysOn && f.notice == nil {
		p.hideEmpty()
		return
	}
	p.idleSince = time.Time{}
	p.setMediaActive(prefs.IslandAlwaysOn)

	// A new scene, details opened or closed, another notice: the shape
	// moves and the content fades in.
	key := strconv.Itoa(int(f.scene)) + "/" + strconv.FormatBool(f.expanded)
	if f.item != nil {
		key += "/" + f.item.Tag
	}
	if key != p.key {
		if f.scene != p.scene {
			p.expander.Reset()
		}
		p.key, p.scene, p.expanded = key, f.scene, f.expanded
		p.contentChanged = now
	}
	bubble := 0.
	if f.second != island.SceneIdle {
		bubble = island.BubbleSize
	}
	if target := islandShape(island.Geometry(f.scene, f.expanded), bubble); target != p.target || !p.shown {
		p.start, p.target, p.changed = p.geometry, target, now
		if !p.shown {
			// Appearing, the island takes its shape at once.
			p.geometry, p.velocity = target, [4]float64{}
			ex, ey := island.EyesAt(f.scene, f.expanded, target[0], target[1])
			p.eyes, p.eyesVelocity = [2]float64{ex, ey}, [2]float64{}
		}
		p.last = now
	}
	moving := p.step(now, reduced)
	p.show()
	p.shown = true

	// Ace's eyes.
	ex, ey := island.EyesAt(f.scene, f.expanded, p.target[0], p.target[1])
	dt := islandFrameStep(p.last, now)
	for i, t := range [2]float64{ex, ey} {
		if reduced {
			p.eyes[i], p.eyesVelocity[i] = t, 0
			continue
		}
		p.eyes[i], p.eyesVelocity[i] = island.Spring(p.eyes[i], p.eyesVelocity[i], t, dt)
		if math.Abs(p.eyes[i]-t) > .05 || math.Abs(p.eyesVelocity[i]) > .05 {
			moving = true
		} else {
			p.eyes[i], p.eyesVelocity[i] = t, 0
		}
	}
	p.last = now
	open, glance, blinkDue := p.blink.Frame(now, reduced)
	gazing := false
	if dx, dy := px-p.eyes[0], py-p.eyes[1]; !reduced && pointerOK && math.Hypot(dx, dy) < island.GazeReach {
		p.gaze.Pointer(now, dx, dy)
	} else {
		p.gaze.Away()
	}
	gazing = p.gaze.Step(now)
	gx, gy := p.gaze.X, p.gaze.Y
	if !p.gaze.Following(now) {
		switch {
		case f.scene == island.SceneTimer && !f.expanded:
			gx = 2.5 // Ace watches the clock.
		case f.scene == island.SceneIdle && !f.expanded:
			gx = glance * island.GazeMax
		}
	}
	eyeH := float64(island.EyeHeight)
	if overPill || f.expanded || f.scene == island.SceneNotice {
		eyeH = 16 // alert, wide, or under the pointer
	}
	if now.Before(p.happyUntil) {
		eyeH, open = 6, 1 // happy arcs, after an action
	}
	// Ace breathes while someone is about or something goes on, and sits
	// still between blinks once nothing has for a while (Breather).
	if p.gaze.Following(now) || overPill || f.notice != nil || f.expanded || f.scene != island.SceneIdle || f.second != island.SceneIdle || now.Sub(p.born) < time.Second {
		p.breath.Wake(now)
	}
	scale, bop := 1., 0.
	breathing := false
	if !reduced {
		scale, breathing = p.breath.Scale(now)
		if f.scene == island.SceneMedia && f.media.Playing {
			bop = island.Bop(now.Sub(p.born))
		}
	}

	alpha := float32(island.Ease((now.Sub(p.contentChanged).Seconds() - .06) / .26))
	if reduced {
		alpha = float32(math.Min(1, now.Sub(p.contentChanged).Seconds()/.15))
	}
	if !f.closing.IsZero() {
		alpha *= float32(1 - island.ExitEase(now.Sub(f.closing).Seconds()/.16))
	}
	theme := *c.Theme()
	theme.Dark = true
	theme.Text, theme.TextMuted = islandText, islandMuted
	theme.Surface, theme.SurfaceHover = islandButton, ui.RGB(58, 58, 60)
	theme.Border = ui.RGBA(255, 255, 255, .12)
	theme.FontSize = 13
	theme.Accent, theme.AccentText = islandText, islandInk
	c.SetTheme(&theme)

	gw, gh, gr := float32(p.geometry[0]), float32(p.geometry[1]), float32(p.geometry[2])
	ui.Box(c).Fill().Children(func() {
		pill := ui.Box(c).Absolute().Left(0).Top(0).Size(gw, gh).Radius(gr).Background(islandInk).Border(1, islandRing).Clip()
		_ = pill.Hovered() // enter/leave redraw without a timer
		if pill.Clicked() {
			p.click(now, f)
			c.After(0)
		}
		pill.Children(func() {
			// The eyes: two capsules, nodding to music, blinking, looking.
			h := float32(math.Max(1.4, eyeH*open*scale))
			w := float32(island.EyeWidth * scale)
			cy := float32(p.eyes[1] + gy + bop)
			for _, side := range [2]float64{-1, 1} {
				cx := float32(p.eyes[0] + side*(island.EyePair-island.EyeWidth)/2*scale + gx)
				ui.Box(c).Absolute().Left(cx-w/2).Top(cy-h/2).Size(w, h).Radius(min(w, h) / 2).Background(islandText)
			}
			p.content(c, f, now, alpha, reduced)
		})
		if p.geometry[3] > 1 {
			b := float32(p.geometry[3])
			dot := ui.ButtonBase(c).Absolute().Left(gw+island.BubbleGap).Top(0).Size(b, b).Radius(b/2).Background(islandInk).Border(1, islandRing).Clip().Center()
			dot.Label(map[island.Scene]string{island.SceneTimer: "Timer rapat", island.SceneMedia: "Musik"}[f.second])
			if dot.Clicked() {
				p.swap = !p.swap
				c.After(0)
			}
			dot.Children(func() { p.mini(c, f, now, float32(p.geometry[3]/island.BubbleSize), reduced) })
		}
	})

	// Frames: none between motions. Fullscreen, the pointer's reach and
	// what the app shows are checked every 500 ms; awake, Ace breathes at
	// 12 frames a second; motion, fades, blinks and the music's wave ask
	// for more while they last.
	next := 500 * time.Millisecond
	if breathing {
		next = 83 * time.Millisecond
	}
	next = min(next, max(time.Millisecond, blinkDue.Sub(now)))
	if p.gaze.Following(now) {
		next = min(next, max(time.Millisecond, p.gaze.RestsAt().Sub(now)))
	}
	if f.notice != nil {
		switch {
		case overPill && !f.expanded:
			next = min(next, 50*time.Millisecond)
		case f.expanded && !overPill:
			next = min(next, 100*time.Millisecond)
		case !f.expanded:
			next = min(next, max(time.Millisecond, f.until.Sub(now)))
		}
	} else if overPill || f.expanded {
		next = min(next, 50*time.Millisecond) // the expander's dwell and leave
	}
	if f.scene == island.SceneTimer || f.second == island.SceneTimer {
		next = min(next, time.Second-time.Duration(now.Nanosecond()))
	}
	if !reduced && (f.scene == island.SceneMedia || f.second == island.SceneMedia) && f.media.Playing {
		next = min(next, 66*time.Millisecond)
	}
	if moving || gazing || alpha < 1 || !f.closing.IsZero() || open < 1 || now.Before(p.happyUntil) {
		next = 16 * time.Millisecond
	}
	c.After(p.obstructionDeadline(now, next))
}

// frame reads what the island shows: the notice first, else the live
// activities (the meeting timer, then music), else Ace at rest.
func (p *islandPresenter) frame(c *ui.Context, now time.Time, prefs devicePreferences, overPill bool) islandFrame {
	var f islandFrame
	live := []island.Scene{}
	if p.spikeLive != nil {
		t, tok, m, mok := p.spikeLive(now)
		if tok {
			f.timer = t
			live = append(live, island.SceneTimer)
		}
		if mok {
			f.media = m
			live = append(live, island.SceneMedia)
		}
	} else if prefs.IslandAlwaysOn {
		if p.d.app != nil {
			if t, ok := p.d.app.IslandTimer(now); ok {
				f.timer = t
				live = append(live, island.SceneTimer)
			}
		}
		if m, ok := p.media.Current(); ok && m.Shown(now) {
			f.media = m
			live = append(live, island.SceneMedia)
		}
	}
	if len(live) < 2 {
		p.swap = false
	} else if p.swap {
		live[0], live[1] = live[1], live[0]
	}
	p.mu.Lock()
	p.m.Tick(now, overPill || p.spikeOpen, false, c.Shortcut(0, ui.KeyEscape))
	if it := p.m.Current; it != nil {
		x := it.Value.(islandNotice)
		f.scene, f.notice, f.item = island.SceneNotice, &x, it
		f.expanded = p.m.State == island.Expanded
		f.queued, f.closing, f.until = len(p.m.Queue), p.m.Closing, p.m.Until
	}
	p.mu.Unlock()
	switch {
	case f.notice != nil:
		if len(live) > 0 {
			f.second = live[0]
		}
	case len(live) > 0:
		f.scene = live[0]
		if len(live) > 1 {
			f.second = live[1]
		}
	}
	if f.notice == nil {
		if f.scene != p.scene {
			p.expander.Reset()
		}
		p.expander.Tick(now, overPill, false)
		f.expanded = p.expander.Expanded || p.spikeOpen
	}
	if f.expanded {
		f.second = island.SceneIdle // the details take the island
	}
	return f
}

// click opens or closes the details: a notice's through its machine.
func (p *islandPresenter) click(now time.Time, f islandFrame) {
	if f.notice != nil {
		p.mu.Lock()
		p.m.Tick(now, true, true, false)
		p.mu.Unlock()
		return
	}
	p.expander.Tick(now, true, true)
}

// pointer is where the pointer is over the window, in DIP.
func (p *islandPresenter) pointer() (float64, float64, bool) {
	w, h := islandWindowSize(p.geometry)
	if x, y, ok := islandPointer(p.win.NativeHandle(), float64(w), float64(h)); ok {
		return x, y, true
	}
	point := mygo.Screen.CursorScreenPoint()
	b := p.win.Bounds()
	return float64(point.X - b.X), float64(point.Y - b.Y), true
}

// obstructionDeadline brings the next frame forward to when fullscreen,
// still on, has lasted its grace.
func (p *islandPresenter) obstructionDeadline(now time.Time, next time.Duration) time.Duration {
	if p.obstructedSince.IsZero() {
		return next
	}
	return max(time.Millisecond, min(next, p.obstructedSince.Add(islandObstructionGrace).Sub(now)))
}

// step advances the shape a frame, fits the window to it, and tells
// whether it still moves.
func (p *islandPresenter) step(now time.Time, reduced bool) bool {
	dt := islandFrameStep(p.last, now)
	moving := false
	for i := range p.geometry {
		if reduced {
			q := island.Ease(now.Sub(p.changed).Seconds() / .18)
			p.geometry[i] = p.start[i] + (p.target[i]-p.start[i])*q
			p.velocity[i] = 0
		} else {
			p.geometry[i], p.velocity[i] = island.Spring(p.geometry[i], p.velocity[i], p.target[i], dt)
		}
		if math.Abs(p.geometry[i]-p.target[i]) > .1 || math.Abs(p.velocity[i]) > .1 {
			moving = true
		} else {
			p.geometry[i] = p.target[i]
			p.velocity[i] = 0
		}
	}
	p.geometry[3] = math.Max(0, p.geometry[3])
	w, h := islandWindowSize(p.geometry)
	if w != p.lastWidth || h != p.lastHeight {
		p.win.SetSize(w, h)
		p.position(w, h)
		p.lastWidth, p.lastHeight = w, h
	}
	if p.region != p.geometry {
		islandWindowShape(p.win.NativeHandle(), p.geometry, w, h)
		p.region = p.geometry
	}
	return moving
}

// wake puts Ace on screen when "Selalu tampil" is on: at start, and when
// the setting is turned on.
func (p *islandPresenter) wake() {
	p.d.win.Update(p.resume)
}

// suspend hides the island while fullscreen lasts, keeping its queue, and
// brings it back when fullscreen ends. The wait polls Windows from a
// goroutine, once a second, and draws nothing meanwhile.
func (p *islandPresenter) suspend() {
	p.obstructedSince = time.Time{}
	if p.win != nil && p.win.IsVisible() {
		p.win.Hide()
	}
	p.shown = false
	p.setMediaActive(false)
	if p.watching {
		return
	}
	p.watching = true
	go func() {
		for {
			time.Sleep(time.Second)
			if !p.d.notificationPrefs.Load().(devicePreferences).IslandEnabled || !p.obstructed() {
				break
			}
		}
		p.d.win.Update(func() {
			p.watching = false
			p.resume()
		})
	}()
}

// resume shows the island again with what it held; a banner gets its full
// time back. Off, or with nothing to show, it stays hidden.
func (p *islandPresenter) resume() {
	if p.watching {
		return
	}
	prefs := p.d.notificationPrefs.Load().(devicePreferences)
	if !prefs.IslandEnabled || !islandSupported() && p.environmentCheck == nil {
		return
	}
	p.mu.Lock()
	has := p.m.Current != nil
	if has && p.m.State == island.Banner {
		p.m.Until = time.Now().Add(island.BannerDuration)
	}
	p.mu.Unlock()
	if !has && !prefs.IslandAlwaysOn {
		return
	}
	p.ensureWindow()
	if !p.win.IsVisible() {
		p.forget()
		p.show()
	}
	p.win.Update(func() {})
}

func (p *islandPresenter) hideEmpty() {
	p.win.Hide()
	p.forget()
	p.setMediaActive(false)
	p.scheduleIdleDestroy()
}

// content is what the pill shows beside the eyes: a notice, the meeting
// timer, the music, or the day at rest, compact or in detail.
func (p *islandPresenter) content(c *ui.Context, f islandFrame, now time.Time, alpha float32, reduced bool) {
	gw, gh := float32(p.geometry[0]), float32(p.geometry[1])
	left := float32(island.ContentLeft(f.scene, f.expanded))
	interactive := f.closing.IsZero() && alpha > .5
	box := ui.Box(c).Absolute().Left(left).Top(0).Width(max(0, gw-left-16)).Height(gh).Opacity(alpha)
	if f.expanded {
		box = ui.Box(c).Absolute().Left(left).Top(24).Width(max(0, gw-left-28)).Height(max(0, gh-24)).Opacity(alpha)
	}
	if !reduced && alpha < 1 {
		box.Translate(0, -4*(1-alpha)).Blur(6 * (1 - alpha))
	}
	if !interactive {
		box.Disabled(true)
	}
	box.Children(func() {
		switch {
		case f.scene == island.SceneNotice && f.expanded:
			p.noticeDetails(c, *f.notice, f.queued)
		case f.scene == island.SceneNotice:
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, f.notice.n.Title).FontSize(14).FontWeight(600).MaxLines(1)
					ui.Text(c, f.notice.n.Body).FontSize(13).MaxLines(1).TextColor(islandMuted)
				})
				if f.queued > 0 {
					ui.Text(c, fmtIslandQueue(f.queued)).FontSize(13).TextColor(islandMuted)
				}
				ui.Icon(c, appui.NoticeGlyph(f.notice.n.Kind)).Size(18, 18).TextColor(islandGlyphColor("A", f.notice.n.Kind))
			})
		case f.scene == island.SceneTimer && f.expanded:
			p.timerDetails(c, f.timer)
		case f.scene == island.SceneTimer:
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, f.timer.Title).FontSize(13).MaxLines(1).Grow(1).MinWidth(0).TextColor(islandMuted)
				ui.Text(c, f.timer.Clock).FontSize(14).FontWeight(600).FontFeatures("tnum").TextColor(islandTimerColor(f.timer))
			})
		case f.scene == island.SceneMedia && f.expanded:
			p.mediaDetails(c, f.media, now, reduced)
		case f.scene == island.SceneMedia:
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, f.media.Title).FontSize(14).FontWeight(600).MaxLines(1).Grow(1).MinWidth(0)
				islandWave(c, now, f.media.Playing && !reduced, 1)
			})
		case f.expanded:
			p.restDetails(c, now)
		}
	})
}

// mini is the second activity in its circle: the music's wave, or the
// meeting's timer glyph.
func (p *islandPresenter) mini(c *ui.Context, f islandFrame, now time.Time, scale float32, reduced bool) {
	box := ui.Box(c).Opacity(min(1, scale)).Scale(.8*scale, .8*scale)
	box.Children(func() {
		switch f.second {
		case island.SceneMedia:
			islandWave(c, now, f.media.Playing && !reduced, 1)
		case island.SceneTimer:
			ui.Icon(c, islandIcons["timer"]).Size(20, 20).TextColor(islandTimerColor(f.timer))
		}
	})
}

func islandTimerColor(t appui.IslandTimer) ui.Color {
	switch {
	case t.Paused:
		return islandMuted
	case t.Over:
		return islandOver
	}
	return islandMeeting
}

// islandWave is the music's five bars, moving while it plays.
func islandWave(c *ui.Context, now time.Time, playing bool, scale float32) {
	ui.Row(c).Height(16 * scale).AlignItems(ui.Center).Gap(2 * scale).Children(func() {
		for i := range 5 {
			h := .3
			if playing {
				h = island.Wave(time.Duration(now.UnixNano()), i)
			}
			ui.Box(c).Size(3*scale, float32(16*h)*scale).Radius(1.5 * scale).Background(islandMusic)
		}
	})
}

// islandAction is a round button of the details; primary is white.
func (p *islandPresenter) islandAction(c *ui.Context, label string, icon *ui.SVG, primary bool) bool {
	b := ui.ButtonBase(c.Key("island-"+label)).Label(label).Height(32).Radius(16).Padding(0, 12).Gap(6).Background(islandButton)
	fg := islandText
	if primary {
		b.Background(islandText)
		fg = islandInk
	}
	if icon != nil {
		b.MinWidth(32).Padding(0, 8)
	}
	b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(16, 16).TextColor(fg)
		} else {
			ui.Text(c, label).FontSize(13).TextColor(fg)
		}
	})
	if b.Clicked() {
		p.happyUntil = time.Now().Add(420 * time.Millisecond)
		return true
	}
	return false
}

func (p *islandPresenter) noticeDetails(c *ui.Context, x islandNotice, queued int) {
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, x.n.Title).FontSize(15).FontWeight(600).MaxLines(1).Grow(1).MinWidth(0)
			if queued > 0 {
				ui.Text(c, fmtIslandQueue(queued)).FontSize(13).TextColor(islandMuted)
			}
		})
		kind := map[string]string{"ping": "Ping", "task": "Tugas", "meeting": "Rapat", "reminder": "Pengingat", "deadline": "Tenggat", "update": "Pembaruan"}[x.n.Kind]
		if kind == "" {
			kind = "Pengingat"
		}
		ui.Text(c, "Space · "+kind).FontSize(13).TextColor(islandMuted)
		ui.Box(c).Height(8)
		ui.Text(c, x.n.Body).FontSize(14).MaxLines(2)
		ui.Box(c).Height(12)
		ui.Row(c).Gap(8).Children(func() {
			open := x.n.OpenLabel
			if open == "" {
				open = "Buka"
			}
			if p.islandAction(c, open, nil, true) {
				p.act(x.n.Link)
				p.dismiss()
			}
			for _, a := range x.n.Actions {
				if p.islandAction(c, a.Label, nil, false) {
					p.act(a.Link)
					p.dismiss()
				}
			}
			// Two actions already fill the row; the second (Nanti) closes too.
			if len(x.n.Actions) < 2 && p.islandAction(c, "Tutup", nil, false) {
				p.dismiss()
			}
		})
	})
}

func (p *islandPresenter) timerDetails(c *ui.Context, t appui.IslandTimer) {
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		where := " · timer bersama"
		if t.Meeting == id.Nil {
			where = " · timer"
		}
		ui.Text(c, t.Title+where).FontSize(13).MaxLines(1).TextColor(islandMuted)
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.Clock).FontSize(32).FontWeight(600).FontFeatures("tnum").MaxLines(1).TextColor(islandTimerColor(t))
			note := t.Agenda
			if t.Paused {
				note = "Dijeda"
			}
			if note != "" {
				ui.Text(c, note).FontSize(13).MaxLines(2).Grow(1).MinWidth(0).TextColor(islandMuted)
			}
		})
		ui.Box(c).Height(8)
		ui.Row(c).Gap(8).Children(func() {
			if t.Control {
				label, action := "Jeda", api.TimerPause
				if t.Paused {
					label, action = "Lanjutkan", api.TimerResume
				}
				if p.islandAction(c, label, nil, true) {
					p.d.app.IslandTimerAct(t.Meeting, action)
				}
			}
			if p.islandAction(c, "Buka Space", nil, !t.Control) {
				p.act("")
			}
		})
	})
}

func (p *islandPresenter) mediaDetails(c *ui.Context, m islandMedia, now time.Time, reduced bool) {
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(44, 44).Radius(10).Background(ui.RGB(58, 58, 60)).Center().Children(func() {
				ui.Icon(c, islandIcons["music"]).Size(18, 18).TextColor(islandText)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, m.Title).FontSize(15).FontWeight(600).MaxLines(1)
				by := m.Artist
				if m.App != "" {
					if by != "" {
						by += " · "
					}
					by += m.App
				}
				ui.Text(c, by).FontSize(13).MaxLines(1).TextColor(islandMuted)
			})
			islandWave(c, now, m.Playing && !reduced, 1)
		})
		ui.Box(c).Height(14)
		if q := m.Progress(now); q >= 0 {
			ui.Box(c).FillWidth().Height(4).Radius(2).Background(ui.RGBA(255, 255, 255, .16)).Clip().Children(func() {
				ui.Box(c).Width(float32(q) * float32(p.geometry[0]-64-28)).Height(4).Background(islandMusic)
			})
		}
		ui.Box(c).Height(12)
		ui.Row(c).FillWidth().Gap(8).Justify(ui.Center).Children(func() {
			if p.islandAction(c, "Sebelumnya", islandIcons["previous"], false) {
				p.media.Command(mediaPrevious)
			}
			icon, label := islandIcons["play"], "Putar"
			if m.Playing {
				icon, label = islandIcons["pause"], "Jeda"
			}
			if p.islandAction(c, label, icon, true) {
				p.media.Command(mediaToggle)
			}
			if p.islandAction(c, "Berikutnya", islandIcons["next"], false) {
				p.media.Command(mediaNext)
			}
		})
	})
}

// restDetails is Ace's details at rest: the day's next meeting, and the
// way into Space.
func (p *islandPresenter) restDetails(c *ui.Context, now time.Time) {
	today := ""
	if p.d.app != nil {
		today = p.d.app.IslandToday(now)
	}
	if today == "" {
		today = "Tidak ada rapat lagi hari ini"
	}
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Text(c, "Hari ini").FontSize(15).FontWeight(600)
		ui.Text(c, today).FontSize(13).MaxLines(1).TextColor(islandMuted)
		ui.Box(c).Height(16)
		ui.Row(c).Gap(8).Children(func() {
			if p.islandAction(c, "Buka Space", nil, true) {
				p.act("")
			}
			if p.islandAction(c, "Pengaturan island", nil, false) {
				p.d.win.Update(func() {
					p.d.win.Show()
					p.d.win.Focus()
					p.d.app.OpenDeviceSettings()
				})
			}
		})
	})
}

// islandIcons are the island's own glyphs, beside appui's.
var islandIcons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"music":    `<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>`,
		"timer":    `<path d="M10 2h4m-2 12 3-3"/><circle cx="12" cy="14" r="8"/>`,
		"play":     `<path d="M7 4.5v15l12-7.5Z" fill="currentColor"/>`,
		"pause":    `<rect x="6" y="4" width="4" height="16" rx="1" fill="currentColor"/><rect x="14" y="4" width="4" height="16" rx="1" fill="currentColor"/>`,
		"next":     `<path d="M5 5v14l10-7Z" fill="currentColor"/><path d="M19 5v14"/>`,
		"previous": `<path d="M19 5v14L9 12Z" fill="currentColor"/><path d="M5 5v14"/>`,
	}
	icons := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return icons
}()

func fmtIslandQueue(n int) string { return "+" + strconv.Itoa(n) }

// islandFrameStep is the time the spring advances in a frame: none on the
// first, and at most 50 ms, so a late frame does not skip the motion.
func islandFrameStep(last, now time.Time) float64 {
	if last.IsZero() {
		return 0
	}
	return math.Max(0, math.Min(now.Sub(last).Seconds(), .05))
}

// islandGlyphColor is the kind's accent of each style, as in the approved
// canvas (dceda2c54836).
func islandGlyphColor(style, kind string) ui.Color {
	switch style {
	case "B":
		switch kind {
		case "deadline", "reminder":
			return ui.RGB(232, 194, 122)
		case "meeting":
			return ui.RGB(168, 194, 172)
		}
		return ui.RGB(227, 160, 127)
	case "C":
		switch kind {
		case "task":
			return ui.RGB(10, 132, 255)
		case "meeting":
			return ui.RGB(52, 199, 89)
		}
		return ui.RGB(255, 159, 10)
	}
	switch kind {
	case "task":
		return ui.RGB(10, 132, 255)
	case "deadline", "reminder":
		return ui.RGB(255, 69, 58)
	case "meeting":
		return ui.RGB(48, 209, 88)
	}
	return ui.RGB(255, 159, 10)
}

// opaque drops a surface's translucency, for text drawn with its color.
func opaque(c ui.Color) ui.Color { return ui.RGB(c.R, c.G, c.B) }
func insideIsland(x, y float64, g [3]float64) bool {
	w, h, r := g[0], g[1], math.Min(g[2], g[1]/2)
	if x < 0 || y < 0 || x >= w || y >= h {
		return false
	}
	dx := math.Max(math.Max(r-x, x-(w-r)), 0)
	dy := math.Max(math.Max(r-y, y-(h-r)), 0)
	return dx*dx+dy*dy <= r*r
}
func (p *islandPresenter) dismiss() {
	p.mu.Lock()
	p.m.Dismiss(time.Now())
	p.mu.Unlock()
	p.win.Update(func() {})
}
func (p *islandPresenter) act(link string) {
	p.d.win.Update(func() {
		if link != "" {
			u, _ := url.Parse(link)
			// An update's actions come from the island only, never from a
			// link opened outside Space.
			if u != nil && u.Scheme == "himasif-space" && u.Host == "update" {
				p.d.updater.handleLink(u)
				return
			}
			activate := u == nil || u.Query().Get("action") != "snooze"
			p.d.handleNotificationLink(link, activate)
		} else {
			p.d.win.Show()
			p.d.win.Focus()
		}
	})
}

type deliveryToast struct {
	appui.Notifier
	n    api.Delivery
	link string
}

func (n deliveryToast) Present(appui.Notice) error { return showDeviceNotification(n.n, n.link) }

type deliveryIsland struct {
	p    *islandPresenter
	n    api.Delivery
	link string
}

func (n deliveryIsland) Present(x appui.Notice) error {
	return n.p.offer(x, func() { n.p.reportFallback(showDeviceNotification(n.n, n.link)) })
}
func (d *desktopDevice) presentDelivery(uid id.ID, n api.Delivery, link string, prefs devicePreferences) error {
	x := appui.Notice{Kind: n.SourceKind, Title: n.Title, Body: n.Body, Tag: uid.String() + ":" + n.ID.String(), Link: link, AccountID: uid.String(), Actions: []appui.NoticeAction{{Label: "Tunda 15 menit", Link: notificationLink(uid, n, "snooze")}}}
	return (appui.MultiNotifier{Base: deliveryToast{d.app.Notifier, n, link}, Extra: []appui.NoticePresenter{deliveryIsland{d.island, n, link}}, ReplaceBase: prefs.IslandReplaceToast}).Present(x)
}
