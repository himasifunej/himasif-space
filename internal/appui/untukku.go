package appui

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Untukku (canvas 26495b2dee24, option 1A with C's "Berikutnya"): the
// tasks the user is PJ of, the meetings they attend and their reminders,
// from every Space, in one timeline per day: overdue, today, tomorrow and
// the rest of the week. Each row carries its Space and one action. It is
// the first page, and the one item of Pribadi; Jobdesk saya and Reminder
// pribadi stay as pages behind it.

// PathUntukku is the personal page.
const PathUntukku = "/untukku"

// untukDayRows is how many rows a day shows before "+N lainnya".
const untukDayRows = 4

// untukMeetingsEvery is how often the meetings are read again.
const untukMeetingsEvery = 5 * time.Minute

// MyMeetingsAPI lists the meetings the account organizes or attends in a
// period, across its Spaces.
type MyMeetingsAPI interface {
	MyMeetings(context.Context, id.ID) ([]api.Meeting, error)
}

func (a *App) myMeetingsService() (MyMeetingsAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(MyMeetingsAPI)
	return s, ok
}

// untukState is the page's: the meetings read, the filter, and the days
// opened past their first rows.
type untukState struct {
	period   id.ID
	loading  bool
	loaded   bool
	err      string
	fetched  time.Time
	meetings []api.Meeting
	filter   int // 0 all, 1 tasks, 2 meetings, 3 reminders
	expanded map[string]bool
}

// ensureMyMeetings reads the period's meetings, again now and then.
func (a *App) ensureMyMeetings(now time.Time) {
	st := &a.untuk
	if st.period != a.periodID {
		*st = untukState{period: a.periodID, filter: st.filter}
	}
	if st.loading || a.periodID == id.Nil || a.isAdmin() {
		return
	}
	if (st.loaded || st.err != "") && now.Sub(st.fetched) < untukMeetingsEvery {
		return
	}
	svc, ok := a.myMeetingsService()
	if !ok {
		st.loaded, st.fetched = true, now
		return
	}
	gen, pid := a.gen, a.periodID
	st.loading, st.fetched = true, now
	a.async(func() func() {
		items, err := svc.MyMeetings(context.Background(), pid)
		return func() {
			if gen != a.gen || a.untuk.period != pid {
				return
			}
			st := &a.untuk
			st.loading = false
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.loaded, st.err, st.meetings = true, "", items
		}
	})
}

// Kinds of an Untukku row.
const (
	untukTask     = "task"
	untukMeeting  = "meeting"
	untukReminder = "reminder"
)

// untukItem is one row: what, when, where, and its action.
type untukItem struct {
	kind   string
	id     id.ID
	title  string
	sub    string
	at     time.Time // when it is due or begins
	timed  bool      // at has a time of day; else the whole day
	late   bool
	space  api.Space
	index  int // the Space's place in the sidebar, -1 for none
	action string
}

// untukItems are the rows of the week from now: open tasks the user is PJ
// of with a deadline, meetings not over, reminders not done. Late tasks
// and reminders come whatever their date.
func (a *App) untukItems(now time.Time, loc *time.Location) []untukItem {
	end := startOfDay(now, loc).AddDate(0, 0, 7)
	var out []untukItem
	if a.tasks.personal && a.tasks.loaded {
		for _, t := range a.tasks.items {
			if t.Progress != api.TaskTodo && t.Progress != api.TaskDoing {
				continue
			}
			if t.DeadlineAt == nil && t.DeadlineDate == nil {
				continue
			}
			due := taskDue(t)
			late := taskLate(t, now)
			if !late && !due.Before(end) {
				continue
			}
			it := untukItem{kind: untukTask, id: t.ID, title: t.Title, at: due, timed: t.DeadlineAt != nil, late: late, index: -1, action: "Buka"}
			if t.DeadlineAt == nil {
				it.at = t.DeadlineDate.At(23, 59, loc)
			}
			it.sub = "tenggat"
			if late {
				it.sub = "lewat tenggat"
			}
			if s, i, ok := a.taskSpace(t); ok {
				it.space, it.index = s, i
			}
			out = append(out, it)
		}
	}
	for _, m := range a.untuk.meetings {
		over := m.StartsAt.Add(2 * time.Hour)
		if m.EndsAt != nil {
			over = *m.EndsAt
		}
		if m.Cancelled || !now.Before(over) || !m.StartsAt.Before(end) {
			continue
		}
		it := untukItem{kind: untukMeeting, id: m.ID, title: m.Title, at: m.StartsAt, timed: true, index: -1, action: "Buka"}
		parts := []string{}
		if m.Location != "" {
			parts = append(parts, m.Location)
		}
		parts = append(parts, fmt.Sprintf("%d peserta", len(m.Participants)))
		it.sub = strings.Join(parts, " · ")
		for i, s := range a.access.spaces {
			if s.ID == m.SpaceID {
				it.space, it.index = s, i
			}
		}
		out = append(out, it)
	}
	if a.reminders.loaded {
		for _, r := range a.reminders.items {
			if r.Completed || (r.DueAt == nil && r.DueOn == nil) {
				continue
			}
			it := untukItem{kind: untukReminder, id: r.ID, title: r.Title, index: -1, action: "Buka", sub: "pengingat"}
			if r.DueAt != nil {
				it.at, it.timed = *r.DueAt, true
				it.late = r.DueAt.Before(now)
			} else {
				it.at = r.DueOn.At(23, 59, loc)
				it.late = it.at.Before(now)
			}
			if !it.late && !it.at.Before(end) {
				continue
			}
			if r.Relation == api.ReminderOwner && r.OwnerID == a.user.ID {
				it.action, it.sub = "Selesai", "pengingat pribadi"
			}
			if it.late {
				it.sub = "lewat waktu"
			}
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// untukDay is a heading of the timeline and its rows.
type untukDay struct {
	key, label, note string
	late, today      bool
	items            []untukItem
}

var untukWeekdays = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
var untukMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func untukDate(t time.Time) string {
	return fmt.Sprintf("%s, %d %s", untukWeekdays[t.Weekday()], t.Day(), untukMonths[t.Month()-1])
}

// untukDays groups the rows: the late ones first, then by day.
func untukDays(items []untukItem, now time.Time, loc *time.Location) []untukDay {
	today := startOfDay(now, loc)
	var late untukDay
	late = untukDay{key: "late", label: "Lewat tenggat", late: true}
	byDay := map[int]*untukDay{}
	var order []int
	for _, it := range items {
		if it.late {
			late.items = append(late.items, it)
			continue
		}
		n := int(startOfDay(it.at, loc).Sub(today).Hours() / 24)
		if n < 0 {
			n = 0 // earlier today, still on
		}
		d, ok := byDay[n]
		if !ok {
			day := today.AddDate(0, 0, n)
			d = &untukDay{key: day.Format("2006-01-02"), note: untukDate(day)}
			switch n {
			case 0:
				d.label, d.today = "Hari ini", true
			case 1:
				d.label = "Besok"
			default:
				d.label, d.note = untukDate(day), ""
			}
			byDay[n] = d
			order = append(order, n)
		}
		d.items = append(d.items, it)
	}
	var out []untukDay
	if len(late.items) > 0 {
		late.note = fmt.Sprintf("%d hal", len(late.items))
		out = append(out, late)
	}
	slices.Sort(order)
	for _, n := range order {
		out = append(out, *byDay[n])
	}
	return out
}

// untukFiltered keeps the rows of the filter chosen.
func untukFiltered(items []untukItem, filter int) []untukItem {
	if filter == 0 {
		return items
	}
	kind := []string{"", untukTask, untukMeeting, untukReminder}[filter]
	return slices.DeleteFunc(slices.Clone(items), func(it untukItem) bool { return it.kind != kind })
}

// untukNext is the next timed row still to come, for "Berikutnya".
func untukNext(items []untukItem, now time.Time) (untukItem, bool) {
	for _, it := range items {
		if it.timed && !it.late && it.at.After(now) {
			return it, true
		}
	}
	return untukItem{}, false
}

// untukCountdown is "dalam 2 j 14 m", "dalam 5 m", "sebentar lagi".
func untukCountdown(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 1:
		return "sebentar lagi"
	case m < 60:
		return fmt.Sprintf("dalam %d m", m)
	case m < 24*60:
		return fmt.Sprintf("dalam %d j %d m", m/60, m%60)
	}
	return fmt.Sprintf("dalam %d hari", m/(24*60))
}

// untukkuView is the page.
func (a *App) untukkuView(c *ui.Context) {
	now := c.Now()
	a.ensureTasks(true)
	a.ensureReminders()
	a.ensureMyMeetings(now)
	t := c.Theme()
	loc := a.homeZone()
	all := a.untukItems(now, loc)
	st := &a.untuk
	items := untukFiltered(all, st.filter)
	phone := isMobile(c)
	ui.Scroll(c.Key("untukku")).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(pageMaxWidth).Padding(phoneSize(phone, 16, 26)).Gap(14).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(12).AlignItems(ui.End).Children(func() {
				ui.Column(c).Grow(1).MinWidth(200).Gap(2).Children(func() {
					ui.Text(c, "Untukku").FontSize(20).Bold()
					muted(c, untukDate(now.In(loc))+" · tugas, rapat, dan pengingatmu dari semua Space").FontSize(13)
				})
				if i := segmented(c, "Tampilkan", []string{"Semua", "Tugas", "Rapat", "Pengingat"}, st.filter); i >= 0 {
					st.filter = i
				}
			})
			if next, ok := untukNext(all, now); ok {
				a.untukNextStrip(c, next, now, loc)
			}
			a.untukSummary(c, all, now, loc)
			// What could not be read, each in its own words.
			for _, err := range []string{a.tasks.err, a.reminders.err} {
				if err != "" {
					muted(c, err).FontSize(12)
				}
			}
			if st.err != "" {
				muted(c, "Rapat belum dapat dimuat: "+st.err).FontSize(12)
			}
			waiting := (!a.tasks.loaded && a.tasks.err == "") || (!a.reminders.loaded && a.reminders.err == "" && !a.isAdmin())
			switch {
			case len(items) == 0 && waiting:
				muted(c, "Memuat…")
			case len(items) == 0:
				a.untukEmpty(c, st.filter)
			default:
				for _, d := range untukDays(items, now, loc) {
					a.untukDayView(c, d, now, loc)
				}
			}
			ui.Row(c.Key("untuk-more")).FillWidth().Wrap().Gap(8).Padding(8, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				if onbLink(c, "Semua tugasku", "Semua tugasku").Clicked() {
					a.navigate(PathJobdesk)
				}
				if onbLink(c, "Semua pengingat", "Semua pengingat").Clicked() {
					a.navigate(PathReminder)
				}
			})
		})
	})
	// The countdown moves by the minute.
	c.After(time.Minute - time.Duration(now.Second())*time.Second)
}

// untukNextStrip is C's "Berikutnya" as a thin strip: what comes next, and
// how soon.
func (a *App) untukNextStrip(c *ui.Context, it untukItem, now time.Time, loc *time.Location) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("untuk-next")).Label("Berikutnya: "+it.title).FillWidth().Row().Gap(10).AlignItems(ui.Center).Padding(8, 12).Radius(10).Background(spaceMix(t.Accent, t.Surface, .9)).Border(1, spaceMix(t.Accent, t.Surface, .65))
	if b.Hovered() {
		b.Background(spaceMix(t.Accent, t.Surface, .84))
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Text(c, "Berikutnya").FontSize(12).FontWeight(600).TextColor(t.Accent).Shrink(0)
		ui.Text(c, untukCountdown(it.at.Sub(now))).Font(FontMono).FontSize(13).FontWeight(600).TextColor(t.Accent).Shrink(0)
		ui.Text(c, it.title).FontSize(13).FontWeight(500).Grow(1).MinWidth(0).MaxLines(1)
		ui.Text(c, it.at.In(loc).Format("15.04")).FontSize(12).TextColor(t.TextMuted).Shrink(0)
		if it.index >= 0 {
			spaceTile(c, it.space, it.index)
		}
	})
	if b.Clicked() {
		a.untukOpen(it)
	}
}

// untukSummary is the week's counts.
func (a *App) untukSummary(c *ui.Context, items []untukItem, now time.Time, loc *time.Location) {
	t := c.Theme()
	today, end := startOfDay(now, loc), startOfDay(now, loc).AddDate(0, 0, 1)
	late, meetings, dues, reminders := 0, 0, 0, 0
	for _, it := range items {
		switch {
		case it.late:
			late++
		case it.kind == untukMeeting && !it.at.Before(today) && it.at.Before(end):
			meetings++
		case it.kind == untukTask:
			dues++
		case it.kind == untukReminder:
			reminders++
		}
	}
	ui.Row(c.Key("untuk-sum")).FillWidth().Wrap().Gap(8).Children(func() {
		chip := func(n int, label string, bad bool) {
			ui.Row(c.Key(label)).Gap(4).AlignItems(ui.Center).Padding(3, 10).Radius(8).Border(1, t.Border).Children(func() {
				num := ui.Text(c, fmt.Sprint(n)).FontSize(12).FontWeight(600).FontFeatures("tnum")
				if bad && n > 0 {
					num.TextColor(t.Danger)
				}
				ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			})
		}
		chip(late, "lewat tenggat", true)
		chip(meetings, "rapat hari ini", false)
		chip(dues, "tenggat minggu ini", false)
		chip(reminders, "pengingat", false)
	})
}

// untukEmpty is a quiet week.
func (a *App) untukEmpty(c *ui.Context, filter int) {
	t := c.Theme()
	ui.Column(c.Key("untuk-empty")).FillWidth().Gap(6).Padding(24).Radius(12).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
		head, body := "Minggu ini lapang", "Tidak ada tenggat, rapat, atau pengingat sampai minggu depan. Tugas dan undangan rapat baru muncul di sini sendiri."
		if filter != 0 {
			head, body = "Tidak ada yang cocok", "Tidak ada "+[]string{"", "tugas", "rapat", "pengingat"}[filter]+" minggu ini."
		}
		ui.Text(c, head).FontSize(15).FontWeight(600)
		muted(c, body).FontSize(13)
		if onbSecondary(c, "Tambah pengingat pribadi", "+ Pengingat pribadi", true).Clicked() {
			a.navigate(PathReminder)
		}
	})
}

// untukDayView is a day's heading and rows: the first untukDayRows, then
// "+N lainnya" until opened.
func (a *App) untukDayView(c *ui.Context, d untukDay, now time.Time, loc *time.Location) {
	t := c.Theme()
	st := &a.untuk
	ui.Column(c.Key("day-" + d.key)).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Padding(0, 0, 6, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			head := ui.Text(c, d.label).FontSize(13).FontWeight(600)
			switch {
			case d.late:
				head.TextColor(t.Danger)
			case d.today:
				head.TextColor(t.Accent)
			}
			if d.note != "" {
				ui.Text(c, d.note).FontSize(12).TextColor(faint(t))
			}
		})
		rows := d.items
		if len(rows) > untukDayRows && !st.expanded[d.key] {
			rows = rows[:untukDayRows]
		}
		for _, it := range rows {
			a.untukRow(c, it, now, loc)
		}
		if more := len(d.items) - len(rows); more > 0 {
			if onbLink(c, fmt.Sprintf("Tampilkan %d lainnya", more), fmt.Sprintf("+%d lainnya", more)).Clicked() {
				if st.expanded == nil {
					st.expanded = map[string]bool{}
				}
				st.expanded[d.key] = true
			}
		}
	})
}

// untukRow is one row: time, kind, title, Space, action. A click on the
// row opens it; the action does what it says.
func (a *App) untukRow(c *ui.Context, it untukItem, now time.Time, loc *time.Location) {
	t := c.Theme()
	when := "seharian"
	switch {
	case it.late:
		when = untukWeekdays[it.at.In(loc).Weekday()][:3]
		if it.timed {
			when += " " + it.at.In(loc).Format("15.04")
		}
	case it.timed:
		when = it.at.In(loc).Format("15.04")
	case it.kind == untukTask:
		when = "23.59"
	}
	icon := map[string]string{untukTask: "task", untukMeeting: "calendar", untukReminder: "bell"}[it.kind]
	row := ui.ButtonBase(c.Key(it.kind+it.id.String())).Label(it.title).FillWidth().Row().Gap(10).AlignItems(ui.Center).Height(40).Padding(0, 8).Radius(8)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.FocusRing(row.FocusVisible()).Children(func() {
		ui.Text(c, when).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Width(64).Shrink(0).MaxLines(1)
		ui.Icon(c, spaceIcon(icon)).Size(15, 15).TextColor(t.TextMuted).Shrink(0)
		ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, it.title).FontSize(13).MaxLines(1).MinWidth(0)
			sub := ui.Text(c, it.sub).FontSize(12).TextColor(faint(t)).MaxLines(1).Shrink(0)
			if it.late {
				sub.TextColor(t.Danger)
			}
		})
		if it.index >= 0 {
			spaceTile(c, it.space, it.index)
		}
		act := ui.ButtonBase(c.Key("act")).Label(it.action+" "+it.title).Height(28).Padding(0, 10).Radius(7).Border(1, t.Border).Center().Shrink(0)
		if act.Hovered() {
			act.Background(t.Surface)
		}
		act.FocusRing(act.FocusVisible()).Children(func() { ui.Text(c, it.action).FontSize(12) })
		if act.Clicked() {
			if it.action == "Selesai" {
				a.untukCompleteReminder(it.id)
			} else {
				a.untukOpen(it)
			}
		}
	})
	if row.Clicked() {
		a.untukOpen(it)
	}
}

// untukOpen opens a row: a task's details here, a meeting in its Space's
// Rapat, a reminder in Reminder pribadi.
func (a *App) untukOpen(it untukItem) {
	switch it.kind {
	case untukTask:
		a.workspace.Open(it.id)
	case untukMeeting:
		a.OpenNotification(api.Delivery{SourceKind: "meeting", SourceID: it.id})
	case untukReminder:
		a.navigate(PathReminder)
		a.openReminder(it.id)
	}
}

// untukCompleteReminder marks one of the user's reminders done.
func (a *App) untukCompleteReminder(rid id.ID) {
	r := a.reminderByID(rid)
	if r == nil || r.Relation != api.ReminderOwner || r.OwnerID != a.user.ID {
		return
	}
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return s.CompleteReminder(context.Background(), rid, api.ReminderCompleteRequest{Completed: true})
	}, a.replaceReminder)
}
