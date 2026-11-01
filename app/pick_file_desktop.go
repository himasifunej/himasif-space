//go:build !js

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/appui"
)

func pickFile(win *mygo.Window, limit int64, done func(appui.SelectedFile, error)) {
	go func() {
		files, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Pilih lampiran untuk diunggah"})
		if err != nil || len(files) == 0 {
			done(appui.SelectedFile{}, err)
			return
		}
		f, err := os.Open(files[0])
		if err != nil {
			done(appui.SelectedFile{}, err)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			done(appui.SelectedFile{}, err)
			return
		}
		if !info.Mode().IsRegular() || info.Size() > limit {
			done(appui.SelectedFile{}, fmt.Errorf("Berkas harus berupa file biasa dengan ukuran maksimal %d byte.", limit))
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, limit+1))
		if err == nil && int64(len(data)) > limit {
			err = fmt.Errorf("Berkas melampaui batas ukuran.")
		}
		if err != nil {
			done(appui.SelectedFile{}, err)
			return
		}
		done(appui.SelectedFile{Name: filepath.Base(files[0]), Data: data}, nil)
	}()
}
