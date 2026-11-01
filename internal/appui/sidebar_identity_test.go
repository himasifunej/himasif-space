package appui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The sidebar separates the Space brand from the period picker and draws the
// division's published emblem instead of a generic mark. Both were reported as
// missing in feedback 01, so the rendered frame is checked directly.
func TestSidebarBrandGapAndDivisionEmblem(t *testing.T) {
	h := newChromeHarness(t, nil)
	// fakeAPI's first division is Mediatek, whose emblem is orange, so the
	// rendered colours can be told apart from the monochrome interface icons.
	space := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.tt.SetSize(1280, 900)
	h.tt.Frame()

	brand, ok := h.tt.Find("space.")
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
	// The brand text sits inside a 62 px row, so the picker's 8 px top margin
	// is measured from the text as roughly 32 px. A growing spacer would leave
	// hundreds of pixels, and no margin at all would leave about 24.
	gap := picker.Y - (brand.Y + brand.H)
	t.Logf("brand %+v, picker %+v, gap %.1f", brand, picker, gap)
	if gap < 26 || gap > 40 {
		t.Errorf("period picker sits %.1f px below the brand; want about 32", gap)
	}

	row, ok := h.tt.Find(space.Name)
	if !ok {
		t.Fatalf("no %q row; texts %q", space.Name, h.tt.Texts())
	}
	if warm := divisionEmblemPixels(h.tt.Image(), row); warm < 60 {
		t.Errorf("%q shows %d emblem pixels beside the name; want the Mediatek artwork", space.Name, warm)
	}
}

// divisionEmblemPixels counts the emblem's warm pixels in the square left of a
// sidebar row's label.
func divisionEmblemPixels(img *image.RGBA, row ui.Rect) int {
	x0 := int(row.X) + 6
	x1 := int(row.X) + 42
	y0 := int(row.Y) - 2
	y1 := int(row.Y+row.H) + 2
	count := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
				continue
			}
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			// Mediatek's emblem is saturated orange; interface icons are grey.
			if r>>8 > 150 && g>>8 > 40 && g>>8 < 170 && b>>8 < 90 {
				count++
			}
		}
	}
	return count
}
