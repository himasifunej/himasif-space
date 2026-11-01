//go:build js

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

type browserPendingFiles struct{}

func newPendingFileStore() appui.PendingFileStore { return browserPendingFiles{} }
func pendingIDBKey(server string, user, note id.ID) (string, string, error) {
	key, err := pendingServerKey(server)
	if err != nil {
		return "", "", err
	}
	if user == id.Nil || note == id.Nil {
		return "", "", errors.New("identitas pending tidak valid")
	}
	prefix := key + "." + user.String() + "."
	return prefix + note.String(), prefix, nil
}
func openPendingDB() (database js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("penyimpanan berkas browser: %v", r)
		}
	}()
	indexed := js.Global().Get("indexedDB")
	if indexed.IsUndefined() || indexed.IsNull() {
		return js.Undefined(), errors.New("IndexedDB tidak tersedia; berkas belum disimpan pada perangkat")
	}
	request := indexed.Call("open", "space.pending-files.v1", 1)
	done := make(chan error, 1)
	upgrade := js.FuncOf(func(js.Value, []js.Value) any { request.Get("result").Call("createObjectStore", "files"); return nil })
	success := js.FuncOf(func(js.Value, []js.Value) any { database = request.Get("result"); done <- nil; return nil })
	failure := js.FuncOf(func(js.Value, []js.Value) any {
		done <- errors.New("IndexedDB belum dapat dibuka; berkas belum disimpan pada perangkat")
		return nil
	})
	defer upgrade.Release()
	defer success.Release()
	defer failure.Release()
	request.Set("onupgradeneeded", upgrade)
	request.Set("onsuccess", success)
	request.Set("onerror", failure)
	err = <-done
	return database, err
}

// A transaction owns all callbacks until commit/abort, so success is not
// reported merely because put/delete was queued. One transaction serializes
// capacity checks and changes across browser tabs.
func pendingTransaction(db js.Value, mode string, work func(js.Value, js.Value, func(func(js.Value, []js.Value) any) js.Func, func(error))) (err error) {
	var callbacks []js.Func
	defer func() {
		for _, callback := range callbacks {
			callback.Release()
		}
		if r := recover(); r != nil {
			err = fmt.Errorf("penyimpanan berkas browser: %v", r)
		}
	}()
	tx := db.Call("transaction", []any{"files"}, mode)
	store := tx.Call("objectStore", "files")
	done := make(chan error, 1)
	var cause error
	abort := func(failure error) { cause = failure; tx.Call("abort") }
	callback := func(fn func(js.Value, []js.Value) any) js.Func {
		cb := js.FuncOf(func(this js.Value, args []js.Value) (result any) {
			defer func() {
				if r := recover(); r != nil {
					abort(fmt.Errorf("rekaman pending browser tidak valid: %v", r))
				}
			}()
			return fn(this, args)
		})
		callbacks = append(callbacks, cb)
		return cb
	}
	tx.Set("oncomplete", callback(func(js.Value, []js.Value) any { done <- nil; return nil }))
	tx.Set("onabort", callback(func(js.Value, []js.Value) any {
		if cause == nil {
			cause = errors.New("Penyimpanan pending dibatalkan, mungkin karena kuota browser. Salinan sebelumnya tetap dipertahankan.")
		}
		done <- cause
		return nil
	}))
	work(tx, store, callback, abort)
	return <-done
}
func pendingBrowserRecord(value js.Value) ([]byte, js.Value, error) {
	if value.Type() != js.TypeObject || value.IsNull() {
		return nil, js.Undefined(), errors.New("rekaman pending browser tidak valid")
	}
	meta := value.Get("metadata")
	data := value.Get("data")
	if meta.Type() != js.TypeString || !data.InstanceOf(js.Global().Get("Uint8Array")) {
		return nil, js.Undefined(), errors.New("format berkas pending browser tidak valid")
	}
	return []byte(meta.String()), data, nil
}
func (browserPendingFiles) Load(server string, user, note id.ID) (*appui.PendingNoteFile, error) {
	key, _, err := pendingIDBKey(server, user, note)
	if err != nil {
		return nil, err
	}
	db, err := openPendingDB()
	if err != nil {
		return nil, err
	}
	defer db.Call("close")
	var result *appui.PendingNoteFile
	err = pendingTransaction(db, "readonly", func(tx, store js.Value, callback func(func(js.Value, []js.Value) any) js.Func, abort func(error)) {
		request := store.Call("get", key)
		request.Set("onsuccess", callback(func(js.Value, []js.Value) any {
			value := request.Get("result")
			if value.IsUndefined() {
				return nil
			}
			metadata, bytes, failure := pendingBrowserRecord(value)
			if failure != nil {
				abort(failure)
				return nil
			}
			result, failure = decodePendingFile(server, user, note, metadata)
			if failure != nil {
				abort(failure)
				return nil
			}
			size := bytes.Get("byteLength").Int()
			if size > pendingFileMaxBytes || int64(size) != result.SizeBytes {
				abort(errors.New("ukuran pending browser tidak valid"))
				return nil
			}
			result.Data = make([]byte, size)
			js.CopyBytesToGo(result.Data, bytes)
			if failure = verifyPendingFile(result); failure != nil {
				abort(failure)
			}
			return nil
		}))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (browserPendingFiles) Save(server string, user id.ID, file appui.PendingNoteFile) error {
	metadata, err := encodePendingFile(server, user, file)
	if err != nil {
		return err
	}
	if err = verifyPendingFile(&file); err != nil {
		return err
	}
	key, prefix, err := pendingIDBKey(server, user, file.NoteID)
	if err != nil {
		return err
	}
	db, err := openPendingDB()
	if err != nil {
		return err
	}
	defer db.Call("close")
	return pendingTransaction(db, "readwrite", func(tx, store js.Value, callback func(func(js.Value, []js.Value) any) js.Func, abort func(error)) {
		var total int64
		var count int
		request := store.Call("openCursor", js.Global().Get("IDBKeyRange").Call("bound", prefix, prefix+"\uffff"))
		request.Set("onsuccess", callback(func(js.Value, []js.Value) any {
			cursor := request.Get("result")
			if cursor.IsNull() {
				if count >= pendingAccountMaxCount || total+file.SizeBytes > pendingAccountMaxBytes {
					abort(errors.New("batas pending perangkat tercapai: 32 berkas atau 256 MiB per akun dan server"))
					return nil
				}
				bytes := js.Global().Get("Uint8Array").New(len(file.Data))
				js.CopyBytesToJS(bytes, file.Data)
				store.Call("put", map[string]any{"metadata": string(metadata), "data": bytes}, key)
				return nil
			}
			data, _, failure := pendingBrowserRecord(cursor.Get("value"))
			if failure != nil {
				abort(failure)
				return nil
			}
			var saved savedPendingFile
			if failure = json.Unmarshal(data, &saved); failure != nil {
				abort(failure)
				return nil
			}
			current, failure := decodePendingFile(server, user, saved.File.NoteID, data)
			if failure != nil {
				abort(failure)
				return nil
			}
			if current.NoteID == file.NoteID {
				if current.UploadID != file.UploadID || current.SHA256 != file.SHA256 || current.Name != file.Name {
					abort(errors.New("catatan ini masih memiliki berkas pending lain; pulihkan atau buang terlebih dahulu"))
					return nil
				}
			} else {
				count++
				total += current.SizeBytes
			}
			cursor.Call("continue")
			return nil
		}))
	})
}
func (browserPendingFiles) Delete(server string, user, note, upload id.ID) error {
	key, _, err := pendingIDBKey(server, user, note)
	if err != nil {
		return err
	}
	db, err := openPendingDB()
	if err != nil {
		return err
	}
	defer db.Call("close")
	return pendingTransaction(db, "readwrite", func(tx, store js.Value, callback func(func(js.Value, []js.Value) any) js.Func, abort func(error)) {
		request := store.Call("get", key)
		request.Set("onsuccess", callback(func(js.Value, []js.Value) any {
			value := request.Get("result")
			if value.IsUndefined() {
				return nil
			}
			metadata, _, failure := pendingBrowserRecord(value)
			if failure != nil {
				abort(failure)
				return nil
			}
			file, failure := decodePendingFile(server, user, note, metadata)
			if failure != nil {
				abort(failure)
				return nil
			}
			if file.UploadID != upload {
				abort(errors.New("berkas pending berubah; salinan baru tidak dihapus"))
				return nil
			}
			store.Call("delete", key)
			return nil
		}))
	})
}
