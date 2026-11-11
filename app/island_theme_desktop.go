//go:build !js

package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The island's styles (Pengaturan › Perangkat › Gaya, canvas 8f83cee819ac):
// "" follows Space's theme and accent, the default, also for those who
// never chose; hitam is the black pill of 0.4.5-rc2; kaca a translucent
// glass; custom a color and a transparency of the user's.
const (
	islandThemeFollow = ""
	islandThemeBlack  = "hitam"
	islandThemeGlass  = "kaca"
	islandThemeCustom = "custom"
)

// islandThemes are the styles in the order the settings show them.
var islandThemes = []struct{ key, label string }{
	{islandThemeFollow, "Ikut tema"},
	{islandThemeBlack, "Hitam"},
	{islandThemeGlass, "Kaca"},
	{islandThemeCustom, "Custom"},
}

// islandPresets are Custom's ready colors.
var islandPresets = []struct{ name, hex string }{
	{"Hitam", "#000000"},
	{"Grafit", "#2b2d31"},
	{"Biru malam", "#14213d"},
	{"Hijau tua", "#1f3b2d"},
	{"Merah bata", "#5c2a22"},
	{"Pasir", "#e8dcc4"},
}

// islandPalette is a style's colors for one frame.
type islandPalette struct {
	ink, text, muted, ring, button, buttonHover ui.Color
	dark                                        bool
}

// use makes p the colors the island draws with this frame.
func (p islandPalette) use() {
	islandInk, islandText, islandMuted = p.ink, p.text, p.muted
	islandRing, islandButton, islandButtonHover = p.ring, p.button, p.buttonHover
}

// islandPaletteFor is the style prefs ask for; base is Space's theme, for
// "Ikut tema", nil where it is not known (the black pill then).
func islandPaletteFor(prefs devicePreferences, base *ui.Theme) islandPalette {
	black := islandPalette{ink: ui.RGB(0, 0, 0), text: ui.RGB(245, 245, 247), muted: ui.RGBA(245, 245, 247, .72),
		ring: ui.RGBA(255, 255, 255, .06), button: ui.RGB(44, 44, 46), buttonHover: ui.RGB(58, 58, 60), dark: true}
	switch prefs.IslandTheme {
	case islandThemeBlack:
		return black
	case islandThemeGlass:
		// Translucent, without the system's blur: Windows draws an acrylic
		// or blur accent over the whole window, past the region, which
		// brought back the box around the island (0.4.4's purple corners).
		return islandOn(ui.RGB(30, 31, 36), .62)
	case islandThemeCustom:
		c, ok := parseIslandHex(prefs.IslandColor)
		if !ok {
			c = ui.RGB(0, 0, 0)
		}
		return islandOn(c, islandOpacity(prefs.IslandOpacity))
	}
	if base == nil {
		return black
	}
	// Ikut tema: Space's surface and text, its border, its buttons.
	return islandPalette{ink: opaque(base.Surface), text: base.Text, muted: base.TextMuted, ring: base.Border,
		button: base.SurfaceHover, buttonHover: base.SurfacePressed, dark: base.Dark}
}

// islandOn is a palette over the background bg, alpha opaque: text, eyes
// and buttons dark or light, whichever reads on it.
func islandOn(bg ui.Color, alpha float64) islandPalette {
	ink := ui.RGBA(bg.R, bg.G, bg.B, float32(alpha))
	if islandLuminance(bg) > .45 {
		fg := ui.RGB(24, 24, 27)
		return islandPalette{ink: ink, text: fg, muted: ui.RGBA(24, 24, 27, .68), ring: ui.RGBA(0, 0, 0, .12),
			button: ui.RGBA(0, 0, 0, .08), buttonHover: ui.RGBA(0, 0, 0, .14)}
	}
	return islandPalette{ink: ink, text: ui.RGB(245, 245, 247), muted: ui.RGBA(245, 245, 247, .74), ring: ui.RGBA(255, 255, 255, .14),
		button: ui.RGBA(255, 255, 255, .12), buttonHover: ui.RGBA(255, 255, 255, .2), dark: true}
}

// islandLuminance is a color's relative luminance (WCAG), 0 to 1.
func islandLuminance(c ui.Color) float64 {
	ch := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .03928 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*ch(c.R) + .7152*ch(c.G) + .0722*ch(c.B)
}

// islandOpacity is a saved opacity within 40% and 100% (0: not set, 1).
func islandOpacity(v float64) float64 {
	if v <= 0 || math.IsNaN(v) {
		return 1
	}
	return math.Max(.4, math.Min(1, v))
}

// parseIslandHex reads "#rrggbb".
func parseIslandHex(s string) (ui.Color, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return ui.Color{}, false
	}
	var v [3]uint8
	for i := range 3 {
		var b uint8
		for _, r := range s[2*i : 2*i+2] {
			var d uint8
			switch {
			case r >= '0' && r <= '9':
				d = uint8(r - '0')
			case r >= 'a' && r <= 'f':
				d = uint8(r-'a') + 10
			case r >= 'A' && r <= 'F':
				d = uint8(r-'A') + 10
			default:
				return ui.Color{}, false
			}
			b = b*16 + d
		}
		v[i] = b
	}
	return ui.RGB(v[0], v[1], v[2]), true
}

// spaceTheme is Space's theme as the main window draws it now, for "Ikut
// tema": it follows a change of theme or accent at the island's next frame.
func (p *islandPresenter) spaceTheme(c *ui.Context) *ui.Theme {
	if p.d.app == nil {
		return c.Theme() // the spike, which has no Space window
	}
	return p.d.app.IslandTheme(c.Theme().Dark)
}
