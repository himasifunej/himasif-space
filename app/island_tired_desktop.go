//go:build !js

package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/island"
)

// Ace ikut capek on the island (island.Tired, canvas 4033a40b94aa B): the
// pill's ink warming toward brown, the lids, the "+N" growing heavier, and
// the sweat, "zZ" and huff, all drawn inside the pill, which is all the
// window's region shows.

// The warm ends of the shift, as the canvas: a dark ink toward brown, a
// light one (Pasir, a light theme) toward caramel.
var (
	tiredWarmDark  = ui.RGB(0x5a, 0x3a, 0x1e)
	tiredWarmLight = ui.RGB(0xb8, 0x94, 0x6a)
	// The "+N" at its heaviest, on a dark and a light ink.
	tiredBadgeDark      = ui.RGB(0x4d, 0x3c, 0x27)
	tiredBadgeDarkText  = ui.RGB(0xf2, 0xe3, 0xc7)
	tiredBadgeLight     = ui.RGB(0xb9, 0x9a, 0x6d)
	tiredBadgeLightText = ui.RGB(0x2a, 0x26, 0x20)
	tiredSweat          = ui.RGB(0xa2, 0xbb, 0xd2)
)

// lightInk tells a light pill (Pasir, a light theme's) from a dark one.
func lightInk(c ui.Color) bool { return int(c.R)+int(c.G)+int(c.B) > 450 }

// mixColor is a toward b by k, keeping a's alpha.
func mixColor(a, b ui.Color, k float64) ui.Color {
	k = math.Max(0, math.Min(1, k))
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*k)) }
	return ui.RGBA(l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), float32(a.A)/255)
}

// tiredInk is the pill's ink, warm by warm (0 to 1): half way to brown at
// most, so a custom color stays itself.
func tiredInk(ink ui.Color, warm float64) ui.Color {
	if warm <= 0 {
		return ink
	}
	if lightInk(ink) {
		return mixColor(ink, tiredWarmLight, warm*.45)
	}
	return mixColor(ink, tiredWarmDark, warm*.5)
}

// tiredEye is one eye, a capsule w×h centred at cx, cy, its top lid
// dropped by lid of its height and slanted by slant degrees (grumpy).
func tiredEye(c *ui.Context, cx, cy, w, h, lid, slant float32) {
	if lid <= .01 && slant == 0 {
		ui.Box(c).Absolute().Left(cx-w/2).Top(cy-h/2).Size(w, h).Radius(min(w, h) / 2).Background(islandText)
		return
	}
	cut := h * min(lid, .9)
	// What the lid leaves: the eye's lower part, under a straight edge
	// that tilts inward, cute rather than cross.
	ui.Box(c).Absolute().Left(cx-w/2-2).Top(cy-h/2+cut).Size(w+4, h-cut).TransformOrigin(.5, 1).Rotate(slant * .6).Clip().Children(func() {
		ui.Box(c).Absolute().Left(2).Top(-cut).Size(w, h).Radius(min(w, h) / 2).Background(islandText)
	})
}

// tiredParticles are the stage's sweat (a drop down the left of the
// pill), "zZ" (rising between the eyes and the "+N") and huff (puffs out
// to the left), eyes the eyes' middle.
func tiredParticles(c *ui.Context, pose island.Pose, eyes, gw, gh float32) {
	if k := pose.Sweat; k >= 0 {
		y := 3 + 7*easeOut3(k)
		a := float32(math.Sin(math.Pi * k))
		ui.Box(c.Key("sweat")).Absolute().Left(16).Top(float32(y)).Size(5, 7).Radius(2.5).Background(tiredSweat.Alpha(a))
	}
	if pose.Zz >= 0 {
		for i, size := range [2]float32{10, 8} {
			k := math.Max(0, math.Min(1, pose.Zz*1.25-float64(i)*.25))
			if k <= 0 || k >= 1 {
				continue
			}
			x := eyes + island.EyePair/2 + 6 + float32(k)*6 + float32(i)*5
			y := gh - 13 - float32(k)*(gh-18) - float32(i)*3
			ui.Text(c.Key(fmt.Sprint("zz", i)), "z").Absolute().Left(x).Top(y).FontSize(size).FontWeight(500).TextColor(islandMuted.Alpha(float32(math.Sin(math.Pi * k))))
		}
	}
	if pose.Puff >= 0 {
		for i := range 3 {
			k := math.Max(0, math.Min(1, pose.Puff*1.3-float64(i)*.15))
			if k <= 0 {
				continue
			}
			s := float32(5+2*i) * float32(.5+.5*easeOut3(k))
			x := 30 - float32(k)*float32(12+i*4) - s/2
			y := gh/2 + 3 - float32(k)*float32(4+i*3) - s/2
			ui.Box(c.Key(fmt.Sprint("puff", i))).Absolute().Left(x).Top(y).Size(s, s).Radius(s / 2).Background(islandMuted.Alpha(float32(.75 * math.Sin(math.Pi*k))))
		}
	}
	_ = gw
}

func easeOut3(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return 1 - math.Pow(1-x, 3)
}

// inboxBadge is "+N" on Ace at rest while notices wait: heavier (bigger,
// bolder, warmer) as Ace tires, and popping away as the inbox clears.
func inboxBadge(c *ui.Context, n int, gw, gh float32, pose island.Pose) {
	if n <= 0 || pose.Out >= 1 {
		return
	}
	label := fmt.Sprintf("+%d", min(n, 99))
	hb := float32(pose.Heavy)
	w, h := 28+6*hb, 16+2*hb
	bg, fg := mixColor(islandButton, tiredBadgeDark, float64(hb)), mixColor(islandText, tiredBadgeDarkText, float64(hb))
	if lightInk(islandInk) {
		bg, fg = mixColor(islandButton, tiredBadgeLight, float64(hb)), mixColor(islandText, tiredBadgeLightText, float64(hb))
	}
	weight := 600
	if hb > .5 {
		weight = 700
	}
	out := float32(1 - pose.Out)
	ui.Box(c.Key("inbox-badge")).Absolute().Left(gw-6-w).Top(gh/2-h/2+1.5*hb).Size(w, h).Radius(h/2).Background(bg).Scale(out, out).Opacity(out).Center().Label(label + " kabar belum dibaca").Children(func() {
		ui.Text(c, label).FontSize(10 + hb).FontWeight(weight).FontFeatures("tnum").TextColor(fg)
	})
}
