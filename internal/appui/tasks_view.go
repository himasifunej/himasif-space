package appui

import (
	"context"
	"fmt"
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
		ui.Row(c).FillWidth().Padding(12, 22).Gap(10).AlignItems(ui.Center).Children(func() {
			label := "Tugas Space"
			if personal {
				label = "Jobdesk saya"
			}
			ui.Text(c, label).FontSize(14).Bold().Grow(1)
			if ui.TextInput(c.Key("task-filter"), &a.tasks.filter).Label("Filter tugas").Placeholder("Judul, kategori, atau PJ…").Width(220).Changed() {
				a.projectTasks()
			}
			if spaceButton(c, "Dibatalkan", "archive", a.tasks.showCancelled).Clicked() {
				a.tasks.showCancelled = !a.tasks.showCancelled
				a.projectTasks()
			}
			if spaceIconButton(c, "Muat ulang tugas", "refresh").Disabled(a.tasks.loading || a.tasks.busy).Clicked() {
				a.loadTasks()
			}
		})
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
			a.workspace.View(c)
			if a.tasks.draft != nil && a.tasks.draft.taskID == id.Nil && !a.chrome.appearance {
				a.taskCreatePanel(c)
			}
		})
	})
}
func (a *App) taskFeedback(c *ui.Context) {
	if a.tasks.err != "" {
		ui.Row(c).FillWidth().Padding(10, 22).Gap(8).Children(func() { ui.Text(c, a.tasks.err).FontSize(12).TextColor(c.Theme().Danger).Grow(1).MinWidth(0) })
	}
	if a.tasks.notice != "" {
		ui.Row(c).FillWidth().Padding(8, 22).Gap(8).Children(func() {
			ui.Text(c, a.tasks.notice).FontSize(12).TextColor(c.Theme().Accent).Grow(1).MinWidth(0)
			if spaceIconButton(c, "Tutup informasi tugas", "close").Clicked() {
				a.tasks.notice = ""
			}
		})
	}
	if a.tasks.busy || a.tasks.loading {
		muted(c, "Menyimpan atau memuat data server…").Padding(6, 22)
	}
	if a.conn == connOffline {
		muted(c, "Tidak terhubung. Data terakhir ditampilkan; perubahan belum dapat dikirim.").Padding(6, 22)
	}
}
func (a *App) taskHeaderAction(c *ui.Context) {
	b := spaceButton(c, "Buat tugas", "plus", true).Disabled(a.isAdmin())
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
	pageSection(c, "Pekerjaan di ruang ini", "Satu tugas tetap tersinkron di jobdesk PJ dan Space yang dituju.", func() {
		if !a.tasks.loaded {
			muted(c, "Memuat tugas…")
			if a.tasks.err != "" {
				ui.Text(c, a.tasks.err).TextColor(c.Theme().Danger)
			}
			return
		}
		counts := [4]int{}
		for _, t := range a.tasks.items {
			if t.Progress != api.TaskCancelled {
				counts[taskLane(t)]++
			}
		}
		ui.Row(c).FillWidth().Gap(18).Children(func() {
			for lane := workspaceui.Todo; lane <= workspaceui.Done; lane++ {
				ui.Column(c.Key(lane)).Grow(1).MinWidth(0).Gap(5).Children(func() { ui.Text(c, fmt.Sprint(counts[lane])).FontSize(24).Bold(); muted(c, lane.String()).FontSize(11) })
			}
		})
		if len(a.tasks.items) == 0 {
			muted(c, "Belum ada tugas di Space ini.")
		}
		if spaceButton(c, "Lihat semua tugas", "board", false).Clicked() {
			a.chrome.tab = "Board tugas"
		}
	})
}
func (a *App) taskCreatePanel(c *ui.Context) {
	ui.Column(c).Width(350).Shrink(0).FillHeight().MinWidth(0).Background(c.Theme().Surface).BorderWidth(0, 0, 0, 1).BorderColor(c.Theme().Border).Label("Buat tugas baru").Children(func() {
		ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Delegasikan tugas").Bold().Grow(1)
			if spaceIconButton(c, "Tutup tugas baru", "close").Disabled(a.tasks.busy).Clicked() {
				a.tasks.draft = nil
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Area scroll form tugas").Children(func() { ui.Column(c).FillWidth().Padding(20).Gap(14).Children(func() { a.taskEditor(c) }) })
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
		taskInput(c, "task-category", "Kategori", &d.category)
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
		muted(c, "Tanggal YYYY-MM-DD · jam opsional HH:MM · "+zone)
		taskInput(c, "task-deadline", "Tenggat tugas *", &d.deadline)
		taskInput(c, "task-deadline-time", "Jam tenggat (opsional)", &d.deadlineTime)
		taskInput(c, "task-posting", "Tanggal posting (opsional)", &d.posting)
		taskInput(c, "task-posting-time", "Jam posting (opsional)", &d.postingTime)
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
		if spaceButton(c, "Simpan tugas", "check", true).FillWidth().Clicked() {
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
		if a.tasks.draft != nil && a.tasks.draft.taskID != tid {
			a.tasks.draft = nil
		}
	}
	if d := a.tasks.draft; d != nil && d.taskID == tid {
		a.taskEditor(c)
		return
	}
	ui.Text(c, t.Title).FontSize(23).Bold()
	muted(c, progressLabel(t.Progress)+" · "+matchLabel(t.MatchStatus))
	if t.Description != "" {
		ui.Text(c, t.Description).FontSize(13)
	}
	info(c, [][2]string{{"PJ utama", t.Assignee.DisplayName}, {"Tenggat tugas", taskDate(t.DeadlineDate, t.DeadlineAt, t.TimeZone)}, {"Posting", taskDate(t.PostingDate, t.PostingAt, t.TimeZone)}, {"Space", a.taskSpaceNames(*t)}})
	ui.Text(c, "Hasil yang diharapkan").Bold()
	ui.Text(c, t.ExpectedOutput).FontSize(13)
	if len(t.Participants) > 0 {
		ui.Text(c, "Delegasi tambahan").Bold()
		for _, p := range t.Participants {
			muted(c, p.DisplayName)
		}
	}
	if t.Permissions.Manage && spaceButton(c, "Ubah tugas / delegasi", "edit", false).Disabled(a.taskWriteBlocked()).FillWidth().Clicked() {
		a.editTask(*t)
	}
	if t.Permissions.Progress || t.Permissions.Manage {
		a.taskProgressActions(c, *t)
	}
	a.taskEvidenceView(c, *t)
	a.taskHelpView(c, *t)
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	ui.Text(c, "Percakapan tugas").Bold()
	for _, comment := range t.Comments {
		ui.Column(c.Key(comment.ID)).FillWidth().Gap(3).Children(func() {
			ui.Text(c, comment.Author.DisplayName).FontSize(12).Bold()
			ui.Text(c, comment.Body).FontSize(12)
			muted(c, taskTimestamp(comment.CreatedAt, t.TimeZone)).FontSize(10)
		})
	}
	if t.Permissions.Comment {
		taskLongInput(c, "task-comment", "Komentar", &a.tasks.comment)
		if spaceButton(c, "Kirim komentar", "send", false).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.comment) == "").Clicked() {
			body := a.tasks.comment
			a.mutateTask("Komentar", func(s TasksAPI) error {
				_, err := s.AddTaskComment(context.Background(), tid, api.TaskCommentRequest{Body: body})
				return err
			}, func() { a.tasks.comment = "" })
		}
	}
	ui.Text(c, "Riwayat perubahan").Bold()
	if !a.tasks.historyLoaded {
		if spaceButton(c, "Muat riwayat", "refresh", false).Disabled(a.tasks.historyBusy).Clicked() {
			a.loadTaskHistory(tid)
		}
	}
	if a.tasks.historyErr != "" {
		ui.Text(c, a.tasks.historyErr).FontSize(12).TextColor(c.Theme().Danger)
	}
	for _, e := range a.tasks.history {
		ui.Column(c.Key(e.ID)).FillWidth().Gap(3).Children(func() {
			ui.Text(c, e.Actor.DisplayName+" · "+taskEventLabel(e.Action)).FontSize(12)
			if e.Reason != "" {
				muted(c, e.Reason)
			}
			muted(c, taskTimestamp(e.CreatedAt, t.TimeZone)).FontSize(10)
		})
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
func (a *App) taskProgressActions(c *ui.Context, t api.Task) {
	ui.Column(c).FillWidth().Gap(8).Disabled(a.taskWriteBlocked()).Children(func() {
		if t.Progress == api.TaskTodo || t.Progress == api.TaskDoing {
			label := "Mulai kerjakan"
			next := api.TaskDoing
			if t.Progress == api.TaskDoing {
				label = "Kembalikan ke belum mulai"
				next = api.TaskTodo
			}
			if spaceButton(c, label, "task", false).FillWidth().Clicked() {
				a.mutateTask("Progres", func(s TasksAPI) error {
					_, e := s.MoveTask(context.Background(), t.ID, api.TaskMoveRequest{Progress: next, ExpectedVersion: t.Version})
					return e
				}, nil)
			}
		}
		if t.Permissions.Manage {
			taskInput(c, "task-reason", "Alasan pembukaan ulang / pembatalan", &a.tasks.reason)
			if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
				if spaceButton(c, "Buka kembali tugas", "refresh", false).FillWidth().Disabled(strings.TrimSpace(a.tasks.reason) == "").Clicked() {
					reason := a.tasks.reason
					a.mutateTask("Pembukaan tugas", func(s TasksAPI) error {
						_, e := s.MoveTask(context.Background(), t.ID, api.TaskMoveRequest{Progress: api.TaskTodo, ExpectedVersion: t.Version, Reason: reason})
						return e
					}, nil)
				}
			} else if spaceButton(c, "Batalkan tugas", "close", false).FillWidth().Disabled(strings.TrimSpace(a.tasks.reason) == "").Clicked() {
				reason := a.tasks.reason
				a.mutateTask("Pembatalan tugas", func(s TasksAPI) error {
					_, e := s.MoveTask(context.Background(), t.ID, api.TaskMoveRequest{Progress: api.TaskCancelled, ExpectedVersion: t.Version, Reason: reason})
					return e
				}, nil)
			}
		}
	})
}
func (a *App) taskEvidenceView(c *ui.Context, t api.Task) {
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	ui.Text(c, "Bukti pekerjaan").Bold()
	muted(c, "Tinjauan mencocokkan hasil dengan tugas. Waktu penyerahan asli tetap tersimpan.")
	if t.Permissions.Review && len(t.Evidence) > 0 {
		taskInput(c, "review-reason", "Alasan pencocokan / koreksi *", &a.tasks.reason)
	}
	for i, e := range t.Evidence {
		ui.Column(c.Key(e.ID)).FillWidth().Gap(6).Padding(10).Radius(8).Border(1, c.Theme().Border).Children(func() {
			ui.Text(c, matchLabel(e.MatchStatus)).Bold().FontSize(12)
			if e.URL != "" {
				ui.Text(c, e.URL).FontSize(11)
				ui.Link(c, "Buka tautan hasil", e.URL).FontSize(12).TextColor(c.Theme().Accent)
			}
			if e.Note != "" {
				ui.Text(c, e.Note).FontSize(12)
			}
			muted(c, e.SubmittedBy.DisplayName+" · "+taskTimestamp(e.SubmittedAt, t.TimeZone)).FontSize(10)
			if e.ReviewReason != "" {
				muted(c, "Alasan: "+e.ReviewReason)
			}
			if t.Permissions.Review && i == len(t.Evidence)-1 && t.Progress != api.TaskCancelled {
				if spaceButton(c, "Bukti sesuai", "check", false).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.reason) == "" || e.MatchStatus == api.MatchVerified).FillWidth().Clicked() {
					reason := a.tasks.reason
					a.mutateTask("Pencocokan bukti", func(s TasksAPI) error {
						_, err := s.ReviewTaskEvidence(context.Background(), t.ID, e.ID, api.TaskEvidenceReviewRequest{Matched: true, Reason: reason, ExpectedVersion: t.Version})
						return err
					}, nil)
				}

				if spaceButton(c, "Bukti tidak sesuai", "close", false).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.reason) == "" || e.MatchStatus == api.MatchRejected).FillWidth().Clicked() {
					reason := a.tasks.reason
					a.mutateTask("Pencocokan bukti", func(s TasksAPI) error {
						_, err := s.ReviewTaskEvidence(context.Background(), t.ID, e.ID, api.TaskEvidenceReviewRequest{Matched: false, Reason: reason, ExpectedVersion: t.Version})
						return err
					}, nil)
				}
			}
		})
	}
	if (t.Permissions.Progress || t.Permissions.Manage) && t.Progress != api.TaskCancelled && t.Progress != api.TaskDone && t.MatchStatus != api.MatchPending {
		taskInput(c, "evidence-url", "Tautan hasil (https://…)", &a.tasks.evidenceURL)
		taskLongInput(c, "evidence-note", "Catatan hasil", &a.tasks.evidenceNote)
		if spaceButton(c, "Serahkan hasil", "send", true).FillWidth().Disabled(a.taskWriteBlocked() || (strings.TrimSpace(a.tasks.evidenceURL) == "" && strings.TrimSpace(a.tasks.evidenceNote) == "")).Clicked() {
			req := api.TaskEvidenceRequest{URL: a.tasks.evidenceURL, Note: a.tasks.evidenceNote, ExpectedVersion: t.Version}
			a.mutateTask("Penyerahan hasil", func(s TasksAPI) error { _, err := s.SubmitTaskEvidence(context.Background(), t.ID, req); return err }, func() { a.tasks.evidenceURL = ""; a.tasks.evidenceNote = "" })
		}
	}
}
func (a *App) taskHelpView(c *ui.Context, t api.Task) {
	ui.Text(c, "Bantu pekerjaan ini").Bold()
	for _, h := range t.HelpRequests {
		ui.Column(c.Key(h.ID)).FillWidth().Gap(5).Children(func() {
			ui.Text(c, h.User.DisplayName+" · "+taskHelpLabel(h.Status)).FontSize(12)
			if h.Message != "" {
				muted(c, h.Message)
			}
			if t.Permissions.Manage && h.Status == "pending" {
				ui.Row(c).Gap(6).Children(func() {
					for _, decision := range []struct {
						label    string
						approved bool
					}{{"Terima bantuan", true}, {"Tolak bantuan", false}} {
						if spaceButton(c, decision.label, "users", false).Disabled(a.taskWriteBlocked()).Clicked() {
							req := api.TaskHelpDecisionRequest{Approved: decision.approved, ExpectedVersion: t.Version}
							a.mutateTask("Keputusan bantuan", func(s TasksAPI) error { _, err := s.DecideTaskHelp(context.Background(), t.ID, h.ID, req); return err }, nil)
						}
					}
				})
			}
		})
	}
	if t.Permissions.Help {
		taskInput(c, "task-help", "Pesan penawaran bantuan", &a.tasks.help)
		if spaceButton(c, "Tawarkan diri", "users", false).Disabled(a.taskWriteBlocked()).FillWidth().Clicked() {
			message := a.tasks.help
			a.mutateTask("Penawaran bantuan", func(s TasksAPI) error {
				_, err := s.RequestTaskHelp(context.Background(), t.ID, api.TaskHelpCreateRequest{Message: message})
				return err
			}, func() { a.tasks.help = "" })
		}
	}
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
		if !strings.Contains(strings.ToLower(task.Title+" "+task.Category+" "+task.Assignee.DisplayName), q) {
			continue
		}
		n++
		if spaceButton(c.Key(task.ID), task.Title, "task", false).FillWidth().Clicked() {
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
								}{{"Tenggat", task.DeadlineDate, task.DeadlineAt}, {"Posting", task.PostingDate, task.PostingAt}} {
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
