package appui

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The Perangkat page in a browser follows the browser's own permission,
// read each time, not a remembered choice.
func TestBrowserNotificationSettingsFollowPermission(t *testing.T) {
	perm := "granted"
	requested, tested := 0, 0
	var testErr error
	b := &BrowserNotifications{
		Permission: func() string { return perm },
		Request:    func() { requested++ },
		Test:       func() error { tested++; return testErr },
	}
	tt := ui.NewTester(func(c *ui.Context) { b.View(c) }, 900, 700)
	click := func(s string) {
		t.Helper()
		tt.Advance(300 * time.Millisecond)
		if err := tt.Click(s); err != nil {
			t.Fatalf("%v; texts %q", err, tt.Texts())
		}
		tt.Advance(300 * time.Millisecond)
	}
	want := func(s string, shown bool) {
		t.Helper()
		if tt.HasText(s) != shown {
			t.Errorf("text %q shown=%v, want %v; texts %q", s, !shown, shown, tt.Texts())
		}
	}

	tt.Frame()
	want("Diizinkan", true)
	want("Kirim notifikasi uji", true)
	want("Izinkan notifikasi browser", false)
	click("Kirim notifikasi uji")
	if tested != 1 {
		t.Fatalf("test notification sent %d times", tested)
	}
	want("Notifikasi uji dikirim. Bila tidak muncul, periksa mode Jangan Ganggu atau pengaturan notifikasi sistem.", true)
	testErr = errors.New("Izin notifikasi browser belum diberikan.")
	click("Kirim notifikasi uji")
	want("Izin notifikasi browser belum diberikan.", true)

	perm = "denied"
	tt.Frame()
	want("Diblokir", true)
	want("Kirim notifikasi uji", false)
	want("Izinkan notifikasi browser", false)
	want("Buka ikon gembok di samping alamat situs, pilih Setelan situs, lalu ubah Notifikasi menjadi Izinkan. Status di sini ikut berubah.", true)

	perm = "default"
	tt.Frame()
	want("Belum diizinkan", true)
	click("Izinkan notifikasi browser")
	if requested != 1 {
		t.Fatalf("permission requested %d times", requested)
	}

	perm = "unsupported"
	tt.Frame()
	want("Browser ini tidak mendukung notifikasi", true)
	want("Izinkan notifikasi browser", false)
}
