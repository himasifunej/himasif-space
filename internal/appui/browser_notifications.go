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

// View draws the page: canvas A's rows, as on the desktop, with what
// only the desktop app has named.
func (b *BrowserNotifications) View(c *ui.Context) {
	perm := BrowserPermissionUnsupported
	if b.Permission != nil {
		perm = b.Permission()
	}
	if perm != BrowserPermissionGranted {
		b.testResult = ""
	}
	status, color := browserPermissionStatus(c, perm)
	nav := []SettingsNavItem{
		{Title: "Notifikasi browser", Icon: "bell", Status: status},
		{Title: "Aplikasi desktop", Icon: "monitor", Status: "Windows"},
	}
	SettingsLayout(c, "Notifikasi dan Dynamic Island", "Browser ini. Tidak ikut ke perangkat lain.", nav, nil, func() {
		t := c.Theme()
		SettingSection(c, "Notifikasi browser", "Pengingat dan ping muncul selama Space terbuka di tab ini.", func() {
			var description string
			switch perm {
			case BrowserPermissionGranted:
				description = "Browser mengizinkan Space menampilkan notifikasi."
			case BrowserPermissionDenied:
				description = "Diblokir di browser. Buka ikon gembok di samping alamat situs, pilih Setelan situs, lalu ubah Notifikasi menjadi Izinkan."
			case BrowserPermissionDefault:
				description = "Izinkan agar pengingat dan ping muncul sebagai notifikasi."
			default:
				description = "Gunakan Chrome, Edge, atau Firefox terbaru, atau aplikasi desktop Space."
			}
			SettingRow(c, "Izin notifikasi browser", description, func() {
				ui.Text(c, status).FontSize(13).FontWeight(500).TextColor(color).Label("Status izin: " + status)
				if perm == BrowserPermissionDefault && SettingButton(c, "Izinkan notifikasi", "bell").Clicked() && b.Request != nil {
					b.Request()
				}
			})
			if perm == BrowserPermissionGranted {
				description := "Mengirim satu notifikasi lewat browser."
				if b.testResult != "" {
					description = b.testResult
				}
				SettingRow(c, "Kirim notifikasi uji", description, func() {
					if SettingButton(c, "Kirim percobaan", "send").Clicked() && b.Test != nil {
						if err := b.Test(); err != nil {
							b.testResult, b.testFailed = err.Error(), true
						} else {
							b.testResult, b.testFailed = "Notifikasi uji dikirim. Bila tidak muncul, periksa mode Jangan Ganggu atau pengaturan notifikasi sistem.", false
						}
					}
				})
				if b.testFailed && b.testResult != "" {
					ui.Text(c, b.testResult).FontSize(12).TextColor(t.Danger).Padding(0, 16, 12, 16)
				}
			}
		})
		SettingSection(c, "Aplikasi desktop", "", func() {
			SettingRow(c, "Dynamic Island, tray, dan autostart", "Tersedia di aplikasi Space untuk Windows.", func() {})
		})
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
