package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// Jobdesk saya, as canvas A draws it: the user's tasks from every Space in
// one list, grouped by what needs doing first, each with its Space, its
// deadline and the Belum / Kerjakan / Serahkan steps. The board stays on the
// Space's "Board tugas" tab.

// jobdeskUIState is the page's Space filter; id.Nil is "Semua ruang".
type jobdeskUIState struct {
	space id.ID
}

// jobdeskGroup is one heading of the list, in the canvas' order.
type jobdeskGroup int

const (
	jobdeskLate jobdeskGroup = iota
	jobdeskWeek
	jobdeskLater
	jobdeskSubmitted
)

func (g jobdeskGroup) String() string {
	return [...]string{"Lewat tenggat", "Minggu ini", "Nanti", "Diserahkan, menunggu tinjauan"}[g]
}

// jobdeskGroupOf places an open task: late first, then due within seven
// days, then later or without a deadline. Submitted tasks wait for review
// in their own group; done and cancelled tasks leave the list.
func jobdeskGroupOf(t api.Task, now time.Time) (jobdeskGroup, bool) {
	switch t.Progress {
	case api.TaskDone, api.TaskCancelled:
		return 0, false
	case api.TaskSubmitted:
		return jobdeskSubmitted, true
	}
	if taskLate(t, now) {
		return jobdeskLate, true
	}
	if taskDue(t).Before(now.AddDate(0, 0, 7)) {
		return jobdeskWeek, true
	}
	return jobdeskLater, true
}

// jobdeskGroups sorts tasks into the groups, each by deadline.
func jobdeskGroups(tasks []api.Task, now time.Time, space id.ID) [4][]api.Task {
	var groups [4][]api.Task
	for _, t := range tasks {
		if space != id.Nil && !slices.Contains(t.SpaceIDs, space) {
			continue
		}
		if g, ok := jobdeskGroupOf(t, now); ok {
			groups[g] = append(groups[g], t)
		}
	}
	for i := range groups {
		slices.SortStableFunc(groups[i], func(x, y api.Task) int { return taskDue(x).Compare(taskDue(y)) })
	}
	return groups
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

// shortTaskDate is a deadline as the canvas writes it: "5 Nov".
func shortTaskDate(t api.Task) string {
	switch {
	case t.DeadlineDate != nil:
		return fmt.Sprintf("%d %s", t.DeadlineDate.Day, shortMonth(int(t.DeadlineDate.Month)))
	case t.DeadlineAt != nil:
		loc, _ := timeutil.LoadZone(t.TimeZone)
		if loc == nil {
			loc = time.UTC
		}
		at := t.DeadlineAt.In(loc)
		return fmt.Sprintf("%d %s", at.Day(), shortMonth(int(at.Month())))
	}
	return "—"
}

// taskSummary is the line under a task's title: its description's first
// line, or what it should produce.
func taskSummary(t api.Task) string {
	s := strings.TrimSpace(t.Description)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if s == "" && strings.TrimSpace(t.ExpectedOutput) != "" {
		s = "Hasil: " + strings.TrimSpace(t.ExpectedOutput)
	}
	return s
}

// segmented is the canvas' segmented control (.seg): returns the option
// clicked this frame, or -1.
func segmented(c *ui.Context, label string, options []string, selected int) int {
	t := c.Theme()
	clicked := -1
	ui.Row(c.Key("seg-"+label)).Label(label).Gap(2).Padding(3).Radius(9).Border(1, t.Border).Shrink(0).Children(func() {
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
			_, ok := jobdeskGroupOf(t, time.Time{})
			return ok && slices.Contains(t.SpaceIDs, s.ID)
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
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(28).Gap(20)
		if phone {
			body.Padding(16).Gap(16)
		}
		wide := !phone && body.Bounds().W >= 860
		body.Children(func() {
			spaces := []api.Space(nil)
			if a.tasks.loaded {
				spaces = a.jobdeskSpaces()
			}
			if a.jobdesk.space != id.Nil && !slices.ContainsFunc(spaces, func(s api.Space) bool { return s.ID == a.jobdesk.space }) {
				a.jobdesk.space = id.Nil
			}
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			if !phone {
				head.Row().AlignItems(ui.Start)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, "Jobdesk saya").FontSize(20).Bold()
					desc := "Semua tugas organisasi yang PJ-nya kamu, dari Space mana pun. Tugas yang sama dengan yang ada di board divisi, bukan salinan."
					if phone {
						desc = "Tugas dari semua Space yang PJ-nya kamu."
					}
					muted(c, desc).FontSize(13)
				})
				if len(spaces) > 1 {
					options, selected := []string{"Semua ruang"}, 0
					for i, s := range spaces {
						options = append(options, s.Name)
						if s.ID == a.jobdesk.space {
							selected = i + 1
						}
					}
					scroll := ui.ScrollHorizontal(c.Key("jobdesk-filter")).MinWidth(0).Shrink(1)
					if phone {
						scroll.FillWidth()
					}
					scroll.Children(func() {
						if i := segmented(c, "Filter ruang", options, selected); i == 0 {
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
			// The detail panel opens from the workspace's projection.
			a.projectTasks()
			groups := jobdeskGroups(a.tasks.items, c.Now(), a.jobdesk.space)
			empty := true
			for g, tasks := range groups {
				if len(tasks) == 0 {
					continue
				}
				empty = false
				group := jobdeskGroup(g)
				ui.Column(c.Key(fmt.Sprint("grp", g))).FillWidth().MinWidth(0).Gap(8).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						h := ui.Text(c, strings.ToUpper(group.String())).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
						if group == jobdeskLate {
							h.TextColor(t.Danger)
						}
						ui.Text(c, fmt.Sprint(len(tasks))).FontSize(12).TextColor(faint(t))
					})
					if wide {
						ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
							for i, task := range tasks {
								a.jobdeskRow(c.Key(task.ID), task, i > 0)
							}
						})
						return
					}
					for _, task := range tasks {
						a.jobdeskCard(c.Key(task.ID), task)
					}
				})
			}
			if empty {
				pageEmpty(c, "check", "Tidak ada tugas terbuka", "Tugas yang PJ-nya kamu akan muncul di sini, dari Space mana pun.")
			}
		})
	})
}

// jobdeskRow is a desktop row: dot, title and summary, Space, deadline,
// progress steps, Buka.
func (a *App) jobdeskRow(c *ui.Context, task api.Task, divider bool) {
	t := c.Theme()
	row := ui.Row(c).FillWidth().MinWidth(0).Gap(14).Padding(12, 16).AlignItems(ui.Center)
	if divider {
		row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Box(c).Size(16, 16).Shrink(0).Center().Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(a.jobdeskDot(task))
		})
		a.jobdeskTitle(c, task)
		ui.Box(c).Width(120).Shrink(0).Children(func() { a.taskSpaceChip(c, task) })
		ui.Box(c).Width(80).Shrink(0).Children(func() { a.jobdeskDue(c, task) })
		ui.Box(c).Width(240).Shrink(0).Children(func() { a.jobdeskSteps(c, task, false) })
		ui.Box(c).Width(72).Shrink(0).Children(func() { a.jobdeskOpen(c, task, false) })
	})
}

// jobdeskCard is the phone's stacked card for one task.
func (a *App) jobdeskCard(c *ui.Context, task api.Task) {
	t := c.Theme()
	ui.Column(c).FillWidth().MinWidth(0).Padding(14).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(10).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Margin(6, 0, 0, 0).Shrink(0).Background(a.jobdeskDot(task))
			a.jobdeskTitle(c, task)
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { a.taskSpaceChip(c, task) })
			a.jobdeskDue(c, task)
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { a.jobdeskSteps(c, task, true) })
			a.jobdeskOpen(c, task, true)
		})
	})
}

func (a *App) jobdeskDot(task api.Task) ui.Color {
	if task.Category != "" {
		return tagColor(task.Category)
	}
	return ui.Hex("#a2bbd2")
}

func (a *App) jobdeskTitle(c *ui.Context, task api.Task) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
		title := task.Title
		if task.LocalPending {
			title += " · perangkat, belum cloud"
		}
		ui.Text(c, title).FontSize(13).FontWeight(500).MaxLines(2)
		if s := taskSummary(task); s != "" {
			ui.Text(c, s).FontSize(11).TextColor(faint(t)).MaxLines(1)
		}
	})
}

// taskSpaceChip is the Space's tile and name.
func (a *App) taskSpaceChip(c *ui.Context, task api.Task) {
	s, i, ok := a.taskSpace(task)
	if !ok {
		return
	}
	ui.Row(c).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(16, 16).Shrink(0).Radius(4).Center().Background(ui.Hex(canvasTileColors[i%len(canvasTileColors)])).Children(func() {
			ui.Text(c, spaceInitials(s)).FontSize(8).FontWeight(600).TextColor(canvasInk)
		})
		muted(c, s.Name).FontSize(12).MaxLines(1)
	})
}

func (a *App) jobdeskDue(c *ui.Context, task api.Task) {
	t := c.Theme()
	d := ui.Text(c, shortTaskDate(task)).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Shrink(0)
	if taskLate(task, c.Now()) {
		d.TextColor(t.Danger)
	}
}

// jobdeskSteps is Belum / Kerjakan / Serahkan. A step moves the task the way
// a drag on the board does (moveTask): with the same permissions, and
// Serahkan opens the task's detail to attach the evidence first.
func (a *App) jobdeskSteps(c *ui.Context, task api.Task, fill bool) {
	t := c.Theme()
	current := map[string]int{api.TaskTodo: 0, api.TaskDoing: 1, api.TaskSubmitted: 2}[task.Progress]
	allowed := task.Permissions.Progress || task.Permissions.Manage
	blocked := !allowed || a.tasks.busy || a.tasks.loading || (a.conn == connOffline && !a.offlineAvailable())
	steps := ui.Row(c).Gap(2)
	if fill {
		steps.FillWidth()
	}
	steps.Children(func() {
		for i, step := range []struct {
			label string
			lane  workspaceui.Lane
		}{{"Belum", workspaceui.Todo}, {"Kerjakan", workspaceui.Doing}, {"Serahkan", workspaceui.Review}} {
			on := i == current
			b := ui.ButtonBase(c.Key(i)).Label(step.label+": "+task.Title).Tooltip(step.label).Height(26).Padding(0, 8).Radius(6).Center().Border(1, t.Border).TextColor(t.TextMuted).Disabled(blocked && !on)
			if fill {
				b.Grow(1).Height(36)
			}
			if on {
				b.Background(accentSoft(t)).BorderColor(t.Accent).TextColor(t.Text)
			} else if b.Hovered() && !blocked {
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, step.label).FontSize(11).MaxLines(1) })
			if b.Clicked() && !on && !blocked {
				a.moveTask(workspaceui.MoveRequest{TaskID: task.ID, Destination: step.lane})
			}
		}
	})
}

func (a *App) jobdeskOpen(c *ui.Context, task api.Task, phone bool) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("open")).Label("Buka "+task.Title).Tooltip("Buka detail tugas").Height(28).Padding(0, 12).Radius(8).Center().Border(1, t.Border)
	if phone {
		b.Height(36)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, "Buka").FontSize(13).FontWeight(500) })
	if b.Clicked() {
		a.workspace.Open(task.ID)
	}
}
