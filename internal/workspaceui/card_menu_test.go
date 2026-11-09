package workspaceui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestCardMenuPlaceFitsTheWindow(t *testing.T) {
	const want = 400
	cases := []struct {
		name     string
		anchor   ui.Rect
		vw, vh   float32
		at, self ui.Anchor
	}{
		{"room below", ui.Rect{X: 600, Y: 100, W: 26, H: 22}, 1440, 900, ui.AnchorBottomRight, ui.AnchorTopRight},
		{"near the bottom", ui.Rect{X: 600, Y: 760, W: 26, H: 22}, 1440, 900, ui.AnchorTopRight, ui.AnchorBottomRight},
		{"short window, room left", ui.Rect{X: 600, Y: 200, W: 26, H: 22}, 1440, 480, ui.AnchorTopLeft, ui.AnchorTopRight},
		{"short window, room right", ui.Rect{X: 40, Y: 200, W: 26, H: 22}, 1440, 480, ui.AnchorTopRight, ui.AnchorTopLeft},
	}
	for _, tc := range cases {
		at, self, maxH := cardMenuPlace(tc.anchor, tc.vw, tc.vh, want)
		if at != tc.at || self != tc.self || maxH < 120 {
			t.Errorf("%s: at=%v self=%v maxH=%v", tc.name, at, self, maxH)
		}
	}
	// No side fits: the roomier of above and below, scrolling.
	at, _, maxH := cardMenuPlace(ui.Rect{X: 100, Y: 300, W: 26, H: 22}, 300, 480, want)
	if at != ui.AnchorTopRight || maxH >= want {
		t.Fatalf("no room anywhere: at=%v maxH=%v", at, maxH)
	}
}

func TestCardMenuGroupsCheckAndShortcuts(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	var moves []MoveRequest
	var actions []string
	s.OnMove = func(m MoveRequest) { moves = append(moves, m) }
	s.OnAction = func(_ id.ID, action string) { actions = append(actions, action) }
	tt := boardTester(t, s, 1300)
	if err := tt.Click("Pindahkan " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Pindahkan ke", "Urutan", "Tugas", "Belum mulai (sekarang)", "Ke Dikerjakan", "Ke urutan pertama", "Ubah tugas", "Duplikat", "Batalkan tugas"} {
		if _, ok := tt.Find(text); !ok && !tt.HasText(text) {
			t.Fatalf("menu lacks %q: %q", text, tt.Texts())
		}
	}
	// Choosing the current lane moves nothing.
	if err := tt.Click("Belum mulai (sekarang)"); err != nil {
		t.Fatal(err)
	}
	if len(moves) != 0 || s.menuOpen {
		t.Fatalf("current lane moved %+v open=%v", moves, s.menuOpen)
	}
	if err := tt.Click("Pindahkan " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.Key3)
	if len(moves) != 1 || moves[0].Destination != Review || s.menuOpen {
		t.Fatalf("key 3 = %+v open=%v", moves, s.menuOpen)
	}
	if err := tt.Click("Pindahkan " + tasks[1].Title); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyE)
	if len(actions) != 1 || actions[0] != "edit" {
		t.Fatalf("key E = %v", actions)
	}
}

// In a short window the menu stays inside it, scrolling, instead of
// running off the bottom.
func TestCardMenuStaysInShortWindow(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	s.OnMove = func(MoveRequest) {}
	s.OnAction = func(id.ID, string) {}
	const h = 380
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { s.View(c) })
	}, 1300, h)
	tt.Frame()
	if err := tt.Click("Pindahkan " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, ok := tt.Find("Ke Dikerjakan")
	if !ok || r.Y < 0 || r.Y+r.H > h {
		t.Fatalf("menu row outside the window: %+v %v", r, ok)
	}
}

func TestCardMenuIsBottomSheetOnPhone(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	s.OnMove = func(MoveRequest) {}
	s.OnAction = func(id.ID, string) {}
	tt := boardTester(t, s, 390)
	if err := tt.Click("Pindahkan " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	r, ok := tt.Find("Menu " + tasks[0].Title)
	if !ok || r.W < 380 || r.Y+r.H < 730 {
		t.Fatalf("phone menu is not a bottom sheet: %+v %v", r, ok)
	}
}

// SPACE_UI_CAPTURE_DIR opts into PNGs of the card menu: desktop, a short
// window and the phone's bottom sheet.
func TestCaptureCardMenu(t *testing.T) {
	dir := os.Getenv("SPACE_UI_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	for _, shot := range []struct {
		name string
		w, h int
		card int
	}{{"card-menu-desktop", 1440, 900, 0}, {"card-menu-short", 1300, 420, 2}, {"card-menu-phone", 390, 780, 0}} {
		s := New()
		tasks := fixtureTasks()
		if err := s.SetTasks(tasks); err != nil {
			t.Fatal(err)
		}
		s.OnMove = func(MoveRequest) {}
		s.OnAction = func(id.ID, string) {}
		tt := ui.NewTester(func(c *ui.Context) {
			theme := *ui.DarkTheme()
			theme.Background = ui.RGB(22, 21, 17)
			theme.Surface = ui.RGB(34, 33, 28)
			theme.Accent = ui.RGB(212, 198, 162)
			c.SetTheme(&theme)
			ui.Column(c).Fill().Background(theme.Background).TextColor(theme.Text).Children(func() { s.View(c) })
		}, shot.w, shot.h)
		tt.Frame()
		if err := tt.Click("Pindahkan " + tasks[shot.card].Title); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, shot.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
