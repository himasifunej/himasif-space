package appui

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// reminderClockField keeps the optional date-only reminder while offering a
// clock face and keyboard-operable sliders for precise minute selection.
func (a *App) reminderClockField(c *ui.Context, value *string) {
	ui.Column(c.Key("reminder-clock-picker")).FillWidth().Gap(8).Children(func() {
		fieldLabel(c, "Jam reminder (opsional)", false)
		label := "Pilih jam"
		if *value != "" {
			label = *value
		}
		if spaceButton(c, label, "clock", a.reminders.clockOpen).Label("Pilih jam reminder").FillWidth().MinHeight(42).Border(1, c.Theme().Border).Clicked() {
			a.reminders.clockOpen = !a.reminders.clockOpen
		}
		if !a.reminders.clockOpen {
			return
		}
		at, err := time.Parse("15:04", *value)
		if err != nil {
			at = time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
		}
		hour, minute := at.Hour(), at.Minute()
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if spaceButton(c, fmt.Sprintf("%02d", hour), "", !a.reminders.clockMinutes).Label("Pilih jam pada jam analog").Grow(1).Clicked() {
				a.reminders.clockMinutes = false
			}
			if spaceButton(c, fmt.Sprintf("%02d", minute), "", a.reminders.clockMinutes).Label("Pilih menit pada jam analog").Grow(1).Clicked() {
				a.reminders.clockMinutes = true
			}
			period := "AM"
			if hour >= 12 {
				period = "PM"
			}
			if spaceButton(c, period, "", false).Label("Ubah AM atau PM").Clicked() {
				hour = (hour + 12) % 24
				*value = fmt.Sprintf("%02d:%02d", hour, minute)
			}
		})
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			face := ui.ButtonBase(c.Key("reminder-clock-face")).Label("Jam analog reminder").Size(210, 210).Radius(105).Background(c.Theme().SurfacePressed)
			face.Children(func() {
				for i := 0; i < 12; i++ {
					angle := float64(i)*math.Pi/6 - math.Pi/2
					text := fmt.Sprint(i)
					selected := minute/5 == i && a.reminders.clockMinutes
					if a.reminders.clockMinutes {
						text = fmt.Sprintf("%02d", i*5)
					} else {
						if i == 0 {
							text = "12"
						}
						selected = hour%12 == i
					}
					box := ui.Box(c.Key(i)).Absolute().Left(float32(89+75*math.Cos(angle))).Top(float32(89+75*math.Sin(angle))).Size(32, 32).Radius(16).Center()
					if selected {
						box.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
					}
					box.Children(func() { ui.Text(c, text).FontSize(12) })
				}
			})
			x, y, over := face.PointerPosition()
			_, _, dragging := face.Dragged()
			if (face.Clicked() || dragging) && over {
				angle := math.Atan2(float64(x-105), float64(105-y))
				if angle < 0 {
					angle += 2 * math.Pi
				}
				if a.reminders.clockMinutes {
					minute = int(math.Round(angle*60/(2*math.Pi))) % 60
				} else {
					hour = int(math.Round(angle*12/(2*math.Pi)))%12 + (hour/12)*12
				}
				*value = fmt.Sprintf("%02d:%02d", hour, minute)
			}
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			muted(c, "Menit").FontSize(11)
			v := float64(minute)
			if ui.StepSlider(c.Key("reminder-minute-slider"), &v, 0, 59, 1).Label("Menit reminder").Grow(1).Changed() {
				*value = fmt.Sprintf("%02d:%02d", hour, int(v))
			}
		})
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if spaceButton(c, "Tanpa jam", "close", false).Clicked() {
				*value = ""
				a.reminders.clockOpen = false
			}
			if spaceButton(c, "Selesai", "check", true).Clicked() {
				if *value == "" {
					*value = fmt.Sprintf("%02d:%02d", hour, minute)
				}
				a.reminders.clockOpen = false
			}
		})
	})
}
