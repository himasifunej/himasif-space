//go:build !js

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

type devicePreferences struct {
	IslandEnabled      bool    `json:"islandEnabled"`
	IslandStyle        string  `json:"islandStyle"`
	IslandReplaceToast bool    `json:"islandReplaceToast"`
	IslandReduceMotion bool    `json:"islandReduceMotion"`
	Autostart          bool    `json:"autostart"`
	AceMode            string  `json:"aceMode"`
	AceName            string  `json:"aceName"`
	Mascot             string  `json:"mascot"`
	Audio              string  `json:"audio"`
	Volume             float64 `json:"volume"`
	Sound              bool    `json:"sound"`
	X, Y               int
}
type desktopDevice struct {
	island              *islandPresenter
	mu                  sync.Mutex
	deliveryMu          sync.Mutex
	notificationPrefs   atomic.Value
	activeUser          id.ID
	pendingLink         string
	root                string
	prefs               devicePreferences
	win, ace            *mygo.Window
	app                 *appui.App
	tray                *mygo.Tray
	quitting            bool
	err                 string
	notificationTest    string
	testingNotification bool
	volume              string
	current             *api.Delivery
	expandedUntil       time.Time
	frames              []*ui.Bitmap
	delays              []time.Duration
	frame               int
	nextFrame           time.Time
	expanded            bool
	journal             map[string]string
}

func atomicDeviceFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".space-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
func attachDevice(win *mygo.Window, app *appui.App) {
	root, err := os.UserConfigDir()
	if err != nil {
		return
	}
	d := &desktopDevice{root: filepath.Join(root, "himasif-space", "device"), win: win, app: app, journal: map[string]string{}, prefs: devicePreferences{AceMode: "hidden", AceName: "Ace", Volume: .45, Sound: true, IslandEnabled: true, IslandStyle: "C", IslandReplaceToast: true}}
	if data, err := os.ReadFile(filepath.Join(d.root, "preferences.json")); err == nil {
		_ = json.Unmarshal(data, &d.prefs)
	}
	if d.prefs.IslandStyle != "A" && d.prefs.IslandStyle != "B" && d.prefs.IslandStyle != "C" {
		d.prefs.IslandStyle = "C"
	}
	d.island = &islandPresenter{d: d}
	if data, err := os.ReadFile(filepath.Join(d.root, "delivery-journal.json")); err == nil {
		_ = json.Unmarshal(data, &d.journal)
	}
	if d.journal == nil {
		d.journal = map[string]string{}
	}
	if d.prefs.Volume < 0 || d.prefs.Volume > 1 {
		d.prefs.Volume = .45
	}
	d.volume = strconv.Itoa(int(d.prefs.Volume * 100))
	if d.prefs.AceName == "" {
		d.prefs.AceName = "Ace"
	}
	d.notificationPrefs.Store(d.prefs)
	// Keep an opted-in startup entry pointing at the executable currently
	// running, including after moving a portable app or installing an update.
	if d.prefs.Autostart {
		if err := setDeviceAutostart(true); err != nil {
			d.err = "Autostart belum diperbarui: " + err.Error()
		}
	}
	d.loadMascot()
	d.ace = mygo.NewWindow(mygo.WindowOptions{Title: "Ace · Space", Width: 300, Height: 240, Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, DisableResize: true, Content: ui.View(d.aceView)})
	d.restorePosition()
	d.ace.OnMove(func() {
		if d.prefs.AceMode == "floating" {
			d.prefs.X, d.prefs.Y = d.ace.Position()
			d.save()
		}
	})
	d.ace.OnClose(func(e *mygo.CloseEvent) {
		if !d.quitting {
			e.PreventDefault()
			d.prefs.AceMode = "hidden"
			d.ace.Hide()
			d.save()
		}
	})
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Buka Space", Click: func(*mygo.MenuItem, *mygo.Window) { win.Show(); win.Focus() }},
		{Label: "Tampilkan Ace", Click: func(*mygo.MenuItem, *mygo.Window) {
			d.prefs.AceMode = "floating"
			d.restorePosition()
			d.ace.ShowInactive()
			d.save()
		}},
		{Label: "Ace di tepi layar", Click: func(*mygo.MenuItem, *mygo.Window) {
			d.prefs.AceMode = "edge"
			d.restorePosition()
			d.ace.ShowInactive()
			d.save()
		}},
		{Label: "Sembunyikan Ace", Click: func(*mygo.MenuItem, *mygo.Window) { d.prefs.AceMode = "hidden"; d.ace.Hide(); d.save() }},
		{Label: "Pengaturan perangkat", Click: func(*mygo.MenuItem, *mygo.Window) { win.Show(); win.Update(func() { app.OpenDeviceSettings() }) }},
		mygo.Separator(),
		{Label: "Keluar aplikasi", Click: func(*mygo.MenuItem, *mygo.Window) {
			if err := app.FlushNoteDraft(); err != nil {
				win.Show()
				return
			}
			d.quitting = true
			mygo.App.Quit()
		}},
	})
	d.tray, err = mygo.NewTray(mygo.TrayOptions{ID: "space-main", ToolTip: "Space — Ruang Kerja HIMASIF", Icon: trayIcon(), Menu: menu})
	if err != nil {
		d.err = "Tray tidak dapat dibuat: " + err.Error()
	}
	win.OnClose(func(e *mygo.CloseEvent) {
		if !d.quitting && d.tray != nil {
			e.PreventDefault()
			if app.FlushNoteDraft() == nil {
				win.Hide()
			}
		}
	})
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if err := app.FlushNoteDraft(); err != nil {
			e.PreventDefault()
			win.Show()
			return
		}
		d.quitting = true
	})
	mygo.Screen.OnDisplaysChanged(func() {
		d.restorePosition()
		d.island.refreshPosition()
	})
	win.OnMove(func() {
		if d.island.win != nil && d.island.win.IsVisible() {
			d.island.refreshPosition()
		}
	})
	app.SessionChanged = func(uid id.ID) {
		d.mu.Lock()
		old := d.activeUser
		d.activeUser = uid
		d.island.reset(false)
		d.mu.Unlock()
		d.current = nil
		d.ace.Update(func() {})
		if old != id.Nil && old != uid {
			go func() { d.deliveryMu.Lock(); defer d.deliveryMu.Unlock(); clearDeviceNotifications(old.String()) }()
		}
		if uid != id.Nil && d.pendingLink != "" {
			raw := d.pendingLink
			win.Update(func() { d.handleLink(raw) })
		}
	}
	app.Notifications = d
	app.DeviceSettings = d.settings
	mygo.App.OnOpenURL(func(raw string) { d.handleLink(raw) })
	mygo.App.OnSecondInstance(func([]string, string) { win.Show(); win.Focus() })
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "himasif-space://") {
			d.handleLink(arg)
		}
	}
	registerErr := registerNotifications()
	if registerErr != nil {
		d.err = registerErr.Error()
	}
	app.Notifier = islandNotifier{Notifier: newDesktopNotifier(win, app, registerErr == nil), p: d.island}
	if d.prefs.AceMode != "hidden" {
		d.ace.ShowInactive()
	}
	for _, arg := range os.Args[1:] {
		if arg == "--background" && d.tray != nil {
			win.Hide()
		}
	}
	done := make(chan struct{})
	mygo.App.OnQuit(func() { close(done) })
	go func() {
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				win.Update(func() { app.PollDeliveries() })
			}
		}
	}()
}
func (d *desktopDevice) save() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notificationPrefs.Store(d.prefs)
	data, err := json.Marshal(d.prefs)
	if err == nil {
		err = atomicDeviceFile(filepath.Join(d.root, "preferences.json"), data)
	}
	if err != nil {
		d.err = "Preferensi belum tersimpan: " + err.Error()
	}
}
func (d *desktopDevice) restorePosition() {
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: d.prefs.X, Y: d.prefs.Y}).WorkArea
	if area.Width == 0 {
		area = mygo.Screen.PrimaryDisplay().WorkArea
	}
	x, y := d.prefs.X, d.prefs.Y
	if x == 0 && y == 0 {
		x = area.X + area.Width - 310
		y = area.Y + area.Height - 280
	}
	x = max(area.X, min(x, area.X+area.Width-300))
	y = max(area.Y, min(y, area.Y+area.Height-240))
	if d.prefs.AceMode == "edge" {
		x = area.X + area.Width - 36
		d.ace.SetSize(36, 180)
		d.expanded = false
	} else {
		d.ace.SetSize(300, 240)
	}
	d.ace.SetPosition(x, y)
}
func (d *desktopDevice) Notify(uid id.ID, n api.Delivery) error {
	return d.notify(uid, n, true)
}

// notify shows one delivery; bell rings the device's bell with it (a batch
// rings once).
func (d *desktopDevice) notify(uid id.ID, n api.Delivery, bell bool) error {
	d.deliveryMu.Lock()
	defer d.deliveryMu.Unlock()
	d.mu.Lock()
	active := d.activeUser == uid
	d.mu.Unlock()
	if !active {
		return errors.New("Akun penerima tidak lagi aktif pada perangkat.")
	}

	key := uid.String() + ":" + n.ID.String() + ":" + n.AvailableAt.Format(time.RFC3339Nano)
	d.mu.Lock()
	if state := d.journal[key]; state != "" {
		d.mu.Unlock()
		if state == "accepted" {
			return nil
		}
		return errors.New("Penerimaan notifikasi sebelumnya tidak dapat dipastikan; periksa kotak notifikasi.")
	}
	d.journal[key] = "unknown"
	data, _ := json.Marshal(d.journal)
	err := atomicDeviceFile(filepath.Join(d.root, "delivery-journal.json"), data)
	prefs := d.notificationPrefs.Load().(devicePreferences)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	link := notificationLink(uid, n, "open")
	err = d.presentDelivery(uid, n, link, prefs)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.journal[key] = "accepted"
	data, _ = json.Marshal(d.journal)
	_ = atomicDeviceFile(filepath.Join(d.root, "delivery-journal.json"), data)
	d.mu.Unlock()
	d.mu.Lock()
	active = d.activeUser == uid
	d.mu.Unlock()
	if bell && active && prefs.Sound && prefs.Volume > 0 && notificationSoundAllowed() {
		d.playBell(prefs)
	}
	d.win.Update(func() {
		if !d.app.IsCurrentAccount(uid) {
			return
		}
		d.current = &n
		d.expandedUntil = time.Now().Add(12 * time.Second)
		if d.prefs.AceMode != "hidden" {
			d.ace.ShowInactive()
			d.ace.Update(func() {})
		}
	})
	return nil
}
func (d *desktopDevice) playBell(p devicePreferences) {
	data := bellWAV()
	if p.Audio != "" {
		if b, err := os.ReadFile(filepath.Join(d.root, "bell.wav")); err == nil {
			data = b
		}
	}
	sound, err := audio.DecodeSound(data)
	if err != nil {
		return
	}
	sound.NewPlayer().SetVolume(p.Volume).Play()
}
func (d *desktopDevice) loadMascot() {
	d.frames = nil
	d.delays = nil
	d.frame = 0
	if d.prefs.Mascot == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(d.root, "mascot"))
	if err != nil {
		d.err = err.Error()
		return
	}
	g, bitmap, err := decodeMascot(data)
	if err != nil {
		d.err = "Maskot tidak dapat dibaca: " + err.Error()
		return
	}
	if g != nil {
		canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
		var previous *image.RGBA
		for i, f := range g.Image {
			if i > 0 && len(g.Disposal) > i-1 {
				switch g.Disposal[i-1] {
				case gif.DisposalBackground:
					draw.Draw(canvas, g.Image[i-1].Bounds(), image.Transparent, image.Point{}, draw.Src)
				case gif.DisposalPrevious:
					if previous != nil {
						draw.Draw(canvas, canvas.Bounds(), previous, image.Point{}, draw.Src)
					}
				}
			}
			previous = image.NewRGBA(canvas.Bounds())
			draw.Draw(previous, previous.Bounds(), canvas, image.Point{}, draw.Src)
			draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
			cp := image.NewRGBA(canvas.Bounds())
			draw.Draw(cp, cp.Bounds(), canvas, image.Point{}, draw.Src)
			d.frames = append(d.frames, ui.NewBitmap(cp))
			d.delays = append(d.delays, time.Duration(max(g.Delay[i], 10))*10*time.Millisecond)
		}
		return
	}
	d.frames = []*ui.Bitmap{bitmap}
}

var aceSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"><path d="M64 9 C72 9 78 35 84 40 C90 45 118 43 120 52 C122 60 99 75 96 82 C93 89 101 115 93 120 C85 125 66 106 58 105 C50 104 28 120 21 114 C14 108 26 84 24 76 C22 68 3 49 7 42 C11 35 37 41 44 36 C51 31 55 9 64 9Z" fill="#d8c49b" stroke="#927d52" stroke-width="3"/><ellipse cx="49" cy="65" rx="4" ry="6" fill="#302a24"/><ellipse cx="78" cy="65" rx="4" ry="6" fill="#302a24"/><path d="M55 79 Q64 87 73 79" fill="none" stroke="#302a24" stroke-width="3" stroke-linecap="round"/><circle cx="39" cy="76" r="6" fill="#ca9485"/><circle cx="88" cy="76" r="6" fill="#ca9485"/></svg>`))

func (d *desktopDevice) aceView(c *ui.Context) {
	if d.prefs.AceMode == "hidden" {
		return
	}
	root := ui.Column(c).Fill().Padding(8).Gap(6)
	hover := root.Hovered()
	if d.prefs.AceMode == "edge" {
		expand := hover || time.Now().Before(d.expandedUntil)
		if expand != d.expanded {
			d.expanded = expand
			area := mygo.Screen.DisplayMatching(d.ace.Bounds()).WorkArea
			w := 36
			if expand {
				w = 300
			}
			d.ace.SetSize(w, 240)
			_, y := d.ace.Position()
			d.ace.SetPosition(area.X+area.Width-w, y)
		}
		if !expand {
			root.Background(ui.RGBA(46, 43, 38, .95)).Radius(12).Children(func() { ui.Text(c, "Ace").TextColor(ui.RGBA(255, 255, 255, 1)) })
			return
		}
	}
	root.Background(ui.RGBA(31, 31, 28, .96)).Radius(16).Children(func() {
		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, d.prefs.AceName).Bold().TextColor(ui.RGBA(255, 255, 255, 1)).Grow(1).DragWindow()
			if ui.Button(c, "×").Clicked() {
				d.prefs.AceMode = "hidden"
				d.ace.Hide()
				d.save()
			}
		})
		if len(d.frames) > 0 {
			if len(d.frames) > 1 && (hover || time.Now().Before(d.expandedUntil)) {
				now := c.Now()
				if !now.Before(d.nextFrame) {
					d.frame = (d.frame + 1) % len(d.frames)
					d.nextFrame = now.Add(d.delays[d.frame])
				}
				c.After(d.delays[d.frame])
			}
			ui.Image(c, d.frames[d.frame]).Size(88, 88).Fit(ui.Contain).DragWindow()
		} else {
			ui.Image(c, aceSVG).Size(88, 88).Fit(ui.Contain).DragWindow()
		}
		if d.current != nil {
			n := *d.current
			ui.Text(c, n.Title).Bold().MaxLines(2).TextColor(ui.RGBA(255, 255, 255, 1))
			ui.Row(c).Wrap().Gap(4).Children(func() {
				if ui.Button(c, "Buka").Clicked() {
					d.win.Show()
					d.win.Update(func() { d.app.OpenNotification(n) })
				}
				if ui.Button(c, "Sudah lihat").Clicked() {
					d.win.Update(func() { d.app.RespondNotification(n.ID, "seen", 0) })
					d.current = nil
				}
				if ui.Button(c, "15 menit lagi").Clicked() {
					d.win.Update(func() { d.app.RespondNotification(n.ID, "snooze", 15) })
					d.current = nil
				}
			})
		}
	})
	if d.prefs.AceMode == "edge" {
		c.After(time.Second)
	}
}
func (d *desktopDevice) settings(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1000).Padding(26).Gap(20).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Perangkat, notifikasi, dan Ace").FontSize(26).Bold()
				ui.Text(c, "Atur notifikasi dan pengalaman pada perangkat ini.").FontSize(12).TextColor(c.Theme().TextMuted)
			})
			if d.err != "" {
				ui.Text(c, d.err).TextColor(c.Theme().Danger)
			}
			d.islandSettings(c)
			deviceSection(c, "Notifikasi perangkat", "Periksa izin Windows dan mode jangan ganggu bila notifikasi tidak terlihat.", func() {
				if ui.Button(c, "Buka pengaturan notifikasi Windows").Clicked() {
					go func() {
						if err := openNotificationSettings(); err != nil {
							d.win.Update(func() { d.err = "Pengaturan notifikasi tidak dapat dibuka: " + err.Error() })
						}
					}()
				}
				if ui.Button(c, "Coba notifikasi perangkat").Disabled(d.testingNotification).Clicked() {
					d.testingNotification = true
					d.notificationTest = "Mengirim notifikasi percobaan ke perangkat ini…"
					go func() {
						err := showTestDeviceNotification()
						d.win.Update(func() {
							d.testingNotification = false
							if err != nil {
								d.notificationTest = "Notifikasi belum diterima sistem: " + err.Error()
							} else {
								d.notificationTest = "Sistem menerima permintaan. Periksa notifikasi Windows dan mode jangan ganggu."
							}
						})
					}()
				}
				if d.notificationTest != "" {
					ui.Text(c, d.notificationTest).FontSize(12).TextColor(c.Theme().TextMuted)
				}
			})
			deviceSection(c, "Mulai bersama Windows", "Space dapat berjalan di latar belakang setelah masuk ke Windows.", func() {
				label := "Mulai saat login Windows: mati"
				if d.prefs.Autostart {
					label = "Mulai saat login Windows: aktif"
				}
				if ui.Button(c, label).Clicked() {
					enabled := !d.prefs.Autostart
					if err := setDeviceAutostart(enabled); err != nil {
						d.err = err.Error()
					} else {
						d.prefs.Autostart = enabled
						d.save()
					}
				}
			})
			deviceSection(c, "Ace dan suara", "Pilih tampilan maskot serta suara pengingat pada perangkat ini.", func() {
				ui.Field(c, "Nama maskot", func() { ui.TextInput(c, &d.prefs.AceName).Label("Nama maskot").FillWidth() })
				ui.Row(c).Wrap().Gap(8).Children(func() {
					for _, mode := range []string{"floating", "edge", "hidden"} {
						if ui.Button(c, map[string]string{"floating": "Mengambang", "edge": "Tepi layar", "hidden": "Disembunyikan"}[mode]).Clicked() {
							d.prefs.AceMode = mode
							d.restorePosition()
							if mode == "hidden" {
								d.ace.Hide()
							} else {
								d.ace.ShowInactive()
							}
							d.save()
						}
					}
				})
				if ui.Button(c, "Pulihkan posisi Ace ke monitor utama").Clicked() {
					d.prefs.X = 0
					d.prefs.Y = 0
					d.restorePosition()
					d.save()
				}
				if ui.Button(c, "Impor maskot PNG / GIF").Clicked() {
					d.importFile("mascot")
				}
				if ui.Button(c, "Gunakan Ace bawaan").Clicked() {
					d.prefs.Mascot = ""
					d.loadMascot()
					d.save()
					d.ace.Update(func() {})
				}
				sound := "Suara bel: mati"
				if d.prefs.Sound {
					sound = "Suara bel: aktif"
				}
				if ui.Button(c, sound).Clicked() {
					d.prefs.Sound = !d.prefs.Sound
					d.save()
				}
				ui.Field(c, "Volume bel · 0–100", func() { ui.TextInput(c, &d.volume).Label("Volume bel · 0–100").FillWidth() })
				if ui.Button(c, "Simpan nama dan volume").Clicked() {
					v, err := strconv.Atoi(d.volume)
					if err != nil || v < 0 || v > 100 {
						d.err = "Volume harus 0–100."
					} else {
						d.prefs.Volume = float64(v) / 100
						d.save()
						d.ace.Update(func() {})
					}
				}
				if ui.Button(c, "Impor bel WAV").Clicked() {
					d.importFile("bell.wav")
				}
				if ui.Button(c, "Gunakan bel bawaan").Clicked() {
					d.prefs.Audio = ""
					d.save()
				}
			})
			ui.Text(c, "Pengaturan berlaku pada perangkat ini. Notifikasi mengikuti izin dan mode senyap Windows. Keluar aplikasi tersedia di tray.")
		})
	})
}

func deviceSection(c *ui.Context, title, description string, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().MinWidth(0).Padding(20).Gap(14).Radius(11).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(15).Bold()
		ui.Text(c, description).FontSize(12).TextColor(t.TextMuted)
		body()
	})
}
func (d *desktopDevice) importFile(kind string) {
	go func() {
		report := func(message string) { d.win.Update(func() { d.err = message }) }
		exts := []string{"png", "gif"}
		if kind == "bell.wav" {
			exts = []string{"wav"}
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Title: "Pilih aset perangkat", Filters: []mygo.FileFilter{{Name: "Aset", Extensions: exts}}})
		if err != nil {
			report("Pemilih berkas gagal: " + err.Error())
			return
		}
		if len(paths) == 0 {
			return
		}
		f, err := os.Open(paths[0])
		if err != nil {
			report("Berkas tidak dapat dibuka: " + err.Error())
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			report("Pilih berkas biasa maksimal 8 MiB.")
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
		if err != nil || len(data) > 8<<20 {
			report("Berkas tidak dapat dibaca atau melebihi 8 MiB.")
			return
		}
		if kind == "mascot" {
			if _, _, err := decodeMascot(data); err != nil {
				report("Maskot tidak didukung: " + err.Error())
				return
			}
		} else {
			sound, err := audio.DecodeSound(data)
			if err != nil {
				report("Audio tidak dapat dibaca: gunakan WAV yang didukung.")
				return
			}
			if sound == nil || sound.Duration() > 5*time.Second {
				report("Gunakan bel WAV maksimal 5 detik.")
				return
			}
		}
		if err = atomicDeviceFile(filepath.Join(d.root, kind), data); err != nil {
			report("Aset belum tersimpan: " + err.Error())
			return
		}
		d.win.Update(func() {
			d.err = ""
			if kind == "mascot" {
				d.prefs.Mascot = filepath.Base(paths[0])
				d.loadMascot()
				d.ace.Update(func() {})
			} else {
				d.prefs.Audio = filepath.Base(paths[0])
			}
			d.save()
		})
	}()
}
func trayIcon() []byte { return applicationIcon }

func bellWAV() []byte {
	const rate = 22050
	const samples = 6615
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+samples*2))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(samples*2))
	for i := 0; i < samples; i++ {
		t := float64(i) / rate
		v := (math.Sin(2*math.Pi*880*t) + .3*math.Sin(2*math.Pi*1320*t)) * math.Exp(-t*16) * math.Min(1, t*150)
		_ = binary.Write(&b, binary.LittleEndian, int16(v*10000))
	}
	return b.Bytes()
}

func (d *desktopDevice) NotifyBatch(uid id.ID, items []api.Delivery) error {
	if len(items) == 0 {
		return nil
	}
	unseen := []api.Delivery{}
	uncertain := false
	d.mu.Lock()
	for _, n := range items {
		key := uid.String() + ":" + n.ID.String() + ":" + n.AvailableAt.Format(time.RFC3339Nano)
		switch d.journal[key] {
		case "":
			unseen = append(unseen, n)
		case "accepted":
			// The device previously accepted this delivery.
		default:
			// An interrupted attempt cannot be promoted to accepted without
			// evidence, even when no new notifications need to be displayed.
			uncertain = true
		}
	}
	d.mu.Unlock()
	// Keep each island delivery actionable, with one bell. Only while the
	// island replaces the toasts: beside them, or for what the island
	// turns away, the batch keeps its one summary toast instead of a burst.
	if d.island != nil && len(unseen) > 0 {
		prefs := d.notificationPrefs.Load().(devicePreferences)
		if prefs.IslandEnabled && prefs.IslandReplaceToast && islandSupported() && islandEnvironmentAllowed() && len(unseen) <= island.Capacity {
			var batchErr error
			for i, n := range unseen {
				if e := d.notify(uid, n, i == 0); e != nil {
					batchErr = errors.Join(batchErr, e)
				}
			}
			if uncertain {
				batchErr = errors.Join(batchErr, errors.New("Penerimaan sebagian notifikasi sebelumnya tidak dapat dipastikan; periksa kotak notifikasi."))
			}
			return batchErr
		}
	}
	if len(unseen) == 0 {
		if uncertain {
			return errors.New("Penerimaan notifikasi sebelumnya tidak dapat dipastikan; periksa kotak notifikasi.")
		}
		return nil
	}
	first := unseen[0]
	if len(unseen) > 1 {
		first.Body = fmt.Sprintf("%s · Ada %d pengingat lain. Buka kotak notifikasi Space.", first.Body, len(unseen)-1)
	}
	err := d.Notify(uid, first)
	d.mu.Lock()
	defer d.mu.Unlock()
	status := "accepted"
	if err != nil {
		status = "unknown"
	}
	for _, n := range unseen {
		key := uid.String() + ":" + n.ID.String() + ":" + n.AvailableAt.Format(time.RFC3339Nano)
		d.journal[key] = status
	}
	data, e := json.Marshal(d.journal)
	if e == nil {
		e = atomicDeviceFile(filepath.Join(d.root, "delivery-journal.json"), data)
	}
	if err != nil {
		return err
	}
	if uncertain {
		return errors.New("Penerimaan sebagian notifikasi sebelumnya tidak dapat dipastikan; periksa kotak notifikasi.")
	}
	return e
}
