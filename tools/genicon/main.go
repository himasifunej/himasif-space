// Command genicon writes icon.png, a PLACEHOLDER icon: a plain rounded square in
// a calm indigo. It is not Space's logo, which is not designed yet (PRD
// OPEN-01); replace icon.png when it is.
//
//	go run ./tools/genicon -o app/resources/icon.png
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

func main() {
	out := flag.String("o", "icon.png", "the file to write")
	flag.Parse()

	const (
		size   = 1024
		margin = 80.0  // room around the square, as icons of macOS leave
		radius = 190.0 // of the corners
	)
	fill := color.NRGBA{R: 0x4f, G: 0x46, B: 0xe5, A: 0xff} // indigo
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	lo, hi := margin, size-margin
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Coverage of the pixel by the rounded square, sampled 4×4 for
			// smooth edges.
			n := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					px, py := float64(x)+(float64(sx)+0.5)/4, float64(y)+(float64(sy)+0.5)/4
					if inside(px, py, lo, hi, radius) {
						n++
					}
				}
			}
			if n > 0 {
				c := fill
				c.A = uint8(int(fill.A) * n / 16)
				img.SetNRGBA(x, y, c)
			}
		}
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

// inside reports whether (x, y) lies in the square from lo to hi whose
// corners are rounded by r.
func inside(x, y, lo, hi, r float64) bool {
	if x < lo || x > hi || y < lo || y > hi {
		return false
	}
	cx := math.Max(lo+r, math.Min(x, hi-r))
	cy := math.Max(lo+r, math.Min(y, hi-r))
	return math.Hypot(x-cx, y-cy) <= r
}
