package workspaceui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

var closeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="m6 6 12 12M18 6 6 18"/></svg>`))
var moreIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.7"/><circle cx="12" cy="12" r="1.7"/><circle cx="19" cy="12" r="1.7"/></svg>`))

// View builds a native board and, when selected, an in-layout task inspector.
func (s *State) View(c *ui.Context) ui.Element {
	t := c.Theme()
	dragging := false
	root := ui.Row(c).Fill().MinWidth(0).Gap(0)
	root.Children(func() {
		ui.Column(c.Key("board-area")).Grow(1).MinWidth(0).FillHeight().Gap(16).Padding(22).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, "Board tugas").FontSize(22).Bold()
					ui.Text(c, "Semua pekerjaan, satu alur bersama.").FontSize(12).TextColor(t.TextMuted)
				})
				ui.Text(c, fmt.Sprintf("%d tugas", len(s.tasks))).FontSize(12).TextColor(t.TextMuted)
			})
			if len(s.tasks) == 0 {
				ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(10).Label("Board kosong").Children(func() {
					ui.Text(c, "Belum ada tugas").FontSize(19).Bold()
					ui.Text(c, "Tugas yang dibagikan ke space ini akan tampil di sini.").FontSize(13).TextColor(t.TextMuted)
				})
				return
			}
			scroll := ui.ScrollHorizontal(c.Key("board-scroll")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.boardScroll).Label("Kolom board")
			scroll.Children(func() {
				ui.Row(c).FillHeight().Gap(14).Padding(0, 0, 10, 0).Children(func() {
					for lane := Todo; lane <= Cancelled; lane++ {
						if lane == Cancelled && !s.hasLane(Cancelled) {
							continue
						}
						s.lane(c.Key(lane), lane, &dragging)
					}
				})
			})
			// MyGo's built-in drag autoscroll currently handles vertical axes.
			// Drive the tracked horizontal viewport while a card approaches its
			// clipped edge, scheduling only while the drag can move further.
			if dragging {
				x, _, over := scroll.PointerPosition()
				width := scroll.Bounds().W
				if over && width > 0 {
					delta := float32(0)
					if x < 46 {
						delta = -14
					} else if x > width-46 {
						delta = 14
					}
					next := min(max(s.boardScroll.X+delta, 0), s.boardScroll.MaxX)
					if next != s.boardScroll.X {
						s.boardScroll.X = next
						c.After(16 * time.Millisecond)
					}
				}
			}
		})
		if task, ok := s.task(s.selected); ok {
			s.inspector(c.Key("task-inspector"), task)
		}
	})
	return root
}

func (s *State) lane(c *ui.Context, lane Lane, dragging *bool) {
	t := c.Theme()
	count := 0
	for _, task := range s.tasks {
		if task.Lane == lane {
			count++
		}
	}
	column := ui.Column(c).Width(268).Shrink(0).FillHeight().MinHeight(0).Gap(12).Label("Kolom " + lane.String())
	column.Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2).Children(func() {
			color := t.TextMuted
			switch lane {
			case Doing:
				color = t.Accent
			case Review:
				color = t.Warning
			case Done:
				color = t.Success
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(color)
			ui.Text(c, lane.String()).FontSize(13).Bold().Grow(1)
			ui.Text(c, fmt.Sprint(count)).FontSize(11).TextColor(t.TextMuted).Padding(3, 7).Background(t.Surface).Radius(5)
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.lanes[lane]).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Padding(1).Children(func() {
				for _, task := range s.tasks {
					if task.Lane == lane {
						s.card(c.Key(task.ID), task, dragging)
					}
				}
				end := ui.Column(c.Key("lane-end")).FillWidth().MinHeight(82).Padding(18).Radius(10).Label("Akhir " + lane.String())
				end.Children(func() {
					if count == 0 {
						ui.Text(c, "Belum ada tugas").FontSize(12).TextColor(t.TextMuted)
					}
				})
				if s.OnMove != nil {
					if _, over := ui.DragOver[taskDrag](end); over {
						end.Border(1, t.Accent)
					}
					s.drop(end, lane, id.Nil, t.Accent)
				}
			})
		})
	})
	if s.OnMove != nil {
		s.drop(column, lane, id.Nil, t.Accent)
	}
}

func (s *State) card(c *ui.Context, task Task, dragging *bool) {
	t := c.Theme()
	categoryInk := t.Accent
	if !t.Dark {
		categoryInk = t.TextMuted
	}
	card := ui.ButtonBase(c).Label("Tugas "+task.Title).FillWidth().MinWidth(0).Padding(15).Radius(11).Background(t.Surface).Border(1, t.Border)
	card.FocusRing(card.FocusVisible())
	if card.Hovered() {
		card.Background(t.SurfaceHover)
	}
	if s.selected == task.ID {
		card.BorderColor(t.Accent)
	}
	if s.OnMove != nil && !task.ReadOnly {
		card.Drag(taskDrag{task.ID})
	}
	if card.Dragging() {
		*dragging = true
		card.Background(t.SurfacePressed)
	}
	card.Children(func() {
		ui.Column(c).FillWidth().Gap(11).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				if task.Category != "" {
					ui.Text(c, task.Category).FontSize(10).TextColor(categoryInk).Padding(3, 7).Radius(5).Background(t.Background)
				}
				ui.Box(c).Grow(1)
				if s.OnMove != nil && !task.ReadOnly {
					more := ui.ButtonBase(c.Key("move-menu")).Label("Pindahkan "+task.Title).Size(28, 24).Radius(5)
					more.FocusRing(more.FocusVisible())
					if more.Hovered() {
						more.Background(t.SurfaceHover)
					}
					more.Children(func() { ui.Icon(c, moreIcon).Size(18, 18).TextColor(t.TextMuted) })
					if more.Clicked() {
						s.menuOpen = s.menuID != task.ID || !s.menuOpen
						s.menuID = task.ID
					}
					if s.menuID == task.ID {
						s.moveMenu(c, more, task)
					}
				}
			})
			ui.Text(c, task.Title).Bold().FontSize(14).MaxLines(3)
			if task.Description != "" {
				ui.Text(c, task.Description).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			}
			if task.Space != "" {
				ui.Text(c, task.Space).FontSize(10).TextColor(t.TextMuted)
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				deadline := task.Deadline
				if deadline == "" {
					deadline = "Tanpa tenggat"
				}
				ui.Text(c, deadline).FontSize(11).TextColor(t.TextMuted).Grow(1)
				if task.Assignee != "" {
					ui.Text(c, initials(task.Assignee)).FontSize(10).Bold().Padding(6).Radius(14).Background(t.Background).Tooltip(task.Assignee)
				}
			})
		})
	})
	if card.Clicked() {
		s.Open(task.ID)
	}
	if s.OnMove != nil {
		before := task.ID
		_, y, _ := card.PointerPosition()
		if y > card.Bounds().H/2 {
			before = s.nextTask(task)
		}
		s.drop(card, task.Lane, before, t.Accent)
	}
}

func (s *State) nextTask(task Task) id.ID {
	found := false
	for _, other := range s.tasks {
		if other.Lane != task.Lane {
			continue
		}
		if found {
			return other.ID
		}
		found = other.ID == task.ID
	}
	return id.Nil
}

func (s *State) drop(target ui.Element, lane Lane, before id.ID, accent ui.Color) {
	if value, over := ui.DragOver[taskDrag](target); over && value.ID != before {
		target.BorderColor(accent)
	}
	if value, ok := ui.Drop[taskDrag](target); ok {
		s.requestMove(value.ID, lane, before)
	}
}

func (s *State) moveMenu(c *ui.Context, anchor ui.Element, task Task) {
	t := c.Theme()
	ui.PopoverBase(c, anchor, &s.menuOpen, func(panel ui.Element) {
		panel.Width(230).Padding(8).Gap(3).Radius(10).Margin(6, 0, 0, 0).Background(t.Surface).Border(1, t.Border).FocusGroup(ui.Vertical)
		ui.Text(c, "Pindahkan ke").FontSize(11).TextColor(t.TextMuted).Padding(8)
		for lane := Todo; lane <= Done; lane++ {
			button := ui.ButtonBase(c.Key(lane)).Label("Ke " + lane.String()).FillWidth().Padding(10).Radius(6)
			if lane == Todo {
				button.AutoFocus()
			}
			button.Children(func() { ui.Text(c, lane.String()).FontSize(12) })
			if button.Hovered() {
				button.Background(t.SurfaceHover)
			}
			button.FocusRing(button.FocusVisible())
			if button.Clicked() {
				s.requestMove(task.ID, lane, id.Nil)
				s.menuOpen = false
			}
		}
		ui.Box(c).Height(1).FillWidth().Background(t.Border).Margin(4, 0)
		for _, move := range []struct {
			label  string
			before id.ID
		}{
			{"Ke urutan pertama", s.firstTask(task.Lane)},
			{"Ke urutan terakhir", id.Nil},
		} {
			button := ui.ButtonBase(c.Key(move.label)).Label(move.label).FillWidth().Padding(10).Radius(6)
			button.Children(func() { ui.Text(c, move.label).FontSize(12) })
			if button.Hovered() {
				button.Background(t.SurfaceHover)
			}
			if button.Clicked() {
				s.requestMove(task.ID, task.Lane, move.before)
				s.menuOpen = false
			}
		}
	})
}

func (s *State) firstTask(lane Lane) id.ID {
	for _, task := range s.tasks {
		if task.Lane == lane {
			return task.ID
		}
	}
	return id.Nil
}

func (s *State) inspector(c *ui.Context, task Task) {
	t := c.Theme()
	categoryInk := t.Accent
	if !t.Dark {
		categoryInk = t.TextMuted
	}
	ui.Column(c).Width(340).Shrink(0).FillHeight().MinWidth(0).Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Label("Detail tugas").Children(func() {
		ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Detail tugas").FontSize(12).TextColor(t.TextMuted).Grow(1)
			close := ui.ButtonBase(c.Key("close-inspector")).Label("Tutup detail tugas").Size(28, 28).Radius(6)
			close.FocusRing(close.FocusVisible())
			if close.Hovered() {
				close.Background(t.SurfaceHover)
			}
			close.Children(func() { ui.Icon(c, closeIcon).Size(17, 17).TextColor(t.TextMuted) })
			if close.Clicked() {
				s.Close()
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.inspectorScroll).Children(func() {
			ui.Column(c).FillWidth().Padding(22).Gap(22).Children(func() {
				if s.Details != nil {
					s.Details(c, task.ID)
					return
				}
				if task.Category != "" {
					ui.Text(c, task.Category).FontSize(11).TextColor(categoryInk)
				}
				ui.Text(c, task.Title).FontSize(23).Bold()
				if task.Description != "" {
					ui.Text(c, task.Description).FontSize(13).TextColor(t.TextMuted)
				}
				ui.Column(c).FillWidth().Gap(15).Children(func() {
					field(c, "Status", task.Lane.String())
					field(c, "Penanggung jawab", task.Assignee)
					field(c, "Tenggat tugas", task.Deadline)
					field(c, "Tanggal posting", task.Posting)
					field(c, "Space", task.Space)
				})
				ui.Box(c).FillWidth().Height(1).Background(t.Border)
				section(c, "Hasil yang diharapkan", task.ExpectedOutput)
				section(c, "Bukti pekerjaan", task.Evidence)
				if task.Lane == Review {
					ui.Text(c, "Tinjau kecocokan bukti dengan tugas sebelum menyatakan pekerjaan selesai.").FontSize(12).TextColor(t.TextMuted)
				}
			})
		})
	})
}

func field(c *ui.Context, label, value string) {
	if value == "" {
		value = "Belum diatur"
	}
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		ui.Text(c, label).Width(115).Shrink(0).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, value).Grow(1).MinWidth(0).FontSize(12)
	})
}
func section(c *ui.Context, label, value string) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, label).FontSize(12).Bold()
		if value == "" {
			value = "Belum ditambahkan"
		}
		ui.Text(c, value).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}
func initials(name string) string {
	words := strings.Fields(name)
	result := ""
	for _, word := range words {
		r := []rune(word)
		if len(r) > 0 {
			result += string(r[0])
		}
		if len([]rune(result)) == 2 {
			break
		}
	}
	return strings.ToUpper(result)
}

func (s *State) hasLane(l Lane) bool {
	for _, t := range s.tasks {
		if t.Lane == l {
			return true
		}
	}
	return false
}
