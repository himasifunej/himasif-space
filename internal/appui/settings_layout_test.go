package appui

import (
	"fmt"
	"testing"
	"unicode/utf8"
)

// The owner's rc.6 report: in Tampilan dan tema, "Skala tampilan" drew its
// label and description one letter per line, the label squeezed by the
// scale choices. No text on any settings page is drawn that narrow, at 100,
// 150 and 200 % and from a phone to a wide screen.
func TestSettingsTextsKeepTheirWidth(t *testing.T) {
	tabs := []string{"Akun", "Perangkat", "Tampilan", "Sinkronisasi", "Space", "WhatsApp", "Pengingat", "Command", "Aktivitas", "Status layanan", "Darurat", "Tentang"}
	type view struct {
		size  [2]int
		scale float32
	}
	var views []view
	for _, size := range [][2]int{{2560, 1440}, {1920, 1080}, {1600, 900}, {1440, 900}, {1400, 900}, {1100, 800}} {
		for _, scale := range []float32{1, 1.5, 2} {
			views = append(views, view{size, scale})
		}
	}
	// A narrow window, and a phone, whose browser zooms on its own.
	views = append(views, view{[2]int{800, 700}, 1}, view{[2]int{800, 700}, 1.25}, view{[2]int{420, 860}, 1})
	for _, v := range views {
		size, scale := v.size, v.scale
		for _, tab := range tabs {
			name := fmt.Sprintf("%dx%d@%v/%s", size[0], size[1], scale, tab)
			h := redesignHarness(t)
			h.tt.SetSize(size[0], size[1])
			// Skala tampilan zooms the whole window; the Tester's
			// coordinates stay the window's.
			if scale != 1 {
				h.app.chrome.preferences.DisplayScale = int(scale * 100)
			}
			h.app.chrome.settingsDetail = true
			h.openSettings(tab)
			h.tt.Frame()
			// The scale choices stay inside the window, wrapping when
			// the row is narrower than they are.
			if tab == "Tampilan" {
				for _, o := range []string{"Ikuti browser", "200%"} {
					if r, ok := h.tt.Find(o); ok && r.W > 0 && r.X+r.W > float32(size[0])-8 {
						t.Errorf("%s: %q spills out at %+v", name, o, r)
					}
				}
			}
			for _, s := range h.tt.Texts() {
				if utf8.RuneCountInString(s) < 10 {
					continue
				}
				r, ok := h.tt.Find(s)
				if !ok {
					continue
				}
				// A text squeezed to a letter a line is not found as
				// drawn; one scrolled away lies outside the window.
				inside := r.Y > 0 && r.Y < float32(size[1])-200 && r.X > 0 && r.X < float32(size[0])
				if inside && (r.W == 0 && r.H == 0 || r.W < 40 && r.H > 2*r.W) {
					t.Errorf("%s: %q squeezed to %+v", name, s, r)
				}
			}
		}
	}
}
