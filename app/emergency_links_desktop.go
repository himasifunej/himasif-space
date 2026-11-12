//go:build !js

package main

import (
	"net/url"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// handleEmergencyLink answers the emergency sheet's island and toast
// buttons (himasif-space://darurat?action=…): Buka sheet opens the sheet
// in the browser, Salin link copies it, WhatsApp shares it; anything else
// shows Pusat darurat. It works with the window hidden in the tray and the
// server down: the links are kept on this device.
func (d *desktopDevice) handleEmergencyLink(u *url.URL) {
	open, copyText, show := d.app.EmergencyAction(u.Query().Get("action"))
	if open != "" {
		_ = mygo.Shell.OpenExternal(open)
	}
	if copyText != "" {
		_ = mygo.Clipboard.Write(transfer.TextData(copyText))
	}
	if show {
		d.win.Show()
		d.win.Focus()
	}
}
