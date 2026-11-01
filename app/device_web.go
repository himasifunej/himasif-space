//go:build js

package main

import (
	"errors"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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
	app.DeviceSettings = func(c *ui.Context) {
		ui.Column(c).Padding(24).Gap(12).Children(func() {
			ui.Text(c, "Notifikasi browser").Bold()
			ui.Text(c, "Notifikasi browser bekerja selama aplikasi terbuka. Tray, autostart, dan Ace tersedia di aplikasi desktop.")
			if ui.Button(c, "Izinkan notifikasi browser").Clicked() {
				n := js.Global().Get("Notification")
				if n.Type() != js.TypeUndefined {
					n.Call("requestPermission")
				}
			}
		})
	}
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
