//go:build !js

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

type devicePreferences struct {
	IslandEnabled      bool   `json:"islandEnabled"`
	IslandStyle        string `json:"islandStyle"`
	IslandReplaceToast bool   `json:"islandReplaceToast"`
	IslandReduceMotion bool   `json:"islandReduceMotion"`
	// IslandAlwaysOn keeps Ace on screen as a small pill between notices.
	// Missing from older files, it takes the default: on.
	IslandAlwaysOn bool    `json:"islandAlwaysOn"`
	Autostart      bool    `json:"autostart"`
	Audio          string  `json:"audio"`
	Volume         float64 `json:"volume"`
	Sound          bool    `json:"sound"`
}

// legacyDeviceKeys are what preferences.json kept for the star mascot's
// window, removed in 0.4.5: Ace is the Dynamic Island now.
var legacyDeviceKeys = []string{"aceMode", "aceName", "mascot", "X", "Y"}

// loadDevicePreferences reads preferences.json over the defaults. legacy
// reports keys of the removed mascot window, for the caller to write the
// file again without them and drop the imported picture.
func loadDevicePreferences(data []byte) (prefs devicePreferences, legacy bool) {
	prefs = devicePreferences{Volume: .45, Sound: true, IslandEnabled: true, IslandStyle: "C", IslandReplaceToast: true, IslandAlwaysOn: true}
	if len(data) == 0 {
		return prefs, false
	}
	_ = json.Unmarshal(data, &prefs)
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) == nil {
		for _, k := range legacyDeviceKeys {
			if _, ok := raw[k]; ok {
				legacy = true
			}
		}
	}
	if prefs.IslandStyle != "A" && prefs.IslandStyle != "B" && prefs.IslandStyle != "C" {
		prefs.IslandStyle = "C"
	}
	if prefs.Volume < 0 || prefs.Volume > 1 {
		prefs.Volume = .45
	}
	return prefs, legacy
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
	win                 *mygo.Window
	app                 *appui.App
	tray                *mygo.Tray
	quitting            bool
	err                 string
	notificationTest    string
	testingNotification bool
	volume              float64   // the bell's volume slider, 0–100
	volumeAt            time.Time // when the slider last moved; saved once it rests
	savedAt             time.Time
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
	d := &desktopDevice{root: filepath.Join(root, "himasif-space", "device"), win: win, app: app, journal: map[string]string{}}
	data, _ := os.ReadFile(filepath.Join(d.root, "preferences.json"))
	prefs, legacy := loadDevicePreferences(data)
	d.prefs = prefs
	if legacy {
		// The star mascot's window is gone: forget its place and name, and
		// the picture imported for it. The bell and the island stay.
		d.save()
		d.savedAt = time.Time{}
		_ = os.Remove(filepath.Join(d.root, "mascot"))
	}
	d.island = &islandPresenter{d: d}
	if data, err := os.ReadFile(filepath.Join(d.root, "delivery-journal.json")); err == nil {
		_ = json.Unmarshal(data, &d.journal)
	}
	if d.journal == nil {
		d.journal = map[string]string{}
	}
	d.volume = math.Round(d.prefs.Volume * 100)
	d.notificationPrefs.Store(d.prefs)
	if islandSupported() {
		d.island.wake() // Ace rests at the top of the screen when "Selalu tampil" is on.
	}
	// Keep an opted-in startup entry pointing at the executable currently
	// running, including after moving a portable app or installing an update.
	if d.prefs.Autostart {
		if err := setDeviceAutostart(true); err != nil {
			d.err = "Autostart belum diperbarui: " + err.Error()
		}
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Buka Space", Click: func(*mygo.MenuItem, *mygo.Window) { win.Show(); win.Focus() }},
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
		if uid != id.Nil {
			// reset hid the island; with "Selalu tampil" Ace rests at the
			// top again for the account now signed in.
			d.island.wake()
		}
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
	d.savedAt = time.Now()
	d.notificationPrefs.Store(d.prefs)
	data, err := json.Marshal(d.prefs)
	if err == nil {
		err = atomicDeviceFile(filepath.Join(d.root, "preferences.json"), data)
	}
	if err != nil {
		d.err = "Preferensi belum tersimpan: " + err.Error()
	}
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

// importBell asks for a WAV of at most 5 seconds and keeps it as the bell.
func (d *desktopDevice) importBell() {
	go func() {
		report := func(message string) { d.win.Update(func() { d.err = message }) }
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Title: "Pilih bunyi bel", Filters: []mygo.FileFilter{{Name: "Bunyi WAV", Extensions: []string{"wav"}}}})
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
		sound, err := audio.DecodeSound(data)
		if err != nil {
			report("Audio tidak dapat dibaca: gunakan WAV yang didukung.")
			return
		}
		if sound == nil || sound.Duration() > 5*time.Second {
			report("Gunakan bel WAV maksimal 5 detik.")
			return
		}
		if err = atomicDeviceFile(filepath.Join(d.root, "bell.wav"), data); err != nil {
			report("Bunyi bel belum tersimpan: " + err.Error())
			return
		}
		d.win.Update(func() {
			d.err = ""
			d.prefs.Audio = filepath.Base(paths[0])
			d.save()
		})
	}()
}
