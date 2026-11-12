package appui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Sakura, the pink palette (owner's rc.2 feedback): text and muted text
// read on the page and on cards in both modes, and its pink accent is a
// legible button, also as a link on the page.
func TestSakuraPaletteContrast(t *testing.T) {
	for _, dark := range []bool{true, false} {
		th := paletteTheme(dark, Preferences{Palette: "sakura", Accent: "pink"})
		for name, pair := range map[string][2]ui.Color{
			"text/page": {th.Text, th.Background}, "text/card": {th.Text, th.Surface},
		} {
			if c := spaceContrast(pair[0], pair[1]); c < 7 {
				t.Errorf("dark %v %s: %.1f", dark, name, c)
			}
		}
		for name, pair := range map[string][2]ui.Color{
			"muted/page": {th.TextMuted, th.Background}, "muted/card": {th.TextMuted, th.Surface}, "muted/hover": {th.TextMuted, th.SurfaceHover},
			"accent/button text": {th.Accent, th.AccentText}, "accent/page": {th.Accent, th.Background},
		} {
			if c := spaceContrast(pair[0], pair[1]); c < 4.5 {
				t.Errorf("dark %v %s: %.1f", dark, name, c)
			}
		}
	}
	if got := NormalizePreferences(Preferences{Palette: "Sakura", Accent: "pink"}); got.Palette != "sakura" || got.Accent != "pink" {
		t.Fatalf("normalized %+v", got)
	}
	if paletteAccent("sakura", "sand") != "pink" || paletteAccent("sakura", "sage") != "sage" || paletteAccent("copper", "sand") != "sand" {
		t.Fatal("paletteAccent")
	}
}

// Picking Sakura in Tampilan turns the app pink; SPACE_UI_CAPTURE_DIR
// keeps the page in both modes.
func TestSakuraPalettePicked(t *testing.T) {
	for _, mode := range []string{"dark", "light"} {
		h := redesignHarness(t)
		h.app.chrome.preferences.Theme = mode
		h.openSettings("Tampilan")
		h.click("Palet Sakura")
		h.tt.Frame()
		if p := h.app.chrome.preferences; p.Palette != "sakura" || p.Accent != "pink" {
			t.Fatalf("%s: %+v", mode, p)
		}
		captureNativeUI(t, h, "tampilan-sakura-"+mode)
	}
}
