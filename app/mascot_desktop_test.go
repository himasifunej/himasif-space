//go:build !js

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"testing"
)

func pngEncode(w io.Writer, img image.Image) error { return png.Encode(w, img) }

func mascotGIF(t *testing.T, frames, width, height int) []byte {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: width, Height: height}, LoopCount: -1}
	for range frames {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White}))
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestMascotGIFBudgetBeforeFrameAllocation(t *testing.T) {
	valid := mascotGIF(t, 2, 2, 2)
	g, bitmap, err := decodeMascot(valid)
	if err != nil || bitmap != nil || len(g.Image) != 2 {
		t.Fatalf("valid GIF rejected %v", err)
	}
	for _, data := range [][]byte{mascotGIF(t, 121, 1, 1), mascotGIF(t, 5, 2000, 2000), valid[:len(valid)-1], []byte("unsupported asset")} {
		if _, _, err := decodeMascot(data); err == nil {
			t.Fatal("unsupported/oversize/truncated mascot accepted")
		}
	}
}

// An imported mascot is kept at the size Ace draws it: a 2048 px PNG and
// the frames of a large GIF are scaled to mascotSide, each frame in its
// own pixels.
func TestMascotKeptSmall(t *testing.T) {
	root := t.TempDir()
	d := &desktopDevice{root: root, prefs: devicePreferences{Mascot: "ace.png"}}
	big := image.NewRGBA(image.Rect(0, 0, 2048, 1024))
	var png bytes.Buffer
	if err := pngEncode(&png, big); err != nil {
		t.Fatal(err)
	}
	if err := atomicDeviceFile(root+"/mascot", png.Bytes()); err != nil {
		t.Fatal(err)
	}
	d.loadMascot()
	if len(d.frames) != 1 {
		t.Fatalf("frames %d, err %q", len(d.frames), d.err)
	}
	if w, h := d.frames[0].Size(); w != mascotSide || h != mascotSide/2 {
		t.Fatalf("PNG kept at %d×%d", w, h)
	}

	// Three 400×400 frames, red, green, blue.
	g := &gif.GIF{Config: image.Config{Width: 400, Height: 400}}
	for _, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		f := image.NewPaletted(image.Rect(0, 0, 400, 400), color.Palette{c})
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 10)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	if err := atomicDeviceFile(root+"/mascot", b.Bytes()); err != nil {
		t.Fatal(err)
	}
	d.prefs.Mascot = "ace.gif"
	d.loadMascot()
	if len(d.frames) != 3 {
		t.Fatalf("frames %d, err %q", len(d.frames), d.err)
	}
	for i, f := range d.frames {
		if w, h := f.Size(); w != mascotSide || h != mascotSide {
			t.Fatalf("frame %d kept at %d×%d", i, w, h)
		}
	}
}
