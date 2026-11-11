//go:build !js

package main

import (
	"encoding/json"
	"testing"
)

// A preferences.json from 0.4.4 keeps the island and the bell; the star
// mascot's keys are reported, for the file to be written without them.
func TestLoadDevicePreferencesDropsStarMascot(t *testing.T) {
	old := `{"islandEnabled":true,"islandStyle":"A","islandReplaceToast":false,"islandReduceMotion":true,"islandAlwaysOn":false,"autostart":true,"aceMode":"edge","aceName":"Bintang","mascot":"kucing.gif","audio":"bel.wav","volume":0.7,"sound":true,"X":120,"Y":340}`
	prefs, legacy := loadDevicePreferences([]byte(old))
	if !legacy {
		t.Fatal("the mascot's keys were not reported")
	}
	want := devicePreferences{IslandEnabled: true, IslandStyle: "A", IslandReduceMotion: true, Autostart: true, Audio: "bel.wav", Volume: .7, Sound: true}
	if prefs != want {
		t.Fatalf("prefs %+v, want %+v", prefs, want)
	}
	data, _ := json.Marshal(prefs)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	for _, k := range legacyDeviceKeys {
		if _, ok := raw[k]; ok {
			t.Errorf("saved file still has %q: %s", k, data)
		}
	}
	if _, legacy := loadDevicePreferences(data); legacy {
		t.Error("a file written by 0.4.5 is reported as legacy")
	}
}

// No file, or a broken one, gives the defaults: the island on, always
// shown, glass, replacing Windows' toasts; the bell on at 45%.
func TestLoadDevicePreferencesDefaults(t *testing.T) {
	want := devicePreferences{IslandEnabled: true, IslandStyle: "C", IslandReplaceToast: true, IslandAlwaysOn: true, Volume: .45, Sound: true}
	for _, data := range []string{"", "{", `{"islandStyle":"Z","volume":4}`} {
		if prefs, legacy := loadDevicePreferences([]byte(data)); prefs != want || legacy {
			t.Errorf("%q: %+v legacy %v", data, prefs, legacy)
		}
	}
}
