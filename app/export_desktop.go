//go:build !js

package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func exportFile(win *mygo.Window, name, mediaType string, data []byte) (string, error) {
	filters := []mygo.FileFilter{}
	if extension := strings.TrimPrefix(filepath.Ext(name), "."); extension != "" {
		filters = append(filters, mygo.FileFilter{Name: "Berkas " + extension, Extensions: []string{extension}})
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: "Simpan ekspor Space", DefaultPath: filepath.Base(name), Filters: filters, CreateDirectories: true})
	if err != nil || path == "" {
		return "", err
	}
	// The native dialog handles overwrite consent. Write a complete private
	// temporary file first so a failed download/write is not an apparent export.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".space-export-*")
	if err != nil {
		return "", err
	}
	tempPath := tmp.Name()
	defer os.Remove(tempPath)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	// os.Rename supports replacing a regular file on the desktop platforms.
	if err = os.Rename(tempPath, path); err != nil {
		return "", err
	}
	return path, nil
}
