package appui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// Display scale (owner's rc.2 feedback, "pengaturan display kaya di
// Windows, 100-200%"): Tampilan offers Windows' own steps and "Ikuti
// Windows". A step is the size against an unscaled screen, as in Windows:
// on a laptop Windows already scales to 150%, 150% looks as before and
// 100% smaller. The choice is kept per device with the other appearance
// preferences, and zooms the whole window (ui.Context.SetZoom), whose
// layout then follows its smaller or larger size in DIPs.

// displayScales are the steps, in percent; 0 follows the system.
var displayScales = []int{0, 100, 125, 150, 175, 200}

// displayZoom is the window's zoom for a scale chosen, given the system's
// own (the monitor's scale factor, 0 where unknown, as in a browser,
// which then scales from its own size).
func displayZoom(scale int, system float64) float32 {
	if scale == 0 {
		return 1
	}
	if system <= 0 || math.IsNaN(system) {
		system = 1
	}
	return float32(math.Max(.5, math.Min(2.5, float64(scale)/100/system)))
}

// systemScale is the system's scale factor from the device, 0 unknown.
func (a *App) systemScale() float64 {
	if a.SystemScale == nil {
		return 0
	}
	return a.SystemScale()
}

// applyDisplayScale zooms the window as chosen; called at each frame.
func (a *App) applyDisplayScale(c *ui.Context) {
	z := displayZoom(a.chrome.preferences.DisplayScale, a.systemScale())
	if c.Zoom() != z {
		c.SetZoom(z)
	}
}

// displayScaleRow is Tampilan's "Skala tampilan".
func (a *App) displayScaleRow(c *ui.Context) {
	p := &a.chrome.preferences
	follow, desc := "Ikuti Windows", "Ukuran teks, ikon, dan tata letak di seluruh Space, seperti Skala di pengaturan Layar Windows."
	if s := a.systemScale(); s > 0 {
		desc += fmt.Sprintf(" Windows saat ini %d%%.", int(math.Round(s*100)))
	} else {
		follow, desc = "Ikuti browser", "Ukuran teks, ikon, dan tata letak di seluruh Space, di atas zoom browser."
	}
	labels := []string{follow}
	for _, s := range displayScales[1:] {
		labels = append(labels, fmt.Sprintf("%d%%", s))
	}
	SettingRow(c, "Skala tampilan", desc, func() {
		sel := 0
		for i, s := range displayScales {
			if s == p.DisplayScale {
				sel = i
			}
		}
		if i := segmented(c, "Skala tampilan", labels, sel); i >= 0 && i != sel {
			p.DisplayScale = displayScales[i]
			a.savePreferences(c)
		}
	})
}
