package appui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The owner's rc.6 report: Pengaturan said its name twice (the top bar and
// a heading over the tabs), and only Notifikasi dan Dynamic Island had a
// search ("Cari di Perangkat ini"). One search now sits at the top of the
// tabs, over every page; a result opens its page at its section.
func TestSettingsFinder(t *testing.T) {
	for _, size := range [][2]int{{1440, 900}, {800, 760}} {
		h := redesignHarness(t)
		h.tt.SetSize(size[0], size[1])
		h.openSettings("Akun")
		h.app.chrome.settingsDetail = false
		h.tt.Frame()
		h.dontWant("Cari di Perangkat ini")
		h.dontWant("Pengaturan perangkat tidak otomatis mengubah perangkat lain.")
		// The sidebar's link (its label and text), and the page's title in
		// the top bar; no heading of its own over the tabs.
		if n := strings.Count("\n"+strings.Join(h.tt.Texts(), "\n")+"\n", "\nPengaturan\n"); n > 3 {
			t.Fatalf("%v: Pengaturan %d times: %q", size, n, h.tt.Texts())
		}
		h.click("Cari pengaturan")
		h.tt.Type("skala")
		h.tt.Frame()
		h.want("Skala tampilan")
		h.want("Tampilan dan tema › Tema")
		h.dontWant("Identitas space") // the tabs give way to the results
		h.tt.Key(0, ui.KeyEnter)
		h.tt.Frame()
		if h.app.chrome.settingsTab != "Tampilan" || h.app.chrome.settingsFind != "" || revealing.title != "Tema" {
			t.Fatalf("%v: Enter opened %q (find %q, reveal %q)", size, h.app.chrome.settingsTab, h.app.chrome.settingsFind, revealing.title)
		}
		revealing.title = ""

		// Every group is searched: a division's page, the account, the
		// device; nothing found says so; Esc clears.
		if size[0] < 900 {
			h.app.chrome.settingsDetail = false
			h.tt.Frame()
		}
		for _, q := range []struct{ query, want string }{
			{"jam kirim", "Pengingat dan ping › Jadwal pengingat tugas"},
			{"kata sandi", "Profil dan keamanan"},
			{"dynamic island", "Notifikasi dan Dynamic Island"},
		} {
			h.app.chrome.settingsFind = q.query
			h.tt.Frame()
			h.want(q.want)
		}
		h.app.chrome.settingsFind = "zzqx"
		h.tt.Frame()
		h.want("Tidak ada pengaturan yang cocok dengan “zzqx”.")
		h.click("Cari pengaturan")
		h.tt.Key(0, ui.KeyEscape)
		h.tt.Frame()
		if h.app.chrome.settingsFind != "" {
			t.Fatalf("%v: Esc kept %q", size, h.app.chrome.settingsFind)
		}
		h.want("Identitas space")
	}
}
