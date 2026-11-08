package workspaceui

import (
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

var (
	checkIcon     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12.5 4.5 4.5L19 7.5"/></svg>`))
	toTopIcon     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M6 4h12M12 20V9M7.5 13.5 12 9l4.5 4.5"/></svg>`))
	toBottomIcon  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M6 20h12M12 4v11M7.5 10.5 12 15l4.5-4.5"/></svg>`))
	editIcon      = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L19 9a2.8 2.8 0 0 0-4-4L4 16v4zM13.5 6.5l4 4"/></svg>`))
	duplicateIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/></svg>`))
	cancelIcon    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><circle cx="12" cy="12" r="8.5"/><path d="m9 9 6 6M15 9l-6 6"/></svg>`))
)

// laneColor is a lane's dot: the caller's color, else the theme's.
func (s *State) laneColor(t *ui.Theme, lane Lane) ui.Color {
	if s.LaneColors[lane].A != 0 {
		return s.LaneColors[lane]
	}
	switch lane {
	case Doing:
		return t.Accent
	case Review:
		return t.Warning
	case Done:
		return t.Success
	}
	return t.TextMuted
}

// Card menu geometry, in DIPs: its width, the height it wants, and the
// room kept from the anchor and the window's edges.
const (
	cardMenuWidth  = 236
	cardMenuGap    = 6
	cardMenuMargin = 8
)

// cardMenuHeight is the height the menu's content wants: its group
// titles, rows and separators.
func (s *State) cardMenuHeight() float32 {
	rows, groups := 6, 2 // four lanes, two orders
	if s.OnAction != nil {
		rows, groups = rows+3, groups+1
	}
	return float32(rows*34+groups*26+(groups-1)*9) + 2*6
}

// cardMenuPlace chooses where the card menu opens beside anchor, a rect in
// a window vw×vh: below it where the menu fits, else above, else beside
// it, else on the roomier of below and above, scrolling. It returns the
// anchor points and the most height the menu may take there.
func cardMenuPlace(anchor ui.Rect, vw, vh, want float32) (at, self ui.Anchor, maxH float32) {
	below := vh - (anchor.Y + anchor.H) - cardMenuGap - cardMenuMargin
	above := anchor.Y - cardMenuGap - cardMenuMargin
	switch {
	case below >= want:
		return ui.AnchorBottomRight, ui.AnchorTopRight, below
	case above >= want:
		return ui.AnchorTopRight, ui.AnchorBottomRight, above
	}
	full := vh - 2*cardMenuMargin
	if anchor.X-cardMenuGap-cardMenuMargin >= cardMenuWidth {
		return ui.AnchorTopLeft, ui.AnchorTopRight, full
	}
	if vw-(anchor.X+anchor.W)-cardMenuGap-cardMenuMargin >= cardMenuWidth {
		return ui.AnchorTopRight, ui.AnchorTopLeft, full
	}
	if above > below {
		return ui.AnchorTopRight, ui.AnchorBottomRight, max(above, 120)
	}
	return ui.AnchorBottomRight, ui.AnchorTopRight, max(below, 120)
}

// moveMenu is a card's "…" menu: where to move it (the lanes, with their
// colors and the current one checked), its order in the lane, and the
// task's actions, "Batalkan tugas" last and in red, which the caller
// confirms. On a desktop it opens where it fits and scrolls when nothing
// fits; on a phone it is a bottom sheet. Keys: 1–4 move, E edits, D
// duplicates.
func (s *State) moveMenu(c *ui.Context, anchor ui.Element, task Task) {
	t := c.Theme()
	vw, vh := c.Size()
	if vw < 720 {
		ui.DialogBase(c, &s.menuOpen, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(0, 0, 0, 0.45)).Justify(ui.End).AlignItems(ui.Stretch)
			panel.FillWidth().Radius(16, 16, 0, 0).Background(t.Surface).Border(1, t.Border).Padding(8, 8, 16, 8).Label("Menu " + task.Title).FocusGroup(ui.Vertical)
			ui.Box(c.Key("grip")).Size(36, 4).Radius(2).Background(t.Border).AlignSelf(ui.Center).Margin(4, 0, 8, 0)
			ui.Text(c, task.Title).FontSize(13).Bold().MaxLines(1).Padding(0, 10, 6, 10)
			ui.Scroll(c).FillWidth().MaxHeight(vh * 0.72).Children(func() {
				// A phone has no keyboard to speak of: no shortcuts here.
				ui.Column(c).FillWidth().Gap(2).Children(func() { s.cardMenuContent(c, task, true) })
			})
		})
		return
	}
	at, self, maxH := cardMenuPlace(anchor.Bounds(), vw, vh, s.cardMenuHeight())
	ui.PopoverBase(c, anchor, &s.menuOpen, func(panel ui.Element) {
		panel.AttachTo(anchor, at, self).Width(cardMenuWidth).Radius(10).Background(t.Surface).Border(1, t.Border).FocusGroup(ui.Vertical)
		switch {
		case self == ui.AnchorTopRight && at == ui.AnchorTopLeft:
			panel.Margin(0, cardMenuGap, 0, 0)
		case self == ui.AnchorTopLeft:
			panel.Margin(0, 0, 0, cardMenuGap)
		case self == ui.AnchorBottomRight:
			panel.Margin(0, 0, cardMenuGap, 0)
		default:
			panel.Margin(cardMenuGap, 0, 0, 0)
		}
		s.cardMenuShortcuts(panel, task)
		ui.Scroll(c).FillWidth().MaxHeight(maxH).Children(func() {
			ui.Column(c).FillWidth().Gap(2).Padding(6).Children(func() { s.cardMenuContent(c, task, false) })
		})
	})
}

func (s *State) cardMenuShortcuts(panel ui.Element, task Task) {
	for lane, key := range []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4} {
		if panel.OverlayShortcut(0, key) {
			s.cardMenuMove(task, Lane(lane))
		}
	}
	if s.OnAction == nil {
		return
	}
	for key, action := range map[ui.Key]string{ui.KeyE: "edit", ui.KeyD: "duplicate"} {
		if panel.OverlayShortcut(0, key) {
			s.menuOpen = false
			s.OnAction(task.ID, action)
		}
	}
}

func (s *State) cardMenuMove(task Task, lane Lane) {
	s.menuOpen = false
	if lane != task.Lane {
		s.requestMove(task.ID, lane, id.Nil)
	}
}

// cardMenuContent builds the menu's rows; a phone's are taller for a
// finger and show no key hints.
func (s *State) cardMenuContent(c *ui.Context, task Task, phone bool) {
	t := c.Theme()
	group := func(title string) {
		ui.Text(c.Key("g-"+title), title).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(8, 10, 4, 10)
	}
	separator := func(key string) {
		ui.Box(c.Key(key)).Height(1).FillWidth().Background(t.Border).Margin(4, 0)
	}
	rowHeight := float32(34)
	if phone {
		rowHeight = 44
	}
	row := func(key, label, text, hint string, danger bool, lead func(), trail func()) bool {
		if phone {
			hint = ""
		}
		b := ui.ButtonBase(c.Key(key)).Label(label).FillWidth().Padding(8, 10).Radius(6).MinHeight(rowHeight)
		b.FocusRing(b.FocusVisible())
		if b.Hovered() || b.FocusVisible() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Box(c).Size(16, 16).Center().Shrink(0).Children(lead)
				text := ui.Text(c, text).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
				if danger {
					text.TextColor(t.Danger)
				}
				if trail != nil {
					trail()
				}
				if hint != "" {
					ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).Padding(1, 6).Radius(4).Border(1, t.Border)
				}
			})
		})
		return b.Clicked()
	}
	icon := func(svg *ui.SVG, ink ui.Color) func() {
		return func() { ui.Icon(c, svg).Size(16, 16).TextColor(ink) }
	}

	group("Pindahkan ke")
	for lane := Todo; lane <= Done; lane++ {
		color := s.laneColor(t, lane)
		current := lane == task.Lane
		trail := func() {}
		if current {
			trail = func() { ui.Icon(c, checkIcon).Size(14, 14).TextColor(t.Accent) }
		}
		// The group title says "Pindahkan ke"; the label, which a screen
		// reader reads alone, says it again.
		label := "Ke " + lane.String()
		if current {
			label = lane.String() + " (sekarang)"
		}
		if row(lane.String(), label, lane.String(), string(rune('1'+lane)), false, func() { ui.Box(c).Size(8, 8).Radius(4).Background(color) }, trail) {
			s.cardMenuMove(task, lane)
		}
	}
	separator("sep-order")
	group("Urutan")
	if row("first", "Ke urutan pertama", "Ke urutan pertama", "", false, icon(toTopIcon, t.TextMuted), nil) {
		s.requestMove(task.ID, task.Lane, s.firstTask(task.Lane))
		s.menuOpen = false
	}
	if row("last", "Ke urutan terakhir", "Ke urutan terakhir", "", false, icon(toBottomIcon, t.TextMuted), nil) {
		s.requestMove(task.ID, task.Lane, id.Nil)
		s.menuOpen = false
	}
	if s.OnAction == nil {
		return
	}
	separator("sep-task")
	group("Tugas")
	for _, a := range []struct {
		key, label, hint string
		svg              *ui.SVG
	}{{"edit", "Ubah tugas", "E", editIcon}, {"duplicate", "Duplikat", "D", duplicateIcon}} {
		if row(a.key, a.label, a.label, a.hint, false, icon(a.svg, t.TextMuted), nil) {
			s.menuOpen = false
			s.OnAction(task.ID, a.key)
		}
	}
	separator("sep-cancel")
	// Cancelling asks for a reason in the caller's dialog: no silent loss.
	if row("cancel", "Batalkan tugas", "Batalkan tugas", "", true, icon(cancelIcon, t.Danger), nil) {
		s.menuOpen = false
		s.OnAction(task.ID, "cancel")
	}
}
