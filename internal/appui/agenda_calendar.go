package appui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// The Space's Agenda tab as on canvas A (#view-agenda): one calendar for
// task deadlines, posting dates and meetings, by month, week or as a list.
// It reads the tasks and meetings the Space already loads.

const (
	agendaMonth = iota
	agendaWeek
	agendaList
)

// agendaState is the calendar's view (Bulan, Minggu, Daftar) and how many
// months or weeks it has moved from today.
type agendaState struct {
	mode, offset int
}

// Kinds of agenda items. Their colors follow the canvas: deadlines in the
// accent (late ones in danger), meetings in the "doing" blue, posting dates
// in the review amber.
const (
	agendaDeadline = "Tenggat"
	agendaPosting  = "Posting"
	agendaMeeting  = "Rapat"
)

type agendaEvent struct {
	kind    string
	title   string
	day     timeutil.Date
	at      *time.Time
	late    bool
	task    id.ID
	meeting id.ID
}

var agendaDayNames = []string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}

// agendaZone is the period's time zone, which the calendar's days follow.
func (a *App) agendaZone() *time.Location {
	zone := timeutil.DefaultZoneName
	if p := a.period(); p != nil && p.TimeZone != "" {
		zone = p.TimeZone
	}
	loc, _ := timeutil.LoadZone(zone)
	if loc == nil {
		loc = time.UTC
	}
	return loc
}

// agendaEvents lists the deadlines and posting dates of the Space's tasks
// that are not cancelled, and its meetings that are not cancelled, by day
// and time. Items without a time come first in their day.
func agendaEvents(tasks []api.Task, meetings []api.Meeting, loc *time.Location, now time.Time) []agendaEvent {
	var out []agendaEvent
	for _, t := range tasks {
		if t.Progress == api.TaskCancelled {
			continue
		}
		for _, e := range []struct {
			kind string
			date *timeutil.Date
			at   *time.Time
		}{{agendaDeadline, t.DeadlineDate, t.DeadlineAt}, {agendaPosting, t.PostingDate, t.PostingAt}} {
			ev := agendaEvent{kind: e.kind, title: t.Title, task: t.ID}
			switch {
			case e.at != nil:
				at := e.at.In(loc)
				ev.at, ev.day = &at, timeutil.DateOf(at)
			case e.date != nil:
				ev.day = *e.date
			default:
				continue
			}
			ev.late = e.kind == agendaDeadline && taskLate(t, now)
			out = append(out, ev)
		}
	}
	for _, m := range meetings {
		if m.Cancelled {
			continue
		}
		at := m.StartsAt.In(loc)
		out = append(out, agendaEvent{kind: agendaMeeting, title: m.Title, day: timeutil.DateOf(at), at: &at, meeting: m.ID})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if c := out[i].day.Compare(out[j].day); c != 0 {
			return c < 0
		}
		if (out[i].at == nil) != (out[j].at == nil) {
			return out[i].at == nil
		}
		if out[i].at != nil && !out[i].at.Equal(*out[j].at) {
			return out[i].at.Before(*out[j].at)
		}
		return out[i].title < out[j].title
	})
	return out
}

// agendaMonthDays are the days a month grid shows: whole weeks from Monday,
// starting with the week of the 1st and ending with the week of the last.
func agendaMonthDays(year int, month time.Month) []timeutil.Date {
	first := timeutil.Date{Year: year, Month: month, Day: 1}
	start := first.AddDays(-((int(time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Weekday()) + 6) % 7))
	last := timeutil.DateOf(time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC))
	end := last.AddDays(6 - (int(time.Date(last.Year, last.Month, last.Day, 0, 0, 0, 0, time.UTC).Weekday())+6)%7)
	var days []timeutil.Date
	for d := start; !end.Before(d); d = d.AddDays(1) {
		days = append(days, d)
	}
	return days
}

// agendaWeekStart is the Monday of the week offset weeks from today's.
func agendaWeekStart(today timeutil.Date, offset int) timeutil.Date {
	wd := int(time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, time.UTC).Weekday())
	return today.AddDays(-(wd+6)%7 + 7*offset)
}

func agendaEventsOn(events []agendaEvent, day timeutil.Date) []agendaEvent {
	var out []agendaEvent
	for _, e := range events {
		if e.day == day {
			out = append(out, e)
		}
	}
	return out
}

func agendaColor(t *ui.Theme, e agendaEvent) ui.Color {
	dark := spaceLuminance(t.Background) < .5
	switch {
	case e.late:
		return t.Danger
	case e.kind == agendaMeeting:
		if dark {
			return ui.Hex("#9fb8d6")
		}
		return ui.Hex("#456b96")
	case e.kind == agendaPosting:
		if dark {
			return ui.Hex("#e0b45c")
		}
		return ui.Hex("#a87515")
	}
	return t.Accent
}

func agendaClock(e agendaEvent) string {
	if e.at == nil {
		return ""
	}
	return e.at.Format("15.04")
}

// openAgendaEvent opens a task's detail on the board, or a meeting's
// participants in the Rapat tab.
func (a *App) openAgendaEvent(e agendaEvent) {
	if e.meeting != id.Nil {
		a.selectMeeting(e.meeting)
		a.chrome.tab = "Rapat"
		return
	}
	a.workspace.Open(e.task)
	a.chrome.tab = "Board tugas"
}

func (a *App) agendaCalendar(c *ui.Context) {
	a.ensureTasks(false)
	a.ensureMeetings()
	t := c.Theme()
	loc := a.agendaZone()
	now := c.Now().In(loc)
	today := timeutil.DateOf(now)
	st := &a.agenda
	phone := isMobile(c)
	events := agendaEvents(a.tasks.items, a.meetings.items, loc, now)

	var title string
	var days []timeutil.Date
	month := time.Date(today.Year, today.Month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, st.offset, 0)
	if st.mode == agendaMonth {
		title = "Agenda " + monthName(month.Month()) + " " + fmt.Sprint(month.Year())
		days = agendaMonthDays(month.Year(), month.Month())
	} else {
		start := agendaWeekStart(today, st.offset)
		end := start.AddDays(6)
		for d := start; !end.Before(d); d = d.AddDays(1) {
			days = append(days, d)
		}
		title = fmt.Sprintf("Agenda %d %s – %d %s %d", start.Day, monthName(start.Month)[:3], end.Day, monthName(end.Month)[:3], end.Year)
	}

	ui.Scroll(c).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(24).Gap(16)
		if phone {
			body.Padding(16).Gap(12)
		}
		body.Children(func() {
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			wide := head.Bounds().W >= 640
			if wide {
				head.Row().AlignItems(ui.Center)
			}
			head.Children(func() {
				titles := ui.Column(c).MinWidth(0).Gap(4)
				if wide {
					titles.Grow(1)
				}
				titles.Children(func() {
					ui.Text(c, title).FontSize(20).Bold()
					ui.Text(c, "Tenggat tugas, rapat, dan jadwal posting dalam satu kalender.").FontSize(13).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					unit := "bulan"
					if st.mode != agendaMonth {
						unit = "minggu"
					}
					if spaceIconButton(c, strings.ToUpper(unit[:1])+unit[1:]+" sebelumnya", "left").Clicked() {
						st.offset--
					}
					if spaceButton(c, "Hari ini", "", false).Padding(6, 10).MinHeight(32).Disabled(st.offset == 0).Clicked() {
						st.offset = 0
					}
					if spaceIconButton(c, strings.ToUpper(unit[:1])+unit[1:]+" berikutnya", "right").Clicked() {
						st.offset++
					}
					if mode := agendaSegmented(c, st.mode, "Bulan", "Minggu", "Daftar"); mode != st.mode {
						// Keep the period in view: a month becomes its first
						// week, and a week becomes its month.
						if st.mode == agendaMonth {
							st.offset = int(agendaMonthWeek(today, month))
						} else if mode == agendaMonth {
							d := agendaWeekStart(today, st.offset)
							st.offset = (d.Year-today.Year)*12 + int(d.Month) - int(today.Month)
						}
						st.mode = mode
					}
				})
			})
			a.taskFeedback(c)
			if a.meetings.err != "" {
				muted(c, "Rapat belum dapat dimuat: "+a.meetings.err)
			}
			if !a.tasks.loaded && !a.meetings.loaded {
				muted(c, "Memuat agenda…")
				return
			}
			switch st.mode {
			case agendaList:
				a.agendaListView(c, events, days, today)
			default:
				a.agendaGrid(c, events, days, today, st.mode == agendaMonth, month.Month(), phone)
				if phone && st.mode == agendaMonth {
					a.agendaListView(c, events, days, today)
				}
			}
			agendaLegend(c)
		})
	})
}

// agendaMonthWeek is how many weeks from today's the first week of month
// starts, for switching from Bulan to Minggu.
func agendaMonthWeek(today timeutil.Date, month time.Time) int64 {
	first := agendaWeekStart(timeutil.DateOf(month), 0)
	this := agendaWeekStart(today, 0)
	if month.Year() == today.Year && month.Month() == today.Month {
		return 0
	}
	return int64(first.At(12, 0, time.UTC).Sub(this.At(12, 0, time.UTC)).Hours() / (24 * 7))
}

// agendaSegmented is the canvas' segmented control: the chosen option on
// the accent, the others muted. It returns the option chosen this frame.
func agendaSegmented(c *ui.Context, selected int, labels ...string) int {
	t := c.Theme()
	choice := selected
	ui.Row(c).Gap(2).Padding(3).Radius(9).Background(t.Surface).Border(1, t.Border).Children(func() {
		for i, label := range labels {
			b := ui.ButtonBase(c.Key(label)).Label(label).Height(28).Padding(0, 12).Radius(7).Center().TextColor(t.TextMuted)
			if i == selected {
				b.Background(t.Accent).TextColor(t.AccentText)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				txt := ui.Text(c, label).FontSize(12)
				if i == selected {
					txt.FontWeight(600)
				}
			})
			if b.Clicked() {
				choice = i
			}
		}
	})
	return choice
}

// agendaGrid draws the month (or one week) as the canvas' bordered grid of
// days. Today's number sits on the accent; days of other months are dim.
func (a *App) agendaGrid(c *ui.Context, events []agendaEvent, days []timeutil.Date, today timeutil.Date, monthView bool, month time.Month, phone bool) {
	t := c.Theme()
	cellHeight := float32(96)
	if !monthView {
		cellHeight = 320
	}
	if phone {
		cellHeight = 54
		if !monthView {
			cellHeight = 160
		}
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(1).Radius(12).Background(t.Border).Border(1, t.Border).Clip().Children(func() {
		ui.Row(c).FillWidth().Gap(1).Children(func() {
			for _, name := range agendaDayNames {
				ui.Box(c.Key(name)).Grow(1).Basis(0).MinWidth(0).Padding(8, 10).Background(t.Surface).Children(func() {
					ui.Text(c, strings.ToUpper(name)).FontSize(11).TextColor(faint(t)).MaxLines(1)
				})
			}
		})
		for w := 0; w*7 < len(days); w++ {
			ui.Row(c.Key(w)).FillWidth().Gap(1).Children(func() {
				for _, day := range days[w*7 : w*7+7] {
					dim := monthView && day.Month != month
					cell := ui.Column(c.Key(day.String())).Grow(1).Basis(0).MinWidth(0).Height(cellHeight).Padding(8).Gap(4).Background(t.Surface).Clip()
					if phone {
						cell.Padding(4).Gap(3)
					}
					if dim {
						cell.Background(t.Background)
					}
					cell.Children(func() {
						num := ui.Text(c, fmt.Sprint(day.Day)).FontSize(12).TextColor(t.TextMuted)
						if dim {
							num.TextColor(faint(t))
						}
						if day == today {
							num.TextColor(t.AccentText).Background(t.Accent).Radius(6).Padding(0, 6).FontWeight(600)
						}
						if dim {
							return
						}
						list := agendaEventsOn(events, day)
						if phone && monthView {
							// A phone's month cell is too small for strips
							// and "+n lainnya": like the canvas, up to three
							// dots on the items' colors; the list below the
							// grid is what opens them.
							agendaDots(c, list)
							return
						}
						limit := 3
						if !monthView {
							limit = 12
						}
						for i, e := range list {
							if i == limit {
								muted(c, fmt.Sprintf("+%d lainnya", len(list)-limit)).FontSize(11)
								break
							}
							a.agendaChip(c.Key(i), e, phone)
						}
					})
				}
			})
		}
	})
}

// agendaDotLimit is how many dots a phone's month cell shows at most.
const agendaDotLimit = 3

// agendaDots draws a day's items as small dots on their colors, at most
// agendaDotLimit of them.
func agendaDots(c *ui.Context, list []agendaEvent) {
	if len(list) == 0 {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
		for i, e := range list {
			if i == agendaDotLimit {
				break
			}
			ui.Box(c.Key(i)).Width(6).Height(6).Radius(3).Background(agendaColor(t, e))
		}
	})
}

// agendaChip is an item in a day: a strip on the item's color, then its
// title (and time in the week view). On a phone it is a bare strip.
func (a *App) agendaChip(c *ui.Context, e agendaEvent, phone bool) {
	t := c.Theme()
	color := agendaColor(t, e)
	label := e.kind + " " + e.title
	if clock := agendaClock(e); clock != "" {
		label += " " + clock
	}
	b := ui.ButtonBase(c).Label(label).Tooltip(label).FillWidth().MinWidth(0).Row().AlignItems(ui.Center).Gap(6).Radius(5).Background(spaceMix(t.Surface, t.Text, .06)).BorderWidth(0, 0, 0, 2).BorderColor(color)
	if phone {
		b.Height(5).Padding(0)
	} else {
		b.Padding(2, 6)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		if phone {
			return
		}
		title := e.title
		if clock := agendaClock(e); clock != "" && e.kind == agendaMeeting {
			title += " " + clock
		}
		ui.Text(c, title).FontSize(11).TextColor(t.Text).MaxLines(1).Grow(1).MinWidth(0)
	})
	if b.Clicked() {
		a.openAgendaEvent(e)
	}
}

// agendaListView lists the items of the shown days, grouped by day.
func (a *App) agendaListView(c *ui.Context, events []agendaEvent, days []timeutil.Date, today timeutil.Date) {
	t := c.Theme()
	shown := 0
	ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		for _, day := range days {
			list := agendaEventsOn(events, day)
			if len(list) == 0 {
				continue
			}
			shown++
			ui.Column(c.Key(day.String())).FillWidth().MinWidth(0).Padding(12, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				weekday := (int(day.At(12, 0, time.UTC).Weekday()) + 6) % 7
				head := fmt.Sprintf("%s, %d %s", agendaDayNames[weekday], day.Day, monthName(day.Month))
				if day == today {
					head += " · hari ini"
				}
				ui.Text(c, head).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
				for i, e := range list {
					row := ui.ButtonBase(c.Key(i)).Label(e.kind+" "+e.title).FillWidth().MinWidth(0).Row().AlignItems(ui.Center).Gap(10).Padding(6, 8).Radius(7)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.FocusRing(row.FocusVisible()).Children(func() {
						ui.Box(c).Size(8, 8).Radius(2).Shrink(0).Background(agendaColor(t, e))
						ui.Text(c, e.title).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
						kind := e.kind
						if e.late {
							kind = "Lewat tenggat"
						}
						ui.Text(c, kind).FontSize(11).TextColor(faint(t)).Shrink(0)
						clock := agendaClock(e)
						if clock == "" {
							clock = "—"
						}
						ui.Text(c, clock).Font(FontMono).FontSize(11).TextColor(t.TextMuted).Width(40).Shrink(0)
					})
					if row.Clicked() {
						a.openAgendaEvent(e)
					}
				}
			})
		}
		if shown == 0 {
			ui.Box(c).FillWidth().Padding(16).Children(func() { muted(c, "Tidak ada tenggat, rapat, atau jadwal posting pada rentang ini.") })
		}
	})
}

func agendaLegend(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Wrap().Gap(14).Children(func() {
		for _, e := range []agendaEvent{{kind: agendaDeadline}, {kind: agendaDeadline, late: true}, {kind: agendaMeeting}, {kind: agendaPosting}} {
			label := e.kind
			if e.late {
				label = "Lewat tenggat"
			}
			ui.Row(c.Key(label)).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Background(agendaColor(t, e))
				ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}
