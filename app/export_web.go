//go:build js

package main

import (
	"fmt"
	"syscall/js"
	"time"

	"github.com/egoist/mygo"
)

func exportFile(_ *mygo.Window, name, mediaType string, data []byte) (saved string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Browser belum dapat memulai unduhan: %v", r)
		}
	}()
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	blob := js.Global().Get("Blob").New([]any{bytes}, map[string]any{"type": mediaType})
	urls := js.Global().Get("URL")
	url := urls.Call("createObjectURL", blob).String()
	document := js.Global().Get("document")
	anchor := document.Call("createElement", "a")
	anchor.Set("href", url)
	anchor.Set("download", name)
	anchor.Get("style").Set("display", "none")
	document.Get("body").Call("appendChild", anchor)
	anchor.Call("click")
	anchor.Call("remove")
	time.AfterFunc(30*time.Second, func() { urls.Call("revokeObjectURL", url) })
	return "Unduhan diminta ke browser: " + name, nil
}
