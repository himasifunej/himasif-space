//go:build !js

package main

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
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

// islandEnvelope is the island's window, in DIP: room for the largest
// shape (a notice's details, 440×184) and the spring's overshoot past it.
// The window keeps this size for life; the shape moves and grows inside
// it, and the window's region follows the shape, which is all Windows shows
// of the window and all it hit-tests. 0.4.5-rc2 resized and re-centred the
// window at every frame of the spring: Windows then showed, until the
// renderer drew a frame of the new size, the old frame stretched over it
// and the part just uncovered undrawn (the glitching eyes and the box seen
// while the island expanded), and the pill's pixels moved with the
// window's rounding.
var islandEnvelope = [2]int{472, 200}

// islandGroupWidth is how wide a shape is: the pill, and the circle 8 DIP
// right of it.
func islandGroupWidth(g [4]float64) float64 {
	if g[3] > .5 {
		return g[0] + island.BubbleGap + g[3]
	}
	return g[0]
}

// islandOffset is where the shape starts in the window: as far along its
// room as the island is along the top of the screen (anchor 0 at the left
// edge, .5 the middle, 1 the right edge), so it grows away from the edge it
// sits at and stays centred in the middle. room is the width the "+N"
// added to Ace at rest (island.BadgeRoom, springing with the shape): it
// grows to the right, the eyes staying put, unless that would leave the
// window at the right edge.
func islandOffset(g [4]float64, room, anchor float64) float64 {
	free := float64(islandEnvelope[0]) - islandGroupWidth(g)
	return math.Max(0, math.Min(free, math.Max(0, free+room)*anchor))
}

// islandAnchor is a saved position along the top of the screen, within
// 0 (left) and 1 (right); the middle where it is not a number.
func islandAnchor(v float64) float64 {
	if math.IsNaN(v) {
		return .5
	}
	return math.Max(0, math.Min(1, v))
}

// The island's colors, of its style (islandPaletteFor), set at the start
// of every frame on the UI thread: the pill, its eyes and text, the ring,
// the buttons. The activities keep the system's accents.
var (
	islandInk         = ui.RGB(0, 0, 0)
	islandText        = ui.RGB(245, 245, 247)
	islandMuted       = ui.RGBA(245, 245, 247, .72)
	islandRing        = ui.RGBA(255, 255, 255, .06)
	islandButton      = ui.RGB(44, 44, 46)
	islandButtonHover = ui.RGB(62, 62, 66)
	islandButtonPress = ui.RGB(78, 78, 84)
	islandPing        = ui.RGB(255, 159, 10)
	islandMeeting     = ui.RGB(48, 209, 88)
	islandMusic       = ui.RGB(10, 132, 255)
	islandOver        = ui.RGB(255, 69, 58)
)

type islandNotice struct {
	n        appui.Notice
	fallback func()
	// count is how many notices of a burst this one gathers (Notice.Group),
	// since first; latest is the last one's title.
	count  int
	first  time.Time
	latest string
}

// islandBurst is how close notices alike must come to gather into one.
const islandBurst = 10 * time.Second

// gather folds n into the queued or shown notice of its burst, if one came
// within islandBurst: "8 rapat baru". It returns the item to push (under
// the burst's tag, replacing that notice in place) and whether it did.
func (p *islandPresenter) gather(n appui.Notice, fallback func(), now time.Time) (island.Item, bool) {
	if n.Group == "" {
		return island.Item{}, false
	}
	// A burst's tag is its group and when it began: a later burst of the
	// same group is a notice of its own.
	prefix := "group:" + n.Group + "#"
	var found *island.Item
	if p.m.Current != nil && strings.HasPrefix(p.m.Current.Tag, prefix) && p.m.Closing.IsZero() {
		found = p.m.Current
	}
	for i := range p.m.Queue {
		if strings.HasPrefix(p.m.Queue[i].Tag, prefix) {
			found = &p.m.Queue[i]
		}
	}
	if found == nil || now.Sub(found.Value.(islandNotice).first) > islandBurst {
		return island.Item{}, false
	}
	x := found.Value.(islandNotice)
	x.count++
	x.latest = n.Title
	// One snooze link per delivery: a gathered notice offers none.
	x.n.Actions = nil
	if prev := x.fallback; prev != nil || fallback != nil {
		x.fallback = func() {
			if prev != nil {
				prev()
			}
			if fallback != nil {
				fallback()
			}
		}
	}
	return island.Item{Tag: found.Tag, Priority: found.Priority, Value: x}, true
}

// noticeTitle is a notice's headline: a gathered burst's count and label,
// with its Space (on the UI thread), else its own title.
func (p *islandPresenter) noticeTitle(x islandNotice) string {
	if x.count < 2 {
		return x.n.Title
	}
	t := strconv.Itoa(x.count) + " " + x.n.GroupLabel
	if p.d.app != nil {
		if name := p.d.app.SpaceName(x.n.Space); name != "" {
			t += " di " + name
		}
	}
	return t
}

// noticeBody is a gathered burst's latest title, else the notice's body.
func noticeBody(x islandNotice) string {
	if x.count < 2 {
		return x.n.Body
	}
	return "Terbaru: " + x.latest
}

// All machine access is serialized. Native window and animation fields belong
// only to MyGo's UI thread; delivery workers only enqueue and request Update.
type islandPresenter struct {
	// hold is the next offered notice's Item.Hold (offerHeld).
	hold time.Duration
	// inbox keeps what lapsed unseen (island_inbox_view_desktop.go).
	inbox inboxState
	// Only the isolated spike supplies an override, to measure rendering even
	// when this Windows session currently has Focus Assist or fullscreen on.
	environmentCheck func() bool
	// spikeLive stands in for the app's timer and Windows' media in the
	// spike, which has neither.
	spikeLive func(time.Time) (appui.IslandTimer, bool, islandMedia, bool)
	spikeOpen bool          // the spike holds the details open
	spikeBlur bool          // the spike tries Windows' acrylic (ace-kaca-blur)
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
	region                            [6]float64 // the shape and offset the window's region was cut to
	// room is the width the "+N" adds to Ace at rest, springing with the
	// shape toward roomTarget from roomFrom (islandOffset).
	room, roomVelocity, roomFrom, roomTarget float64
	// tired is Ace ikut capek: its pose at rest (pose) as unread notices
	// pile up; awake whether Ace breathed last frame.
	tired island.Tired
	pose  island.Pose
	awake bool
	// anchor is where the island sits along the top of the screen (0 left,
	// 1 right), placed at; drag follows a press on the pill that moves it.
	anchor, placed float64
	// slide carries Ace to a new anchor; jelly wobbles it as it moves,
	// from its speed (prevAnchor a frame ago, moveDir its last direction);
	// motionAt is their last frame.
	slide      island.Slide
	jelly      island.Jelly
	prevAnchor float64
	moveDir    float64
	motionAt   time.Time
	drag       islandDrag
	// eyeH is the eyes' height, easing to what the moment asks for.
	eyeH float64
	// key names what the island shows (scene, details, which notice): a
	// new one fades the content in.
	key      string
	scene    island.Scene
	expanded bool
	shown    bool // the window shows a frame of this presenter's
	watching bool // a goroutine waits for fullscreen to end
	swap     bool // the second activity was clicked: it leads
	// Ace's life: blinks and glances, the gaze after the pointer, breath.
	blink    island.Eyes
	gaze     island.Gaze
	expander island.Expander
	// today: "Hari ini" was chosen over the inbox, for as long as the
	// details stay open; todayPending opens them once the notice shown
	// has left (showToday).
	today, todayPending bool
	breath              island.Breather
	born                time.Time
	happyUntil          time.Time
	media               *islandMediaWatcher
	mediaActive         bool
}

// islandDrag is a press on the pill: where the pointer and the island were
// when it began (physical pixels, an anchor), and whether it has moved far
// enough to be a drag rather than a click.
type islandDrag struct {
	on, moved bool
	x0, a0    float64
}

// islandDragSlop is how far, in physical pixels, a press moves before it
// drags the island instead of clicking it.
const islandDragSlop = 6

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
	now := time.Now()
	item, gathered := p.gather(n, fallback, now)
	if !gathered {
		if n.Group != "" {
			n.Tag = "group:" + n.Group + "#" + strconv.FormatInt(now.UnixNano(), 36) // the first of a possible burst
		}
		item = island.Item{Tag: n.Tag, Priority: priority, Hold: p.hold, Value: islandNotice{n: n, fallback: fallback, count: 1, first: now, latest: n.Title}}
	}
	ok := p.m.Push(item, now)
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
	w, h := islandEnvelope[0], islandEnvelope[1]
	p.win = mygo.NewWindow(mygo.WindowOptions{Title: "Dynamic Island · Space", Width: w, Height: h, Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, DisableResize: true, Panel: true, Content: ui.View(p.view)})
	islandPrepareWindow(p.win.NativeHandle())
	islandClearBackdrop(p.win.NativeHandle())
	if p.spikeBlur {
		// Only the spike tries the system's acrylic, to see on a real
		// Windows whether it stays inside the region (it did not in 0.4.4).
		islandSpikeAcrylic(p.win.NativeHandle())
	}
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
	p.eyeH = 0
	p.drag = islandDrag{}
	p.region = [6]float64{} // a new or hidden window is cut again
	p.room, p.roomVelocity, p.roomFrom, p.roomTarget = 0, 0, 0, 0
	p.tired, p.pose = island.Tired{}, island.RestPose()
	p.expander.Reset()
}

// show puts the hidden window on screen, cut to its shape.
func (p *islandPresenter) show() {
	if p.win.IsVisible() {
		return
	}
	p.position()
	p.cut()
	p.win.ShowInactive()
}

// cut fits the window's region to the shape where the island draws it.
func (p *islandPresenter) cut() {
	g, ox, oy := p.shape()
	key := [6]float64{g[0], g[1], g[2], g[3], ox, oy}
	if p.region == key {
		return
	}
	islandWindowShape(p.win.NativeHandle(), g, ox, oy, islandEnvelope[0])
	p.region = key
}

// shape is the pill as drawn, and where it starts in the window (left,
// top): the geometry, stretched by the jelly about its middle, sagging and
// flattening about its bottom middle as Ace gets tired (pose), and kept
// inside the window. The region follows it, so the window never shows
// past it.
func (p *islandPresenter) shape() ([4]float64, float64, float64) {
	g := p.geometry
	ox := islandOffset(g, p.room, p.anchor)
	pose := p.pose
	if pose.SX == 0 {
		pose = island.RestPose()
	}
	sx, sy := pose.SX, pose.SY
	if p.jelly.Stretch != 0 {
		jx, jy := p.jelly.Scale()
		sx, sy = sx*jx, sy*jy
	}
	if sx == 1 && sy == 1 && pose.X == 0 && pose.Y == 0 {
		return g, ox, 0
	}
	w, h := g[0]*sx, math.Min(g[1]*sy, float64(islandEnvelope[1]))
	d := [4]float64{w, h, math.Min(g[2], h/2), g[3]}
	ox += (g[0]-w)/2 + pose.X
	oy := math.Max(0, math.Min(g[1]*(1-pose.SY)+pose.Y, float64(islandEnvelope[1])-h))
	return d, math.Max(0, math.Min(ox, float64(islandEnvelope[0])-islandGroupWidth(d))), oy
}

// travel is how far, in pixels, the island goes from the left edge to the
// right one, for its speed; a guess where the screen does not say.
func (p *islandPresenter) travel() float64 {
	if t := islandTravel(p.d.win.NativeHandle(), !p.d.win.IsVisible(), islandGroupWidth(p.geometry)); t > 0 {
		return t
	}
	return 1200
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
	p.inbox.open = false
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

// position puts the window at the top of the screen, at its anchor. It is
// asked when the island shows, when the displays change and while it is
// dragged, never while it changes shape.
func (p *islandPresenter) position() {
	w, h := islandEnvelope[0], islandEnvelope[1]
	p.placed = p.anchor
	if islandPosition(p.win.NativeHandle(), p.d.win.NativeHandle(), !p.d.win.IsVisible(), w, h, p.anchor) {
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
	p.win.SetPosition(a.X+islandTopGap+int(p.anchor*float64(a.Width-2*islandTopGap-w)+.5), a.Y+islandTopGap)
}

func (p *islandPresenter) refreshPosition() {
	if p.win == nil {
		return
	}
	p.position()
	p.region = [6]float64{}
	p.cut()
	p.win.Update(func() {})
}

// dragFrame follows a press on the pill (held): once it has moved past
// the slop, the island slides along the top of the screen after the
// pointer, from the left edge to the right one, and keeps the place when
// it is let go. It reports whether the press moved the island, then and
// in the frame it ends, whose click it is not.
func (p *islandPresenter) dragFrame(held bool) bool {
	if !held {
		if !p.drag.on {
			return false
		}
		moved := p.drag.moved
		p.drag = islandDrag{}
		if moved {
			p.d.prefs.IslandAnchor = p.anchor
			p.d.save()
		}
		return moved
	}
	x, ok := islandCursorX()
	if !ok {
		return p.drag.moved
	}
	if !p.drag.on {
		p.drag = islandDrag{on: true, x0: x, a0: p.anchor}
		return false
	}
	if !p.drag.moved && math.Abs(x-p.drag.x0) < islandDragSlop {
		return false
	}
	p.drag.moved = true
	if travel := islandTravel(p.d.win.NativeHandle(), !p.d.win.IsVisible(), islandGroupWidth(p.geometry)); travel > 0 {
		p.anchor = islandAnchor(p.drag.a0 + (x-p.drag.x0)/travel)
	}
	if p.anchor != p.placed {
		p.position()
	}
	return true
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
	inbox         int // notices kept unseen, for the account
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
	// The saved place, unless a drag holds the island. A new one (Kiri,
	// Tengah, Kanan, "Ke tengah") slides the window there on a spring.
	mdt := islandFrameStep(p.motionAt, now)
	p.motionAt = now
	sliding := false
	if p.drag.on {
		p.slide.Jump(p.anchor)
	} else {
		target := islandAnchor(prefs.IslandAnchor)
		if !p.shown {
			p.slide.Jump(target)
		} else {
			sliding = p.slide.Step(target, mdt, now, reduced)
		}
		p.anchor = p.slide.Anchor
		if p.shown && p.anchor != p.placed {
			p.position()
		}
	}
	// Sliding or dragged, Ace wobbles like jelly with its speed; never
	// under reduced motion.
	wobbling := false
	if reduced || !p.shown {
		p.jelly = island.Jelly{}
	} else {
		speed := 0.
		if mdt > 0 && p.anchor != p.prevAnchor {
			speed = (p.anchor - p.prevAnchor) / mdt * p.travel()
			p.moveDir = math.Copysign(1, speed)
		}
		wobbling = p.jelly.Step(speed, mdt)
	}
	p.prevAnchor = p.anchor
	px, py, pointerOK := p.pointer()
	// A press that drags the island neither hovers nor opens it.
	overPill := pointerOK && !p.drag.moved && insideIsland(px, py, [3]float64{p.geometry[0], p.geometry[1], p.geometry[2]})
	f := p.frame(c, now, prefs, overPill)
	// Notices kept unseen hold Ace on screen even without "Selalu tampil".
	if f.scene == island.SceneIdle && !prefs.IslandAlwaysOn && f.notice == nil && f.inbox == 0 {
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
	if p.openToday(f) {
		c.After(0)
	}
	bubble := 0.
	if f.second != island.SceneIdle {
		bubble = island.BubbleSize
	}
	// Ace at rest grows to the right with a "+N", clear of its eyes, and
	// back once the inbox is empty; tired, it sags (island.Tired).
	rest := f.scene == island.SceneIdle && !f.expanded && f.notice == nil
	geometry, room := island.Geometry(f.scene, f.expanded), 0.
	if rest && f.inbox > 0 {
		geometry[0] += island.BadgeRoom
		room = island.BadgeRoom
	}
	th := island.Thresholds(prefs.AceTiredRatio, prefs.AceTiredTop, prefs.AceTiredHeavy, prefs.AceTiredSleepy)
	pose, badge := p.tired.Frame(now, f.inbox, th, prefs.AceTired, rest, p.awake, reduced)
	p.pose = pose
	if target := islandShape(geometry, bubble); target != p.target || room != p.roomTarget || !p.shown {
		p.start, p.target, p.changed = p.geometry, target, now
		p.roomFrom, p.roomTarget = p.room, room
		if !p.shown {
			// Appearing, the island takes its shape at once.
			p.geometry, p.velocity = target, [4]float64{}
			p.room, p.roomVelocity = room, 0
			ex, ey := island.EyesAt(f.scene, f.expanded, target[0]-room, target[1])
			p.eyes, p.eyesVelocity = [2]float64{ex, ey}, [2]float64{}
		}
		p.last = now
	}
	moving := p.step(now, reduced)
	p.show()
	p.shown = true

	// Ace's eyes.
	ex, ey := island.EyesAt(f.scene, f.expanded, p.target[0]-p.roomTarget, p.target[1])
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
	// Ace speaking in the onboarding has a face of its own.
	faceH, wink, down, speaking := aceFace(f.notice)
	if speaking {
		eyeH, gy = faceH, gy+down
		if faceH < island.EyeHeight {
			open = 1
		}
	}
	// The eyes widen and narrow over about 100 ms, as the pointer comes and
	// goes, rather than jumping a frame: the jump read as a glitch.
	if p.eyeH == 0 || reduced {
		p.eyeH = eyeH
	} else if d := eyeH - p.eyeH; math.Abs(d) > .05 {
		p.eyeH += d * (1 - math.Exp(-dt/.035))
		moving = true
	} else {
		p.eyeH = eyeH
	}
	eyeH = p.eyeH
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

	p.awake = breathing
	alpha := float32(island.Ease((now.Sub(p.contentChanged).Seconds() - .06) / .26))
	if reduced {
		alpha = float32(math.Min(1, now.Sub(p.contentChanged).Seconds()/.15))
	}
	if !f.closing.IsZero() {
		alpha *= float32(1 - island.ExitEase(now.Sub(f.closing).Seconds()/.16))
	}
	theme := *c.Theme()
	pal := islandPaletteFor(prefs, p.spaceTheme(c))
	pal.use()
	theme.Dark = pal.dark
	theme.Text, theme.TextMuted = islandText, islandMuted
	theme.Surface, theme.SurfaceHover = islandButton, islandButtonHover
	theme.Border = islandRing
	theme.FontSize = 13
	theme.Accent, theme.AccentText = islandText, opaque(islandInk)
	c.SetTheme(&theme)

	drawn, dox, doy := p.shape()
	gw, gh, gr := float32(drawn[0]), float32(drawn[1]), float32(drawn[2])
	ox, oy := float32(dox), float32(doy)
	// The eyes stay in the middle of the stretched pill and trail the
	// motion a little.
	eyeShift := (drawn[0]-p.geometry[0])/2 + p.jelly.EyeLag(p.moveDir)
	eyeScaleY := drawn[1] / p.geometry[1]
	dragging := false
	ui.Box(c).Fill().Children(func() {
		pill := ui.Box(c).Absolute().Left(ox).Top(oy).Size(gw, gh).Radius(gr).Background(tiredInk(islandInk, pose.Warm)).Border(1, islandRing).Clip()
		_ = pill.Hovered() // enter/leave redraw without a timer
		_, _, held := pill.Dragged()
		dragged := p.dragFrame(held)
		dragging = p.drag.on
		if pill.Clicked() && !dragged {
			p.click(now, f)
			c.After(0)
		}
		pill.Children(func() {
			// Kaca reads as glass without the system's blur: a soft light
			// sheen over the top half, under a light edge (the ring).
			if prefs.IslandTheme == islandThemeGlass {
				ui.Box(c.Key("sheen")).Absolute().Left(0).Top(0).Size(gw, gh*.5).Radius(gr, gr, 0, 0).Background(ui.RGBA(255, 255, 255, .09))
			}
			// The eyes: two capsules, nodding to music, blinking, looking.
			// Tired, the lids drop and slant and the eyes look down.
			open = math.Min(open, 1-pose.Close)
			h := float32(math.Max(1.4, eyeH*open*scale*pose.Pop))
			w := float32(island.EyeWidth * scale * pose.Pop)
			cy := float32((p.eyes[1]+gy+bop)*eyeScaleY + pose.EyeY)
			for _, side := range [2]float64{-1, 1} {
				cx := float32(p.eyes[0] + side*(island.EyePair-island.EyeWidth)/2*scale + gx + eyeShift + pose.GlanceX)
				h := h
				if wink && side < 0 {
					h = 1.4
				}
				tiredEye(c, cx, cy, w, h, float32(pose.Lid), float32(-side*pose.Angle))
			}
			if rest {
				tiredParticles(c, pose, float32(p.eyes[0]+eyeShift), gw, gh)
			}
			p.content(c, f, now, alpha, reduced)
			if rest {
				inboxBadge(c, badge, gw, gh, pose)
			}
		})
		if p.geometry[3] > 1 {
			b := float32(p.geometry[3])
			dot := ui.ButtonBase(c).Absolute().Left(ox+gw+island.BubbleGap).Top(0).Size(b, b).Radius(b/2).Background(islandInk).Border(1, islandRing).Clip().Center()
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
	if d := p.tired.Next(now, breathing, reduced); d > 0 {
		next = min(next, d)
	}
	if moving || gazing || alpha < 1 || !f.closing.IsZero() || open < 1 || now.Before(p.happyUntil) || dragging || sliding || wobbling {
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
	account := p.account() // before p.mu: offer takes p.d.mu, then p.mu
	p.mu.Lock()
	p.m.Tick(now, overPill || p.spikeOpen, false, c.Shortcut(0, ui.KeyEscape))
	p.keepLapsed(now)
	f.inbox = p.inboxCount(account)
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
		if !p.expander.Expanded {
			p.today = false
		}
		f.expanded = p.expander.Expanded || p.spikeOpen
		// Unread first (owner's rc.2/rc.3 report: "+1" led to Hari ini).
		// Ace at rest opens its details after 300 ms under the pointer,
		// before a click can land, and those details were the day's,
		// with the "+N" hidden: details opened at rest with notices
		// waiting are the inbox, unless Hari ini was chosen from it.
		if f.scene == island.SceneIdle && p.expander.Expanded && f.inbox > 0 && !p.today && !p.spikeOpen {
			p.mu.Lock()
			if p.openInbox(now, account) {
				it := p.m.Current
				x := it.Value.(islandNotice)
				f.scene, f.notice, f.item, f.expanded = island.SceneNotice, &x, it, true
				f.queued, f.closing, f.until = len(p.m.Queue), p.m.Closing, p.m.Until
				f.second = island.SceneIdle
				p.expander.Reset()
			}
			p.mu.Unlock()
		}
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
	// Ace at rest with notices kept: the newest opens, to page through;
	// a click on Hari ini, chosen over them, closes it.
	account := p.account()
	p.mu.Lock()
	opened := !f.expanded && f.inbox > 0 && p.openInbox(now, account)
	p.mu.Unlock()
	if opened {
		p.today = false
		return
	}
	p.expander.Tick(now, true, true)
}

// showToday is the inbox's "Hari ini": the notice shown goes back to the
// inbox, unread, and the day's details open in its place.
func (p *islandPresenter) showToday() {
	p.mu.Lock()
	p.inbox.open = false
	p.m.Dismiss(time.Now())
	p.mu.Unlock()
	p.todayPending = true
	p.redraw()
}

// openToday opens the day's details asked for by showToday, once the
// notice shown has left; it reports whether it did.
func (p *islandPresenter) openToday(f islandFrame) bool {
	if !p.todayPending || f.notice != nil {
		return false
	}
	p.todayPending, p.today = false, true
	p.expander.Open()
	return true
}

// redraw asks the island for a frame.
func (p *islandPresenter) redraw() {
	if p.win != nil {
		p.win.Update(func() {})
	}
}

// showInbox is Hari ini's "Kabar": back to the notices waiting.
func (p *islandPresenter) showInbox() {
	account := p.account()
	p.mu.Lock()
	if p.openInbox(time.Now(), account) {
		p.today = false
		p.expander.Reset()
	}
	p.mu.Unlock()
	p.redraw()
}

// pointer is where the pointer is from the pill's top left, in DIP.
func (p *islandPresenter) pointer() (float64, float64, bool) {
	ox := islandOffset(p.geometry, p.room, p.anchor)
	if x, y, ok := islandPointer(p.win.NativeHandle(), float64(islandEnvelope[0]), float64(islandEnvelope[1])); ok {
		return x - ox, y, true
	}
	point := mygo.Screen.CursorScreenPoint()
	b := p.win.Bounds()
	return float64(point.X-b.X) - ox, float64(point.Y - b.Y), true
}

// obstructionDeadline brings the next frame forward to when fullscreen,
// still on, has lasted its grace.
func (p *islandPresenter) obstructionDeadline(now time.Time, next time.Duration) time.Duration {
	if p.obstructedSince.IsZero() {
		return next
	}
	return max(time.Millisecond, min(next, p.obstructedSince.Add(islandObstructionGrace).Sub(now)))
}

// step advances the shape a frame, cuts the window's region to it, and
// tells whether it still moves. The window itself stays as it is.
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
	if reduced {
		q := island.Ease(now.Sub(p.changed).Seconds() / .18)
		p.room, p.roomVelocity = p.roomFrom+(p.roomTarget-p.roomFrom)*q, 0
	} else {
		p.room, p.roomVelocity = island.Spring(p.room, p.roomVelocity, p.roomTarget, dt)
	}
	if math.Abs(p.room-p.roomTarget) > .1 || math.Abs(p.roomVelocity) > .1 {
		moving = true
	} else {
		p.room, p.roomVelocity = p.roomTarget, 0
	}
	p.room = math.Max(0, math.Min(p.room, island.BadgeRoom))
	p.geometry[3] = math.Max(0, p.geometry[3])
	// The spring's overshoot stays within the window.
	p.geometry[0] = math.Min(p.geometry[0], float64(islandEnvelope[0]))
	p.geometry[1] = math.Min(p.geometry[1], float64(islandEnvelope[1]))
	if p.win != nil && p.win.IsVisible() {
		p.cut()
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
	// The content rises into place as it fades in. No blur: MyGo draws a
	// blurred element into a bitmap of its own, sized in steps of 64 px as
	// the pill grows, and the text and icons jumped as the blur ended and
	// they were drawn directly again.
	if !reduced && alpha < 1 {
		box.Translate(0, -4*(1-alpha))
	}
	if !interactive {
		box.Disabled(true)
	}
	box.Children(func() {
		switch {
		case f.scene == island.SceneNotice && f.expanded:
			p.noticeDetails(c, *f.notice, f.queued)
		case f.scene == island.SceneNotice && f.notice.n.Kind == appui.NoticeAce:
			// Ace speaks: its line on two lines, nothing else.
			ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.notice.n.Body).FontSize(13).LineHeight(1.3).MaxLines(2).Grow(1).MinWidth(0)
			})
		case f.scene == island.SceneNotice:
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(10).Children(func() {
				// The kind's icon, beside the eyes: a ping, a meeting, a task.
				islandKindIcon(c, f.notice.n.Kind, 18)
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, p.noticeTitle(*f.notice)).FontSize(14).FontWeight(600).MaxLines(1)
					ui.Text(c, noticeBody(*f.notice)).FontSize(13).MaxLines(1).TextColor(islandMuted)
				})
				switch {
				case f.queued > 0:
					ui.Text(c, fmtIslandQueue(f.queued)).FontSize(13).TextColor(islandMuted)
				case f.inbox > 1:
					ui.Text(c, fmtIslandQueue(f.inbox-1)).FontSize(13).TextColor(islandMuted)
				}
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
			p.restDetails(c, now, f.inbox)
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
	b := ui.ButtonBase(c.Key("island-"+label)).Label(label).Height(32).Radius(16).Padding(0, 12).Gap(6)
	bg, fg, ring := islandActionColors(primary, b.Hovered(), b.Pressed())
	// The style's own shades, eased, and a ring of its text color for the
	// keyboard: MyGo's ring is Space's accent, foreign on Hitam or Kaca.
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).FocusRing(false)
	if b.FocusVisible() {
		b.Border(2, ring)
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

// islandActionColors are an island button's background, text and focus
// ring at rest, hovered or pressed, in the colors of the island's style:
// the primary (light on a dark pill, dark on a light one) darkens toward
// the pill as it is pressed; the others lighten on a dark pill and darken
// on a light one.
func islandActionColors(primary, hovered, pressed bool) (bg, fg, ring ui.Color) {
	if primary {
		bg, fg = islandText, opaque(islandInk)
		switch {
		case pressed:
			bg = islandMix(islandText, opaque(islandInk), .22)
		case hovered:
			bg = islandMix(islandText, opaque(islandInk), .1)
		}
		return bg, fg, islandMix(islandText, opaque(islandInk), .5)
	}
	bg, fg = islandButton, islandText
	switch {
	case pressed:
		bg = islandButtonPress
	case hovered:
		bg = islandButtonHover
	}
	return bg, fg, ui.RGBA(islandText.R, islandText.G, islandText.B, .6)
}

// islandLink is a small text button of the details, aside.
func islandLink(c *ui.Context, label string) bool {
	b := ui.ButtonBase(c.Key("island-link-"+label)).Label(label).Padding(2, 8).Radius(10).Shrink(0).FocusRing(false)
	fg := islandMuted
	if b.Hovered() || b.FocusVisible() {
		b.Background(islandButton)
		fg = islandText
	}
	if b.Pressed() {
		b.Background(islandButtonPress)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12).TextColor(fg).MaxLines(1) })
	return b.Clicked()
}

func (p *islandPresenter) noticeDetails(c *ui.Context, x islandNotice, queued int) {
	inbox := p.inboxShowing(x.n.Tag)
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			islandKindIcon(c, x.n.Kind, 16)
			ui.Text(c, p.noticeTitle(x)).FontSize(15).FontWeight(600).MaxLines(1).Grow(1).MinWidth(0)
			if queued > 0 {
				ui.Text(c, fmtIslandQueue(queued)).FontSize(13).TextColor(islandMuted)
			}
			p.inboxPager(c, x)
			// From the inbox, each notice can be marked read on its own,
			// whatever its actions.
			if inbox && p.islandAction(c, "Tandai dibaca", islandIcons["close"], false) {
				p.readInApp([]string{x.n.Tag})
				p.dismiss()
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Space · "+islandKindLabel(x.n.Kind)).FontSize(13).TextColor(islandMuted).Grow(1).MinWidth(0)
			// In the inbox: the day instead, or all read at once. With more
			// queued, all of them can go: small, aside.
			switch {
			case inbox:
				if islandLink(c, "Hari ini") {
					p.showToday()
				}
				if islandLink(c, "Tandai semua dibaca") {
					p.dismissAll()
				}
			case queued > 0:
				if islandLink(c, "Tutup semua") {
					p.dismissAll()
				}
			}
		})
		ui.Box(c).Height(8)
		ui.Text(c, noticeBody(x)).FontSize(14).MaxLines(2)
		ui.Box(c).Height(12)
		ui.Row(c).Gap(8).Children(func() {
			open := x.n.OpenLabel
			if open == "" {
				open = "Buka"
			}
			if x.count > 1 && x.n.Kind == "meeting" {
				open = "Buka Rapat"
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
			// In the inbox, "Tandai dibaca" beside the pager does it.
			if !inbox && len(x.n.Actions) < 2 && p.islandAction(c, "Tutup", nil, false) {
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
			// Open goes where the timer runs: the meeting, or the note.
			open := "Buka rapat"
			if t.Meeting == id.Nil {
				open = "Buka catatan"
			}
			if p.islandAction(c, open, nil, !t.Control) {
				p.d.win.Update(func() {
					p.d.win.Show()
					p.d.win.Focus()
					p.d.app.IslandOpenTimer(t)
				})
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
func (p *islandPresenter) restDetails(c *ui.Context, now time.Time, inbox int) {
	today := ""
	if p.d.app != nil {
		today = p.d.app.IslandToday(now)
	}
	if today == "" {
		today = "Tidak ada rapat lagi hari ini"
	}
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Hari ini").FontSize(15).FontWeight(600).Grow(1).MinWidth(0)
			// Back to the notices waiting, chosen away from.
			if inbox > 0 && islandLink(c, fmt.Sprintf("Kabar · %d", inbox)) {
				p.showInbox()
			}
		})
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
		"close":    `<path d="M6 6l12 12M18 6 6 18"/>`,
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

// islandKindLabel names where a notice comes from: "Rapat" for a
// meeting's, whatever channel carried it.
func islandKindLabel(kind string) string {
	if l, ok := map[string]string{"ping": "Ping", "task": "Tugas", "meeting": "Rapat", "reminder": "Pengingat", "deadline": "Tenggat", "update": "Pembaruan", "note": "Catatan", appui.NoticeAce: "Kenalan", appui.NoticeTest: "Percobaan"}[kind]; ok {
		return l
	}
	return "Pengingat"
}

// dismissAll closes the notice and drops the ones waiting: Tutup semua.
func (p *islandPresenter) dismissAll() {
	account := p.account()
	p.mu.Lock()
	var read []string
	if p.m.Current != nil {
		read = append(read, p.m.Current.Tag)
	}
	for _, it := range p.m.Queue {
		read = append(read, it.Tag)
	}
	p.ensureInbox(time.Now())
	for _, e := range p.inbox.box.of(account) {
		read = append(read, e.Notice.Tag)
	}
	p.m.DismissAll(time.Now())
	if p.inbox.box.clear(account) {
		p.inbox.changed = true
		p.saveInbox()
	}
	p.inbox.open = false
	p.mu.Unlock()
	p.readInApp(read)
	p.redraw()
}

func (p *islandPresenter) dismiss() {
	account := p.account()
	p.mu.Lock()
	// Acted on (Buka, Tunda, Tutup): out of the inbox; in the inbox, the
	// next one kept follows.
	if !p.handled(time.Now(), account) {
		p.m.Dismiss(time.Now())
	}
	p.mu.Unlock()
	p.redraw()
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
			if u != nil && u.Scheme == "himasif-space" && u.Host == "ace" {
				p.d.handleAceLink(u)
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

// deliveryNotice is a delivery as the island and Windows show it.
func deliveryNotice(uid id.ID, n api.Delivery) appui.Notice {
	link := notificationLink(uid, n, "open")
	x := appui.Notice{Kind: appui.DeliveryKind(n), Title: n.Title, Body: appui.NoticeDate(n.Body), Tag: uid.String() + ":" + n.ID.String(), Link: link, AccountID: uid.String(), Actions: []appui.NoticeAction{{Label: "Tunda 15 menit", Link: notificationLink(uid, n, "snooze")}}}
	x.Group, x.GroupLabel = appui.DeliveryGroup(n)
	if n.SpaceID != nil {
		x.Space = *n.SpaceID
	}
	return x
}

func (d *desktopDevice) presentDelivery(uid id.ID, n api.Delivery, link string, prefs devicePreferences) error {
	x := deliveryNotice(uid, n)
	x.Link = link
	return (appui.MultiNotifier{Base: deliveryToast{d.app.Notifier, n, link}, Extra: []appui.NoticePresenter{deliveryIsland{d.island, n, link}}, ReplaceBase: prefs.IslandReplaceToast}).Present(x)
}
