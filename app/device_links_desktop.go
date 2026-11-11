//go:build !js

package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/url"
	"strings"
)

func (d *desktopDevice) handleLink(raw string) {
	if kind, cid, ok := cardLinkTarget(raw); ok {
		d.openCard(raw, kind, cid)
		return
	}
	d.handleNotificationLink(raw, true)
}

// cardLinkTarget reads a card's link from the web, space://tugas/<id> or
// space://rapat/<id> (himasif-space:// alike).
func cardLinkTarget(raw string) (kind string, cid id.ID, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != appui.AppScheme && u.Scheme != "himasif-space") {
		return "", id.Nil, false
	}
	switch u.Host {
	case "tugas":
		kind = "task"
	case "rapat":
		kind = "meeting"
	default:
		return "", id.Nil, false
	}
	cid, err = id.Parse(strings.Trim(u.Path, "/"))
	return kind, cid, err == nil
}

// openCard brings Space forward on the card. Signed out, it waits for the
// sign-in (pendingLink), as a notification's link does.
func (d *desktopDevice) openCard(raw, kind string, cid id.ID) {
	d.win.Show()
	d.win.Focus()
	d.win.Update(func() {
		d.mu.Lock()
		signedIn := d.activeUser != id.Nil
		d.mu.Unlock()
		if !signedIn {
			d.pendingLink = raw
			return
		}
		d.pendingLink = ""
		d.app.OpenCard(kind, cid)
	})
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

// registerDeviceScheme makes this Space the handler of its links at every
// start: himasif-space:// (notifications) and space:// (cards from the
// web), so an installation updated in place gets space:// too.
func registerDeviceScheme() {
	_ = mygo.App.RegisterURLScheme("himasif-space")
	_ = mygo.App.RegisterURLScheme(appui.AppScheme)
}
