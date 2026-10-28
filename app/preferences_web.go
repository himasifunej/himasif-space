//go:build js

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/himasifunej/himasif-space/internal/appui"
)

const preferencesKey = "space.appearance"

var errNoPreferencesStorage = errors.New("localStorage tidak tersedia untuk menyimpan penampilan")

// browserPreferences stores non-sensitive appearance only. Authentication keeps
// using the separate sessionStorage token store in platform_web.go.
type browserPreferences struct{}

func newPreferencesStore() appui.PreferencesStore { return browserPreferences{} }

func preferencesStorageCall(method string, args ...any) (value js.Value, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			value = js.Undefined()
			if cause, ok := failure.(error); ok {
				err = fmt.Errorf("appearance storage: %w", cause)
			} else {
				err = fmt.Errorf("appearance storage: %v", failure)
			}
		}
	}()
	storage := js.Global().Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return js.Undefined(), errNoPreferencesStorage
	}
	return storage.Call(method, args...), nil
}

func (browserPreferences) Load() (appui.Preferences, error) {
	defaults := appui.DefaultPreferences()
	value, err := preferencesStorageCall("getItem", preferencesKey)
	if err != nil {
		return defaults, err
	}
	if value.IsNull() || value.IsUndefined() {
		return defaults, nil
	}
	var preferences *appui.Preferences
	if err := json.Unmarshal([]byte(value.String()), &preferences); err != nil {
		return defaults, fmt.Errorf("decode appearance config: %w", err)
	}
	if preferences == nil {
		return defaults, errors.New("decode appearance config: expected a JSON object")
	}
	return appui.NormalizePreferences(*preferences), nil
}

func (browserPreferences) Save(preferences appui.Preferences) error {
	data, err := json.Marshal(appui.NormalizePreferences(preferences))
	if err != nil {
		return fmt.Errorf("encode appearance config: %w", err)
	}
	_, err = preferencesStorageCall("setItem", preferencesKey, string(data))
	return err
}
