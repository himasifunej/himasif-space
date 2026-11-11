//go:build js

package main

import (
	"errors"
	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"sync"
	"syscall/js"
)

type webNotifications struct {
	app    *appui.App
	win    *mygo.Window
	mu     sync.Mutex
	active id.ID
	shown  []js.Value
}

func attachDevice(win *mygo.Window, app *appui.App) {
	notifications := &webNotifications{app: app, win: win}
	app.Notifications = notifications
	app.Notifier = notifications
	app.Web = webPlatform{}
	appui.HoldTouchPan = func() { js.Global().Set("space_pan_hold", true) }
	app.SessionChanged = func(uid id.ID) {
		notifications.mu.Lock()
		notifications.active = uid
		shown := notifications.shown
		notifications.shown = nil
		notifications.mu.Unlock()
		for _, n := range shown {
			n.Call("close")
		}
	}
	settings := &appui.BrowserNotifications{
		Permission: browserNotificationPermission,
		Request:    func() { requestBrowserNotifications(win) },
		Test:       notifications.test,
	}
	app.DeviceSettings = settings.View
	watchBrowserNotificationPermission(win)
}

// browserNotificationPermission reads Notification.permission as it is now;
// nothing about it is remembered, so a change made in the browser's site
// settings shows on the next frame.
func browserNotificationPermission() (perm string) {
	defer func() {
		if recover() != nil {
			perm = appui.BrowserPermissionUnsupported
		}
	}()
	n := js.Global().Get("Notification")
	if n.Type() != js.TypeFunction {
		return appui.BrowserPermissionUnsupported
	}
	switch p := n.Get("permission").String(); p {
	case appui.BrowserPermissionGranted, appui.BrowserPermissionDenied, appui.BrowserPermissionDefault:
		return p
	}
	return appui.BrowserPermissionUnsupported
}

// redraw draws the window again, from a browser callback.
func redraw(win *mygo.Window) js.Func {
	return js.FuncOf(func(js.Value, []js.Value) any {
		win.Update(func() {})
		return nil
	})
}

func requestBrowserNotifications(win *mygo.Window) {
	defer func() { recover() }()
	n := js.Global().Get("Notification")
	if n.Type() != js.TypeFunction {
		return
	}
	// Promise in current browsers; older Safari takes a callback instead.
	done := redraw(win)
	if p := n.Call("requestPermission", done); p.Type() == js.TypeObject && p.Get("then").Type() == js.TypeFunction {
		p.Call("then", done, done)
	}
}

// watchBrowserNotificationPermission draws the page again when the
// permission changes: through the Permissions API where the browser has
// it, and on returning to the tab, after a change in site settings.
func watchBrowserNotificationPermission(win *mygo.Window) {
	defer func() { recover() }()
	update := redraw(win)
	js.Global().Call("addEventListener", "focus", update)
	js.Global().Get("document").Call("addEventListener", "visibilitychange", update)
	perms := js.Global().Get("navigator").Get("permissions")
	if perms.Type() != js.TypeObject || perms.Get("query").Type() != js.TypeFunction {
		return
	}
	p := perms.Call("query", map[string]any{"name": "notifications"})
	var watch js.Func
	watch = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Type() == js.TypeObject {
			args[0].Call("addEventListener", "change", update)
		}
		win.Update(func() {})
		return nil
	})
	p.Call("then", watch, js.FuncOf(func(js.Value, []js.Value) any { return nil }))
}

// test shows a notification now, from the Perangkat page.
func (n *webNotifications) test() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("Browser menolak menampilkan notifikasi uji: " + jsError(r).Error())
		}
	}()
	constructor := js.Global().Get("Notification")
	if constructor.Type() != js.TypeFunction || constructor.Get("permission").String() != "granted" {
		return errors.New("Izin notifikasi browser belum diberikan.")
	}
	toast := constructor.New("Notifikasi uji Space", map[string]any{"body": "Notifikasi browser berfungsi di perangkat ini.", "tag": "space-test"})
	var click js.Func
	click = js.FuncOf(func(js.Value, []js.Value) any {
		js.Global().Call("focus")
		toast.Call("close")
		click.Release()
		return nil
	})
	toast.Set("onclick", click)
	return nil
}
func (n *webNotifications) Notify(uid id.ID, d api.Delivery) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.active != uid {
		return errors.New("Akun penerima tidak aktif.")
	}
	constructor := js.Global().Get("Notification")
	if constructor.Type() == js.TypeUndefined || constructor.Get("permission").String() != "granted" {
		return errors.New("Izin notifikasi browser belum diberikan.")
	}
	key := "space.delivery." + uid.String() + "." + d.ID.String() + "." + d.AvailableAt.String()
	storage := js.Global().Get("localStorage")
	if !storage.Call("getItem", key).IsNull() {
		return nil
	}
	storage.Call("setItem", key, "offered")
	toast := constructor.New(d.Title, map[string]any{"body": d.Body, "tag": d.ID.String(), "silent": false})
	n.shown = append(n.shown, toast)
	var click js.Func
	click = js.FuncOf(func(js.Value, []js.Value) any {
		n.win.Update(func() {
			if n.app.IsCurrentAccount(uid) {
				n.app.OpenNotification(d)
			}
		})
		js.Global().Call("focus")
		toast.Call("close")
		return nil
	})
	var close js.Func
	close = js.FuncOf(func(js.Value, []js.Value) any { click.Release(); close.Release(); return nil })
	toast.Set("onclick", click)
	toast.Set("onclose", close)
	return nil
}

func registerDeviceScheme() {}

func (n *webNotifications) NotifyBatch(uid id.ID, items []api.Delivery) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	storage := js.Global().Get("localStorage")
	unseen := []api.Delivery{}
	for _, d := range items {
		key := "space.delivery." + uid.String() + "." + d.ID.String() + "." + d.AvailableAt.String()
		if storage.Call("getItem", key).IsNull() {
			unseen = append(unseen, d)
		}
	}
	if len(unseen) == 0 {
		return nil
	}
	if err := n.Notify(uid, unseen[0]); err != nil {
		return err
	}
	for _, d := range unseen {
		key := "space.delivery." + uid.String() + "." + d.ID.String() + "." + d.AvailableAt.String()
		storage.Call("setItem", key, "offered")
	}
	return nil
}

// The web app's notification layer (appui.Notifier): browser popups.

func (n *webNotifications) Permission() string { return browserNotificationPermission() }
func (n *webNotifications) RequestPermission() { requestBrowserNotifications(n.win) }

// DeniedHint tells where a blocked site is allowed again.
func (n *webNotifications) DeniedHint() string {
	return "Diblokir di browser. Klik ikon gembok di samping alamat situs, pilih Setelan situs, lalu ubah Notifikasi menjadi Izinkan. Langkah ini tercentang sendiri."
}

// Present shows a browser popup now; a click brings Space forward.
func (n *webNotifications) Present(x appui.Notice) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("Browser menolak menampilkan notifikasi: " + jsError(r).Error())
		}
	}()
	constructor := js.Global().Get("Notification")
	if constructor.Type() != js.TypeFunction || constructor.Get("permission").String() != "granted" {
		return errors.New("Izin notifikasi browser belum diberikan.")
	}
	toast := constructor.New(x.Title, map[string]any{"body": x.Body, "tag": "space-" + x.Tag})
	var click js.Func
	click = js.FuncOf(func(js.Value, []js.Value) any {
		js.Global().Call("focus")
		toast.Call("close")
		click.Release()
		return nil
	})
	toast.Set("onclick", click)
	return nil
}
