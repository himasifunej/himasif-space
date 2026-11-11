package appui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

var testTime = time.Date(2026, 11, 11, 9, 30, 0, 0, time.Local)

// devicePageForTest is a device page drawn with the rows' helpers, as the
// desktop's Perangkat page is.
func devicePageForTest(island, sound *bool, toggled *int) func(*ui.Context) {
	return func(c *ui.Context) {
		nav := []SettingsNavItem{{Title: "Dynamic Island", Icon: "spark", Status: "Kaca"}, {Title: "Suara bel", Icon: "play", Status: "45%"}}
		SettingsLayout(c, "Notifikasi dan Dynamic Island", "Laptop ini.", nav, func() { SettingsSaved(c, testTime) }, func() {
			SettingSection(c, "Dynamic Island", "Ace di atas tengah layar.", func() {
				SettingPreview(c, 120, func() { ui.Text(c, "Pratinjau") })
				if SettingSwitch(c, "Tampilkan Dynamic Island", "Saat mati, kabar muncul sebagai toast Windows.", island) {
					*toggled++
				}
				SettingRow(c, "Kurangi gerak", "Ikut pengaturan animasi Windows.", func() { Segmented(c, "Kurangi gerak", []string{"Ikut Windows", "Kurangi"}, 0) })
			})
			SettingSection(c, "Suara bel", "", func() {
				SettingSwitch(c, "Suara bel", "Bunyi saat pengingat dan ping masuk.", sound)
				SettingRow(c, "Bunyi bel", "Bel bawaan Space", func() { SettingButton(c, "Ganti…", "upload") })
			})
		})
	}
}

// The device page's search keeps the sections and rows that hold every
// word typed, and says so when none does; a switch reports its change.
func TestSettingsRowsSearchAndSwitch(t *testing.T) {
	settingsPage.query, settingsPage.words = "", nil
	island, sound, toggled := true, true, 0
	tt := ui.NewTester(devicePageForTest(&island, &sound, &toggled), 1400, 900)
	tt.Frame()
	for _, s := range []string{"Dynamic Island", "Pratinjau", "Kurangi gerak", "Bunyi bel", "Tersimpan · 09.30"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("Tampilkan Dynamic Island"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if island || toggled != 1 {
		t.Fatalf("switch: island %v, toggled %d", island, toggled)
	}

	settingsPage.query = "bunyi"
	tt.Frame()
	if !tt.HasText("Bunyi bel") || tt.HasText("Kurangi gerak") || tt.HasText("Pratinjau") || tt.HasText("Tampilkan Dynamic Island") {
		t.Fatalf("search bunyi: %q", tt.Texts())
	}
	settingsPage.query = "island"
	tt.Frame()
	if !tt.HasText("Tampilkan Dynamic Island") || !tt.HasText("Kurangi gerak") || tt.HasText("Bunyi bel") {
		t.Fatalf("search island (a section's title shows all its rows): %q", tt.Texts())
	}
	settingsPage.query = "zzqx"
	tt.Frame()
	if !tt.HasText(`Tidak ada pengaturan yang cocok dengan "zzqx".`) {
		t.Fatalf("no empty state: %q", tt.Texts())
	}
	if err := tt.Click("Hapus pencarian"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if settingsPage.query != "" || !tt.HasText("Bunyi bel") {
		t.Fatalf("clearing the search: %q", settingsPage.query)
	}
}
