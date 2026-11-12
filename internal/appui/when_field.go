package appui

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// whenState belongs to one date-and-time field: its dialog, which part the
// analog clock edits, and, on phones, whether the clock replaces the
// calendar. Range fields also track which endpoint is being edited.
type whenState struct {
	open, minutes, clockTab, editEnd bool
	picker                           datePickerState
}

func (a *App) when(key string) *whenState {
	if a.chrome.whens == nil {
		a.chrome.whens = map[string]*whenState{}
	}
	st := a.chrome.whens[key]
	if st == nil {
		st = &whenState{}
		a.chrome.whens[key] = st
	}
	return st
}

var shortDays = [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}

// whenText shows a date and an optional HH:MM as "Sel, 10 Nov 2026 · 08.00".
func whenText(date, clock string) string {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return ""
	}
	s := fmt.Sprintf("%s, %d %s %d", shortDays[d.Weekday()], d.Day(), monthName(d.Month())[:3], d.Year())
	if at, err := time.Parse("15:04", clock); err == nil {
		s += " · " + at.Format("15.04")
	}
	return s
}

func whenButton(c *ui.Context, key, label, text string, active bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).FillWidth().Row().Gap(10).AlignItems(ui.Center).MinHeight(44).Padding(0, 12).Radius(9).Background(t.Background).Border(1, t.Border)
	if active || b.Hovered() {
		b.BorderColor(t.Accent)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("calendar")).Size(18, 18).TextColor(t.TextMuted)
		ui.Text(c, text).Grow(1).MinWidth(0).MaxLines(1)
		ui.Text(c, "Ubah").FontSize(12).TextColor(t.TextMuted)
	})
	return b
}

// whenField is one field for a date and an optional time. It opens the
// calendar and the analog clock together; on phones as a bottom sheet.
func (a *App) whenField(c *ui.Context, key, label string, date, clock *string, required bool) {
	st := a.when(key)
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(8).Children(func() {
		fieldLabel(c, label, required)
		text := whenText(*date, *clock)
		if text == "" {
			text = "Pilih tanggal dan jam"
		}
		if whenButton(c, "button", label, text, st.open).Clicked() {
			st.open, st.clockTab, st.minutes = true, false, false
			if d, err := time.Parse(time.DateOnly, *date); err == nil {
				st.picker.month = d
			}
		}
		if !required && *date != "" && spaceButton(c, "Hapus "+label, "close", false).Padding(4, 8).MinHeight(28).Clicked() {
			*date, *clock = "", ""
		}
	})
	a.whenDialog(c, key, label, st, func(wide bool) {
		ui.Text(c, whenSummary(*date, *clock)).FontSize(13).TextColor(c.Theme().TextMuted)
		whenBody(c, st, wide, func() {
			before := *date
			calendarGrid(c, &st.picker, date)
			if *date != before && !wide {
				st.clockTab = true
			}
		}, func() { analogClock(c, key, clock, &st.minutes) })
	}, func() {
		if spaceButton(c, "Tanpa jam", "close", false).Clicked() {
			*clock = ""
		}
	})
}

func whenSummary(date, clock string) string {
	s := whenText(date, clock)
	if s == "" {
		return "Belum ada tanggal"
	}
	if clock == "" {
		s += " · tanpa jam"
	}
	return s
}

// whenBody lays out the calendar and the clock side by side when the
// dialog is wide, and as two tabs on phones.
func whenBody(c *ui.Context, st *whenState, wide bool, calendar, clock func()) {
	if wide {
		ui.Row(c).FillWidth().Gap(18).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(calendar)
			ui.Column(c).Width(240).Shrink(0).Gap(8).AlignItems(ui.Center).Children(clock)
		})
		return
	}
	ui.Row(c).FillWidth().Gap(4).Children(func() {
		if spaceButton(c, "Tanggal", "calendar", !st.clockTab).Grow(1).Clicked() {
			st.clockTab = false
		}
		if spaceButton(c, "Jam", "clock", st.clockTab).Grow(1).Clicked() {
			st.clockTab = true
		}
	})
	if st.clockTab {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(clock)
	} else {
		calendar()
	}
}

func (a *App) whenDialog(c *ui.Context, key, title string, st *whenState, body func(wide bool), extra func()) {
	if !st.open {
		return
	}
	t := c.Theme()
	w, h := c.Size()
	wide := w >= mobileMaxWidth
	ui.DialogBase(c.Key(key+"-dialog"), &st.open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if wide {
			panel.Width(min(float32(660), w-48)).Radius(16)
		} else {
			backdrop.Justify(ui.End)
			panel.Width(w).Radius(18, 18, 0, 0)
		}
		panel.MaxHeight(h-40).Column().Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).FillWidth().Padding(14, 18).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Icon(c, spaceIcon("calendar")).Size(18, 18)
				ui.Text(c, title).FontSize(16).Bold().Grow(1).Padding(0, 8).MaxLines(1)
				if spaceIconButton(c, "Tutup "+title, "close").Clicked() {
					st.open = false
				}
			})
			ui.Scroll(c.Key("when-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(16, 18).Gap(12).Children(func() { body(wide) })
			})
			ui.Row(c).FillWidth().Padding(12, 18).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				extra()
				ui.Box(c).Grow(1)
				if primarySpaceButton(c, "Selesai", "check").Clicked() {
					st.open = false
				}
			})
		})
	})
}

// analogClock edits HH:MM with a clock face: pick the hour, then it moves
// to the minutes. The minute slider keeps keyboard and exact-minute input.
func analogClock(c *ui.Context, key string, value *string, minutes *bool) {
	t := c.Theme()
	at, err := time.Parse("15:04", *value)
	if err != nil {
		at = time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
	}
	hour, minute := at.Hour(), at.Minute()
	set := func(h, m int) { *value = fmt.Sprintf("%02d:%02d", h, m) }
	ui.Row(c.Key(key + "-hm")).Gap(6).AlignItems(ui.Center).Children(func() {
		if spaceButton(c, fmt.Sprintf("%02d", hour), "", !*minutes).Label("Pilih jam pada jam analog").Clicked() {
			*minutes = false
		}
		ui.Text(c, ":").FontSize(20).Bold()
		if spaceButton(c, fmt.Sprintf("%02d", minute), "", *minutes).Label("Pilih menit pada jam analog").Clicked() {
			*minutes = true
		}
		period := "AM"
		if hour >= 12 {
			period = "PM"
		}
		if spaceButton(c, period, "", false).Label("Ubah AM atau PM").Clicked() {
			set((hour+12)%24, minute)
		}
	})
	size := float32(210)
	face := ui.ButtonBase(c.Key(key+"-face")).Label("Jam analog").Size(size, size).Radius(size / 2).Background(t.SurfacePressed)
	if *value == "" {
		face.Opacity(.6)
	}
	face.Children(func() {
		for i := 0; i < 12; i++ {
			angle := float64(i)*math.Pi/6 - math.Pi/2
			text := fmt.Sprint(i)
			selected := hour%12 == i
			if *minutes {
				text = fmt.Sprintf("%02d", i*5)
				selected = (minute+2)/5%12 == i
			} else if i == 0 {
				text = "12"
			}
			box := ui.Box(c.Key(i)).Absolute().Left(float32(89+75*math.Cos(angle))).Top(float32(89+75*math.Sin(angle))).Size(32, 32).Radius(16).Center()
			if selected && *value != "" {
				box.Background(t.Accent).TextColor(t.AccentText)
			}
			box.Children(func() { ui.Text(c, text).FontSize(12) })
		}
	})
	x, y, over := face.PointerPosition()
	holdTouchPan(face)
	_, _, dragging := face.Dragged()
	if (face.Clicked() || dragging) && over {
		angle := math.Atan2(float64(x-size/2), float64(size/2-y))
		if angle < 0 {
			angle += 2 * math.Pi
		}
		if *minutes {
			set(hour, int(math.Round(angle*60/(2*math.Pi)))%60)
		} else {
			set(int(math.Round(angle*12/(2*math.Pi)))%12+(hour/12)*12, minute)
			if face.Clicked() {
				*minutes = true
			}
		}
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		muted(c, "Menit").FontSize(11)
		v := float64(minute)
		if ui.StepSlider(c.Key(key+"-minute"), &v, 0, 59, 1).Label("Menit").Grow(1).Changed() {
			set(hour, int(v))
		}
	})
}

// rangeWhenField edits a task's start and deadline, each with an optional
// time, from one field. Posting stays a separate field (PRD MED-01).
func (a *App) rangeWhenField(c *ui.Context, key string, d *taskDraft) {
	st := a.when(key)
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(8).Children(func() {
		fieldLabel(c, "Jadwal", true)
		text := "Pilih mulai dan tenggat"
		if d.plannedStart != "" || d.deadline != "" {
			start := whenText(d.plannedStart, d.plannedStartTime)
			if start == "" {
				start = "Tanpa tanggal mulai"
			}
			end := whenText(d.deadline, d.deadlineTime)
			if end == "" {
				end = "tanpa tenggat"
			}
			text = start + "  →  " + end
		}
		if whenButton(c, "button", "Jadwal tugas", text, st.open).Clicked() {
			st.open, st.clockTab, st.minutes = true, false, false
			st.editEnd = d.plannedStart != "" && d.deadline == ""
		}
	})
	a.whenDialog(c, key, "Jadwal tugas", st, func(wide bool) {
		t := c.Theme()
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			for _, end := range []bool{false, true} {
				label, date, clock := "Mulai pengerjaan", d.plannedStart, d.plannedStartTime
				if end {
					label, date, clock = "Tenggat", d.deadline, d.deadlineTime
				}
				b := ui.ButtonBase(c.Key(end)).Label("Edit "+label).Grow(1).MinWidth(0).Column().Gap(2).Padding(8, 12).Radius(10).Border(1, t.Border)
				if st.editEnd == end {
					b.BorderColor(t.Accent).Background(t.SurfaceHover)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
					s := whenSummary(date, clock)
					ui.Text(c, s).FontSize(13).Bold().MaxLines(1)
				})
				if b.Clicked() {
					st.editEnd = end
					st.clockTab = false
				}
			}
		})
		value, clock := &d.plannedStart, &d.plannedStartTime
		if st.editEnd {
			value, clock = &d.deadline, &d.deadlineTime
		}
		whenBody(c, st, wide, func() {
			st.picker.rangeStart, st.picker.rangeEnd = d.plannedStart, d.deadline
			before := *value
			calendarGrid(c, &st.picker, value)
			if *value != before {
				if !st.editEnd {
					if d.deadline != "" && d.plannedStart > d.deadline {
						d.deadline, d.deadlineTime = "", ""
					}
					st.editEnd = true
				} else if d.plannedStart != "" && d.plannedStart > d.deadline {
					d.plannedStart, d.deadline = d.deadline, d.plannedStart
					d.plannedStartTime, d.deadlineTime = d.deadlineTime, d.plannedStartTime
				}
				if st.editEnd && d.deadlineTime == "" && d.deadline != "" {
					d.deadlineTime = "23:59"
				}
				if !wide && *value == d.deadline {
					st.clockTab = true
				}
			}
		}, func() { analogClock(c, key, clock, &st.minutes) })
	}, func() {
		if spaceButton(c, "Tanpa jam", "close", false).Clicked() {
			if st.editEnd {
				d.deadlineTime = ""
			} else {
				d.plannedStartTime = ""
			}
		}
		if d.plannedStart != "" && spaceButton(c, "Tanpa tanggal mulai", "close", false).Clicked() {
			d.plannedStart, d.plannedStartTime = "", ""
			st.editEnd = true
		}
	})
}
