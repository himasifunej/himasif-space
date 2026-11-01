//go:build !js

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"

	"github.com/egoist/mygo/ui"
)

// Inspect records before DecodeAll allocates frame images. A small compressed
// GIF can otherwise exceed the device budget before its frame count is checked.
func checkGIFBudget(data []byte, width, height int) error {
	invalid := fmt.Errorf("GIF tidak utuh atau tidak didukung")
	if len(data) < 13 {
		return invalid
	}
	offset := 13
	palette := func(packed byte) int {
		if packed&0x80 == 0 {
			return 0
		}
		return 3 * (1 << ((packed & 7) + 1))
	}
	offset += palette(data[10])
	frames := 0
	blocks := func() bool {
		for offset < len(data) {
			size := int(data[offset])
			offset++
			if size == 0 {
				return true
			}
			if size > len(data)-offset {
				return false
			}
			offset += size
		}
		return false
	}
	for offset < len(data) {
		marker := data[offset]
		offset++
		switch marker {
		case 0x3b:
			if frames == 0 || offset != len(data) {
				return invalid
			}
			return nil
		case 0x21:
			if offset >= len(data) {
				return invalid
			}
			offset++
			if !blocks() {
				return invalid
			}
		case 0x2c:
			if len(data)-offset < 9 {
				return invalid
			}
			left, top := int(binary.LittleEndian.Uint16(data[offset:])), int(binary.LittleEndian.Uint16(data[offset+2:]))
			w, h := int(binary.LittleEndian.Uint16(data[offset+4:])), int(binary.LittleEndian.Uint16(data[offset+6:]))
			if w == 0 || h == 0 || left+w > width || top+h > height {
				return invalid
			}
			packed := data[offset+8]
			offset += 9 + palette(packed)
			frames++
			if frames > 120 || int64(frames)*int64(width)*int64(height) > 16000000 {
				return fmt.Errorf("GIF maksimal 120 frame dan total 16 juta piksel")
			}
			if offset >= len(data) {
				return invalid
			}
			offset++ // LZW code size; decoder validates its value.
			if !blocks() {
				return invalid
			}
		default:
			return invalid
		}
	}
	return invalid
}
func decodeMascot(data []byte) (*gif.GIF, *ui.Bitmap, error) {
	if len(data) > 8<<20 {
		return nil, nil, fmt.Errorf("maskot maksimal 8 MiB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 || (format != "png" && format != "gif") {
		return nil, nil, fmt.Errorf("gunakan PNG/GIF maksimal 2048 × 2048 piksel")
	}
	if format == "gif" {
		if err = checkGIFBudget(data, cfg.Width, cfg.Height); err != nil {
			return nil, nil, err
		}
		g, err := gif.DecodeAll(bytes.NewReader(data))
		return g, nil, err
	}
	bitmap, err := ui.DecodeBitmap(data)
	return nil, bitmap, err
}
