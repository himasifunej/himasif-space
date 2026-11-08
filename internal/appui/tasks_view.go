package appui

import (
	"context"
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

func (a *App) personalTasksView(c *ui.Context) { a.ensureTasks(true); a.taskBoardView(c, true) }
func (a *App) spaceTasksView(c *ui.Context)    { a.ensureTasks(false); a.taskBoardView(c, false) }
func (a *App) taskBoardView(c *ui.Context, personal bool) {
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Gap(0).Children(func() {
		phone := isMobile(c)
		// The space board's own header carries its toolbar (boardToolbar);
		// the Jobdesk page keeps its bar.
		if personal {
			bar := ui.Row(c).FillWidth().Padding(12, 22).Gap(10).AlignItems(ui.Center)
			if phone {
				bar.Padding(8, 12).Gap(6)
			}
			bar.Children(func() {
				label := "Tugas Space"
				if personal {
					label = "Jobdesk saya"
				}
				if !phone {
					// The phone's top bar already names the page.
					ui.Text(c, label).FontSize(14).Bold().Grow(1)
				}
				filter := ui.TextInput(c.Key("task-filter"), &a.tasks.filter).Label("Filter tugas").Placeholder("Judul, kategori, atau PJ…").Grow(1).MinWidth(90)
				if !phone {
					filter.MaxWidth(220)
				}
				if filter.Changed() {
					a.projectTasks()
				}
				var cancelled ui.Element
				if phone {
					cancelled = spaceIconButton(c, "Dibatalkan", "archive").Size(44, 44)
					if a.tasks.showCancelled {
						cancelled.Background(c.Theme().SurfaceHover)
					}
				} else {
					cancelled = spaceButton(c, "Dibatalkan", "archive", a.tasks.showCancelled)
				}
				if cancelled.Clicked() {
					a.tasks.showCancelled = !a.tasks.showCancelled
					a.projectTasks()
				}
				if !phone && !personal {
					a.cardColorChoices(c)
				}
				if spaceIconButton(c, "Muat ulang tugas", "refresh").Disabled(a.tasks.loading || a.tasks.busy).Clicked() {
					a.loadTasks()
				}
			})
		}
		a.taskFeedback(c)
		if !a.tasks.loaded {
			ui.Column(c).Fill().Padding(26).Children(func() {
				title := "Memuat tugas…"
				detail := "Mengambil pekerjaan dari server."
				if a.tasks.err != "" {
					title = "Tugas belum dapat dimuat"
					detail = a.tasks.err
				}
				if a.tasks.scope == id.Nil {
					title = "Pilih periode atau Space"
					detail = "Tugas akan ditampilkan setelah ruang kerja dipilih."
				}
				pageUnavailable(c, "task", title, detail)
			})
			return
		}
		a.projectTasks()
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Children(func() {
			a.workspace.ViewBoard(c)
		})
		a.cancelTaskDialog(c)
	})
}
func (a *App) taskFeedback(c *ui.Context) {
	if a.tasks.notice != "" {
		kind := "info"
		if strings.HasSuffix(a.tasks.notice, "tersimpan di server.") {
			kind = "success"
		}
		c.AddToast(ui.Toast{Title: a.tasks.notice, Type: kind, Timeout: 8 * time.Second})
		a.tasks.notice = ""
	}
	if a.tasks.err != "" {
		ui.Row(c).FillWidth().Padding(10, 22).Gap(8).Children(func() { ui.Text(c, a.tasks.err).FontSize(12).TextColor(c.Theme().Danger).Grow(1).MinWidth(0) })
	}
	if a.tasks.busy || a.tasks.loading {
		muted(c, "Menyimpan atau memuat data server…").Padding(6, 22)
	}
	if a.conn == connOffline {
		muted(c, "Tidak terhubung. Data terakhir ditampilkan; perubahan belum dapat dikirim.").Padding(6, 22)
	}
}
func (a *App) taskHeaderAction(c *ui.Context) {
	b := primarySpaceButton(c, "Buat tugas", "plus").Disabled(a.isAdmin())
	if b.Clicked() {
		if len(a.manageableDivisions()) > 0 {
			a.ensureTasks(false)
			a.beginTask()
			a.navigate(PathSpaces)
			a.chrome.tab = "Board tugas"
		} else {
			a.tasks.createOpen = !a.tasks.createOpen
		}
	}
	popover(c, b, &a.tasks.createOpen, 260, func() {
		ui.Text(c, "Pilih Space tugas").Bold()
		count := 0
		for _, s := range a.access.spaces {
			if s.ReadOnly {
				continue
			}
			allowed := false
			for _, r := range s.Roles {
				for _, cap := range r.Capabilities {
					if cap == api.CapTasksManage {
						allowed = true
					}
				}
			}
			if !allowed {
				continue
			}
			count++
			if spaceButton(c.Key(s.ID), s.Name, "grid", false).FillWidth().Clicked() {
				a.selectSpace(s.ID)
				a.ensureTasks(false)
				a.beginTask()
				a.navigate(PathSpaces)
				a.chrome.tab = "Board tugas"
				a.tasks.createOpen = false
			}
		}
		if count == 0 {
			muted(c, "Kamu belum memiliki kewenangan delegasi tugas pada periode ini.")
		}
	})
}
func (a *App) tasksOverview(c *ui.Context) {
	a.ensureTasks(false)
	t := c.Theme()
	if !a.tasks.loaded {
		pageSection(c, "Pekerjaan di ruang ini", "Satu tugas tetap tersinkron di jobdesk PJ dan Space yang dituju.", func() {
			muted(c, "Memuat tugas…")
			if a.tasks.err != "" {
				ui.Text(c, a.tasks.err).TextColor(t.Danger)
			}
		})
		return
	}
	now := c.Now()
	var active, late, review, done int
	type load struct {
		name  string
		lanes [3]int
		late  int
	}
	loads := map[string]*load{}
	var order []string
	var upcoming []api.Task
	for _, task := range a.tasks.items {
		switch {
		case task.Progress == api.TaskCancelled:
			continue
		case task.Progress == api.TaskDone:
			done++
			continue
		}
		active++
		lane := taskLane(task)
		if lane == workspaceui.Review || task.MatchStatus == api.MatchPending {
			review++
		}
		isLate := taskLate(task, now)
		if isLate {
			late++
		}
		name := task.Assignee.DisplayName
		if name == "" {
			name = "Belum ada PJ"
		}
		l := loads[name]
		if l == nil {
			l = &load{name: name}
			loads[name] = l
			order = append(order, name)
		}
		if isLate {
			l.late++
		} else if lane <= workspaceui.Review {
			l.lanes[lane]++
		}
		upcoming = append(upcoming, task)
	}
	slices.SortFunc(upcoming, func(x, y api.Task) int { return taskDue(x).Compare(taskDue(y)) })
	phone := isMobile(c)
	cols := 4
	if phone {
		cols = 2
	}
	ui.Grid(c.Key("home-kpis")).FillWidth().Columns(cols).Gap(12).Children(func() {
		for _, k := range []struct {
			label, detail string
			value         int
			color         ui.Color
		}{
			{"Tugas aktif", "Belum selesai di ruang ini", active, ui.Hex("#a2bbd2")},
			{"Lewat tenggat", "Pengingat harian berjalan", late, t.Danger},
			{"Perlu tinjauan", "Bukti menunggu dicocokkan", review, t.Warning},
			{"Selesai", "Periode ini", done, t.Success},
		} {
			ui.Column(c.Key(k.label)).MinWidth(0).Padding(14).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(k.color)
					muted(c, k.label)
				})
				v := ui.Text(c, fmt.Sprint(k.value)).FontSize(26).Bold()
				if k.label == "Lewat tenggat" && k.value > 0 {
					v.TextColor(t.Danger)
				}
				muted(c, k.detail).FontSize(11).MaxLines(1)
			})
		}
	})
	sections := ui.Column(c.Key("home-sections")).FillWidth().Gap(16)
	if !phone {
		sections.Row().AlignItems(ui.Stretch)
	}
	sections.Children(func() {
		ui.Column(c.Key("home-load")).Grow(1).MinWidth(0).Children(func() {
			pageSection(c, "Beban kerja anggota", "Tugas aktif per PJ.", func() {
				most := 1
				for _, l := range loads {
					most = max(most, l.lanes[0]+l.lanes[1]+l.lanes[2]+l.late)
				}
				if len(order) == 0 {
					muted(c, "Belum ada tugas aktif.")
				}
				colors := []ui.Color{ui.Hex("#8f8878"), ui.Hex("#a2bbd2"), t.Warning}
				for _, name := range order {
					l := loads[name]
					total := l.lanes[0] + l.lanes[1] + l.lanes[2] + l.late
					ui.Row(c.Key(name)).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, name).FontSize(12).Width(110).Shrink(0).MaxLines(1)
						track := ui.Row(c).Grow(1).MinWidth(0).Height(12).Radius(6).Background(t.Background)
						width := track.Bounds().W
						if width == 0 {
							// Bars size from the track's last layout.
							c.After(0)
						}
						track.Children(func() {
							for i, n := range l.lanes {
								if n > 0 && width > 0 {
									ui.Box(c.Key(i)).Width(width * float32(n) / float32(most)).FillHeight().Background(colors[i])
								}
							}
							if l.late > 0 && width > 0 {
								ui.Box(c.Key("late")).Width(width * float32(l.late) / float32(most)).FillHeight().Background(t.Danger)
							}
						})
						ui.Text(c, fmt.Sprintf("%d tugas", total)).FontSize(11).TextColor(t.TextMuted).Width(52).Shrink(0)
					})
				}
				ui.Row(c).Wrap().Gap(12).Children(func() {
					for i, label := range []string{"Belum mulai", "Dikerjakan", "Perlu tinjauan", "Lewat tenggat"} {
						color := t.Danger
						if i < 3 {
							color = colors[i]
						}
						ui.Row(c.Key(label)).Gap(5).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(8, 8).Radius(2).Background(color)
							muted(c, label).FontSize(11)
						})
					}
				})
			})
		})
		ui.Column(c.Key("home-due")).Grow(1).MinWidth(0).Children(func() {
			pageSection(c, "Tenggat terdekat", "Diurutkan dari yang paling dekat.", func() {
				if len(upcoming) == 0 {
					muted(c, "Tidak ada tugas aktif.")
				}
				for i, task := range upcoming {
					if i == 6 {
						break
					}
					row := ui.ButtonBase(c.Key(task.ID)).Label("Buka "+task.Title).FillWidth().Row().Gap(10).AlignItems(ui.Center).MinHeight(44).Padding(6, 8).Radius(8)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					isLate := taskLate(task, now)
					row.FocusRing(row.FocusVisible()).Children(func() {
						dot := tagColor(task.Category)
						if isLate {
							dot = t.Danger
						}
						ui.Box(c).Size(8, 8).Radius(4).Background(dot).Shrink(0)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, task.Title).FontSize(13).MaxLines(1)
							muted(c, task.Assignee.DisplayName).FontSize(11).MaxLines(1)
						})
						due := taskDate(task.DeadlineDate, task.DeadlineAt, task.TimeZone)
						if isLate {
							due += " · lewat"
						}
						d := ui.Text(c, due).FontSize(11).Shrink(0)
						if isLate {
							d.TextColor(t.Danger)
						} else {
							d.TextColor(t.TextMuted)
						}
					})
					if row.Clicked() {
						a.chrome.tab = "Board tugas"
						a.workspace.Open(task.ID)
					}
				}
				if spaceButton(c, "Lihat semua tugas", "board", false).Clicked() {
					a.chrome.tab = "Board tugas"
				}
			})
		})
	})
}

// taskDue orders tasks by deadline; tasks without one come last.
func taskDue(t api.Task) time.Time {
	if t.DeadlineAt != nil {
		return *t.DeadlineAt
	}
	if t.DeadlineDate != nil {
		loc, _ := timeutil.LoadZone(t.TimeZone)
		if loc == nil {
			loc = time.UTC
		}
		return t.DeadlineDate.At(23, 59, loc)
	}
	return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
}

// taskLate reports an unfinished task past its deadline. A date-only
// deadline lasts the whole day in the task's time zone.
func taskLate(t api.Task, now time.Time) bool {
	if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled || t.Progress == api.TaskSubmitted {
		return false
	}
	if t.DeadlineAt == nil && t.DeadlineDate == nil {
		return false
	}
	return taskDue(t).Before(now)
}
func (a *App) taskCreatePanel(c *ui.Context) {
	t := c.Theme()
	workspaceui.Drawer(c.Key("create-task-drawer"), &a.chrome.drawerWidth, func() {
		ui.Row(c).FillWidth().Padding(16, 20).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Tugas baru").FontSize(17).Bold().Grow(1)
			if spaceIconButton(c, "Tutup tugas baru", "close").Disabled(a.tasks.busy).Clicked() {
				a.tasks.draft = nil
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Area scroll form tugas").Children(func() { ui.Column(c).FillWidth().Padding(20).Gap(14).Children(func() { a.taskEditor(c) }) })
		ui.Column(c).FillWidth().Padding(14, 20).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			if a.tasks.err != "" {
				ui.Text(c, a.tasks.err).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1)
				if spaceButton(c, "Batal", "", false).Disabled(a.tasks.busy).Clicked() {
					a.tasks.draft = nil
				}
				if primarySpaceButton(c, "Simpan tugas", "check").Disabled(a.taskWriteBlocked()).Clicked() {
					a.saveTask()
				}
			})
		})
	})
}
func taskInput(c *ui.Context, key, label string, value *string) {
	ui.Field(c, label, func() { ui.TextInput(c.Key(key), value).FillWidth().Label(label) })
}
func taskLongInput(c *ui.Context, key, label string, value *string) {
	ui.Field(c, label, func() { ui.TextArea(c.Key(key), value).FillWidth().Height(76).Label(label) })
}
func (a *App) taskEditor(c *ui.Context) {
	d := a.tasks.draft
	if d == nil {
		return
	}
	disabled := a.taskWriteBlocked()
	ui.Column(c).FillWidth().Gap(14).Disabled(disabled).Children(func() {
		if d.taskID == id.Nil && len(a.manageableDivisions()) > 1 {
			name := "Pilih divisi pemilik"
			for _, div := range a.manageableDivisions() {
				if div.ID == d.owner {
					name = div.Name
				}
			}
			b := spaceButton(c, name, "grid", false).FillWidth()
			if b.Clicked() {
				a.tasks.ownerOpen = !a.tasks.ownerOpen
			}
			popover(c, b, &a.tasks.ownerOpen, 270, func() {
				for _, div := range a.manageableDivisions() {
					if spaceButton(c.Key(div.ID), div.Name, "grid", false).FillWidth().Clicked() {
						d.owner = div.ID
						d.assignee = id.Nil
						a.tasks.ownerOpen = false
						a.loadTaskMembers(div.ID)
					}
				}
			})
		}
		taskInput(c, "task-title", "Judul tugas *", &d.title)
		taskLongInput(c, "task-output", "Hasil yang diharapkan *", &d.output)
		taskLongInput(c, "task-description", "Deskripsi", &d.description)
		taskInput(c, "task-category", "Label · pisahkan dengan koma", &d.category)
		label := "Pilih PJ utama *"
		for _, m := range a.tasks.members[d.owner] {
			if m.UserID == d.assignee {
				label = m.DisplayName
			}
		}
		b := spaceButton(c, label, "users", false).FillWidth()
		if b.Clicked() {
			a.tasks.assigneeOpen = !a.tasks.assigneeOpen
		}
		popover(c, b, &a.tasks.assigneeOpen, 280, func() {
			if a.tasks.membersBusy[d.owner] {
				muted(c, "Memuat anggota…")
			}
			if e := a.tasks.memberErrors[d.owner]; e != "" {
				ui.Text(c, e).TextColor(c.Theme().Danger)
			}
			ui.Scroll(c).MaxHeight(240).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(3).Children(func() {
					for _, m := range a.tasks.members[d.owner] {
						if spaceButton(c.Key(m.UserID), m.DisplayName, "user", d.assignee == m.UserID).FillWidth().Clicked() {
							d.assignee = m.UserID
							a.tasks.assigneeOpen = false
						}
					}
				})
			})
		})
		zone := timeutil.DefaultZoneName
		if p := a.period(); p != nil {
			zone = p.TimeZone
		}
		muted(c, "Jadwal pengerjaan · "+zone)
		a.rangeWhenField(c, "task-schedule", d)
		// Posting is its own schedule; the H-2 design deadline follows it.
		a.whenField(c, "task-posting", "Jadwal posting", &d.posting, &d.postingTime, false)
		if d.posting == "" {
			d.postingTime = ""
		}
		muted(c, "Tanpa jam tetap berupa tanggal, tanpa menambahkan jam otomatis. Tenggat dan posting disimpan terpisah.")
		var shared []api.Space
		for _, s := range a.access.spaces {
			if s.Kind != api.SpaceShared || s.ReadOnly {
				continue
			}
			for _, div := range s.Divisions {
				if div.ID == d.owner {
					shared = append(shared, s)
				}
			}
		}
		if len(shared) > 0 {
			ui.Text(c, "Bagikan juga ke").FontSize(12).Bold()
			for _, s := range shared {
				on := d.shared[s.ID]
				if spaceButton(c.Key(s.ID), s.Name, "grid", on).FillWidth().Clicked() {
					d.shared[s.ID] = !on
				}
			}
			muted(c, "Space divisi pemilik selalu memuat tugas ini. Tugas internal tidak dibagikan otomatis.")
		}
		if d.taskID != id.Nil {
			taskInput(c, "task-change-reason", "Alasan perubahan PJ/tenggat", &d.reason)
		}
		if d.taskID != id.Nil && spaceButton(c, "Simpan tugas", "check", true).FillWidth().Clicked() {
			a.saveTask()
		}
		if d.taskID != id.Nil {
			if spaceButton(c, "Buang isian & gunakan data server", "refresh", false).FillWidth().Clicked() {
				if current := a.taskByID(d.taskID); current != nil {
					a.editTask(*current)
				}
			}
		}
		if d.taskID != id.Nil && spaceButton(c, "Batalkan perubahan", "close", false).FillWidth().Clicked() {
			a.tasks.draft = nil
		}
	})
}
func (a *App) taskDetails(c *ui.Context, tid id.ID) {
	t := a.taskByID(tid)
	if t == nil {
		return
	}
	if a.tasks.inspected != tid {
		a.tasks.inspected = tid
		a.tasks.inspectorSeq++
		a.tasks.historySeq++
		a.tasks.comment = ""
		a.tasks.help = ""
		a.tasks.evidenceURL = ""
		a.tasks.evidenceNote = ""
		a.tasks.reason = ""
		a.tasks.history = nil
		a.tasks.historyBusy = false
		a.tasks.historyLoaded = false
		a.tasks.historyErr = ""
		a.tasks.descDraft = t.Description
		a.tasks.labelDraft, a.tasks.pjReason, a.tasks.checklistNew = "", "", ""
		a.tasks.statusOpen, a.tasks.moreOpen, a.tasks.labelOpen, a.tasks.pjOpen = false, false, false, false
		a.tasks.evidenceOpen, a.tasks.helpOpen, a.tasks.reminderOpen = a.tasks.evidenceFor == tid, false, false
		a.tasks.evidenceFor = id.Nil
		if a.tasks.draft != nil && a.tasks.draft.taskID != tid {
			a.tasks.draft = nil
		}
	}
	if d := a.tasks.draft; d != nil && d.taskID == tid {
		a.taskEditor(c)
		return
	}
	a.taskDetailMain(c, t)
	if !a.tasks.detailSplit {
		a.taskDetailSide(c, t)
	}
}

func progressLabel(p string) string {
	switch p {
	case api.TaskTodo:
		return "Belum mulai"
	case api.TaskDoing:
		return "Dikerjakan"
	case api.TaskSubmitted:
		return "Diserahkan"
	case api.TaskDone:
		return "Selesai"
	case api.TaskCancelled:
		return "Dibatalkan"
	}
	return p
}
func matchLabel(m string) string {
	switch m {
	case api.MatchNone:
		return "Belum ada bukti"
	case api.MatchPending:
		return "Bukti perlu tinjauan"
	case api.MatchVerified:
		return "Bukti cocok"
	case api.MatchRejected:
		return "Bukti tidak cocok"
	}
	return m
}
func taskTimestamp(t time.Time, zone string) string {
	loc, _ := timeutil.LoadZone(zone)
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("02 Jan 2006 · 15:04 MST")
}
func (a *App) taskNotifications(c *ui.Context) {
	if !a.tasks.notificationsLoaded && !a.tasks.notificationsBusy && a.tasks.notificationsErr == "" {
		a.loadTaskNotifications()
	}
	if a.tasks.notificationsBusy {
		muted(c, "Memuat notifikasi…")
	}
	if a.tasks.notificationsErr != "" {
		ui.Text(c, a.tasks.notificationsErr).FontSize(12).TextColor(c.Theme().Danger)
	}
	if a.tasks.notificationsLoaded && len(a.tasks.notifications) == 0 {
		muted(c, "Belum ada pembaruan tugas.")
	}
	ui.Scroll(c).FillWidth().MaxHeight(280).Children(func() {
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			for _, n := range a.tasks.notifications {
				b := ui.ButtonBase(c.Key(n.ID)).FillWidth().Padding(10).Radius(8).Border(1, c.Theme().Border).Label(n.Title)
				if n.ReadAt == nil {
					b.Background(c.Theme().SurfaceHover)
				}
				b.Children(func() {
					ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
						ui.Text(c, n.Title).FontSize(12).Bold()
						muted(c, n.Body).FontSize(11)
						muted(c, taskTimestamp(n.CreatedAt, timeutil.DefaultZoneName)).FontSize(10)
					})
				})
				if b.Clicked() {
					a.openTaskNotification(n)
				}
			}
		})
	})
	if spaceButton(c, "Perbarui notifikasi", "refresh", false).Disabled(a.tasks.notificationsBusy).FillWidth().Clicked() {
		a.loadTaskNotifications()
	}
}
func (a *App) openTaskNotification(n api.Notification) {
	s, ok := a.taskService()
	if !ok {
		return
	}
	gen, seq := a.gen, a.tasks.seq
	a.tasks.notificationsBusy = true
	a.async(func() func() {
		task, err := s.GetTask(context.Background(), n.TaskID)
		if err == nil {
			err = s.ReadNotification(context.Background(), n.ID)
		}
		return func() {
			if gen != a.gen || seq != a.tasks.seq {
				return
			}
			a.tasks.notificationsBusy = false
			if err != nil {
				a.tasks.notificationsErr = a.taskFailure(err)
				a.tasks.notifications = nil
				return
			}
			if !a.openCanonicalTask(task) {
				a.tasks.notificationsErr = "Pilih periode tugas tersebut, lalu muat ulang Space yang dapat kamu akses."
			}
		}
	})
}

func (a *App) openCanonicalTask(task api.Task) bool {
	for _, sid := range task.SpaceIDs {
		for _, space := range a.access.spaces {
			if space.ID != sid {
				continue
			}
			a.selectSpace(sid)
			a.ensureTasks(false)
			a.tasks.filter = ""
			if a.tasks.loaded {
				a.projectTasks()
			}
			a.tasks.pendingOpen = task.ID
			if a.tasks.loaded {
				a.workspace.Open(task.ID)
				a.tasks.pendingOpen = id.Nil
			}
			a.navigate(PathSpaces)
			a.chrome.tab = "Board tugas"
			return true
		}
	}
	return false
}
func (a *App) taskSearchResults(c *ui.Context, q string) int {
	if q == "" || !a.tasks.loaded {
		return 0
	}
	n := 0
	for _, task := range a.tasks.items {
		if !a.taskMatches(task, q) {
			continue
		}
		n++
		if spaceButton(c.Key(task.ID), a.taskCode(task)+" · "+task.Title, "task", false).FillWidth().Clicked() {
			a.openCanonicalTask(task)
		}
		if n >= 6 {
			break
		}
	}
	return n
}
func (a *App) taskAgenda(c *ui.Context) {
	a.ensureTasks(false)
	zone := timeutil.DefaultZoneName
	if p := a.period(); p != nil && p.TimeZone != "" {
		zone = p.TimeZone
	}
	loc, _ := timeutil.LoadZone(zone)
	if loc == nil {
		loc = time.UTC
	}
	now := c.Now().In(loc)
	start := now.AddDate(0, 0, -(int(now.Weekday())+6)%7+7*a.tasks.weekOffset)
	pageBody(c, "Agenda tugas", func() {
		muted(c, "Tenggat pekerjaan dan jadwal posting dari tugas di Space ini. Rapat dan pengingat terjadwal menyusul.")
		a.taskFeedback(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if spaceButton(c, "Minggu sebelumnya", "arrow", false).Clicked() {
				a.tasks.weekOffset--
			}
			ui.Text(c, start.Format("02 Jan")+" – "+start.AddDate(0, 0, 6).Format("02 Jan 2006")).Bold().Grow(1)
			if spaceButton(c, "Minggu ini", "calendar", false).Clicked() {
				a.tasks.weekOffset = 0
			}
			if spaceButton(c, "Minggu berikutnya", "arrow", false).Clicked() {
				a.tasks.weekOffset++
			}
		})
		if !a.tasks.loaded {
			muted(c, "Data agenda belum dapat ditampilkan.")
			return
		}
		ui.ScrollHorizontal(c).FillWidth().Height(430).Children(func() {
			ui.Row(c).FillHeight().Gap(8).Children(func() {
				for day := 0; day < 7; day++ {
					date := timeutil.DateOf(start.AddDate(0, 0, day))
					ui.Scroll(c.Key(day)).Width(172).Shrink(0).FillHeight().Radius(9).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
						ui.Column(c).FillWidth().Padding(12).Gap(9).Children(func() {
							ui.Text(c, []string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}[day]).FontSize(11).TextColor(c.Theme().TextMuted)
							ui.Text(c, fmt.Sprint(date.Day)).FontSize(24).Bold()
							count := 0
							for _, task := range a.tasks.items {
								if task.Progress == api.TaskCancelled {
									continue
								}
								for _, event := range []struct {
									label string
									date  *timeutil.Date
									at    *time.Time
								}{{"Mulai", task.PlannedStartDate, task.PlannedStartAt}, {"Tenggat", task.DeadlineDate, task.DeadlineAt}, {"Posting", task.PostingDate, task.PostingAt}} {
									matches := event.date != nil && *event.date == date
									if event.at != nil {
										matches = timeutil.DateOf(event.at.In(loc)) == date
									}
									if !matches {
										continue
									}
									count++
									b := ui.ButtonBase(c.Key(task.ID.String()+event.label)).FillWidth().Padding(10).Radius(7).Border(1, c.Theme().Border).Label(event.label + " " + task.Title)
									b.Children(func() {
										ui.Column(c).FillWidth().Gap(6).Children(func() {
											ui.Text(c, event.label).FontSize(10).TextColor(c.Theme().Accent)
											ui.Text(c, task.Title).FontSize(12).Bold()
											clock := "Tanpa jam"
											if event.at != nil {
												clock = event.at.In(loc).Format("15:04 MST")
											}
											muted(c, clock).FontSize(10)
											muted(c, task.Assignee.DisplayName).FontSize(10)
										})
									})
									if b.Clicked() {
										a.workspace.Open(task.ID)
										a.chrome.tab = "Board tugas"
									}
								}
							}
							if count == 0 {
								muted(c, "—")
							}
						})
					})
				}
			})
		})
	})
}

func taskHelpLabel(status string) string {
	switch status {
	case "pending":
		return "Menunggu keputusan"
	case "approved":
		return "Diterima"
	case "rejected":
		return "Ditolak"
	}
	return "Diperbarui"
}
func taskEventLabel(action string) string {
	switch action {
	case "task.create":
		return "Tugas dibuat"
	case "task.update":
		return "Detail / delegasi diubah"
	case "task.move":
		return "Progres / urutan diperbarui"
	case "task.comment":
		return "Komentar ditambahkan"
	case "task.help.offer":
		return "Bantuan ditawarkan"
	case "task.help.decide":
		return "Pengajuan bantuan diputuskan"
	case "task.evidence.submit":
		return "Hasil diserahkan"
	case "task.evidence.review":
		return "Bukti dicocokkan"
	}
	return "Tugas diperbarui"
}

func (a *App) cancelTaskDialog(c *ui.Context) {
	if !a.tasks.cancelOpen {
		return
	}
	t := a.taskByID(a.tasks.cancelID)
	if t == nil {
		a.tasks.cancelOpen = false
		return
	}
	th := c.Theme()
	w, _ := c.Size()
	ui.DialogBase(c.Key("cancel-task"), &a.tasks.cancelOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		panel.Width(min(float32(460), w-32)).Column().Padding(20).Gap(12).Radius(14).Background(th.Surface).Border(1, th.Border).Children(func() {
			ui.Text(c, "Batalkan “"+t.Title+"”?").FontSize(17).Bold()
			muted(c, "Tugas pindah ke lajur Dibatalkan dan pengingatnya berhenti. Riwayat, komentar dan bukti tetap tersimpan untuk evaluasi.")
			taskInput(c, "cancel-reason", "Alasan pembatalan *", &a.tasks.cancelReason)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Box(c).Grow(1)
				if spaceButton(c, "Kembali", "", false).Clicked() {
					a.tasks.cancelOpen = false
				}
				b := spaceButton(c, "Batalkan tugas", "archive", false).Background(th.Danger).TextColor(ui.Hex("#ffffff")).Disabled(strings.TrimSpace(a.tasks.cancelReason) == "")
				if b.Clicked() {
					a.confirmCancelTask()
				}
			})
		})
	})
}
