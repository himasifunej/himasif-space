package workspaceui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDrawerEdgeResizesAndKeepsBoardUsable(t *testing.T) {
	width := float32(416)
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			ui.Box(c).Label("Board area").Grow(1).MinWidth(0).FillHeight()
			if open {
				Drawer(c, &width, func() { ui.Text(c, "Inspector") })
			}
		})
	}, 1200, 800)
	open = true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	edge := rect(t, tt, "Ubah lebar drawer")
	if edge.W != 6 {
		t.Fatalf("resize handle is not a discreet edge: %+v", edge)
	}
	x, y := edge.X+3, edge.Y+100
	tt.Move(x, y)
	tt.Press(x, y)
	tt.Move(x-80, y)
	tt.Release(x-80, y)
	tt.Advance(300 * time.Millisecond)
	if width < 490 || width > 502 {
		t.Fatalf("edge drag did not resize: %f", width)
	}
	if board := rect(t, tt, "Board area"); board.W < 600 {
		t.Fatalf("drawer consumed board: %+v", board)
	}
	tt.SetSize(960, 700)
	tt.Advance(300 * time.Millisecond)
	if board := rect(t, tt, "Board area"); board.W < 600 {
		t.Fatalf("resized window consumed board: %+v", board)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	open = false
	tt.Frame()
	if _, ok := tt.Find("Ubah lebar drawer"); ok {
		t.Fatal("reduced-motion close retained drawer")
	}
	open = true
	tt.Frame()
	if _, ok := tt.Find("Ubah lebar drawer"); !ok {
		t.Fatal("reduced-motion open hid drawer")
	}
}
