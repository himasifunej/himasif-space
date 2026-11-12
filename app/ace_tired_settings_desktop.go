//go:build !js

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/island"
)

// Pengaturan › Ace › Ace ikut capek (canvas 4033a40b94aa B, the owner's
// changes): on or off, how far apart the stages begin (1,5×, 2×, 2,5× or
// Atur sendiri) and where Cemberut begins, 15 at most. A preview shows Ace
// at each stage with the numbers chosen.

// aceTiredRatioLabels are the choices of Jarak antar tahap, as
// island.Ratios.
var aceTiredRatioLabels = []string{"1,5×", "2×", "2,5×", "Atur sendiri"}

// aceTiredThresholds are the stages' beginnings under p.
func (p devicePreferences) aceTiredThresholds() [4]int {
	return island.Thresholds(p.AceTiredRatio, p.AceTiredTop, p.AceTiredHeavy, p.AceTiredSleepy)
}

// aceSummary is Ace's state in the column of sections.
func (p devicePreferences) aceSummary() string {
	if !p.AceTired {
		return "Tidak ikut capek"
	}
	return fmt.Sprintf("Cemberut di +%d", p.aceTiredThresholds()[3])
}

// aceTiredRange is where stage s begins and ends under th: "2–5", "15+".
func aceTiredRange(th [4]int, s int) string {
	switch s {
	case island.StageFresh:
		return "0"
	case island.StageGrumpy:
		return strconv.Itoa(th[3]) + "+"
	}
	a, b := th[s-1], th[s]-1
	if a == b {
		return strconv.Itoa(a)
	}
	return strconv.Itoa(a) + "–" + strconv.Itoa(b)
}

// aceTiredNote says how the stages are counted.
func (p devicePreferences) aceTiredNote() string {
	if p.AceTiredRatio == island.RatioCustom {
		return "Angka sendiri; Cemberut tetap paling tinggi 15."
	}
	return "Tiap tahap mulai " + strings.ReplaceAll(p.AceTiredRatio, ".", ",") + "× tahap sebelumnya, dihitung mundur dari " + strconv.Itoa(p.AceTiredTop) + "."
}

func (d *desktopDevice) aceSettings(c *ui.Context) {
	p := &d.prefs
	appui.SettingSection(c, "Ace", "Ace di island ikut merasakan kabar yang belum dibaca, di lonceng dan di Ace.", func() {
		appui.SettingPreview(c, 128, func() { d.aceTiredPreview(c) })
		if appui.SettingSwitch(c, "Ace ikut capek", "Makin banyak kabar menumpuk, Ace pelan-pelan lesu sampai cemberut. Begitu kotak masuk bersih, Ace segar lagi.", &p.AceTired) {
			d.saveIslandLook()
		}
		if !p.AceTired {
			return
		}
		sel := max(0, indexOf(island.Ratios, p.AceTiredRatio))
		appui.SettingRow(c, "Jarak antar tahap", p.aceTiredNote(), func() {
			if i := appui.Segmented(c, "Jarak antar tahap", aceTiredRatioLabels, sel); i >= 0 && i != sel {
				p.AceTiredRatio = island.Ratios[i]
				if p.AceTiredRatio == island.RatioCustom {
					// Atur sendiri starts from the stages shown.
					th := island.Thresholds(island.Ratios[sel], p.AceTiredTop, 0, 0)
					p.AceTiredHeavy, p.AceTiredSleepy = th[1], th[2]
				}
				d.saveAceTired()
			}
		})
		appui.SettingRow(c, "Cemberut mulai di", "Jumlah kabar belum dibaca, maks. 15.", func() {
			if v, ok := aceStepper(c, "Cemberut mulai di", p.AceTiredTop, island.TopMin, island.TopMax); ok {
				p.AceTiredTop = v
				d.saveAceTired()
			}
		})
		if p.AceTiredRatio != island.RatioCustom {
			return
		}
		th := p.aceTiredThresholds()
		appui.SettingRow(c, "Mulai berat mulai di", "Sebelumnya Ace hanya melirik angkanya.", func() {
			if v, ok := aceStepper(c, "Mulai berat mulai di", th[1], 2, th[3]-2); ok {
				p.AceTiredHeavy = v
				d.saveAceTired()
			}
		})
		appui.SettingRow(c, "Ngantuk mulai di", "Di antara Mulai berat dan Cemberut.", func() {
			if v, ok := aceStepper(c, "Ngantuk mulai di", th[2], th[1]+1, th[3]-1); ok {
				p.AceTiredSleepy = v
				d.saveAceTired()
			}
		})
	})
}

// saveAceTired keeps the numbers in order, saves and redraws Ace.
func (d *desktopDevice) saveAceTired() {
	d.prefs.normalizeAceTired()
	d.saveIslandLook()
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// aceStepper is − value + for a number from lo to hi; it reports the new
// value when a button changed it.
func aceStepper(c *ui.Context, label string, v, lo, hi int) (int, bool) {
	t := c.Theme()
	next, changed := v, false
	ui.Row(c.Key("step-" + label)).Gap(8).AlignItems(ui.Center).Label(label).Children(func() {
		if appui.SettingButton(c, "−", "").Label("Kurangi "+strings.ToLower(label)).Disabled(v <= lo).Clicked() && v > lo {
			next, changed = v-1, true
		}
		ui.Text(c, strconv.Itoa(v)).FontSize(14).FontWeight(600).FontFeatures("tnum").MinWidth(24).TextAlign(ui.Center).TextColor(t.Text)
		if appui.SettingButton(c, "+", "").Label("Tambah "+strings.ToLower(label)).Disabled(v >= hi).Clicked() && v < hi {
			next, changed = v+1, true
		}
	})
	return next, changed
}

// aceTiredPreview is Ace at each stage, still, with the numbers chosen:
// grey when Ace ikut capek is off, but Segar.
func (d *desktopDevice) aceTiredPreview(c *ui.Context) {
	t := c.Theme()
	var base *ui.Theme
	if d.app != nil {
		base = d.app.IslandTheme(t.Dark)
	}
	pal := islandPaletteFor(d.prefs, base)
	th := d.prefs.aceTiredThresholds()
	ui.Row(c).Fill().Padding(14, 12).Gap(4).Justify(ui.SpaceAround).AlignItems(ui.Center).Children(func() {
		for s := range 5 {
			opacity := float32(1)
			if !d.prefs.AceTired && s > island.StageNews {
				opacity = .35
			}
			ui.Column(c.Key(s)).Gap(6).AlignItems(ui.Center).Opacity(opacity).Children(func() {
				n := 0
				if s > 0 {
					n = th[s-1]
				}
				stage := s
				if !d.prefs.AceTired {
					stage = min(s, island.StageNews)
				}
				aceStill(c, pal, island.TiredPose(stage, 0, true), n, .78)
				ui.Text(c, island.StageNames[s]).FontSize(12).FontWeight(500).TextColor(t.Text)
				ui.Text(c, "+"+aceTiredRange(th, s)).FontSize(11).FontFeatures("tnum").TextColor(t.TextMuted)
			})
		}
	})
}

// aceStill is Ace at rest in pose with n unread, still, at scale, in pal:
// the island's pill as it draws it, for the settings and the screenshots.
func aceStill(c *ui.Context, pal islandPalette, pose island.Pose, n int, scale float32) {
	room := 0.
	if n > 0 {
		room = island.BadgeRoom
	}
	aceStillRoom(c, pal, pose, n, room, scale)
}

// aceStillRoom is aceStill with the badge's room as it springs.
func aceStillRoom(c *ui.Context, pal islandPalette, pose island.Pose, n int, room float64, scale float32) {
	pal.use()
	g := island.Geometry(island.SceneIdle, false)
	w, h := g[0]+room, g[1]
	dw, dh := w*pose.SX, h*pose.SY
	ox := (w-dw)/2 + pose.X
	oy := h*(1-pose.SY) + pose.Y
	box := ui.Box(c.Key("ace")).Size(float32(g[0]+island.BadgeRoom)*scale, float32(h+6)*scale)
	box.Children(func() {
		ui.Box(c).Absolute().Left(0).Top(0).Size(float32(g[0]+island.BadgeRoom), float32(h+6)).TransformOrigin(0, 0).Scale(scale, scale).Children(func() {
			gw, gh := float32(dw), float32(dh)
			ui.Box(c.Key("pill")).Absolute().Left(float32(ox)).Top(float32(oy)).Size(gw, gh).Radius(gh/2).Background(tiredInk(islandInk, pose.Warm)).Border(1, islandRing).Clip().Children(func() {
				// The eyes stay where they were as the pill scales.
				eyes := float32(g[0]/2 - (w-dw)/2)
				eyeW := float32(island.EyeWidth * pose.Pop)
				eyeH := float32(island.EyeHeight*pose.Pop) * float32(1-pose.Close)
				cy := gh/2 + float32(pose.EyeY)
				for _, side := range [2]float64{-1, 1} {
					cx := eyes + float32(side*(island.EyePair-island.EyeWidth)/2+pose.GlanceX)
					tiredEye(c, cx, cy, eyeW, max(1.4, eyeH), float32(pose.Lid), float32(-side*pose.Angle))
				}
				tiredParticles(c, pose, eyes, gw, gh)
				inboxBadge(c, n, gw, gh, pose)
			})
		})
	})
}
