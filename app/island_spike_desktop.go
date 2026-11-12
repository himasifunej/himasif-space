//go:build !js

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

// An isolated native-window spike: no account, API, tray, protocol registration,
// notification shortcuts or preference writes. It closes itself after 45 s.
func runIslandSpike() bool {
	mode := ""
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--island-spike=") {
			mode = strings.TrimPrefix(arg, "--island-spike=")
		}
	}
	if mode == "" {
		return false
	}
	mygo.App.WhenReady(func() {
		anchor := mygo.NewWindow(mygo.WindowOptions{Title: "Space spike anchor", Width: 640, Height: 480, Hidden: true, Content: ui.View(func(c *ui.Context) { ui.Box(c).Fill().Background(ui.RGB(36, 33, 28)) })})
		anchor.ShowInactive()
		// banner holds one banner; idle shows one that closes itself after
		// 2 s, and drops the window 3 s later, as an idle island does after
		// 10 minutes: what it then costs is what an idle island costs.
		if mode == "banner" || mode == "idle" {
			d := &desktopDevice{win: anchor, prefs: devicePreferences{IslandEnabled: true, IslandStyle: "C", IslandReplaceToast: true, IslandAnchor: .5}}
			mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { d.quitting = true })
			d.notificationPrefs.Store(d.prefs)
			p := &islandPresenter{d: d, environmentCheck: func() bool { return true }, idleAfter: 3 * time.Second}
			d.island = p
			p.m.Push(island.Item{Tag: "spike", Value: islandNotice{n: appui.Notice{Title: "Space · Spike", Body: "Pengukuran idle presenter native", Tag: "spike"}}}, time.Now())
			p.m.Until = time.Now().Add(time.Hour)
			if mode == "idle" {
				p.m.Until = time.Now().Add(2 * time.Second)
				// The measurement script reads whether the window is gone.
				time.AfterFunc(9*time.Second, func() {
					anchor.Update(func() {
						_ = os.WriteFile(filepath.Join(filepath.Dir(os.Args[0]), "island-idle-state.txt"), []byte(fmt.Sprintf("destroyed=%v\n", p.win == nil)), 0o644)
					})
				})
			}
			p.ensureWindow()
			p.show()
			p.shown = true
			p.win.Update(func() {})
		}
		// ace is Ace at rest with "Selalu tampil": breathing, blinking,
		// glancing; what it costs is what an idle Space pays for its island.
		// ace-still is the same with reduced motion: blinks only.
		// ace-timer, ace-media, ace-split and ace-ping show Ace with an
		// activity, ace-*-open its details, to look at each scene.
		if strings.HasPrefix(mode, "ace") {
			d := &desktopDevice{win: anchor, prefs: devicePreferences{IslandEnabled: true, IslandStyle: "A", IslandAlwaysOn: true, IslandReduceMotion: strings.HasSuffix(mode, "-still"), IslandAnchor: .5}}
			mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { d.quitting = true })
			// ace-hitam, ace-kaca, ace-custom: Ace at rest in that style (the
			// idle CPU of each); plain ace follows the theme.
			for _, th := range []string{islandThemeBlack, islandThemeGlass, islandThemeCustom} {
				if strings.HasSuffix(mode, "-"+th) || strings.HasSuffix(mode, "-"+th+"-blur") {
					d.prefs.IslandTheme = th
				}
			}
			if d.prefs.IslandTheme == islandThemeCustom {
				d.prefs.IslandColor, d.prefs.IslandOpacity = "#14213d", .7
			}
			d.notificationPrefs.Store(d.prefs)
			// ace-fullscreen: as if a fullscreen app covered the screen.
			fullscreen := strings.Contains(mode, "fullscreen")
			p := &islandPresenter{d: d, environmentCheck: func() bool { return !fullscreen }}
			d.island = p
			start := time.Now()
			timer := strings.Contains(mode, "timer") || strings.Contains(mode, "split")
			media := strings.Contains(mode, "media") || strings.Contains(mode, "split")
			p.spikeLive = func(now time.Time) (appui.IslandTimer, bool, islandMedia, bool) {
				left := 25*time.Minute - now.Sub(start)
				t := appui.IslandTimer{Title: "Rapat Divisi", Agenda: "Evaluasi proker", Clock: fmt.Sprintf("%02d:%02d", int(left.Minutes()), int(left.Seconds())%60), Countdown: true, Control: true}
				m := islandMedia{Title: "Judul lagu (contoh)", Artist: "Artis", App: "Spotify", Playing: true, Position: time.Minute, Duration: 3 * time.Minute, At: start, Since: start}
				return t, timer, m, media
			}
			if strings.Contains(mode, "ping") {
				p.m.Push(island.Item{Tag: "spike", Priority: 3, Value: islandNotice{n: appui.Notice{Kind: "ping", Title: "Ping · Mediatek", Body: "Cek grup divisi ya, ada info penting.", Tag: "spike"}}}, time.Now())
				p.m.Until = time.Now().Add(time.Hour)
			}
			p.spikeOpen = strings.HasSuffix(mode, "-open")
			// ace-kaca-blur: Kaca over Windows' acrylic, to see whether it
			// stays inside the pill (no box) on this PC.
			p.spikeBlur = strings.HasSuffix(mode, "-blur")
			// ace-inbox: Ace at rest with eight notices kept unseen ("+8").
			if strings.Contains(mode, "inbox") {
				d.activeUser = id.New()
				p.inbox.loaded = true
				for i := range 8 {
					p.inbox.box.add(appui.Notice{Kind: []string{"ping", "task", "meeting", "reminder"}[i%4], Title: fmt.Sprintf("Kabar %d", i+1), Body: "Contoh kabar yang belum dibuka", Tag: fmt.Sprintf("spike:%d", i), AccountID: d.activeUser.String()}, time.Now())
				}
			}
			p.resume()
			// ace-slide: Kiri at 4 s, Kanan at 7 s, Tengah at 10 s, then
			// rest, as the settings' choice moves Ace; ace-slide-still with
			// reduced motion. What the slide costs, and that it stops.
			if strings.HasPrefix(mode, "ace-slide") {
				for _, step := range []struct {
					at     time.Duration
					anchor float64
				}{{4 * time.Second, 0}, {7 * time.Second, 1}, {10 * time.Second, .5}} {
					step := step
					time.AfterFunc(step.at, func() {
						anchor.Update(func() {
							d.prefs.IslandAnchor = step.anchor
							d.notificationPrefs.Store(d.prefs)
							if p.win != nil {
								p.win.Update(func() {})
							}
						})
					})
				}
			}
		}
		if mode == "hidden" || mode == "pill" {
			w := mygo.NewWindow(mygo.WindowOptions{Title: "Space island spike", Width: 200, Height: 36, Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, DisableResize: true, Panel: true, Content: ui.View(func(c *ui.Context) { ui.Column(c).Fill().Radius(18).Background(ui.RGBA(30, 30, 32, .78)) })})
			a := mygo.Screen.PrimaryDisplay().WorkArea
			w.SetPosition(a.X+(a.Width-200)/2, a.Y+8)
			islandPrepareWindow(w.NativeHandle())
			islandWindowShape(w.NativeHandle(), [4]float64{200, 36, 18, 0}, 0, 0, 200)
			islandClearBackdrop(w.NativeHandle())
			if mode == "pill" {
				w.ShowInactive()
			}
		}
		go func() { time.Sleep(45 * time.Second); mygo.App.Quit() }()
	})
	_ = mygo.App.Run()
	return true
}
