package appui

import "github.com/egoist/mygo/ui"

// Browser notification permissions, as Notification.permission names
// them; "unsupported" when the browser has no Notification API.
const (
	BrowserPermissionGranted     = "granted"
	BrowserPermissionDenied      = "denied"
	BrowserPermissionDefault     = "default"
	BrowserPermissionUnsupported = "unsupported"
)

// BrowserNotifications is the Perangkat page of the web app. It shows the
// browser's own permission, read on every frame, so a permission given in
// the browser's site settings shows without a choice remembered here.
type BrowserNotifications struct {
	// Permission returns one of the BrowserPermission constants.
	Permission func() string
	// Request asks the browser for permission; the page is drawn again
	// when the browser answers.
	Request func()
	// Test shows a notification now.
	Test func() error

	testResult string
	testFailed bool
}

// View draws the page.
func (b *BrowserNotifications) View(c *ui.Context) {
	perm := BrowserPermissionUnsupported
	if b.Permission != nil {
		perm = b.Permission()
	}
	if perm != BrowserPermissionGranted {
		b.testResult = ""
	}
	settingsBody(c, "Notifikasi, suara, dan Ace", "Pengaturan ini hanya untuk perangkat ini.", func() {
		t := c.Theme()
		ui.Column(c).FillWidth().MinWidth(0).Padding(18).Gap(12).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).FillWidth().MinWidth(0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("bell")).Size(18, 18).Shrink(0).TextColor(t.Accent)
				ui.Text(c, "Notifikasi browser").FontSize(14).Bold().Grow(1).MinWidth(0)
				status, color := browserPermissionStatus(c, perm)
				ui.Text(c, status).FontSize(12).Bold().TextColor(color).Label("Status izin: " + status)
			})
			switch perm {
			case BrowserPermissionGranted:
				muted(c, "Browser mengizinkan Space menampilkan notifikasi selama aplikasi terbuka di tab ini.")
				if ui.Button(c, "Kirim notifikasi uji").Clicked() && b.Test != nil {
					if err := b.Test(); err != nil {
						b.testResult, b.testFailed = err.Error(), true
					} else {
						b.testResult, b.testFailed = "Notifikasi uji dikirim. Bila tidak muncul, periksa mode Jangan Ganggu atau pengaturan notifikasi sistem.", false
					}
				}
				if b.testResult != "" {
					if b.testFailed {
						ui.Text(c, b.testResult).FontSize(12).TextColor(t.Danger)
					} else {
						muted(c, b.testResult)
					}
				}
			case BrowserPermissionDenied:
				muted(c, "Notifikasi untuk situs ini diblokir di browser, jadi Space tidak bisa memintanya lagi.")
				muted(c, "Buka ikon gembok di samping alamat situs, pilih Setelan situs, lalu ubah Notifikasi menjadi Izinkan. Status di sini ikut berubah.")
			case BrowserPermissionDefault:
				muted(c, "Izinkan agar pengingat dan ping muncul sebagai notifikasi selama Space terbuka.")
				if ui.Button(c, "Izinkan notifikasi browser").Clicked() && b.Request != nil {
					b.Request()
				}
			default:
				muted(c, "Gunakan Chrome, Edge, atau Firefox terbaru, atau aplikasi desktop Space.")
			}
		})
		pageNotice(c, "monitor", "Tray, autostart, dan Ace", "Tersedia di aplikasi desktop Space untuk Windows.")
	})
}

func browserPermissionStatus(c *ui.Context, perm string) (string, ui.Color) {
	t := c.Theme()
	switch perm {
	case BrowserPermissionGranted:
		return "Diizinkan", t.Success
	case BrowserPermissionDenied:
		return "Diblokir", t.Danger
	case BrowserPermissionDefault:
		return "Belum diizinkan", t.TextMuted
	}
	return "Browser ini tidak mendukung notifikasi", t.TextMuted
}
