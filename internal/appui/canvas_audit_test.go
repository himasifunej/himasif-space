package appui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Canvas A's light theme darkens the preset accents; the pastel ones of
// the dark theme were too faint as a button on a light page.
func TestLightThemeUsesCanvasLightAccents(t *testing.T) {
	for accent, want := range map[string]string{"sand": "#2f2c25", "": "#2f2c25", "sage": "#4d6a3a", "blue": "#2f5d86", "rose": "#9c4f2c", "tidak-dikenal": "#2f2c25"} {
		th := spaceTheme(false, accent, false)
		if th.Accent != ui.Hex(want) {
			t.Errorf("light %q accent = %+v, want %s", accent, th.Accent, want)
		}
		if spaceContrast(th.Accent, th.AccentText) < 4.5 || spaceContrast(th.Accent, th.Background) < 4.5 {
			t.Errorf("light %q accent not legible: text %.1f, page %.1f", accent, spaceContrast(th.Accent, th.AccentText), spaceContrast(th.Accent, th.Background))
		}
	}
	for _, accent := range []string{"sand", "sage", "blue", "lavender", "rose"} {
		if th := spaceTheme(false, accent, false); spaceContrast(th.Accent, th.Background) < 4.5 {
			t.Errorf("light %q accent faint on the page: %.1f", accent, spaceContrast(th.Accent, th.Background))
		}
	}
	if th := spaceTheme(true, "sand", false); th.Accent != ui.Hex("#d7c3a0") {
		t.Errorf("dark sand changed: %+v", th.Accent)
	}
	if th := spaceTheme(false, "#123456", false); th.Accent != ui.Hex("#123456") {
		t.Errorf("custom accent changed on light: %+v", th.Accent)
	}
}

// Canvas A · Tablet: the topbar keeps the Space's title whole (no
// breadcrumb, the sync pill a dot, search and Ping as icons) and the
// Beranda's KPIs sit 2×2.
func TestTabletTopbarAndKPIsFollowCanvas(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(768, 1024)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Beranda"
	h.tt.Frame()
	h.app.tasks.items, h.app.tasks.loaded = nil, true
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	if _, ok := h.tt.Find("Buka pencarian"); !ok {
		t.Fatalf("tablet topbar has no search button: %q", h.tt.Texts())
	}
	if _, ok := h.tt.Find("Cari di Space"); ok {
		t.Fatal("tablet topbar still shows the search field")
	}
	h.dontWant("Space · Periode 2025/2026")
	a, _ := h.tt.Find("Tugas aktif")
	b, _ := h.tt.Find("Lewat tenggat")
	c, _ := h.tt.Find("Perlu tinjauan")
	if a.Y != b.Y || c.Y <= a.Y {
		t.Fatalf("KPIs not 2×2: %+v %+v %+v", a, b, c)
	}
	h.click("Buka pencarian")
	h.tt.Frame()
	if !h.app.chrome.searchOpen || !h.tt.Focused("Cari di Space") {
		t.Fatal("search button did not open and focus the search field")
	}
	captureNativeUI(t, h, "tablet-beranda")
}
