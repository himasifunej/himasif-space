package appui

import (
	"testing"
	"time"
)

// The logo stays at the left on the nav icons' column, open or collapsed
// (owner's rc.2 feedback: it moved toward the middle as the sidebar
// opened). SPACE_UI_CAPTURE_DIR keeps both states.
func TestSidebarBrandOnIconColumn(t *testing.T) {
	for _, collapsed := range []bool{false, true} {
		h := redesignHarness(t)
		h.app.chrome.preferences.SidebarCollapsed = collapsed
		h.app.navigate(PathUntukku)
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		brand, ok := h.tt.Find("Ke daftar Space")
		nav, ok2 := h.tt.Find("Untukku")
		if !ok || !ok2 {
			t.Fatalf("collapsed %v: brand %v nav %v", collapsed, ok, ok2)
		}
		if brand.X != nav.X || brand.W != nav.W {
			t.Fatalf("collapsed %v: brand %+v, nav row %+v", collapsed, brand, nav)
		}
		// The logo's center is the nav icon's.
		logo := brand.X + brandInset(collapsed) + brandLogo/2
		icon := nav.X + 10 + 8
		if collapsed {
			icon = nav.X + nav.W/2
		}
		if logo != icon {
			t.Fatalf("collapsed %v: logo center %v, icon center %v", collapsed, logo, icon)
		}
		name := "sidebar-brand-open"
		if collapsed {
			name = "sidebar-brand-collapsed"
		}
		captureNativeUI(t, h, name)
	}
}
