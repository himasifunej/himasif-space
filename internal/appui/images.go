package appui

import (
	"bytes"
	"image"
	_ "image/png" // the embedded division logos
	"sync"

	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"
)

// FitImage returns a copy of img scaled down, keeping its proportions, so
// that its longer side is at most side pixels; a smaller image is copied
// as it is. A bitmap made with ui.NewBitmap keeps every pixel it is given
// for as long as it lives, and takes an *image.RGBA without copying it, so
// pictures drawn small are kept small and in a buffer of their own: a
// 320 px logo shown at 44 DIP would keep five times the pixels a 200%
// display draws.
func FitImage(img image.Image, side int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > side || h > side {
		if w >= h {
			w, h = side, max(1, h*side/w)
		} else {
			w, h = max(1, w*side/h), side
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == b.Dx() && h == b.Dy() {
		xdraw.Draw(dst, dst.Rect, img, b.Min, xdraw.Src)
	} else {
		xdraw.CatmullRom.Scale(dst, dst.Rect, img, b, xdraw.Src, nil)
	}
	return dst
}

// logoSide is the size division logos are kept at: their 44 DIP mark at
// 300% scale.
const logoSide = 132

var divisionLogoCache struct {
	sync.Mutex
	bitmaps map[string]*ui.Bitmap
}

// divisionLogo is the emblem of the division slug, decoded the first time
// a page shows it; nil for a division without one.
func divisionLogo(slug string) *ui.Bitmap {
	slug = normalizedSlug(slug)
	data, ok := spaceDivisionLogoData[slug]
	if !ok {
		return nil
	}
	c := &divisionLogoCache
	c.Lock()
	defer c.Unlock()
	if b, ok := c.bitmaps[slug]; ok {
		return b
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic("appui: invalid embedded division logo: " + err.Error())
	}
	b := ui.NewBitmap(FitImage(img, logoSide))
	if c.bitmaps == nil {
		c.bitmaps = map[string]*ui.Bitmap{}
	}
	c.bitmaps[slug] = b
	return b
}
