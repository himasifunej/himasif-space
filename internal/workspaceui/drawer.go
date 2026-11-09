package workspaceui

import (
	"github.com/egoist/mygo/ui"
	"time"
)

// Drawer shares the same edge resize gesture across task detail, forms and
// appearance. It participates in layout, leaving the board interactive.
func Drawer(c *ui.Context, width *float32, body func()) ui.Element {
	w, _ := c.Size()
	maxWidth := max(float32(280), min(float32(640), w-600))
	if *width == 0 {
		*width = 416
	}
	shown := min(max(*width, 280), maxWidth)
	// Phones show the drawer as a full-screen page without the resize edge.
	phone := w < 720
	if phone {
		shown = w
	}
	t := c.Theme()
	panel := ui.Column(c).Width(shown).Shrink(0).FillHeight().MinWidth(0).Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).
		Transition(ui.ElementTransition{Duration: 260 * time.Millisecond, Size: true, Position: true, Enter: &ui.Motion{Collapse: true, Opacity: 1}, Exit: &ui.Motion{X: 20, Opacity: 0}})
	panel.Children(func() {
		body()
		if phone {
			return
		}
		edge := ui.Box(c.Key("drawer-resize")).Label("Ubah lebar drawer").Tooltip("Tarik untuk mengubah lebar · panah kiri/kanan").Absolute().Left(0).Top(0).Width(6).FillHeight().Cursor(ui.CursorResizeEW).Focusable()
		dx, _, held := edge.Dragged()
		if held && dx != 0 {
			*width = min(max(shown-dx, 280), maxWidth)
		}
		if edge.Shortcut(0, ui.KeyLeft) {
			*width = min(shown+24, maxWidth)
		}
		if edge.Shortcut(0, ui.KeyRight) {
			*width = max(shown-24, 280)
		}
		if edge.Hovered() || held || edge.FocusVisible() {
			edge.Background(t.Accent)
		}
	})
	return panel
}
