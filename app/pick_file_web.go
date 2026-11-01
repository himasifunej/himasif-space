//go:build js

package main

import (
	"fmt"
	"sync"
	"syscall/js"

	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/appui"
)

func pickFile(_ *mygo.Window, limit int64, done func(appui.SelectedFile, error)) {
	document := js.Global().Get("document")
	input := document.Call("createElement", "input")
	input.Set("type", "file")
	input.Get("style").Set("display", "none")
	document.Get("body").Call("appendChild", input)
	var once sync.Once
	var change, cancel, loaded, failed js.Func
	finish := func(file appui.SelectedFile, err error) {
		once.Do(func() {
			input.Set("onchange", js.Null())
			input.Set("oncancel", js.Null())
			input.Call("remove")
			change.Release()
			cancel.Release()
			loaded.Release()
			failed.Release()
			done(file, err)
		})
	}
	name := ""
	loaded = js.FuncOf(func(_ js.Value, args []js.Value) any {
		bytes := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, bytes.Get("byteLength").Int())
		js.CopyBytesToGo(data, bytes)
		if int64(len(data)) > limit {
			finish(appui.SelectedFile{}, fmt.Errorf("Berkas melampaui batas ukuran."))
		} else {
			finish(appui.SelectedFile{Name: name, Data: data}, nil)
		}
		return nil
	})
	failed = js.FuncOf(func(js.Value, []js.Value) any {
		finish(appui.SelectedFile{}, fmt.Errorf("Browser gagal membaca berkas yang dipilih."))
		return nil
	})
	change = js.FuncOf(func(js.Value, []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			finish(appui.SelectedFile{}, nil)
			return nil
		}
		file := files.Index(0)
		if file.Get("size").Float() > float64(limit) {
			finish(appui.SelectedFile{}, fmt.Errorf("Berkas melebihi batas %d byte.", limit))
			return nil
		}
		name = file.Get("name").String()
		file.Call("arrayBuffer").Call("then", loaded, failed)
		return nil
	})
	cancel = js.FuncOf(func(js.Value, []js.Value) any { finish(appui.SelectedFile{}, nil); return nil })
	input.Set("onchange", change)
	input.Set("oncancel", cancel)
	input.Call("click")
}
