package appui

import (
	"image"
	"image/color"
	"testing"
)

func TestFitImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 320, 277))
	if b := FitImage(src, 132).Bounds(); b.Dx() != 132 || b.Dy() != 114 {
		t.Fatalf("scaled to %v", b)
	}
	small := image.NewRGBA(image.Rect(0, 0, 10, 20))
	small.Set(1, 1, color.RGBA{255, 0, 0, 255})
	out := FitImage(small, 132)
	if out.Bounds() != small.Bounds() || out.RGBAAt(1, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("small image changed: %v", out.Bounds())
	}
	if &out.Pix[0] == &small.Pix[0] {
		t.Fatal("a small image must be copied: NewBitmap keeps the buffer it gets")
	}
}

// Division logos decode once, the first time a page shows one, at the
// size their mark is drawn; both names of Kaderisasi share one bitmap.
func TestDivisionLogosSmallAndShared(t *testing.T) {
	for _, slug := range []string{"mediatek", "humas", "psdm", "kader", "litbang"} {
		b := divisionLogo(slug)
		if b == nil {
			t.Fatalf("%s: no logo", slug)
		}
		if w, h := b.Size(); w > logoSide || h > logoSide {
			t.Fatalf("%s kept at %d×%d", slug, w, h)
		}
		if divisionLogo(" "+slug+" ") != b {
			t.Fatalf("%s decoded twice", slug)
		}
	}
	if divisionLogo("kaderisasi") != divisionLogo("kader") {
		t.Fatal("kaderisasi and kader decoded apart")
	}
	if divisionLogo("bph") != nil || divisionLogo("medhum") != nil {
		t.Fatal("a division without an emblem got one")
	}
}
