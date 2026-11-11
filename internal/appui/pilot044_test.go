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

// openMemberAccessMenu opens a member's access drawer as a user does now:
// the row's ⋯, then Ubah role & akses.
func openMemberAccessMenu(h *harness, name string) {
	h.t.Helper()
	h.click("Lainnya untuk " + name)
	h.tt.Frame()
	h.click("Ubah akses " + name)
}

// The Anggota rows keep Ping and ⋯ inside the table, with Ubah role & akses
// in the ⋯ menu instead of beside them.
func TestMemberRowActionsFitAndMenu(t *testing.T) {
	for _, w := range []int{1280, 1440} {
		h, _, _, _ := memberAccessHarness(t, w, 900)
		more, ok := h.tt.Find("Lainnya untuk Nara Tekno")
		if !ok {
			t.Fatal("no ⋯ on Nara's row")
		}
		head, ok := h.tt.Find("WhatsApp")
		if !ok {
			t.Fatal("no WhatsApp column")
		}
		// The last column starts 16 DIPs after WhatsApp (112 wide); the
		// row's padding (16) is left after it.
		if right := more.X + more.W; right > head.X+112+16+104+0.5 {
			t.Fatalf("width %d: ⋯ ends at %v, past its column", w, right)
		}
		h.dontWant("Ubah akses Nara Tekno")
		h.click("Lainnya untuk Nara Tekno")
		h.tt.Frame()
		for _, s := range []string{"Ubah role & akses", "Lihat tugas", "Keluarkan…"} {
			h.want(s)
		}
		h.click("Ubah akses Nara Tekno")
		h.tt.Frame()
		h.want("Ubah akses · Nara Tekno")
	}
}
