//go:build js

package appui

import (
	"embed"
	"errors"
	"syscall/js"
	"time"
)

//go:embed assets/fonts/*.woff2
var fontFiles embed.FS

// RegisterFonts adds the embedded Plex fonts to the page's fonts, one
// FontFace per weight, and waits until the browser has read them, so that
// the first frame measures text with Plex and not a fallback. MyGo's
// ui.RegisterFont gives every face the normal weight, which would make
// browsers draw bold text by thickening the regular face; hence the faces
// are added here with their weights. Call it once, before the first window.
func RegisterFonts() error {
	fonts := js.Global().Get("document").Get("fonts")
	if fonts.IsUndefined() || js.Global().Get("FontFace").IsUndefined() {
		return errors.New("appui: this browser cannot add fonts")
	}
	done := make(chan bool, len(fontFaces))
	loaded := js.FuncOf(func(js.Value, []js.Value) any { done <- true; return nil })
	failed := js.FuncOf(func(js.Value, []js.Value) any { done <- false; return nil })
	pending := 0
	for _, f := range fontFaces {
		data, err := fontFiles.ReadFile("assets/fonts/" + f.file + ".woff2")
		if err != nil {
			return err
		}
		buf := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(buf, data)
		face := js.Global().Get("FontFace").New(f.family, buf.Get("buffer"), map[string]any{"weight": f.weights, "style": "normal"})
		fonts.Call("add", face)
		face.Call("load").Call("then", loaded, failed)
		pending++
	}
	timeout := time.After(3 * time.Second)
	var err error
	for ; pending > 0; pending-- {
		select {
		case ok := <-done:
			if !ok {
				err = errors.New("appui: a font did not load")
			}
		case <-timeout:
			// The callbacks stay alive for the faces still loading.
			return errors.New("appui: fonts took too long to load")
		}
	}
	loaded.Release()
	failed.Release()
	return err
}
