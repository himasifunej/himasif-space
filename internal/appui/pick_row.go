package appui

import "github.com/egoist/mygo/ui"

// pickHover remembers which row of a pick list (the @ lists) the mouse was
// over, so only moving onto another row moves the highlight: a mouse left
// resting on a row does not take it back from ↑/↓.
type pickHover struct{ last, now int }

// begin starts a frame of the list's rows.
func (h *pickHover) begin() { h.last, h.now = h.now, -1 }

// pickRow styles row i of a pick list: a pointer, and one highlight, in the
// accent's tint, on the active row. Pointing at a row makes it the active
// one, so the mouse and ↑/↓ always show the same single row.
func pickRow(c *ui.Context, row ui.Element, i int, active *int, hover *pickHover) {
	row.Cursor(ui.CursorPointer)
	if row.Hovered() {
		hover.now = i
		if hover.last != i {
			*active = i
		}
	}
	if i == *active {
		row.Background(accentSoft(c.Theme()))
	}
	row.FocusRing(row.FocusVisible())
}
