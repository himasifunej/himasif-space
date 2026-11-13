package appui

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Jobdesk saya, "Semua tugasku" of Untukku (canvas 3747dbbac0d0, design
// 6): the tasks the user is PJ of, from every Space, grouped by progress,
// with a Space filter; a row opens Detail tugas. The board stays on the
// Space's "Board tugas" tab.

// jobdeskUIState is the page's Space filter; id.Nil is "Semua Space".
type jobdeskUIState struct {
	space id.ID
}

// jobdeskGroup is one heading of the list: Dikerjakan, Belum mulai,
// Diserahkan (waiting for review), Selesai.
type jobdeskGroup struct {
	label    string
	progress string
}

var jobdeskGroupsOrder = []jobdeskGroup{
	{"Dikerjakan", api.TaskDoing},
	{"Belum mulai", api.TaskTodo},
	{"Diserahkan", api.TaskSubmitted},
	{"Selesai", api.TaskDone},
}

// jobdeskGroups sorts the tasks (of one Space, unless space is id.Nil)
// into the groups, each by deadline, those without one last. Cancelled
// tasks leave the list.
func jobdeskGroups(tasks []api.Task, space id.ID) [][]api.Task {
	out := make([][]api.Task, len(jobdeskGroupsOrder))
	for _, t := range tasks {
		if space != id.Nil && !slices.Contains(t.SpaceIDs, space) {
			continue
		}
		for i, g := range jobdeskGroupsOrder {
			if t.Progress == g.progress {
				out[i] = append(out[i], t)
			}
		}
	}
	for i := range out {
		slices.SortStableFunc(out[i], func(x, y api.Task) int {
			xn, yn := x.DeadlineAt == nil && x.DeadlineDate == nil, y.DeadlineAt == nil && y.DeadlineDate == nil
			switch {
			case xn && !yn:
				return 1
			case yn && !xn:
				return -1
			}
			return taskDue(x).Compare(taskDue(y))
		})
	}
	return out
}

// taskSpace is the Space a task is shown under: a shared Space when it is in
// one (a Medhum task is Medhum's), else its first readable Space. index is
// the Space's place in the sidebar, which picks its tile color.
func (a *App) taskSpace(t api.Task) (api.Space, int, bool) {
	found, at := api.Space{}, -1
	for i, s := range a.access.spaces {
		if !slices.Contains(t.SpaceIDs, s.ID) {
			continue
		}
		if at < 0 || (s.Kind == api.SpaceShared && found.Kind != api.SpaceShared) {
			found, at = s, i
		}
	}
	return found, at, at >= 0
}

// jobdeskDue is a deadline as the list writes it: "Sel, 10 Nov · 23.59",
// or "tanpa tenggat".
func jobdeskDue(t api.Task) string {
	loc, _ := timeutil.LoadZone(t.TimeZone)
	if loc == nil {
		loc = wib()
	}
	var at time.Time
	switch {
	case t.DeadlineAt != nil:
		at = t.DeadlineAt.In(loc)
	case t.DeadlineDate != nil:
		at = t.DeadlineDate.At(23, 59, loc)
	default:
		return "tanpa tenggat"
	}
	return fmt.Sprintf("%s, %d %s · %s", shortDays[at.Weekday()], at.Day(), untukMonths[at.Month()-1], at.Format("15.04"))
}

// segmented is the canvas' segmented control (.seg): returns the option
// clicked this frame, or -1.
func segmented(c *ui.Context, label string, options []string, selected int) int {
	t := c.Theme()
	clicked := -1
	// Wider than its place, the options wrap instead of spilling out.
	ui.Row(c.Key("seg-"+label)).Label(label).Gap(2).Padding(3).Radius(9).Border(1, t.Border).Wrap().Children(func() {
		for i, o := range options {
			b := ui.ButtonBase(c.Key(i)).Label(o).Height(30).Padding(0, 12).Radius(7).Center().TextColor(t.TextMuted)
			if i == selected {
				b.Background(t.Accent).TextColor(t.AccentText)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				txt := ui.Text(c, o).FontSize(13).MaxLines(1)
				if i == selected {
					txt.FontWeight(600)
				}
			})
			if b.Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}

// jobdeskSpaces are the Spaces the user's tasks are in, in sidebar order.
func (a *App) jobdeskSpaces() []api.Space {
	var out []api.Space
	for _, s := range a.access.spaces {
		if slices.ContainsFunc(a.tasks.items, func(t api.Task) bool {
			return t.Progress != api.TaskCancelled && slices.Contains(t.SpaceIDs, s.ID)
		}) {
			out = append(out, s)
		}
	}
	return out
}

func (a *App) jobdeskView(c *ui.Context) {
	a.ensureTasks(true)
	t := c.Theme()
	phone := isMobile(c)
	ui.Scroll(c.Key("jobdesk")).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(1000).MinWidth(0).Padding(20, phoneSize(phone, 16, 32), 40, phoneSize(phone, 16, 32)).Gap(18).Children(func() {
			if backLink(c, "Untukku").Clicked() {
				a.navigate(PathUntukku)
			}
			spaces := []api.Space(nil)
			if a.tasks.loaded {
				spaces = a.jobdeskSpaces()
			}
			if a.jobdesk.space != id.Nil && !slices.ContainsFunc(spaces, func(s api.Space) bool { return s.ID == a.jobdesk.space }) {
				a.jobdesk.space = id.Nil
			}
			ui.Row(c.Key("jd-head")).FillWidth().Wrap().Gap(14).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(240).Gap(2).Children(func() {
					ui.Text(c, "Jobdesk saya").FontSize(20).FontWeight(600)
					muted(c, "Tugas yang kamu pegang sebagai PJ, dari semua Space.").FontSize(13)
				})
				if len(spaces) > 1 {
					options, selected := []string{"Semua Space"}, 0
					for i, s := range spaces {
						options = append(options, s.Name)
						if s.ID == a.jobdesk.space {
							selected = i + 1
						}
					}
					ui.ScrollHorizontal(c.Key("jobdesk-filter")).MinWidth(0).Shrink(1).Children(func() {
						if i := quietSeg(c, "Filter Space", options, nil, selected); i == 0 {
							a.jobdesk.space = id.Nil
						} else if i > 0 {
							a.jobdesk.space = spaces[i-1].ID
						}
					})
				}
			})
			a.taskFeedback(c)
			if !a.tasks.loaded {
				title, detail := "Memuat tugas…", "Mengambil pekerjaan dari server."
				if a.tasks.err != "" {
					title, detail = "Tugas belum dapat dimuat", a.tasks.err
				}
				if a.tasks.scope == id.Nil {
					title, detail = "Pilih periode", "Jobdesk ditampilkan setelah periode dipilih."
				}
				pageUnavailable(c, "task", title, detail)
				return
			}
			empty := true
			now := c.Now()
			for g, tasks := range jobdeskGroups(a.tasks.items, a.jobdesk.space) {
				if len(tasks) == 0 {
					continue
				}
				empty = false
				group := jobdeskGroupsOrder[g]
				ui.Column(c.Key("grp-" + group.progress)).FillWidth().MinWidth(0).Gap(2).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Padding(0, 0, 6, 0).Margin(0, 0, 2, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
						ui.Text(c, group.label).FontSize(13).FontWeight(600)
						ui.Text(c, fmt.Sprint(len(tasks))).Font(FontMono).FontSize(12).TextColor(faint(t))
					})
					for _, task := range tasks {
						a.jobdeskRow(c.Key(task.ID), task, now, phone)
					}
				})
			}
			if empty {
				pageEmpty(c, "check", "Tidak ada tugas di Space ini", "Tugas yang PJ-nya kamu akan muncul di sini, dari Space mana pun.")
			}
		})
	})
}

// jobdeskRow is one task: its progress dot, number, title, deadline and
// Space. It opens Detail tugas.
func (a *App) jobdeskRow(c *ui.Context, task api.Task, now time.Time, phone bool) {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("Buka "+task.Title).Row().FillWidth().MinWidth(0).Gap(12).Height(44).Padding(0, 8).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	done := task.Progress == api.TaskDone
	row.FocusRing(row.FocusVisible()).Children(func() {
		dot := spaceMix(t.TextMuted, t.Background, .4)
		switch task.Progress {
		case api.TaskDoing:
			dot = t.Accent
		case api.TaskSubmitted:
			dot = t.Warning
		case api.TaskDone:
			dot = t.Success
		}
		ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(dot)
		if !phone {
			ui.Text(c, a.taskCode(task)).Font(FontMono).FontSize(12).TextColor(faint(t)).Width(44).Shrink(0).MaxLines(1)
		}
		title := task.Title
		if task.LocalPending {
			title += " · perangkat, belum cloud"
		}
		tt := ui.Text(c, title).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
		if done {
			tt.TextColor(faint(t)).Strikethrough()
		}
		due := ui.Text(c, jobdeskDue(task)).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Shrink(0).MaxLines(1)
		if !phone {
			due.Width(150)
		}
		if !done && taskLate(task, now) {
			due.TextColor(t.Danger)
		}
		if s, i, ok := a.taskSpace(task); ok {
			spaceTile(c, s, i)
		}
	})
	if row.Clicked() {
		a.openUntukTask(task.ID, PathJobdesk)
	}
}
