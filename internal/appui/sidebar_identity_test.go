package appui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The sidebar separates the Space brand from the period picker, and marks
// each Space with a colored letter tile as on canvas A (the division emblem
// of feedback 01 gave way to the canvas' tiles). The rendered frame is
// checked directly.
func TestSidebarBrandGapAndSpaceTile(t *testing.T) {
	h := newChromeHarness(t, nil)
	space := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.tt.SetSize(1280, 900)
	h.tt.Frame()

	brand, ok := h.tt.Find("Ruang Kerja HIMASIF")
	if !ok {
		t.Fatalf("no Space brand; texts %q", h.tt.Texts())
	}
	p := h.app.period()
	if p == nil {
		t.Fatal("no selected period")
	}
	picker, ok := h.tt.Find(periodLabel(*p))
	if !ok {
		t.Fatalf("no period picker; texts %q", h.tt.Texts())
	}
	// The brand row's 12 px bottom padding separates it from the picker; a
	// growing spacer would leave hundreds of pixels.
	gap := picker.Y - (brand.Y + brand.H)
	t.Logf("brand %+v, picker %+v, gap %.1f", brand, picker, gap)
	if gap < 8 || gap > 30 {
		t.Errorf("period picker sits %.1f px below the brand; want about 12", gap)
	}

	row, ok := h.tt.Find(space.Name)
	if !ok {
		t.Fatalf("no %q row; texts %q", space.Name, h.tt.Texts())
	}
	if tile := tilePixels(h.tt.Image(), row); tile < 150 {
		t.Errorf("%q shows %d tile pixels beside the name; want the cream letter tile", space.Name, tile)
	}
}

// tilePixels counts the first tile color's (#dfcba8) pixels in the square
// at the start of a sidebar row (the row button spans tile and label).
func tilePixels(img *image.RGBA, row ui.Rect) int {
	count := 0
	for y := int(row.Y) - 4; y < int(row.Y+row.H)+4; y++ {
		for x := int(row.X); x < int(row.X)+40; x++ {
			if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
				continue
			}
			r, g, b, _ := img.At(x, y).RGBA()
			if d := absDiff(r>>8, 0xdf) + absDiff(g>>8, 0xcb) + absDiff(b>>8, 0xa8); d < 24 {
				count++
			}
		}
	}
	return count
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
