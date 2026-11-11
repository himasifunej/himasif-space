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

// islandIdleGeometry is Ace at rest: a small pill with two eyes, as the
// iPhone's island at rest (width, height, radius in DIP).
var islandIdleGeometry = [3]float64{124, 32, 16}

// islandObstructionGrace is how long fullscreen must last before the island
// steps aside: a screenshot overlay or a splash screen comes and goes.
const islandObstructionGrace = 1500 * time.Millisecond

type islandNotice struct {
	n        appui.Notice
	fallback func()
}

// All machine access is serialized. Native window and animation fields belong
// only to MyGo's UI thread; delivery workers only enqueue and request Update.
type islandPresenter struct {
	// Only the isolated spike supplies an override, to measure rendering even
	// when this Windows session currently has Focus Assist or fullscreen on.
	environmentCheck                         func() bool
	idleAfter                                time.Duration // 0: islandIdleDestroy; the spike shortens it
	d                                        *desktopDevice
	mu                                       sync.Mutex
	m                                        island.Machine
	win                                      *mygo.Window
	last, changed, contentChanged, idleSince time.Time
	state                                    island.State
	current                                  *island.Item
	shown, previous                          *islandNotice
	previousState                            island.State
	geometry, velocity, start                [3]float64
	target                                   [3]float64
	lastWidth, lastHeight                    int
	dark                                     bool
	// idle is set while the window shows Ace at rest; watching while a
	// goroutine waits for fullscreen to end to bring the island back.
	idle, watching  bool
	obstructedSince time.Time
	eyes            island.Eyes
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
			p.position(200, 36)
			islandWindowShape(p.win.NativeHandle(), 200, 36, 18)
			p.win.ShowInactive()
		}
		p.win.Update(func() {})
	})
	return nil
}

func (p *islandPresenter) ensureWindow() {
	if p.win != nil {
		return
	}
	p.geometry = [3]float64{200, 36, 18}
	p.velocity = [3]float64{}
	p.state = island.Hidden
	p.current, p.shown, p.previous = nil, nil, nil
	p.contentChanged = time.Time{}
	p.lastWidth, p.lastHeight = 0, 0
	p.last = time.Time{}
	p.idle = false
	p.win = mygo.NewWindow(mygo.WindowOptions{Title: "Dynamic Island · Space", Width: 200, Height: 36, Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, DisableResize: true, Panel: true, Content: ui.View(p.view)})
	islandPrepareWindow(p.win.NativeHandle())
	islandClearBackdrop(p.win.NativeHandle())
	p.win.OnClose(func(e *mygo.CloseEvent) {
		if !p.d.quitting {
			e.PreventDefault()
			p.reset(false)
		}
	})
}

// reset discards account content, or sends accepted content to the OS when
// settings/DND/fullscreen make the island unavailable while it is visible.
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
	p.current, p.shown, p.previous = nil, nil, nil
	p.state = island.Hidden
	p.idle = false
	p.geometry = [3]float64{200, 36, 18}
	p.velocity = [3]float64{}
	p.contentChanged = time.Time{}
	p.last = time.Time{}
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
			if empty && p.win != nil && !(prefs.IslandEnabled && prefs.IslandAlwaysOn) {
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
	w, h := int(math.Round(p.geometry[0])), int(math.Round(p.geometry[1]))
	p.position(w, h)
	islandWindowShape(p.win.NativeHandle(), w, h, p.geometry[2])
	p.win.Update(func() {})
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
	p.mu.Lock()
	if p.m.Current == nil {
		p.mu.Unlock()
		p.idleView(c, prefs, now)
		return
	}
	hover := p.hovered()
	wasClosing := !p.m.Closing.IsZero()
	p.m.Tick(now, hover, false, c.Shortcut(0, ui.KeyEscape))
	if p.m.Current == nil {
		p.mu.Unlock()
		p.idleView(c, prefs, now)
		return
	}
	x := p.m.Current.Value.(islandNotice)
	state, closing, until := p.m.State, p.m.Closing, p.m.Until
	queued := len(p.m.Queue)
	newContent := p.current != p.m.Current || state != p.state
	p.current = p.m.Current
	p.mu.Unlock()
	p.idleSince = time.Time{}
	reduced := prefs.IslandReduceMotion || c.Preferences().ReduceMotion
	if newContent || p.contentChanged.IsZero() {
		p.previous = p.shown
		if wasClosing {
			p.previous = nil
		} // the previous item already faded out.
		p.previousState = p.state
		copyNotice := x
		p.shown = &copyNotice
		p.contentChanged = now
	}
	if state != p.state {
		p.state = state
		p.idle = false
		target := [3]float64{380, 64, 32}
		if state == island.Expanded {
			target = [3]float64{440, 184, 30}
		}
		p.retarget(now, target)
	}
	w, h, moving := p.step(now, reduced)
	if newContent {
		p.position(w, h)
		islandWindowShape(p.win.NativeHandle(), w, h, p.geometry[2])
	}
	if !p.win.IsVisible() {
		p.win.ShowInactive()
	}
	bg, fg, muted, accent, dark := p.palette(c, prefs)
	theme := *c.Theme()
	theme.Dark = dark
	theme.Text = fg
	theme.TextMuted = muted
	theme.Surface = ui.RGBA(127, 127, 127, .16)
	theme.SurfaceHover = ui.RGBA(127, 127, 127, .28)
	theme.Border = ui.RGBA(127, 127, 127, .2)
	theme.FontSize = 13
	theme.Accent = accent
	theme.AccentText = opaque(bg)
	c.SetTheme(&theme)
	alpha := float32(island.Ease((now.Sub(p.contentChanged).Seconds() - .06) / .24))
	if reduced {
		alpha = float32(math.Min(1, now.Sub(p.contentChanged).Seconds()/.15))
	}
	if !closing.IsZero() {
		alpha *= float32(1 - island.ExitEase(now.Sub(closing).Seconds()/.12))
	}
	root := ui.Column(c).Fill().Radius(float32(p.geometry[2])).Background(bg).Border(1, ui.RGBA(255, 255, 255, .18)).Clip().Padding(14).Gap(10)
	_ = root.Hovered() // subscribe to enter/leave redraws even without a timer.
	if root.Clicked() {
		p.mu.Lock()
		p.m.Tick(now, true, true, false)
		p.mu.Unlock()
		c.After(0)
	}
	root.Children(func() {
		if p.previous != nil {
			duration := .12
			if reduced {
				duration = .15
			}
			out := float32(1 - island.ExitEase(now.Sub(p.contentChanged).Seconds()/duration))
			if out > 0 {
				ui.Box(c).Absolute().Left(14).Top(14).Width(float32(w - 28)).Children(func() { p.content(c, *p.previous, p.previousState, queued, bg, fg, muted, accent, out, reduced, false) })
			} else {
				p.previous = nil
			}
		}
		p.content(c, x, state, queued, bg, fg, muted, accent, alpha, reduced, closing.IsZero())
	})
	// No hidden-frame loop. Visible idle polls QUNS at 2 s; shorter deadlines
	// service hover and expiry, and only animation requests 60 Hz frames.
	next := 2 * time.Second
	if hover && state == island.Banner {
		next = 50 * time.Millisecond
	}
	if !hover && state == island.Expanded {
		next = 100 * time.Millisecond
	}
	if state == island.Banner && !hover {
		next = min(next, max(time.Millisecond, until.Sub(now)))
	}
	if moving || alpha < 1 || !closing.IsZero() || now.Sub(p.contentChanged) < 300*time.Millisecond {
		next = 16 * time.Millisecond
	}
	c.After(p.obstructionDeadline(now, next))
}

// idleView is Ace at rest, when "Selalu tampil" keeps the island on screen:
// a small pill whose eyes blink and glance, and follow the pointer over it.
// A click opens Space.
func (p *islandPresenter) idleView(c *ui.Context, prefs devicePreferences, now time.Time) {
	if !prefs.IslandAlwaysOn {
		p.hideEmpty()
		return
	}
	reduced := prefs.IslandReduceMotion || c.Preferences().ReduceMotion
	if !p.idle {
		p.idle = true
		p.state = island.Hidden
		p.current, p.shown, p.previous = nil, nil, nil
		p.retarget(now, islandIdleGeometry)
		if !p.win.IsVisible() {
			// Appearing, it rests at once; from a banner, it shrinks back.
			p.geometry, p.velocity = islandIdleGeometry, [3]float64{}
		}
	}
	p.idleSince = time.Time{}
	w, h, moving := p.step(now, reduced)
	if !p.win.IsVisible() {
		p.position(w, h)
		islandWindowShape(p.win.NativeHandle(), w, h, p.geometry[2])
		p.win.ShowInactive()
	}
	bg, fg, _, _, _ := p.palette(c, prefs)
	hover := p.hovered()
	open, gaze, due := p.eyes.Frame(now, reduced)
	if hover {
		// Ace looks at the pointer, eyes a little wider.
		point := mygo.Screen.CursorScreenPoint()
		b := p.win.Bounds()
		gaze = math.Max(-1, math.Min(1, (float64(point.X-b.X)-p.geometry[0]/2)/(p.geometry[0]/2)))
		if reduced {
			gaze = 0
		}
	}
	alpha := float32(island.Ease((now.Sub(p.contentChanged).Seconds() - .06) / .24))
	if reduced {
		alpha = float32(math.Min(1, now.Sub(p.contentChanged).Seconds()/.15))
	}
	eye := float32(12)
	if hover {
		eye = 14
	}
	root := ui.Row(c).Fill().Radius(float32(p.geometry[2])).Background(bg).Border(1, ui.RGBA(255, 255, 255, .12)).Clip().Center().Gap(14)
	_ = root.Hovered()
	if root.Clicked() {
		p.act("")
	}
	root.Children(func() {
		for range 2 {
			ui.Box(c).Size(7, max(1.5, eye*float32(open))).Radius(3.5).Background(fg).Opacity(alpha).Translate(float32(gaze*4), 0)
		}
	})
	next := max(time.Millisecond, due.Sub(now))
	if hover {
		next = min(next, 33*time.Millisecond)
	}
	if moving || alpha < 1 {
		next = 16 * time.Millisecond
	}
	// Fullscreen is checked at least every 500 ms while Ace rests: the
	// pill is always on top, so it steps aside within the grace.
	c.After(p.obstructionDeadline(now, min(next, 500*time.Millisecond)))
}

// hovered tells whether the pointer is over the pill, not its clear corners.
func (p *islandPresenter) hovered() bool {
	point := mygo.Screen.CursorScreenPoint()
	b := p.win.Bounds()
	hover := insideIsland(float64(point.X-b.X), float64(point.Y-b.Y), p.geometry)
	if inside, ok := islandPointer(p.win.NativeHandle(), p.geometry); ok {
		hover = inside
	}
	return hover
}

func (p *islandPresenter) palette(c *ui.Context, prefs devicePreferences) (bg, fg, muted, accent ui.Color, dark bool) {
	dark = c.Theme().Dark
	if p.d.app != nil {
		dark = p.d.app.DarkAppearance(dark)
	}
	p.dark = dark
	bg, fg, muted, accent = islandPalette(prefs.IslandStyle, dark)
	return bg, fg, muted, accent, dark
}

// obstructionDeadline brings the next frame forward to when fullscreen,
// still on, has lasted its grace.
func (p *islandPresenter) obstructionDeadline(now time.Time, next time.Duration) time.Duration {
	if p.obstructedSince.IsZero() {
		return next
	}
	return max(time.Millisecond, min(next, p.obstructedSince.Add(islandObstructionGrace).Sub(now)))
}

// retarget starts the shape moving toward target. The time since the last
// frame (up to 2 s while a banner rests, minutes since the island last hid)
// must not advance the spring, or it lands at once without its motion.
func (p *islandPresenter) retarget(now time.Time, target [3]float64) {
	p.changed = now
	p.start = p.geometry
	p.contentChanged = now
	p.target = target
	p.last = now
}

// step advances the shape a frame and fits the window to it.
func (p *islandPresenter) step(now time.Time, reduced bool) (int, int, bool) {
	dt := islandFrameStep(p.last, now)
	p.last = now
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
	w, h := int(math.Round(p.geometry[0])), int(math.Round(p.geometry[1]))
	if w != p.lastWidth || h != p.lastHeight {
		p.win.SetSize(w, h)
		p.position(w, h)
		islandWindowShape(p.win.NativeHandle(), w, h, p.geometry[2])
		p.lastWidth, p.lastHeight = w, h
	}
	return w, h, moving
}

// suspend hides the island while fullscreen lasts, keeping its queue, and
// brings it back when fullscreen ends. The wait polls Windows from a
// goroutine, once a second, and redraws nothing meanwhile.
// wake puts Ace on screen when "Selalu tampil" is on: at start, and when
// the setting is turned on.
func (p *islandPresenter) wake() {
	p.d.win.Update(p.resume)
}

func (p *islandPresenter) suspend() {
	p.obstructedSince = time.Time{}
	if p.win != nil && p.win.IsVisible() {
		p.win.Hide()
	}
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

// resume shows the island again where it was, with what it held; a banner
// gets its full time back. Off, or with nothing to show, it stays hidden.
func (p *islandPresenter) resume() {
	if p.watching {
		return
	}
	prefs := p.d.notificationPrefs.Load().(devicePreferences)
	if !prefs.IslandEnabled || !islandSupported() {
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
		w, h := int(math.Round(p.geometry[0])), int(math.Round(p.geometry[1]))
		if p.idle || !has {
			w, h = int(islandIdleGeometry[0]), int(islandIdleGeometry[1])
		}
		if !has {
			// The resting pill: the window takes its size now, so it is
			// centred and does not spring from the banner's size.
			p.geometry, p.velocity = islandIdleGeometry, [3]float64{}
			p.win.SetSize(w, h)
			p.lastWidth, p.lastHeight = w, h
		}
		p.position(w, h)
		islandWindowShape(p.win.NativeHandle(), w, h, p.geometry[2])
		p.win.ShowInactive()
	}
	p.win.Update(func() {})
}

func (p *islandPresenter) hideEmpty() {
	p.win.Hide()
	p.state = island.Hidden
	p.idle = false
	p.current, p.shown, p.previous = nil, nil, nil
	p.contentChanged = time.Time{}
	p.last = time.Time{}
	p.geometry = [3]float64{200, 36, 18}
	p.velocity = [3]float64{}
	p.scheduleIdleDestroy()
}

func (p *islandPresenter) content(c *ui.Context, x islandNotice, state island.State, queued int, bg, fg, muted, accent ui.Color, alpha float32, reduced, interactive bool) {
	content := ui.Column(c).FillWidth().Gap(10).Opacity(alpha)
	if !interactive {
		content.Disabled(true)
	}
	if !reduced {
		if interactive {
			content.Translate(0, -4*(1-alpha)).Scale(.96+.04*alpha, .96+.04*alpha).Blur(6 * (1 - alpha))
		} else {
			content.Scale(.98+.02*alpha, .98+.02*alpha).Blur(4 * (1 - alpha))
		}
	}
	content.Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			style := p.d.notificationPrefs.Load().(devicePreferences).IslandStyle
			glyphColor := islandGlyphColor(style, x.n.Kind)
			lead := ui.Box(c).Size(34, 34).Radius(17).Padding(7)
			if style == "B" {
				lead.Background(ui.RGBA(199, 116, 79, .22))
			}
			if style == "C" {
				lead.Background(ui.RGBA(glyphColor.R, glyphColor.G, glyphColor.B, .14))
			}
			lead.Children(func() { ui.Icon(c, appui.NoticeGlyph(x.n.Kind)).Size(20, 20).TextColor(glyphColor) })
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, x.n.Title).FontSize(14).Bold().MaxLines(1).TextColor(fg)
				if state == island.Banner {
					ui.Text(c, x.n.Body).FontSize(13).MaxLines(1).TextColor(muted)
				} else {
					kind := map[string]string{"ping": "Ping", "task": "Tugas", "meeting": "Rapat", "reminder": "Pengingat", "deadline": "Tenggat"}[x.n.Kind]
					if kind == "" {
						kind = "Pengingat"
					}
					ui.Text(c, "Space · "+kind).FontSize(13).TextColor(muted)
				}
			})
			if queued > 0 {
				ui.Text(c, fmtIslandQueue(queued)).FontSize(13).TextColor(muted)
			}
		})
		if state == island.Expanded {
			ui.Text(c, x.n.Body).FontSize(13).MaxLines(3).TextColor(fg)
			ui.Row(c).Gap(8).Children(func() {
				primary := fg
				if p.d.notificationPrefs.Load().(devicePreferences).IslandStyle == "B" {
					primary = accent
				}
				// C's surface is translucent: its color would leave the label faint.
				open := x.n.OpenLabel
				if open == "" {
					open = "Buka"
				}
				if ui.Button(c, open).Background(primary).TextColor(opaque(bg)).Clicked() && interactive {
					p.act(x.n.Link)
					p.dismiss()
				}
				for _, a := range x.n.Actions {
					if ui.Button(c, a.Label).Clicked() && interactive {
						p.act(a.Link)
						p.dismiss()
					}
				}
				if ui.Button(c, "Tutup").Clicked() && interactive {
					p.dismiss()
				}
			})
		}
	})
}

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
func islandPalette(style string, dark bool) (ui.Color, ui.Color, ui.Color, ui.Color) {
	fg, muted := ui.RGB(245, 245, 247), ui.RGB(185, 185, 191)
	switch style {
	case "B":
		return ui.RGB(36, 33, 28), ui.RGB(246, 241, 230), ui.RGB(207, 198, 182), ui.RGB(199, 116, 79)
	case "C":
		// No blur behind it any more (islandClearBackdrop): a denser tint
		// keeps the text legible over any wallpaper.
		if !dark {
			return ui.RGBA(250, 249, 246, .9), ui.RGB(29, 29, 31), ui.RGB(80, 80, 84), ui.RGB(29, 29, 31)
		}
		return ui.RGBA(30, 30, 32, .88), fg, muted, ui.RGB(255, 159, 10)
	default:
		return ui.RGB(0, 0, 0), fg, muted, ui.RGB(255, 159, 10)
	}
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

func (d *desktopDevice) islandSettings(c *ui.Context) {
	deviceSection(c, "Dynamic Island", "Kabar di atas tengah layar. Windows menerima notifikasi saat island mati, layar penuh, presentasi, atau Focus Assist.", func() {
		label := "Tampilkan Dynamic Island: mati"
		if d.prefs.IslandEnabled {
			label = "Tampilkan Dynamic Island: aktif"
		}
		if ui.Button(c, label).Clicked() {
			d.prefs.IslandEnabled = !d.prefs.IslandEnabled
			d.save()
			if !d.prefs.IslandEnabled {
				d.island.reset(true)
			} else {
				d.island.wake()
			}
		}
		ui.Text(c, "Gaya Dynamic Island").FontSize(13).Bold()
		ui.Row(c).Gap(8).Children(func() {
			for _, s := range []string{"A", "B", "C"} {
				label := map[string]string{"A": "A · Hitam", "B": "B · Aksen Space", "C": "C · Kaca"}[s]
				if d.prefs.IslandStyle == s {
					label += " ✓"
				}
				if ui.Button(c, label).Clicked() {
					d.prefs.IslandStyle = s
					d.save()
					if d.island.win != nil {
						d.island.win.Update(func() {})
					}
				}
			}
		})
		label = "Selalu tampil: mati"
		if d.prefs.IslandAlwaysOn {
			label = "Selalu tampil: aktif"
		}
		if ui.Button(c, label).Clicked() {
			d.prefs.IslandAlwaysOn = !d.prefs.IslandAlwaysOn
			d.save()
			if d.prefs.IslandAlwaysOn {
				d.island.wake()
			} else if d.island.win != nil {
				d.island.win.Update(func() {})
			}
		}
		ui.Text(c, "Selalu tampil: Ace menunggu sebagai pil kecil di atas tengah layar dan berubah menjadi kabar saat ada notifikasi. Klik untuk membuka Space. Island menyingkir selama aplikasi layar penuh, lalu kembali.").FontSize(12).TextColor(c.Theme().TextMuted)
		label = "Gantikan toast Windows: mati"
		if d.prefs.IslandReplaceToast {
			label = "Gantikan toast Windows: aktif"
		}
		if ui.Button(c, label).Clicked() {
			d.prefs.IslandReplaceToast = !d.prefs.IslandReplaceToast
			d.save()
		}
		label = "Kurangi gerak: ikut Windows"
		if d.prefs.IslandReduceMotion {
			label = "Kurangi gerak: aktif"
		}
		if ui.Button(c, label).Clicked() {
			d.prefs.IslandReduceMotion = !d.prefs.IslandReduceMotion
			d.save()
			if d.island.win != nil {
				d.island.win.Update(func() {})
			}
		}
		if ui.Button(c, "Coba Dynamic Island").Clicked() {
			_ = d.app.Notifier.Present(appui.Notice{Kind: appui.NoticeTest, Title: "Dynamic Island siap", Body: "Arahkan kursor selama 300 ms atau klik untuk membuka detail.", Tag: "island-test"})
		}
	})
}
