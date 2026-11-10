package appui

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Beranda of a Space, as canvas A draws it: the date and week of the
// period, four KPIs with sparklines over the chosen range, a quick ping,
// the members' workload, deadlines of the next 14 days, submissions of the
// last four weeks and recent activity. Everything is computed from the
// tasks, meetings and members the app has already loaded.

// homeUIState holds the Beranda's choices: the range of the KPIs and the
// quick ping's recipients and message.
type homeUIState struct {
	rng     homeRange
	pingTo  int // 0 late assignees, 1 everyone, 2 one person
	person  id.ID
	message int
}

type homeRange int

const (
	homeWeek homeRange = iota
	homeMonth
	homePeriod
)

var homeRanges = []string{"Minggu ini", "Bulan ini", "Periode"}

// homeQuickMessages are the quick ping's prepared messages.
var homeQuickMessages = []string{"Mohon perbarui progres tugas hari ini.", "Cek grup divisi ya, ada info penting.", "Tugas yang lewat tenggat tolong dikirim hari ini."}

var longDays = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

// canvas status colors: doing is the canvas' blue, the others follow the
// theme's warning, success and danger.
var homeDoing = ui.Hex("#9fb8d6")

// homeZone is the period's time zone, else UTC.
func (a *App) homeZone() *time.Location {
	if p := a.period(); p != nil {
		if loc, _ := timeutil.LoadZone(p.TimeZone); loc != nil {
			return loc
		}
	}
	loc, _ := timeutil.LoadZone(timeutil.DefaultZoneName)
	if loc == nil {
		return time.UTC
	}
	return loc
}

// periodWeek is the week of the period now is in, from 1; 0 outside it.
func periodWeek(p *api.Period, now time.Time, loc *time.Location) int {
	if p == nil || p.StartsOn.IsZero() {
		return 0
	}
	start := p.StartsOn.At(0, 0, loc)
	if now.Before(start) {
		return 0
	}
	if !p.EndsOn.IsZero() && now.After(p.EndsOn.At(23, 59, loc)) {
		return 0
	}
	n := now.In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	first := time.Date(p.StartsOn.Year, p.StartsOn.Month, p.StartsOn.Day, 0, 0, 0, 0, time.UTC)
	return int(today.Sub(first).Hours()/24)/7 + 1
}

// homeDateLine is "Jumat, 6 November 2026 · minggu ke-10 periode".
func homeDateLine(p *api.Period, now time.Time, loc *time.Location) string {
	n := now.In(loc)
	s := fmt.Sprintf("%s, %d %s %d", longDays[n.Weekday()], n.Day(), monthName(n.Month()), n.Year())
	if w := periodWeek(p, now, loc); w > 0 {
		s += fmt.Sprintf(" · minggu ke-%d periode", w)
	}
	return s
}

// rangeStart is when the chosen range begins.
func (a *App) rangeStart(r homeRange, now time.Time) time.Time {
	switch r {
	case homeMonth:
		return now.AddDate(0, 0, -30)
	case homePeriod:
		if p := a.period(); p != nil && !p.StartsOn.IsZero() {
			if s := p.StartsOn.At(0, 0, a.homeZone()); s.Before(now) {
				return s
			}
		}
		return now.AddDate(0, 0, -90)
	}
	return now.AddDate(0, 0, -7)
}

func rangePhrase(r homeRange) string {
	return [...]string{"minggu ini", "bulan ini", "periode ini"}[r]
}

// submission is how a task's evidence was handed in: on time, late, or
// with evidence the reviewer found insufficient. It counts from when the
// evidence was sent, so forgetting to mark a task done is not lateness.
type submission int

const (
	subOnTime submission = iota
	subRejected
	subLate
)

// taskSubmission reports a task's first hand-in, at, and its outcome.
func taskSubmission(t api.Task) (time.Time, submission, bool) {
	if len(t.Evidence) == 0 || t.Progress == api.TaskCancelled {
		return time.Time{}, 0, false
	}
	first, last := t.Evidence[0], t.Evidence[0]
	for _, e := range t.Evidence[1:] {
		if e.SubmittedAt.Before(first.SubmittedAt) {
			first = e
		}
		if !e.SubmittedAt.Before(last.SubmittedAt) {
			last = e
		}
	}
	switch {
	case last.MatchStatus == api.MatchRejected:
		return first.SubmittedAt, subRejected, true
	case (t.DeadlineAt != nil || t.DeadlineDate != nil) && first.SubmittedAt.After(taskDue(t)):
		return first.SubmittedAt, subLate, true
	}
	return first.SubmittedAt, subOnTime, true
}

// submissionCounts counts hand-ins from from to to by outcome.
func submissionCounts(tasks []api.Task, from, to time.Time) (counts [3]int, total int) {
	for _, t := range tasks {
		at, s, ok := taskSubmission(t)
		if ok && !at.Before(from) && !at.After(to) {
			counts[s]++
			total++
		}
	}
	return counts, total
}

// finishedAt is when a done or cancelled task left the open work, as far as
// the task list tells: its last update.
func finishedAt(t api.Task) (time.Time, bool) {
	if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
		return t.UpdatedAt, true
	}
	return time.Time{}, false
}

// homeStats are the Beranda's numbers for one range.
type homeStats struct {
	active, late, review, fresh int
	lateSince                   time.Time
	subs                        [3]int
	subTotal                    int
	// series are the KPIs' sparklines, oldest first.
	series [4][]float32
}

func computeHomeStats(tasks []api.Task, from, now time.Time) homeStats {
	var s homeStats
	for _, t := range tasks {
		if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
			continue
		}
		s.active++
		if !t.CreatedAt.Before(from) {
			s.fresh++
		}
		if t.Progress == api.TaskSubmitted || t.MatchStatus == api.MatchPending {
			s.review++
		}
		if taskLate(t, now) {
			s.late++
			if due := taskDue(t); s.lateSince.IsZero() || due.Before(s.lateSince) {
				s.lateSince = due
			}
		}
	}
	s.subs, s.subTotal = submissionCounts(tasks, from, now)
	const points = 8
	for i := range points {
		at := from.Add(time.Duration(float64(now.Sub(from)) * float64(i) / float64(points-1)))
		var active, late, review float32
		for _, t := range tasks {
			if t.CreatedAt.After(at) {
				continue
			}
			if end, ok := finishedAt(t); ok && !end.After(at) {
				continue
			}
			active++
			sent, _, handed := taskSubmission(t)
			handed = handed && !sent.After(at)
			if !handed && (t.DeadlineAt != nil || t.DeadlineDate != nil) && taskDue(t).Before(at) {
				late++
			}
			if handed && pendingAt(t, at) {
				review++
			}
		}
		counts, total := submissionCounts(tasks, from, at)
		rate := float32(math.NaN())
		if total > 0 {
			rate = float32(counts[subOnTime]) / float32(total)
		}
		s.series[0] = append(s.series[0], active)
		s.series[1] = append(s.series[1], late)
		s.series[2] = append(s.series[2], review)
		s.series[3] = append(s.series[3], rate)
	}
	return s
}

// pendingAt reports whether a task's latest evidence by at was still
// waiting for review then.
func pendingAt(t api.Task, at time.Time) bool {
	var latest *api.TaskEvidence
	for i := range t.Evidence {
		e := &t.Evidence[i]
		if !e.SubmittedAt.After(at) && (latest == nil || !e.SubmittedAt.Before(latest.SubmittedAt)) {
			latest = e
		}
	}
	return latest != nil && (latest.ReviewedAt == nil || latest.ReviewedAt.After(at))
}

func percent(n, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(n) * 100 / float64(total)))
}

func (a *App) overviewPage(c *ui.Context, title string, shared bool) {
	a.ensureTasks(false)
	a.ensureMeetings()
	t := c.Theme()
	phone := isMobile(c)
	ui.Scroll(c.Key("beranda")).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(28).Gap(16)
		if phone {
			body.Padding(16).Gap(12)
		}
		wide := !phone && body.Bounds().W >= 900
		body.Children(func() {
			now := c.Now()
			loc := a.homeZone()
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			if !phone {
				head.Row().AlignItems(ui.Center)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					h := ui.Text(c, "Beranda "+title).FontSize(20).Bold()
					if phone {
						h.FontSize(18)
					}
					muted(c, homeDateLine(a.period(), now, loc)).FontSize(13)
				})
				if i := segmented(c, "Rentang beranda", homeRanges, int(a.home.rng)); i >= 0 {
					a.home.rng = homeRange(i)
				}
			})
			a.onboardingBeranda(c, phone)
			if !a.tasks.loaded {
				detail := "Mengambil pekerjaan dari server."
				if a.tasks.err != "" {
					detail = a.tasks.err
				}
				pageUnavailable(c, "task", "Memuat beranda…", detail)
				return
			}
			stats := computeHomeStats(a.tasks.items, a.rangeStart(a.home.rng, now), now)
			a.homeKPIs(c, stats, phone)
			ping := canPing(a.selectedSpace())
			homeRow(c, "row-ping", wide, func() {
				if ping {
					homePanel(c, "Ping cepat", a.pingChannels(), 5, wide, func() { a.quickPing(c, now) })
				}
				homePanel(c, "Beban kerja anggota", "Tugas aktif per PJ", 7, wide, func() { a.homeWorkload(c, now) })
			})
			if phone {
				// The phone's Beranda stops at the workload, as canvas A's.
				return
			}
			counts, total := submissionCounts(a.tasks.items, now.AddDate(0, 0, -28), now)
			homeRow(c, "row-due", wide, func() {
				homePanel(c, "Tenggat 14 hari ke depan", "Tiap blok satu tugas, warna mengikuti label", 7, wide, func() { a.homeDeadlines(c, now, loc) })
				if total > 0 {
					homePanel(c, "Penyerahan 4 minggu", "Dari bukti yang dikirim", 5, wide, func() { homeDonut(c, counts, total) })
				}
			})
			homeRow(c, "row-feed", wide, func() {
				homePanel(c, "Agenda", "", 5, wide, func() {
					a.homeAgenda(c)
					if spaceButton(c, "Lihat kalender", "calendar", false).Clicked() {
						a.chrome.tab = "Agenda"
					}
				})
				if items := a.homeActivity(now); len(items) > 0 {
					homePanel(c, "Aktivitas terbaru", "Tugas di Space ini", 7, wide, func() { homeFeed(c, items, now, loc) })
				}
			})
			if shared {
				pageNotice(c, "link", "Ruang kolaborasi", "Akses ruang bersama tidak membuka pekerjaan internal divisi lain.")
			}
			_ = t
		})
	})
}

// homeRow lays panels side by side when wide, else stacked.
func homeRow(c *ui.Context, key string, wide bool, fn func()) {
	row := ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(16)
	if wide {
		row.Row().AlignItems(ui.Stretch)
	}
	row.Children(fn)
}

// homePanel is a canvas panel with a heading and, at the right, a note.
// span is its share of a wide row, out of 12.
func homePanel(c *ui.Context, title, note string, span float32, wide bool, fn func()) {
	t := c.Theme()
	p := ui.Column(c.Key(title)).MinWidth(0).Padding(16).Gap(12).Radius(12).Background(t.Surface).Border(1, t.Border)
	if wide {
		p.Basis(0).Grow(span)
	} else {
		p.FillWidth()
	}
	p.Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(15).Bold().MinWidth(0).MaxLines(1)
			ui.Box(c).Grow(1)
			if note != "" {
				muted(c, note).FontSize(12).MaxLines(1).MinWidth(0)
			}
		})
		fn()
	})
}

func (a *App) homeKPIs(c *ui.Context, s homeStats, phone bool) {
	t := c.Theme()
	// Canvas A · Tablet sets the four KPIs 2×2: four across a tablet cut
	// their details short.
	cols := 4
	if phone || viewportMode(c) == layoutTablet {
		cols = 2
	}
	rate, rateDetail := "—", "Belum ada penyerahan "+rangePhrase(a.home.rng)
	if s.subTotal > 0 {
		rate = fmt.Sprintf("%d%%", percent(s.subs[subOnTime], s.subTotal))
		rateDetail = fmt.Sprintf("%d dari %d penyerahan · %s", s.subs[subOnTime], s.subTotal, rangePhrase(a.home.rng))
	}
	lateDetail := "Tidak ada yang lewat tenggat"
	if s.late > 0 {
		lateDetail = "Sejak " + shortClock(s.lateSince, a.homeZone()) + " · pengingat harian aktif"
	}
	ui.Grid(c.Key("home-kpis")).FillWidth().Columns(cols).Gap(16).Children(func() {
		for i, k := range []struct {
			label, value, detail string
			color                ui.Color
		}{
			{"Tugas aktif", fmt.Sprint(s.active), fmt.Sprintf("%d baru %s", s.fresh, rangePhrase(a.home.rng)), homeDoing},
			{"Lewat tenggat", fmt.Sprint(s.late), lateDetail, t.Danger},
			{"Perlu tinjauan", fmt.Sprint(s.review), "Bukti menunggu dicocokkan", t.Warning},
			{"Tepat waktu", rate, rateDetail, t.Success},
		} {
			ui.Column(c.Key(k.label)).MinWidth(0).Padding(16).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(k.color)
					muted(c, k.label).FontSize(12)
				})
				v := ui.Text(c, k.value).FontSize(28).FontWeight(600)
				if i == 1 && s.late > 0 {
					v.TextColor(t.Danger)
				}
				if phone {
					v.FontSize(26)
					return
				}
				ui.Text(c, k.detail).FontSize(12).TextColor(faint(t)).MaxLines(1)
				sparkline(c, s.series[i], k.color)
			})
		}
	})
}

// sparkline draws a series as a line 34 DIPs high; gaps (NaN) take the
// previous value.
func sparkline(c *ui.Context, values []float32, color ui.Color) {
	pts := slices.Clone(values)
	lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
	for i, v := range pts {
		if v != v { // NaN
			if i > 0 {
				pts[i] = pts[i-1]
			} else {
				pts[i] = 0
			}
		}
		lo, hi = min(lo, pts[i]), max(hi, pts[i])
	}
	ui.Box(c).FillWidth().Height(34).Margin(4, 0, 0, 0).Draw(func(p *ui.Painter, r ui.Rect) {
		if len(pts) < 2 || r.W <= 0 {
			return
		}
		line := make([][2]float32, len(pts))
		for i, v := range pts {
			y := float32(.5)
			if hi > lo {
				y = (v - lo) / (hi - lo)
			}
			line[i] = [2]float32{r.X + r.W*float32(i)/float32(len(pts)-1), r.Y + 4 + (r.H-8)*(1-y)}
		}
		p.StrokePolyline(line, 2, color)
	})
}

// shortClock is "5 Nov".
func shortClock(at time.Time, loc *time.Location) string {
	at = at.In(loc)
	return fmt.Sprintf("%d %s", at.Day(), shortMonth(int(at.Month())))
}

func (a *App) pingChannels() string {
	st := &a.notificationSettingsState
	if sp := a.selectedSpace(); sp == nil || st.space != sp.ID || st.period != sp.PeriodID {
		return "Perangkat"
	}
	switch {
	case st.device && st.whatsapp:
		return "Perangkat + WhatsApp"
	case st.whatsapp:
		return "WhatsApp"
	}
	return "Perangkat"
}

// quickPing is canvas A's Ping cepat: whom, a prepared message, send. It
// fills the shared ping composer's state and sends through it; Opsi lain
// opens the full composer.
func (a *App) quickPing(c *ui.Context, now time.Time) {
	t := c.Theme()
	sp := a.selectedSpace()
	st := &a.notificationSettingsState
	if st.period != sp.PeriodID || st.space != sp.ID {
		*st = notificationSettingsState{period: sp.PeriodID, space: sp.ID, division: st.division, device: true, pingKind: "general"}
	}
	var late []id.ID
	for _, task := range a.tasks.items {
		if taskLate(task, now) && task.Assignee.ID != id.Nil && !slices.Contains(late, task.Assignee.ID) {
			late = append(late, task.Assignee.ID)
		}
	}
	people := a.homePeople()
	if a.home.pingTo == 0 && len(late) == 0 {
		a.home.pingTo = 1
	}
	ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
		chip := func(key, label string, on bool, person string) bool {
			b := ui.ButtonBase(c.Key(key)).Label("Ping ke "+label).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 10).Radius(14).Border(1, t.Border).TextColor(t.TextMuted)
			if person != "" {
				b.Padding(0, 10, 0, 4)
			}
			if on {
				b.Background(accentSoft(t)).BorderColor(t.Accent).TextColor(t.Text)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				if person != "" {
					avatar(c, person, 20)
				}
				ui.Text(c, label).FontSize(12).MaxLines(1)
			})
			return b.Clicked()
		}
		if len(late) > 0 && chip("late", "Yang lewat tenggat", a.home.pingTo == 0, "") {
			a.home.pingTo = 0
		}
		if chip("all", "Semua", a.home.pingTo == 1, "") {
			a.home.pingTo = 1
		}
		for _, p := range people {
			if p.ID == a.user.ID {
				continue
			}
			name := firstName(p.DisplayName)
			if chip(p.ID.String(), name, a.home.pingTo == 2 && a.home.person == p.ID, p.DisplayName) {
				a.home.pingTo, a.home.person = 2, p.ID
			}
		}
	})
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		for i, m := range homeQuickMessages {
			b := ui.ButtonBase(c.Key(i)).Label(m).FillWidth().Padding(8, 10).Radius(9).Border(1, t.Border).TextColor(t.TextMuted)
			if i == a.home.message {
				b.Background(accentSoft(t)).BorderColor(t.Accent).TextColor(t.Text)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, m).FontSize(13).MinWidth(0) })
			if b.Clicked() {
				a.home.message = i
			}
		}
	})
	var to []id.ID
	switch a.home.pingTo {
	case 0:
		to = late
	case 1:
		for _, p := range people {
			to = append(to, p.ID)
		}
	case 2:
		to = []id.ID{a.home.person}
	}
	to = slices.DeleteFunc(to, func(x id.ID) bool { return x == id.Nil })
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		if spaceButton(c, "Opsi lain", "", false).Clicked() {
			a.chrome.pingOpen = true
		}
		ui.Box(c).Grow(1)
		if primarySpaceButton(c, "Kirim ping", "send").Disabled(st.busy || a.conn != connOnline || len(to) == 0).Clicked() {
			st.pingKind, st.pingTarget = "general", id.Nil
			st.pingMessage = homeQuickMessages[a.home.message]
			st.recipients = to
			a.sendManualPing()
		}
	})
	if st.err != "" {
		ui.Text(c, st.err).FontSize(11).TextColor(faint(t))
	}
}

func firstName(name string) string {
	f := strings.Fields(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "[DUMMY]")))
	if len(f) == 0 {
		return name
	}
	return f[0]
}

// homePeople are the Space's members, when the user may read them, then
// anyone else who leads a task here.
func (a *App) homePeople() []api.PersonRef {
	people := slices.Clone(a.meetings.members)
	for _, t := range a.tasks.items {
		if t.Assignee.ID != id.Nil && !slices.ContainsFunc(people, func(p api.PersonRef) bool { return p.ID == t.Assignee.ID }) {
			people = append(people, t.Assignee)
		}
	}
	return people
}

// homeWorkload is each person's open tasks: not started, doing, submitted
// and late, as one stacked bar scaled to the busiest.
func (a *App) homeWorkload(c *ui.Context, now time.Time) {
	t := c.Theme()
	type load struct{ lanes [4]int }
	loads := map[id.ID]*load{}
	for _, task := range a.tasks.items {
		if task.Progress == api.TaskDone || task.Progress == api.TaskCancelled {
			continue
		}
		l := loads[task.Assignee.ID]
		if l == nil {
			l = &load{}
			loads[task.Assignee.ID] = l
		}
		switch {
		case taskLate(task, now):
			l.lanes[3]++
		case task.Progress == api.TaskSubmitted:
			l.lanes[2]++
		case task.Progress == api.TaskDoing:
			l.lanes[1]++
		default:
			l.lanes[0]++
		}
	}
	people := a.homePeople()
	if len(people) == 0 {
		muted(c, "Belum ada tugas aktif.")
		return
	}
	total := func(p api.PersonRef) int {
		n := 0
		if l := loads[p.ID]; l != nil {
			for _, v := range l.lanes {
				n += v
			}
		}
		return n
	}
	most := 1
	for _, p := range people {
		most = max(most, total(p))
	}
	sort.SliceStable(people, func(i, j int) bool { return total(people[i]) > total(people[j]) })
	colors := [4]ui.Color{faint(t), homeDoing, t.Warning, t.Danger}
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		for _, p := range people {
			n := total(p)
			ui.Row(c.Key(p.ID)).FillWidth().MinWidth(0).Gap(12).Padding(6, 0).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Width(130).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
					avatar(c, p.DisplayName, 20)
					ui.Text(c, firstName(p.DisplayName)).FontSize(13).MinWidth(0).MaxLines(1)
				})
				ui.Row(c).Grow(1).MinWidth(0).Height(12).Radius(6).Background(t.SurfaceHover).Clip().Children(func() {
					if l := loads[p.ID]; l != nil {
						for i, v := range l.lanes {
							if v > 0 {
								ui.Box(c.Key(i)).WidthPercent(100 * float32(v) / float32(most)).FillHeight().Background(colors[i])
							}
						}
					}
				})
				ui.Text(c, fmt.Sprintf("%d tugas", n)).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Width(60).Shrink(0)
			})
		}
	})
	ui.Row(c).Wrap().Gap(14).Children(func() {
		for i, label := range []string{"Belum mulai", "Dikerjakan", "Diserahkan", "Lewat tenggat"} {
			ui.Row(c.Key(label)).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Background(colors[i])
				muted(c, label).FontSize(11)
			})
		}
	})
}

// dueDay is the day a task is due, in loc.
func dueDay(t api.Task, loc *time.Location) (time.Time, bool) {
	var at time.Time
	switch {
	case t.DeadlineDate != nil:
		return time.Date(t.DeadlineDate.Year, t.DeadlineDate.Month, t.DeadlineDate.Day, 0, 0, 0, 0, loc), true
	case t.DeadlineAt != nil:
		at = t.DeadlineAt.In(loc)
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, loc), true
	}
	return at, false
}

// deadlineDays are the open tasks due on each of the 14 days from today.
func deadlineDays(tasks []api.Task, now time.Time, loc *time.Location) [14][]api.Task {
	var days [14][]api.Task
	n := now.In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	for _, t := range tasks {
		if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
			continue
		}
		day, ok := dueDay(t, loc)
		if !ok {
			continue
		}
		i := int(math.Round(day.Sub(today).Hours() / 24))
		if i >= 0 && i < 14 {
			days[i] = append(days[i], t)
		}
	}
	return days
}

func (a *App) homeDeadlines(c *ui.Context, now time.Time, loc *time.Location) {
	t := c.Theme()
	days := deadlineDays(a.tasks.items, now, loc)
	n := now.In(loc)
	ui.Row(c.Key("strip")).FillWidth().MinWidth(0).Gap(4).AlignItems(ui.End).Children(func() {
		for i, tasks := range days {
			day := n.AddDate(0, 0, i)
			ui.Column(c.Key(i)).Basis(0).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Column(c).FillWidth().Height(96).Gap(3).Justify(ui.End).Children(func() {
					shown := tasks
					if len(shown) > 5 {
						shown = shown[:4]
					}
					for _, task := range shown {
						color := homeDoing
						if task.Category != "" {
							color = tagColor(task.Category)
						}
						b := ui.ButtonBase(c.Key(task.ID)).Label("Buka " + task.Title).Tooltip(task.Title).FillWidth().Height(16).Radius(4).Background(color)
						b.FocusRing(b.FocusVisible())
						if b.Clicked() {
							a.chrome.tab = "Board tugas"
							a.workspace.Open(task.ID)
						}
					}
					if len(tasks) > len(shown) {
						ui.Text(c, fmt.Sprintf("+%d", len(tasks)-len(shown))).FontSize(10).TextColor(t.TextMuted).AlignSelf(ui.Center)
					}
				})
				lab := ui.Column(c).FillWidth().Gap(0).Margin(6, 0, 0, 0).AlignItems(ui.Center).Children(func() {
					d := ui.Text(c, shortDays[day.Weekday()]).FontSize(10).TextColor(faint(t)).MaxLines(1)
					m := ui.Text(c, fmt.Sprint(day.Day())).Font(FontMono).FontSize(10).TextColor(faint(t))
					if i == 0 {
						d.TextColor(t.Accent).FontWeight(600)
						m.TextColor(t.Accent).FontWeight(600)
					}
				})
				_ = lab
			})
		}
	})
}

// homeDonut is the share of hand-ins on time, insufficient and late.
func homeDonut(c *ui.Context, counts [3]int, total int) {
	t := c.Theme()
	colors := [3]ui.Color{t.Success, t.Warning, t.Danger}
	ui.Row(c).FillWidth().Wrap().Gap(18).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(132, 132).Shrink(0).Center().Draw(func(p *ui.Painter, r ui.Rect) {
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			rad := min(r.W, r.H)/2 - 9
			ring := new(ui.Path).Circle(cx, cy, rad)
			p.StrokePath(ring, 16, t.SurfaceHover)
			start := float32(0)
			for i, n := range counts {
				if n == 0 {
					continue
				}
				end := start + float32(n)/float32(total)
				arc := new(ui.Path).Cap(ui.ButtCap)
				const steps = 64
				for s := 0; s <= steps; s++ {
					f := start + (end-start)*float32(s)/steps
					ang := float64(f)*2*math.Pi - math.Pi/2
					x, y := cx+rad*float32(math.Cos(ang)), cy+rad*float32(math.Sin(ang))
					if s == 0 {
						arc.MoveTo(x, y)
					} else {
						arc.LineTo(x, y)
					}
				}
				p.StrokePath(arc, 16, colors[i])
				start = end
			}
		}).Children(func() {
			ui.Column(c).AlignItems(ui.Center).Gap(0).Children(func() {
				ui.Text(c, fmt.Sprintf("%d%%", percent(counts[subOnTime], total))).FontSize(22).FontWeight(600)
				muted(c, "tepat waktu").FontSize(10)
			})
		})
		ui.Column(c).MinWidth(0).Gap(8).Children(func() {
			for i, label := range []string{"tepat waktu", "bukti belum cukup", "terlambat"} {
				ui.Row(c.Key(label)).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(colors[i])
					ui.Text(c, fmt.Sprintf("%d %s", counts[i], label)).FontSize(13)
				})
			}
			ui.Text(c, "Dihitung dari waktu bukti dikirim; lupa klik selesai tidak dihitung terlambat.").FontSize(11).TextColor(faint(t)).MaxWidth(200)
		})
	})
}

// homeItem is one line of recent activity.
type homeItem struct {
	at          time.Time
	icon        string
	who, action string
	task        string
	detail      string
}

// homeActivity is what happened to this Space's tasks lately, from the
// task list itself: tasks made, evidence sent and reviewed, comments.
func (a *App) homeActivity(now time.Time) []homeItem {
	var items []homeItem
	for _, t := range a.tasks.items {
		if !t.CreatedAt.IsZero() {
			items = append(items, homeItem{at: t.CreatedAt, icon: "plus", who: t.CreatedBy.DisplayName, action: "membuat tugas", task: t.Title, detail: "PJ " + t.Assignee.DisplayName})
		}
		for _, e := range t.Evidence {
			items = append(items, homeItem{at: e.SubmittedAt, icon: "send", who: e.SubmittedBy.DisplayName, action: "mengirim hasil", task: t.Title, detail: strings.ToLower(matchLabel(e.MatchStatus))})
			if e.ReviewedAt != nil && e.ReviewedBy != nil {
				verb := "mencocokkan bukti"
				if e.MatchStatus == api.MatchRejected {
					verb = "menandai bukti tidak cocok untuk"
				}
				items = append(items, homeItem{at: *e.ReviewedAt, icon: "check", who: e.ReviewedBy.DisplayName, action: verb, task: t.Title})
			}
		}
		for _, cm := range t.Comments {
			items = append(items, homeItem{at: cm.CreatedAt, icon: "message", who: cm.Author.DisplayName, action: "berkomentar di", task: t.Title})
		}
	}
	items = slices.DeleteFunc(items, func(i homeItem) bool { return i.at.After(now) || i.at.Before(now.AddDate(0, 0, -30)) })
	sort.SliceStable(items, func(i, j int) bool { return items[i].at.After(items[j].at) })
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

// homeWhen is "Hari ini 00.29", "Kemarin 00.16" or "5 Nov 15.10".
func homeWhen(at, now time.Time, loc *time.Location) string {
	at, now = at.In(loc), now.In(loc)
	clock := at.Format("15.04")
	y1, m1, d1 := at.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return "Hari ini " + clock
	}
	if y, m, d := now.AddDate(0, 0, -1).Date(); y1 == y && m1 == m && d1 == d {
		return "Kemarin " + clock
	}
	return fmt.Sprintf("%d %s %s", at.Day(), shortMonth(int(at.Month())), clock)
}

func homeFeed(c *ui.Context, items []homeItem, now time.Time, loc *time.Location) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		for i, it := range items {
			row := ui.Row(c.Key(i)).FillWidth().MinWidth(0).Gap(10).Padding(9, 0).AlignItems(ui.Start)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			}
			row.Children(func() {
				ui.Box(c).Size(26, 26).Shrink(0).Radius(8).Center().Background(t.SurfaceHover).Children(func() {
					ui.Icon(c, spaceIcon(it.icon)).Size(14, 14).TextColor(t.TextMuted)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.RichText(c, ui.Span{Text: it.who, Weight: 600}, ui.Span{Text: " " + it.action + " "}, ui.Span{Text: it.task, Weight: 600}).FontSize(13).MinWidth(0)
					when := homeWhen(it.at, now, loc)
					if it.detail != "" {
						when += " · " + it.detail
					}
					ui.Text(c, when).FontSize(11).TextColor(faint(t)).MaxLines(1)
				})
			})
		}
	})
}
