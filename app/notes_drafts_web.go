//go:build js

package main

import (
	"errors"
	"fmt"
	"syscall/js"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

type browserNoteDrafts struct{}

func newNoteDraftStore() appui.NoteDraftStore { return browserNoteDrafts{} }

func noteDraftKey(userID, noteID id.ID) string {
	return "space.note-drafts." + userID.String() + "." + noteID.String()
}

func noteDraftStorageCall(method string, args ...any) (value js.Value, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			value = js.Undefined()
			if cause, ok := failure.(error); ok {
				err = fmt.Errorf("penyimpanan draf perangkat: %w", cause)
			} else {
				err = fmt.Errorf("penyimpanan draf perangkat: %v", failure)
			}
		}
	}()
	storage := js.Global().Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return js.Undefined(), errors.New("localStorage tidak tersedia untuk draf perangkat")
	}
	return storage.Call(method, args...), nil
}

func (browserNoteDrafts) Load(userID, noteID id.ID) (*appui.LocalNoteDraft, error) {
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return nil, err
	}
	value, err := noteDraftStorageCall("getItem", noteDraftKey(userID, noteID))
	if err != nil {
		return nil, err
	}
	if value.IsNull() || value.IsUndefined() {
		return nil, nil
	}
	if value.Type() != js.TypeString {
		return nil, errors.New("format penyimpanan draf perangkat tidak valid")
	}
	return decodeLocalNoteDraft(userID, noteID, []byte(value.String()))
}

func (browserNoteDrafts) Save(userID, noteID id.ID, draft appui.LocalNoteDraft) error {
	data, err := encodeLocalNoteDraft(userID, noteID, draft)
	if err != nil {
		return err
	}
	_, err = noteDraftStorageCall("setItem", noteDraftKey(userID, noteID), string(data))
	return err
}

func (browserNoteDrafts) Delete(userID, noteID id.ID) error {
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return err
	}
	_, err := noteDraftStorageCall("removeItem", noteDraftKey(userID, noteID))
	return err
}
