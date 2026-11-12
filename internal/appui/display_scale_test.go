package appui

import (
	"fmt"
	"testing"
	"time"
)

// A step is the size against an unscaled screen, as in Windows: on a
// monitor Windows scales to 150%, 150% is as before and 100% smaller;
// Ikuti Windows never zooms. Without a known system scale (the web) a
// step zooms from the browser's size.
func TestDisplayZoom(t *testing.T) {
	for _, c := range []struct {
		scale  int
		system float64
		want   float32
	}{
		{0, 1.5, 1}, {150, 1.5, 1}, {100, 1.5, 1 / 1.5}, {200, 1.5, 2 / 1.5}, {125, 1, 1.25}, {200, 0, 2}, {175, 1.25, 1.4},
	} {
		if got := displayZoom(c.scale, c.system); fmt.Sprintf("%.3f", got) != fmt.Sprintf("%.3f", c.want) {
			t.Errorf("%d%% at %.2f: %.3f, want %.3f", c.scale, c.system, got, c.want)
		}
	}
	for in, want := range map[int]int{0: 0, 100: 100, 175: 175, 200: 200, 110: 0, 300: 0, -1: 0} {
		if got := NormalizePreferences(Preferences{DisplayScale: in}).DisplayScale; got != want {
			t.Errorf("normalize %d: %d", in, got)
		}
	}
}

// Picking a step in Tampilan zooms the whole window at once and is saved
// with the device's preferences; Ikuti Windows goes back. With
// SPACE_UI_CAPTURE_DIR, Untukku at 100%, 150% and 200% of a 1440×900
// window on an unscaled screen.
func TestDisplayScaleSetting(t *testing.T) {
	h := redesignHarness(t)
	store := &fakePreferences{value: DefaultPreferences()}
	h.app.PreferencesStore = store
	h.app.SystemScale = func() float64 { return 1.5 }
	h.tt.SetSize(1440, 2400) // the row in view, at every scale
	h.openSettings("Tampilan")
	h.want("Skala tampilan")
	h.want("Ikuti Windows")
	h.want("Windows saat ini 150%.")
	row := func() float32 {
		r, ok := h.tt.Find("Profil dan keamanan")
		if !ok {
			t.Fatalf("no settings nav; %q", h.tt.Texts())
		}
		return r.W
	}
	before := row()
	h.click("100%")
	h.tt.Frame()
	if h.app.chrome.preferences.DisplayScale != 100 || store.value.DisplayScale != 100 {
		t.Fatalf("not saved: %+v / %+v", h.app.chrome.preferences, store.value)
	}
	if after := row(); after > before*0.7 || after < before*0.6 {
		t.Fatalf("100%% on a 150%% screen: row %.0f, was %.0f", after, before)
	}
	h.click("Ikuti Windows")
	h.tt.Frame()
	if h.app.chrome.preferences.DisplayScale != 0 || row() != before {
		t.Fatalf("follow: %+v, row %.0f", h.app.chrome.preferences, row())
	}

	h.app.SystemScale = func() float64 { return 1 }
	h.tt.SetSize(1440, 900)
	for _, s := range []int{100, 150, 200} {
		h.app.chrome.preferences.DisplayScale = s
		h.app.navigate(PathUntukku)
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, fmt.Sprintf("skala-%d", s))
	}
}
