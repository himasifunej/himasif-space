package workspaceui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

var closeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="m6 6 12 12M18 6 6 18"/></svg>`))
var plusIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`))
var clipIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m21 11-8.5 8.5a5 5 0 0 1-7-7L14 4a3.5 3.5 0 0 1 5 5l-8.5 8.5a2 2 0 0 1-3-3L15 7"/></svg>`))
var checkboxIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8.5 12 2.5 2.5 4.5-5"/></svg>`))
var moreIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.7"/><circle cx="12" cy="12" r="1.7"/><circle cx="19" cy="12" r="1.7"/></svg>`))

// View builds a native board and, when selected, an in-layout task inspector.
func (s *State) View(c *ui.Context) ui.Element {
	return s.view(c, true)
}

// ViewBoard leaves the selected inspector to the surrounding app shell.
func (s *State) ViewBoard(c *ui.Context) ui.Element {
	return s.view(c, false)
}

// Inspector builds the selected task detail when the surrounding shell has room.
func (s *State) Inspector(c *ui.Context) {
	if task, ok := s.task(s.selected); ok {
		s.inspector(c.Key("task-inspector"), task)
	}
}

func (s *State) view(c *ui.Context, inlineInspector bool) ui.Element {
	t := c.Theme()
	dragging := false
	if s.OnBulkCancel == nil {
		s.SetSelecting(false)
	}
	// Esc leaves select mode, unless it closes the card menu first.
	if s.selecting && !s.menuOpen && c.Shortcut(0, ui.KeyEscape) {
		s.SetSelecting(false)
	}
	vw, _ := c.Size()
	phone := vw < 720
	pad, titleSize := float32(22), float32(22)
	if phone {
		pad, titleSize = 12, 18
	}
	root := ui.Row(c).Grow(1).FillHeight().MinWidth(0).Gap(0)
	root.Children(func() {
		ui.Column(c.Key("board-area")).Transition(ui.ElementTransition{Duration: 260 * time.Millisecond, Size: true, Position: true}).Grow(1).MinWidth(0).FillHeight().Gap(16).Padding(pad).Children(func() {
			head := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12)
			// A narrow board (phone, or a drawer beside it) stacks the
			// toolbar under the title instead of squeezing the title.
			if s.Toolbar != nil && (phone || s.boardWidth > 0 && s.boardWidth < 820) {
				head.Column().AlignItems(ui.Start)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(min(float32(220), vw/2)).Gap(4).Children(func() {
					ui.Text(c, "Board tugas").FontSize(titleSize).Bold()
					subtitle := s.Subtitle
					if subtitle == "" {
						subtitle = "Semua pekerjaan, satu alur bersama."
					}
					ui.Text(c, subtitle).FontSize(12).TextColor(t.TextMuted)
				})
				if s.Toolbar != nil {
					s.Toolbar(c)
				} else {
					ui.Text(c, fmt.Sprintf("%d tugas", len(s.tasks))).FontSize(12).TextColor(t.TextMuted)
				}
				s.selectToggle(c)
			})
			s.selectionBar(c)

			scroll := ui.ScrollHorizontal(c.Key("board-scroll")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.boardScroll).Label("Kolom board")
			count := 4
			if s.hasLane(Cancelled) {
				count++
			}
			// Bounds are from the preceding frame. Redraw after resizing so the
			// lanes use the width left by a newly opened or resized inspector.
			width := scroll.Bounds().W
			if width != s.boardWidth {
				s.boardWidth = width
				c.After(0)
			}
			laneWidth := float32(268)
			if width > 0 {
				laneWidth = max(160, min(268, (width-float32(count-1)*14)/float32(count)))
			}
			if phone {
				// One lane per screen; swipe sideways to the next lane. The
				// window width is known this frame, so the lanes don't jump.
				laneWidth = max(160, vw-2*pad-28)
			}
			scroll.Children(func() {
				ui.Row(c).FillHeight().Gap(14).Padding(0, 0, 10, 0).Children(func() {
					for lane := Todo; lane <= Cancelled; lane++ {
						if lane == Cancelled && !s.hasLane(Cancelled) {
							continue
						}
						s.lane(c.Key(lane), lane, laneWidth, &dragging)
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
		if inlineInspector {
			s.Inspector(c)
		}
	})
	return root
}

func (s *State) lane(c *ui.Context, lane Lane, width float32, dragging *bool) {
	t := c.Theme()
	count := 0
	for _, task := range s.tasks {
		if task.Lane == lane {
			count++
		}
	}
	column := ui.Column(c).Width(width).Shrink(0).FillHeight().MinHeight(0).Gap(10).Padding(10).Radius(12).Background(t.Surface).Border(1, t.Border).Label("Kolom " + lane.String())
	column.Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			color := s.laneColor(t, lane)
			ui.Box(c).Size(7, 7).Radius(4).Background(color)
			ui.Text(c, lane.String()).FontSize(13).Bold().Grow(1)
			ui.Text(c, fmt.Sprint(count)).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().TrackScroll(&s.lanes[lane]).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Padding(1).Children(func() {
				for _, task := range s.tasks {
					if task.Lane == lane {
						s.card(c.Key(task.ID), task, dragging)
					}
				}
				// The slot shows where a dragged card lands, as on the canvas.
				end := ui.Column(c.Key("lane-end")).FillWidth().MinHeight(96).Padding(12).Radius(10).Label("Akhir " + lane.String())
				if s.OnMove != nil || count == 0 {
					end.Border(1.5, t.Border).BorderStyle(ui.BorderDashed).Justify(ui.Center).AlignItems(ui.Center).Gap(2)
				}
				end.Children(func() {
					switch {
					case count == 0:
						ui.Text(c, "Belum ada tugas").FontSize(12).TextColor(t.TextMuted)
						if s.OnMove != nil {
							ui.Text(c, "Tarik kartu ke kolom ini").FontSize(12).TextColor(t.TextMuted)
						}
					case s.OnMove != nil:
						ui.Text(c, "Lepas kartu di sini").FontSize(12).TextColor(t.TextMuted)
					}
				})
				if s.OnMove != nil {
					if _, over := ui.DragOver[taskDrag](end); over {
						end.BorderColor(t.Accent)
					}
					s.drop(end, lane, id.Nil, t.Accent)
				}
				if s.OnAdd != nil && lane <= Done {
					add := ui.ButtonBase(c.Key("lane-add")).Label("Tambah tugas di "+lane.String()).FillWidth().Row().AlignItems(ui.Center).Gap(8).Padding(8, 10).Radius(8).TextColor(t.TextMuted)
					if add.Hovered() {
						add.Background(t.SurfaceHover).TextColor(t.Text)
					}
					add.FocusRing(add.FocusVisible()).Children(func() {
						ui.Icon(c, plusIcon).Size(15, 15)
						ui.Text(c, "Tambah tugas").FontSize(13)
					})
					if add.Clicked() {
						s.OnAdd(lane)
					}
				}
			})
		})
	})
	if s.OnMove != nil {
		s.drop(column, lane, id.Nil, t.Accent)
	}
}

// chipInk is dark text on the light tag colors, as on the canvas.
var chipInk = ui.Hex("#24221c")

func (s *State) card(c *ui.Context, task Task, dragging *bool) {
	t := c.Theme()
	selecting := s.selecting
	picked := selecting && s.picked[task.ID]
	label := "Tugas " + task.Title
	if selecting {
		label = "Pilih tugas " + task.Title
		if !task.Selectable {
			label = task.Title + ", tidak dapat kamu batalkan"
		}
	}
	card := ui.ButtonBase(c).Label(label).FillWidth().MinWidth(0).Padding(12, 12, 12, 14).Radius(10).Background(t.SurfacePressed).Border(1, t.Border)
	card.FocusRing(card.FocusVisible())
	if card.Hovered() && (!selecting || task.Selectable) {
		card.Background(t.SurfaceHover)
	}
	if s.selected == task.ID && !selecting || picked {
		card.BorderColor(t.Accent)
	}
	if selecting {
		card.Checked(picked)
		if !task.Selectable {
			// Shown, so the board keeps its shape, but not pickable.
			card.Opacity(0.45).Tooltip("Tidak dapat kamu batalkan")
		}
	}
	if s.OnMove != nil && !task.ReadOnly && !selecting {
		card.Drag(taskDrag{task.ID})
	}
	if card.Dragging() {
		*dragging = true
		card.Background(t.Surface)
	}
	labels := task.Labels
	if len(labels) == 0 && task.Category != "" {
		labels = []Label{{Name: task.Category, Color: task.Color}}
	}
	card.Children(func() {
		if task.Color.A != 0 {
			// Only the left edge carries the color; the rest keeps the
			// card's own border.
			ui.Box(c.Key("accent")).Absolute().Left(0).Top(0).Bottom(0).Width(3).Radius(10, 0, 0, 10).Background(task.Color)
		}
		ui.Column(c).FillWidth().MinWidth(0).Gap(8).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(4).Children(func() {
				if selecting && task.Selectable {
					box := ui.Box(c.Key("pick")).Size(18, 18).Radius(5).Border(1.5, t.TextMuted).Center().Shrink(0).Margin(0, 4, 0, 0)
					if picked {
						box.Background(t.Accent).BorderColor(t.Accent).Children(func() {
							ui.Icon(c, checkIcon).Size(13, 13).TextColor(t.AccentText)
						})
					}
				}
				ui.Row(c).Grow(1).MinWidth(0).Wrap().Gap(4).Children(func() {
					for i, l := range labels {
						chip := ui.Text(c.Key(i), l.Name).FontSize(10).Bold().Padding(2, 7).Radius(5).MaxLines(1)
						if l.Color.A != 0 {
							chip.Background(l.Color).TextColor(chipInk)
						} else {
							chip.Background(t.Background).TextColor(t.TextMuted)
						}
					}
				})
				if s.OnMove != nil && !task.ReadOnly && !selecting {
					more := ui.ButtonBase(c.Key("move-menu")).Label("Pindahkan "+task.Title).Size(26, 22).Radius(6).Center()
					more.FocusRing(more.FocusVisible())
					if more.Hovered() {
						more.Background(t.SurfaceHover)
					}
					more.Children(func() { ui.Icon(c, moreIcon).Size(16, 16).TextColor(t.TextMuted) })
					if more.Clicked() {
						s.menuOpen = s.menuID != task.ID || !s.menuOpen
						s.menuID = task.ID
					}
					if s.menuID == task.ID {
						s.moveMenu(c, more, task)
					}
				}
			})
			ui.Text(c, task.Title).FontWeight(600).FontSize(14).MaxLines(3)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				if task.Assignee != "" {
					avatar := ui.Box(c).Size(20, 20).Radius(10).Center().Shrink(0).Tooltip(task.Assignee)
					if task.AssigneeColor.A != 0 {
						avatar.Background(task.AssigneeColor)
					} else {
						avatar.Background(t.Background)
					}
					avatar.Children(func() {
						ink := chipInk
						if task.AssigneeColor.A == 0 {
							ink = t.Text
						}
						ui.Text(c, initials(task.Assignee)).FontSize(9).Bold().TextColor(ink)
					})
					ui.Text(c, firstName(task.Assignee)).FontSize(12).TextColor(t.TextMuted).MaxLines(1).MinWidth(0)
				}
				ui.Box(c).Grow(1)
				due := task.Due
				if due == "" {
					due = task.Deadline
				}
				if due == "" {
					due = "Tanpa tenggat"
				}
				if task.Late {
					due += " · lewat"
				}
				d := ui.Text(c, due).FontSize(12).MaxLines(1).Shrink(0)
				if task.Late {
					d.TextColor(t.Danger)
				} else {
					d.TextColor(t.TextMuted)
				}
			})
			if task.Ref != "" || task.Attachments > 0 || task.ChecklistTotal > 0 {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
					if task.Ref != "" {
						ui.Text(c, task.Ref).FontSize(11).TextColor(t.TextMuted)
					}
					if task.Attachments > 0 {
						ui.Row(c.Key("attach")).Gap(3).AlignItems(ui.Center).Tooltip("Bukti dan lampiran").Children(func() {
							ui.Icon(c, clipIcon).Size(12, 12).TextColor(t.TextMuted)
							ui.Text(c, fmt.Sprint(task.Attachments)).FontSize(11).TextColor(t.TextMuted)
						})
					}
					if task.ChecklistTotal > 0 {
						ui.Row(c.Key("check")).Gap(3).AlignItems(ui.Center).Tooltip("Checklist").Children(func() {
							ink := t.TextMuted
							if task.ChecklistDone == task.ChecklistTotal {
								ink = t.Success
							}
							ui.Icon(c, checkboxIcon).Size(12, 12).TextColor(ink)
							ui.Text(c, fmt.Sprintf("%d/%d", task.ChecklistDone, task.ChecklistTotal)).FontSize(11).TextColor(ink)
						})
					}
				})
			}
		})
	})
	if card.Clicked() {
		switch {
		case selecting:
			s.toggle(task.ID)
		case s.OnBulkCancel != nil && task.Selectable && card.ClickModifiers()&(ui.Ctrl|ui.Shift|ui.Super) != 0:
			// Ctrl/Shift+click starts select mode with this card.
			s.selecting = true
			s.toggle(task.ID)
		default:
			s.Open(task.ID)
		}
	}
	if s.OnMove != nil && !selecting {
		before := task.ID
		_, y, _ := card.PointerPosition()
		if y > card.Bounds().H/2 {
			before = s.nextTask(task)
		}
		s.drop(card, task.Lane, before, t.Accent)
	}
}

// firstName is the first word of a name after any bracketed tag, as the
// canvas shows "Aruna" for "Aruna Desain".
func firstName(name string) string {
	rest := strings.TrimSpace(name)
	for strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			break
		}
		rest = strings.TrimSpace(rest[end+1:])
	}
	if f := strings.Fields(rest); len(f) > 0 {
		return f[0]
	}
	return name
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
	Drawer(c, &s.InspectorWidth, func() {
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
func initials(name string) string { return Initials(name) }

// Initials returns up to two capital letters for an avatar: the first
// letter or digit of the first two words. Leading tags in brackets, as the
// "[DUMMY] " of demo accounts, and words without letters or digits are
// skipped, so "[DUMMY] Aruna Desain" gives "AD", not "[A".
func Initials(name string) string {
	rest := strings.TrimSpace(name)
	for rest != "" {
		closing := map[byte]byte{'[': ']', '(': ')', '{': '}', '<': '>'}[rest[0]]
		end := strings.IndexByte(rest, closing)
		if closing == 0 || end < 0 {
			break
		}
		rest = strings.TrimSpace(rest[end+1:])
	}
	pick := func(s string, max int) string {
		var out []rune
		for _, word := range strings.Fields(s) {
			for _, r := range word {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					out = append(out, unicode.ToUpper(r))
					break
				}
			}
			if len(out) == max {
				break
			}
		}
		return string(out)
	}
	if out := pick(rest, 2); out != "" {
		return out
	}
	// A name that is only a tag still gets a letter.
	return pick(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, name), 1)
}

func (s *State) hasLane(l Lane) bool {
	for _, t := range s.tasks {
		if t.Lane == l {
			return true
		}
	}
	return false
}

// selectToggle is the board header's "Pilih", which enters or leaves
// select mode; shown only when some task can be picked.
func (s *State) selectToggle(c *ui.Context) {
	if s.OnBulkCancel == nil || !s.selecting && !s.anySelectable() {
		return
	}
	t := c.Theme()
	label := "Pilih"
	if s.selecting {
		label = "Selesai memilih"
	}
	b := ui.ButtonBase(c.Key("select-mode")).Label(label).Row().AlignItems(ui.Center).Gap(8).Height(34).Padding(0, 12).Radius(8).Border(1, t.Border).Shrink(0).Tooltip("Pilih beberapa tugas (Ctrl/Shift+klik kartu)")
	switch {
	case s.selecting:
		b.Background(t.SurfaceHover).BorderColor(t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, checkboxIcon).Size(15, 15).Shrink(0)
		ui.Text(c, label).FontSize(13).MaxLines(1)
	})
	if b.Clicked() {
		s.SetSelecting(!s.selecting)
		s.menuOpen = false
	}
}

// selectionBar is select mode's bar: how many are picked, "Batalkan (N)"
// and the way out.
func (s *State) selectionBar(c *ui.Context) {
	if !s.selecting {
		return
	}
	t := c.Theme()
	n := len(s.Selected())
	ui.Row(c.Key("selection-bar")).FillWidth().AlignItems(ui.Center).Gap(10).Padding(8, 8, 8, 14).Radius(10).Background(t.Surface).Border(1, t.Accent).Wrap().Label("Pilihan tugas").Children(func() {
		count := "Belum ada yang dipilih"
		if n > 0 {
			count = fmt.Sprintf("%d dipilih", n)
		}
		ui.Text(c, count).FontSize(13).Bold()
		ui.Text(c, "Klik kartu untuk memilih · Esc untuk keluar").FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0).MaxLines(1)
		if n > 0 {
			clear := ui.ButtonBase(c.Key("selection-clear")).Label("Kosongkan pilihan").Height(32).Padding(0, 10).Radius(8).Center()
			if clear.Hovered() {
				clear.Background(t.SurfaceHover)
			}
			clear.FocusRing(clear.FocusVisible()).Children(func() { ui.Text(c, "Kosongkan").FontSize(13) })
			if clear.Clicked() {
				s.picked = nil
			}
		}
		cancel := ui.ButtonBase(c.Key("selection-cancel")).Label(fmt.Sprintf("Batalkan (%d)", n)).Row().AlignItems(ui.Center).Gap(6).Height(32).Padding(0, 12).Radius(8).Background(t.Danger).TextColor(ui.Hex("#ffffff")).Disabled(n == 0)
		if n == 0 {
			cancel.Opacity(0.5)
		}
		cancel.FocusRing(cancel.FocusVisible()).Children(func() {
			ui.Icon(c, cancelIcon).Size(15, 15)
			ui.Text(c, fmt.Sprintf("Batalkan (%d)", n)).FontSize(13).FontWeight(600)
		})
		if cancel.Clicked() && n > 0 {
			s.OnBulkCancel(s.Selected())
		}
		done := ui.ButtonBase(c.Key("selection-close")).Label("Keluar dari mode pilih").Size(32, 32).Radius(8).Center().Tooltip("Keluar (Esc)")
		if done.Hovered() {
			done.Background(t.SurfaceHover)
		}
		done.FocusRing(done.FocusVisible()).Children(func() { ui.Icon(c, closeIcon).Size(16, 16).TextColor(t.TextMuted) })
		if done.Clicked() {
			s.SetSelecting(false)
		}
	})
}
