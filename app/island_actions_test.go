//go:build !js

package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// islandActionsView is the island's "Hari ini" card in a style: Buka Space
// and Pengaturan island on the pill's ink.
func islandActionsView(p *islandPresenter, pal islandPalette) func(*ui.Context) {
	return func(c *ui.Context) {
		pal.use()
		ui.Column(c).Fill().Padding(16).Background(ui.RGB(96, 110, 140)).Children(func() {
			ui.Column(c).FillWidth().Padding(20).Radius(28).Background(islandInk).TextColor(islandText).Gap(2).Children(func() {
				ui.Text(c, "Hari ini").FontSize(15).FontWeight(600)
				ui.Text(c, "Tidak ada rapat lagi hari ini").FontSize(13).TextColor(islandMuted)
				ui.Box(c).Height(16)
				ui.Row(c).Gap(8).Children(func() {
					p.islandAction(c, "Buka Space", nil, true)
					p.islandAction(c, "Pengaturan island", nil, false)
				})
			})
		})
	}
}

// The island's buttons answer the pointer in every style (owner's
// feedback on rc.2: "seperti tombol tidak aktif"): hovered they change
// shade, pressed more, and the keyboard's focus rings them in the style's
// own color. SPACE_UI_CAPTURE_DIR keeps a picture of each state.
func TestIslandActionsHoverPressFocus(t *testing.T) {
	light := &ui.Theme{Surface: ui.RGB(250, 248, 244), SurfaceHover: ui.RGB(240, 236, 230), SurfacePressed: ui.RGB(228, 223, 215), Text: ui.RGB(30, 28, 25), TextMuted: ui.RGB(110, 104, 96), Border: ui.RGB(220, 214, 205)}
	dark := &ui.Theme{Surface: ui.RGB(28, 27, 31), SurfaceHover: ui.RGB(40, 39, 44), SurfacePressed: ui.RGB(52, 50, 57), Text: ui.RGB(240, 238, 234), TextMuted: ui.RGB(160, 156, 150), Border: ui.RGB(60, 58, 64), Dark: true}
	styles := []struct {
		name string
		pal  islandPalette
	}{
		{"hitam", islandPaletteFor(devicePreferences{IslandTheme: islandThemeBlack}, nil)},
		{"kaca", islandPaletteFor(devicePreferences{IslandTheme: islandThemeGlass}, nil)},
		{"ikut-tema-terang", islandPaletteFor(devicePreferences{}, light)},
		{"ikut-tema-gelap", islandPaletteFor(devicePreferences{}, dark)},
		{"custom-pasir", islandPaletteFor(devicePreferences{IslandTheme: islandThemeCustom, IslandColor: "#e8dcc4"}, nil)},
	}
	defer islandPaletteFor(devicePreferences{IslandTheme: islandThemeBlack}, nil).use()
	var shots []image.Image
	for _, s := range styles {
		p := &islandPresenter{}
		tt := ui.NewTester(islandActionsView(p, s.pal), 420, 150)
		frame := func() {
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
		}
		frame()
		sample := func(label string) color.RGBA {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s: no %q", s.name, label)
			}
			// Inside the button's padding, left of its text.
			return tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
		}
		at := func(label string) (float32, float32) {
			r, _ := tt.Find(label)
			return r.X + r.W/2, r.Y + r.H/2
		}
		var row []image.Image
		snap := func() { row = append(row, cloneImage(tt.Image())) }
		for _, label := range []string{"Buka Space", "Pengaturan island"} {
			tt.Move(400, 140)
			frame()
			rest := sample(label)
			if label == "Buka Space" {
				snap()
			}
			x, y := at(label)
			tt.Move(x, y)
			frame()
			hover := sample(label)
			snap()
			tt.Press(x, y)
			frame()
			press := sample(label)
			snap()
			tt.Release(x, y)
			if rest == hover || hover == press || rest == press {
				t.Fatalf("%s %q: rest %v hover %v press %v", s.name, label, rest, hover, press)
			}
		}
		// The keyboard's focus: a ring of the style's text color.
		tt.Move(400, 140)
		tt.Key(0, ui.KeyTab)
		frame()
		snap()
		shots = append(shots, joinImages(row, false))
	}
	if dir := os.Getenv("SPACE_UI_CAPTURE_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "island-actions-states.png"), joinImages(shots, true))
	}
}

func cloneImage(m *image.RGBA) image.Image {
	out := image.NewRGBA(m.Bounds())
	copy(out.Pix, m.Pix)
	return out
}

// joinImages puts images side by side, or one under another.
func joinImages(ms []image.Image, vertical bool) image.Image {
	w, h := 0, 0
	for _, m := range ms {
		b := m.Bounds()
		if vertical {
			w, h = max(w, b.Dx()), h+b.Dy()
		} else {
			w, h = w+b.Dx(), max(h, b.Dy())
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	x, y := 0, 0
	for _, m := range ms {
		b := m.Bounds()
		r := image.Rect(x, y, x+b.Dx(), y+b.Dy())
		for py := 0; py < b.Dy(); py++ {
			for px := 0; px < b.Dx(); px++ {
				out.Set(r.Min.X+px, r.Min.Y+py, m.At(b.Min.X+px, b.Min.Y+py))
			}
		}
		if vertical {
			y += b.Dy()
		} else {
			x += b.Dx()
		}
	}
	return out
}

func writePNG(t *testing.T, path string, m image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, m); err != nil {
		t.Fatal(err)
	}
}
