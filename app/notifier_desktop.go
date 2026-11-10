//go:build !js

package main

import (
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/appui"
)

// desktopNotifier is the desktop app's notification layer: the operating
// system's notifications. Windows asks no permission; whether Space may
// show notifications is a setting (Settings → Notifications), read in the
// background every minute, because reading it starts PowerShell.
type desktopNotifier struct {
	win  *mygo.Window
	app  *appui.App
	perm atomic.Value // string, an appui.BrowserPermission constant
}

func newDesktopNotifier(win *mygo.Window, app *appui.App, registered bool) *desktopNotifier {
	n := &desktopNotifier{win: win, app: app}
	if registered {
		n.perm.Store(appui.BrowserPermissionGranted)
		go n.watch()
	} else {
		n.perm.Store(appui.BrowserPermissionUnsupported)
	}
	return n
}

func (n *desktopNotifier) watch() {
	for {
		perm := appui.BrowserPermissionDenied
		if notificationSettingEnabled() {
			perm = appui.BrowserPermissionGranted
		}
		if old := n.perm.Swap(perm); old != perm {
			n.win.Update(func() {})
		}
		time.Sleep(time.Minute)
	}
}

func (n *desktopNotifier) Permission() string { return n.perm.Load().(string) }

// RequestPermission opens the Windows setting: there is no prompt to show.
func (n *desktopNotifier) RequestPermission() { go func() { _ = openNotificationSettings() }() }

// DeniedHint tells where to turn notifications back on.
func (n *desktopNotifier) DeniedHint() string {
	return "Notifikasi Space dimatikan di Windows. Buka Pengaturan perangkat → Buka pengaturan notifikasi Windows, lalu aktifkan Space. Langkah ini tercentang sendiri."
}

// Present shows the notice off the window's thread (PowerShell takes a
// moment); a failure comes back as a toast in Space.
func (n *desktopNotifier) Present(x appui.Notice) error {
	go func() {
		if err := showNoticeNotification(x.Title, x.Body, "space-"+x.Tag); err != nil {
			n.win.Update(func() { n.app.ShowNotice("Notifikasi belum muncul: " + err.Error()) })
		}
	}()
	return nil
}
