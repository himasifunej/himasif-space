package appui

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// spaceTheme creates a fresh theme so appearance changes never mutate MyGo's
// defaults or another window. The colors follow the approved Space prototype.
func spaceTheme(dark bool, accent string, compact bool) *ui.Theme {
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
		t.Background = ui.Hex("#151411")
		t.Surface = ui.Hex("#1d1c17")
		t.SurfaceHover = ui.Hex("#2b2821")
		t.SurfacePressed = ui.Hex("#25231d")
		t.Border = ui.Hex("#363229")
		t.Text = ui.Hex("#e9e5dc")
		t.TextMuted = ui.Hex("#aaa294")
		t.Scrollbar = ui.Hex("#615b4e")
		t.Danger = ui.Hex("#d2a499")
		t.Warning = ui.Hex("#d7c3a0")
		t.Success = ui.Hex("#a8c2ac")
	} else {
		t.Background = ui.Hex("#f7f5ef")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#e9e5da")
		t.SurfacePressed = ui.Hex("#f1eee5")
		t.Border = ui.Hex("#d8d3c6")
		t.Text = ui.Hex("#2d3029")
		t.TextMuted = ui.Hex("#6d6c60")
		t.Scrollbar = ui.Hex("#b1a99a")
		t.Danger = ui.Hex("#9b473c")
		t.Warning = ui.Hex("#856021")
		t.Success = ui.Hex("#3d7650")
	}
	t.Accent = spaceAccent(accent)
	t.AccentHover = spaceMix(t.Accent, ui.Hex("#ffffff"), .08)
	t.AccentPressed = spaceMix(t.Accent, ui.Hex("#000000"), .08)
	// Custom accents can be very dark: choose the more legible button text.
	t.AccentText = ui.Hex("#24251f")
	if spaceContrast(t.Accent, ui.Hex("#ffffff")) > spaceContrast(t.Accent, t.AccentText) {
		t.AccentText = ui.Hex("#ffffff")
	}
	t.Focus = ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .65)
	t.Selection = ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .25)
	t.Inverse = t.Text
	t.InverseText = t.Background
	t.ScrollbarWidth = 6
	t.Radius = 7
	t.Spacing = 4
	if compact {
		t.Spacing = 3
	}
	t.FontSize = 14
	return t
}

func spaceAccent(value string) ui.Color {
	value = strings.ToLower(strings.TrimSpace(value))
	presets := map[string]string{
		"sand": "#d7c3a0", "sage": "#a8c2ac", "blue": "#a2bbd2",
		"lavender": "#b6a8d0", "rose": "#d2a499",
	}
	if color, ok := presets[value]; ok {
		return ui.Hex(color)
	}
	if len(value) == 7 && value[0] == '#' {
		valid := true
		for _, digit := range value[1:] {
			if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f') {
				valid = false
				break
			}
		}
		if valid {
			return ui.Hex(value)
		}
	}
	return ui.Hex(presets["sand"])
}

func spaceMix(first, second ui.Color, amount float64) ui.Color {
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-amount) + float64(b)*amount)) }
	return ui.RGB(mix(first.R, second.R), mix(first.G, second.G), mix(first.B, second.B))
}

func spaceLuminance(color ui.Color) float64 {
	linear := func(channel uint8) float64 {
		s := float64(channel) / 255
		if s <= .04045 {
			return s / 12.92
		}
		return math.Pow((s+.055)/1.055, 2.4)
	}
	return .2126*linear(color.R) + .7152*linear(color.G) + .0722*linear(color.B)
}

func spaceContrast(first, second ui.Color) float64 {
	a, b := spaceLuminance(first), spaceLuminance(second)
	return (math.Max(a, b) + .05) / (math.Min(a, b) + .05)
}
