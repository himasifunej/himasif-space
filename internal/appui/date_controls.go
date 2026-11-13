package appui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type datePickerState struct {
	open                 bool
	month                time.Time
	selectStart          bool
	rangeStart, rangeEnd string
}

// Visible labels are separate from accessibility labels. Keep the required
// marker in the same text run, so it cannot wrap onto its own flex row.
func fieldLabel(c *ui.Context, label string, required bool) {
	if required {
		label += " *"
	}
	// As ui.Field draws its label, so all of a form's labels look alike.
	ui.Text(c, label).FontWeight(500)
}

func (a *App) dateField(c *ui.Context, key, label string, value *string, required, inline bool) {
	if a.chrome.datePickers == nil {
		a.chrome.datePickers = map[string]*datePickerState{}
	}
	p := a.chrome.datePickers[key]
	if p == nil {
		p = &datePickerState{}
		a.chrome.datePickers[key] = p
	}
	ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		fieldLabel(c, label, required)
		text := "Pilih tanggal"
		if date, err := time.Parse(time.DateOnly, *value); err == nil {
			text = fmt.Sprintf("%d %s %d", date.Day(), monthName(date.Month()), date.Year())
		}
		b := spaceButton(c, text, "calendar", p.open).Label(label).FillWidth().MinHeight(42).Border(1, c.Theme().Border).Background(c.Theme().Background)
		if b.Clicked() {
			p.open = !p.open
			if d, err := time.Parse(time.DateOnly, *value); err == nil {
				p.month = d
			}
		}
		body := func() { calendarGrid(c, p, value) }
		if inline {
			if p.open {
				ui.Column(c).FillWidth().Padding(10).Gap(8).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(body)
			}
		} else {
			popover(c, b, &p.open, 292, body)
		}
		if !required && *value != "" && spaceButton(c, "Hapus "+strings.ToLower(label), "close", false).Padding(4, 8).MinHeight(24).Clicked() {
			*value = ""
		}
	})
}

// Calendar drawn with Space tokens on both desktop and web. No platform date
// input or browser picker participates in selection.
func calendarGrid(c *ui.Context, p *datePickerState, value *string) {
	t := c.Theme()
	if p.month.IsZero() {
		p.month = c.Now()
	}
	month := time.Date(p.month.Year(), p.month.Month(), 1, 12, 0, 0, 0, time.UTC)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		if spaceIconButton(c, "Bulan sebelumnya", "left").Clicked() {
			p.month = month.AddDate(0, -1, 0)
		}
		ui.Text(c, fmt.Sprintf("%s %d", monthName(month.Month()), month.Year())).Grow(1).Bold()
		if spaceIconButton(c, "Bulan berikutnya", "right").Clicked() {
			p.month = month.AddDate(0, 1, 0)
		}
	})
	ui.Row(c).FillWidth().Gap(2).Children(func() {
		for _, day := range []string{"S", "S", "R", "K", "J", "S", "M"} {
			ui.Box(c).Grow(1).Height(24).Center().Children(func() { ui.Text(c, day).FontSize(10).TextColor(t.TextMuted) })
		}
	})
	first := month.AddDate(0, 0, -(int(month.Weekday())+6)%7)
	for week := 0; week < 6; week++ {
		ui.Row(c.Key(week)).FillWidth().Gap(2).Children(func() {
			for day := 0; day < 7; day++ {
				d := first.AddDate(0, 0, week*7+day)
				iso := d.Format(time.DateOnly)
				b := ui.ButtonBase(c.Key(iso)).Label("Tanggal " + iso).Grow(1).MinWidth(0).Height(30).Radius(7).Center().TextColor(t.Text)
				if d.Month() != month.Month() {
					b.TextColor(t.TextMuted)
				}
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				if p.rangeStart != "" && p.rangeEnd != "" && iso >= p.rangeStart && iso <= p.rangeEnd {
					b.Background(t.SurfaceHover)
				}
				if iso == *value {
					b.Background(t.Accent).TextColor(t.AccentText)
				}
				b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, strconv.Itoa(d.Day())).FontSize(12) })
				if b.Clicked() {
					*value = iso
					p.open = false
				}
			}
		})
	}
	if spaceButton(c, "Hari ini", "calendar", false).FillWidth().Clicked() {
		*value = c.Now().Format(time.DateOnly)
		p.open = false
	}
}

func clockField(c *ui.Context, key, label string, value *string, required bool) {
	t := c.Theme()
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(8).Children(func() {
		fieldLabel(c, label, required)
		ui.Row(c).FillWidth().MinWidth(0).Height(44).Padding(4, 8).Gap(8).Radius(9).Background(t.Background).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("clock")).Size(16, 16).TextColor(t.TextMuted)
			ui.TextInput(c.Key("value"), value).Label(label).Placeholder("HH:MM").Grow(1).MinWidth(0).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0)
		})
		ui.Row(c).FillWidth().Wrap().Gap(4).Children(func() {
			for _, delta := range []int{-15, 15} {
				label := fmt.Sprintf("%+d", delta)
				if spaceButton(c, label, "", false).Label(fmt.Sprintf("%+d menit %s", delta, key)).Tooltip(fmt.Sprintf("%+d menit", delta)).MinHeight(24).Padding(4, 6).Grow(1).MinWidth(0).Clicked() {
					at, err := time.Parse("15:04", *value)
					if err != nil {
						at = time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
					} else {
						at = at.Add(time.Duration(delta) * time.Minute)
					}
					*value = at.Format("15:04")
				}
			}
			if !required && spaceButton(c, "Lewati", "", false).Label("Lewati "+label).MinHeight(24).Padding(4, 6).Clicked() {
				*value = ""
			}
		})
	})
}

// One inline calendar edits the planned range. Picking a start advances to the
// deadline, while each endpoint can also be edited directly. Posting is an
// independent field and never serves as a hidden work-start date.
func (a *App) taskSchedule(c *ui.Context, d *taskDraft) {
	if a.chrome.datePickers == nil {
		a.chrome.datePickers = map[string]*datePickerState{}
	}
	p := a.chrome.datePickers["task-range"]
	if p == nil {
		p = &datePickerState{selectStart: d.plannedStart == ""}
		a.chrome.datePickers["task-range"] = p
	}
	dateLabel := func(value string) string {
		if at, err := time.Parse(time.DateOnly, value); err == nil {
			return fmt.Sprintf("%d %s", at.Day(), monthName(at.Month()))
		}
		return "Pilih tanggal"
	}
	ui.Column(c.Key("work-range")).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(10).Children(func() {
			for _, start := range []bool{true, false} {
				ui.Column(c.Key(start)).Grow(1).MinWidth(0).Gap(6).Children(func() {
					label, value := "Tenggat tugas", d.deadline
					if start {
						label, value = "Mulai pengerjaan", d.plannedStart
					}
					fieldLabel(c, label, !start)
					b := spaceButton(c, dateLabel(value), "calendar", p.open && p.selectStart == start).Label("Pilih "+strings.ToLower(label)).FillWidth().MinHeight(40).Border(1, c.Theme().Border)
					if b.Clicked() {
						p.selectStart = start
						p.open = true
						if at, err := time.Parse(time.DateOnly, value); err == nil {
							p.month = at
						}
					}
				})
			}
		})
		if p.open {
			ui.Column(c).FillWidth().Padding(10).Gap(6).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(func() {
				if p.selectStart {
					muted(c, "Pilih mulai pengerjaan, lalu tenggat.")
				} else {
					muted(c, "Pilih tenggat tugas.")
				}
				p.rangeStart, p.rangeEnd = d.plannedStart, d.deadline
				value := &d.deadline
				if p.selectStart {
					value = &d.plannedStart
				}
				wasStart := p.selectStart
				calendarGrid(c, p, value)
				if !p.open && wasStart {
					if d.deadline != "" && d.plannedStart > d.deadline {
						d.deadline = ""
						d.deadlineTime = ""
					}
					p.selectStart = false
					p.open = true
				} else if !p.open && d.plannedStart != "" && d.plannedStart > d.deadline {
					d.plannedStart, d.deadline = d.deadline, d.plannedStart
					d.plannedStartTime, d.deadlineTime = d.deadlineTime, d.plannedStartTime
				}
			})
		}
		if d.plannedStart != "" && spaceButton(c, "Tanpa tanggal mulai", "close", false).MinHeight(24).Padding(4, 6).Clicked() {
			d.plannedStart = ""
			d.plannedStartTime = ""
		}
		if d.plannedStart != "" || d.deadline != "" {
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				if d.plannedStart != "" {
					ui.Column(c).Grow(1).MinWidth(0).Children(func() { clockField(c, "task-start-time", "Jam mulai", &d.plannedStartTime, false) })
				}
				if d.deadline != "" {
					ui.Column(c).Grow(1).MinWidth(0).Children(func() { clockField(c, "task-deadline-time", "Jam tenggat", &d.deadlineTime, false) })
				}
			})
		}
	})
}
