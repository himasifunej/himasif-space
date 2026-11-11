package appui

import (
	"testing"
	"time"
)

// Pilot 0.4.4: the tab row hides the engine's scroll bar, which showed
// over the tabs while the pointer was on them; a mouse wheel scrolls it
// sideways, and a thin bar shows only while it scrolls.
func TestSpaceTabRowScrollsWithoutStandingBar(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	h.tt.SetSize(700, 800)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	st := &h.app.spaceTabs
	if st.scroll.MaxX <= 0 {
		t.Fatalf("tabs fit in 700 DIPs: %+v", st.scroll)
	}
	r, ok := h.tt.Find("Board tugas")
	if !ok {
		t.Fatal("no Board tugas tab")
	}
	h.tt.Move(r.X+r.W/2, r.Y+r.H/2)
	h.tt.Advance(100 * time.Millisecond)
	if !st.scrolledAt.IsZero() {
		t.Fatal("bar shown without scrolling")
	}
	h.tt.Wheel(r.X+r.W/2, r.Y+r.H/2, 0, 120)
	h.tt.Advance(100 * time.Millisecond)
	if st.scroll.X <= 0 || st.scrolledAt.IsZero() {
		t.Fatalf("wheel did not scroll the tab row: %+v", st.scroll)
	}
	h.tt.Advance(2 * time.Second)
	if h.tt.Now().Sub(st.scrolledAt) < spaceTabShowBar {
		t.Fatal("bar still showing")
	}
}
