//go:build !js

package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/url"
)

func (d *desktopDevice) handleLink(raw string) {
	d.handleNotificationLink(raw, true)
}

// Island actions such as snooze reuse account validation and API handling
// without bringing the main window forward. Only Open requests focus.
func (d *desktopDevice) handleNotificationLink(raw string, activate bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "himasif-space" || u.Host != "notification" {
		return
	}
	uid, err := id.Parse(u.Query().Get("account"))
	if err != nil {
		return
	}
	payload, err := base64.RawURLEncoding.DecodeString(u.Query().Get("payload"))
	if err != nil || len(payload) > 16000 {
		return
	}
	var n api.Delivery
	if json.Unmarshal(payload, &n) != nil {
		return
	}
	if activate {
		d.pendingLink = raw
		d.win.Show()
		d.win.Focus()
	}
	// Deep links carry navigation hints only. Responses still require the
	// matching signed-in account, and server rechecks the recipient and scope.
	d.win.Update(func() {
		if !d.app.IsCurrentAccount(uid) {
			d.err = "Masuk ke akun penerima untuk membuka notifikasi ini."
			return
		}
		d.pendingLink = ""
		switch u.Query().Get("action") {
		case "seen":
			d.app.RespondNotification(n.ID, "seen", 0)
		case "snooze":
			d.app.RespondNotification(n.ID, "snooze", 15)
		default:
			d.app.OpenNotification(n)
		}
	})
}
func registerDeviceScheme() { _ = mygo.App.RegisterURLScheme("himasif-space") }
