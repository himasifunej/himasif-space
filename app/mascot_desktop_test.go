//go:build !js

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

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
