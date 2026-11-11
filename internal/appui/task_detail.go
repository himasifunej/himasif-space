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
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// checklistAPI is optional on TasksAPI so test doubles without checklists
// still satisfy it; the HTTP client implements it.
type checklistAPI interface {
	AddTaskChecklist(context.Context, id.ID, string) (api.TaskChecklistItem, error)
	UpdateTaskChecklist(context.Context, id.ID, id.ID, api.TaskChecklistRequest) error
	DeleteTaskChecklist(context.Context, id.ID, id.ID) error
}

func (a *App) checklistService() (checklistAPI, bool) {
	s, ok := a.taskService()
	if !ok {
		return nil, false
	}
	cl, ok := s.(checklistAPI)
	return cl, ok
}

// detailIcons are the task modal's icons from canvas A (stroke 1.7).
var detailIcons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"submit":    `<circle cx="12" cy="12" r="9"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`,
		"tag":       `<path d="M3 12V4h8l9 9-8 8-9-9Z"/><circle cx="7.5" cy="8.5" r="1.2"/>`,
		"checklist": `<rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`,
		"userplus":  `<circle cx="9" cy="8" r="3.5"/><path d="M3 20a6 6 0 0 1 12 0M19 8v6M16 11h6"/>`,
		"clip":      `<path d="m21 11-8.5 8.5a5 5 0 0 1-7-7L14 4a3.5 3.5 0 0 1 5 5l-8.5 8.5a2 2 0 0 1-3-3L15 7"/>`,
		"download":  `<path d="M12 4v11M7 10l5 5 5-5M5 20h14"/>`,
		"help":      `<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5v.7M12 17h.01"/>`,
		"lines":     `<path d="M4 6h16M4 12h16M4 18h10"/>`,
		"comment":   `<path d="M4 5h16v11H9l-5 4Z"/>`,
		"more":      `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
		"bell":      `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10 21h4"/>`,
	}
	icons := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return icons
}()

func detailIcon(name string) *ui.SVG {
	if icon := detailIcons[name]; icon != nil {
		return icon
	}
	return chromeIcon(name)
}

// outlineButton is canvas A's .btn-outline: icon and label in a bordered
// button. primary fills it with the accent, as "Serahkan hasil".
func outlineButton(c *ui.Context, label, icon string, primary bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Gap(8).Height(34).Padding(0, 12).Radius(8).Border(1, t.Border).TextColor(t.Text).Shrink(0)
	switch {
	case primary:
		b.Background(t.Accent).BorderColor(t.Accent).TextColor(t.AccentText)
		if b.Hovered() {
			b.Background(t.AccentHover)
		}
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		if icon != "" {
			ui.Icon(c, detailIcon(icon)).Size(15, 15).Shrink(0)
		}
		label := ui.Text(c, label).FontSize(13).MaxLines(1)
		if primary {
			label.FontWeight(600)
		}
	})
	return b
}

// labelChip is a colored label as on the cards.
func labelChip(c *ui.Context, key any, name string) ui.Element {
	return ui.Text(c.Key(key), name).FontSize(11).Bold().Padding(4, 9).Radius(6).Background(tagColor(name)).TextColor(ui.Hex("#24221c")).MaxLines(1)
}

func sectionTitle(c *ui.Context, icon, title, note string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, detailIcon(icon)).Size(16, 16).Shrink(0)
		ui.Text(c, title).FontSize(14).Bold()
		if note != "" {
			ui.Text(c, note).FontSize(11).TextColor(c.Theme().TextMuted)
		}
	})
}

// boardToolbar is the space board's toolbar on canvas A: card colors, a
// filter popover (text, cancelled tasks, reload) and "Saya saja".
func (a *App) boardToolbar(c *ui.Context) {
	t := c.Theme()
	phone := isMobile(c)
	ui.Row(c.Key("board-toolbar")).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		if !phone {
			a.cardColorChoices(c)
		}
		filtered := strings.TrimSpace(a.tasks.filter) != "" || a.tasks.showCancelled
		label := "Filter"
		if filtered {
			label = "Filter aktif"
		}
		f := outlineButton(c.Key("board-filter"), label, "filter", false)
		if filtered {
			f.Background(t.SurfaceHover)
		}
		if f.Clicked() {
			a.tasks.filterOpen = !a.tasks.filterOpen
		}
		popover(c, f, &a.tasks.filterOpen, 280, func() {
			ui.Text(c, "Filter board").Bold()
			in := ui.TextInput(c.Key("task-filter"), &a.tasks.filter).Label("Filter tugas").Placeholder("Judul, label, atau PJ…").FillWidth()
			if in.Changed() {
				a.projectTasks()
			}
			if spaceButton(c.Key("show-cancelled"), "Tampilkan yang dibatalkan", "archive", a.tasks.showCancelled).FillWidth().Clicked() {
				a.tasks.showCancelled = !a.tasks.showCancelled
				a.projectTasks()
			}
			if filtered && spaceButton(c.Key("filter-reset"), "Hapus filter", "close", false).FillWidth().Clicked() {
				a.tasks.filter, a.tasks.showCancelled = "", false
				a.projectTasks()
			}
			if spaceButton(c.Key("filter-reload"), "Muat ulang tugas", "refresh", false).FillWidth().Disabled(a.tasks.loading || a.tasks.busy).Clicked() {
				a.loadTasks()
			}
		})
		mine := outlineButton(c.Key("board-mine"), "Saya saja", "", false)
		if a.tasks.mineOnly {
			mine.Background(t.SurfaceHover).BorderColor(t.Accent)
		}
		if mine.Clicked() {
			a.tasks.mineOnly = !a.tasks.mineOnly
			a.projectTasks()
		}
	})
}

func (a *App) taskChecklistView(c *ui.Context, t api.Task) {
	th := c.Theme()
	cl, ok := a.checklistService()
	if !ok {
		return
	}
	edit := (t.Permissions.Manage || t.Permissions.Progress) && t.Progress != api.TaskCancelled
	if len(t.Checklist) == 0 && !edit {
		return
	}
	done := 0
	for _, item := range t.Checklist {
		if item.Done {
			done++
		}
	}
	ui.Column(c.Key("checklist")).FillWidth().Gap(8).Children(func() {
		note := ""
		if len(t.Checklist) > 0 {
			note = fmt.Sprintf("%d/%d", done, len(t.Checklist))
		}
		sectionTitle(c, "checklist", "Checklist", note)
		if len(t.Checklist) > 0 {
			track := ui.Row(c).FillWidth().Height(6).Radius(3).Background(th.SurfaceHover)
			width := track.Bounds().W
			if width == 0 {
				c.After(0)
			}
			track.Children(func() {
				if width > 0 && done > 0 {
					ui.Box(c).Width(width * float32(done) / float32(len(t.Checklist))).FillHeight().Radius(3).Background(th.Success)
				}
			})
		}
		for _, item := range t.Checklist {
			item := item
			ui.Row(c.Key(item.ID)).FillWidth().Gap(10).AlignItems(ui.Center).MinHeight(34).Children(func() {
				box := ui.ButtonBase(c.Key("tick")).Label("Centang "+item.Body).Size(20, 20).Radius(5).Border(1, th.Border).Center().Disabled(!edit || a.taskWriteBlocked())
				if item.Done {
					box.Background(th.Accent).BorderColor(th.Accent)
				}
				box.FocusRing(box.FocusVisible()).Children(func() {
					if item.Done {
						ui.Icon(c, spaceIcon("check")).Size(14, 14).TextColor(th.AccentText)
					}
				})
				if box.Clicked() {
					next := !item.Done
					a.mutateTask("Checklist", func(TasksAPI) error {
						return cl.UpdateTaskChecklist(context.Background(), t.ID, item.ID, api.TaskChecklistRequest{Done: &next})
					}, nil)
				}
				label := ui.Text(c, item.Body).FontSize(13).Grow(1).MinWidth(0)
				if item.Done {
					label.TextColor(th.TextMuted).Strikethrough()
				}
				if edit && spaceIconButton(c, "Hapus "+item.Body, "close").Size(26, 26).Disabled(a.taskWriteBlocked()).Clicked() {
					a.mutateTask("Checklist", func(TasksAPI) error {
						return cl.DeleteTaskChecklist(context.Background(), t.ID, item.ID)
					}, nil)
				}
			})
		}
		if edit {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c.Key("checklist-new"), &a.tasks.checklistNew).Label("Item checklist baru").Placeholder("Tambah item lalu Enter").Grow(1).MinWidth(0).Bind(&a.tasks.checklistRef)
				add := spaceButton(c, "Tambah", "plus", false).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.checklistNew) == "")
				if (in.Submitted() || add.Clicked()) && strings.TrimSpace(a.tasks.checklistNew) != "" {
					body := strings.TrimSpace(a.tasks.checklistNew)
					a.mutateTask("Checklist", func(TasksAPI) error {
						_, err := cl.AddTaskChecklist(context.Background(), t.ID, body)
						return err
					}, func() { a.tasks.checklistNew = "" })
				}
			})
		}
	})
}

// taskWriteFrom is the task's current metadata as a write request, so one
// field can change without the full editor.
func taskWriteFrom(t api.Task) api.TaskWriteRequest {
	return api.TaskWriteRequest{OwnerDivisionID: t.OwnerDivisionID, SpaceIDs: slices.Clone(t.SpaceIDs), Title: t.Title, Description: t.Description, ExpectedOutput: t.ExpectedOutput, Category: t.Category, AssigneeID: t.Assignee.ID, PlannedStartDate: t.PlannedStartDate, PlannedStartAt: t.PlannedStartAt, DeadlineDate: t.DeadlineDate, DeadlineAt: t.DeadlineAt, PostingDate: t.PostingDate, PostingAt: t.PostingAt, ExpectedVersion: t.Version}
}

// quickUpdateTask saves one change to a task's metadata (label, PJ,
// description) through the same update the editor uses.
func (a *App) quickUpdateTask(t api.Task, action string, change func(*api.TaskWriteRequest), after func()) {
	req := taskWriteFrom(t)
	change(&req)
	tid := t.ID
	a.mutateTask(action, func(s TasksAPI) error { _, err := s.UpdateTask(context.Background(), tid, req); return err }, after)
}

// taskDetailMain is the task's own content, as the canvas's modal: title,
// actions, PJ/label/deadline/output, description, checklist, attachments
// and evidence. taskDetailSide holds comments and activity.
func (a *App) taskDetailMain(c *ui.Context, t *api.Task) {
	th := c.Theme()
	task := *t
	if !a.tasks.inModal {
		// The phone's inspector has no modal header: status and code here.
		ui.Row(c.Key("status-row")).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			a.taskStatusPill(c, task)
			muted(c, a.taskCode(task)).FontSize(12)
		})
	}
	ui.Text(c, t.Title).FontSize(23).Bold()
	if t.LocalPending {
		muted(c, "Tersimpan di perangkat — belum tersinkron. Isi resmi server belum berubah.")
	}
	a.taskActionRow(c, task)
	a.taskFieldGrid(c, task)
	if len(t.Participants) > 0 {
		names := make([]string, 0, len(t.Participants))
		for _, p := range t.Participants {
			names = append(names, p.DisplayName)
		}
		muted(c, "Delegasi tambahan: "+strings.Join(names, ", ")).FontSize(12)
	}
	muted(c, "Mulai "+taskDate(t.PlannedStartDate, t.PlannedStartAt, t.TimeZone)+" · Posting "+taskDate(t.PostingDate, t.PostingAt, t.TimeZone)+" · "+matchLabel(t.MatchStatus)).FontSize(12)

	ui.Column(c.Key("description")).FillWidth().Gap(8).Children(func() {
		sectionTitle(c, "lines", "Deskripsi", "")
		if t.Permissions.Manage && !a.offlineMode() {
			ui.TextArea(c.Key("task-desc"), &a.tasks.descDraft).Label("Deskripsi tugas").Placeholder("Tambahkan deskripsi…").FillWidth().Height(90)
			if a.tasks.descDraft != t.Description {
				ui.Row(c).Gap(8).Children(func() {
					if primarySpaceButton(c, "Simpan deskripsi", "check").Disabled(a.taskWriteBlocked()).Clicked() {
						desc := a.tasks.descDraft
						a.quickUpdateTask(task, "Deskripsi", func(r *api.TaskWriteRequest) { r.Description = desc }, nil)
					}
					if spaceButton(c, "Batal", "", false).Clicked() {
						a.tasks.descDraft = t.Description
					}
				})
			}
		} else {
			desc := t.Description
			box := ui.Column(c).FillWidth().Padding(12).Radius(8).Border(1, th.Border)
			box.Children(func() {
				if desc == "" {
					muted(c, "Belum ada deskripsi.").FontSize(13)
				} else {
					ui.Text(c, desc).FontSize(13)
				}
			})
		}
	})
	a.taskChecklistView(c, task)
	a.taskEvidenceView(c, task)
	a.taskFilesView(c, task)
	a.taskHelpView(c, task)
	a.taskReopenView(c, task)
	if t.Permissions.Manage && a.notificationClient() != nil {
		ui.Column(c.Key("reminder")).FillWidth().Gap(8).Padding(10, 12).Radius(8).Border(1, th.Border).Children(func() {
			ui.Collapsible(c, "Pengaturan pengingat tugas ini", &a.tasks.reminderOpen, func() {
				ui.Column(c).FillWidth().Gap(8).Padding(8, 0, 0, 0).Children(func() { a.taskNotificationControls(c, task) })
			})
		})
	}
}

// taskActionRow is the canvas's row of actions under the title. Only
// actions the API and the user's permissions support are shown.
func (a *App) taskActionRow(c *ui.Context, t api.Task) {
	open := t.Progress != api.TaskCancelled && t.Progress != api.TaskDone
	canSubmit := (t.Permissions.Progress || t.Permissions.Manage) && open && t.MatchStatus != api.MatchPending && !a.offlineMode()
	_, hasChecklist := a.checklistService()
	ui.Row(c.Key("actions")).FillWidth().Wrap().Gap(8).Disabled(a.taskWriteBlocked()).Children(func() {
		if canSubmit && outlineButton(c.Key("act-submit"), "Serahkan hasil", "submit", true).Clicked() {
			a.tasks.evidenceOpen = !a.tasks.evidenceOpen
		}
		if t.Permissions.Manage {
			b := outlineButton(c.Key("act-label"), "Label", "tag", false)
			if b.Clicked() {
				a.tasks.labelDraft = t.Category
				a.tasks.labelOpen = !a.tasks.labelOpen
			}
			a.taskLabelPopover(c, b, t)
		}
		if hasChecklist && (t.Permissions.Manage || t.Permissions.Progress) && t.Progress != api.TaskCancelled {
			if outlineButton(c.Key("act-checklist"), "Checklist", "checklist", false).Clicked() {
				a.tasks.checklistRef.Focus(c)
			}
		}
		if t.Permissions.Manage {
			b := outlineButton(c.Key("act-pj"), "PJ", "userplus", false)
			if b.Clicked() {
				a.tasks.pjReason = ""
				a.tasks.pjOpen = !a.tasks.pjOpen
				a.loadTaskMembers(t.OwnerDivisionID)
			}
			a.taskPJPopover(c, b, t)
		}
		if canSubmit && outlineButton(c.Key("act-attach"), "Lampiran", "clip", false).Clicked() {
			a.tasks.evidenceOpen = true
		}
		if a.taskRemindable(t) {
			label, ready := a.pingRemindLabel(t.ID, firstName(t.Assignee.DisplayName), c.Now())
			b := outlineButton(c.Key("act-ping"), label, "ping", false).Tooltip("Kirim ping tentang tugas ini ke " + t.Assignee.DisplayName).Disabled(!ready || a.conn != connOnline)
			if !ready {
				c.After(pingRemindGap)
			}
			if b.Clicked() {
				a.remindTask(t)
			}
		}
		if t.Permissions.Help && outlineButton(c.Key("act-help"), "Ajukan bantuan", "help", false).Clicked() {
			a.tasks.helpOpen = !a.tasks.helpOpen
		}
	})
}

func (a *App) taskLabelPopover(c *ui.Context, anchor ui.Element, t api.Task) {
	ui.PopoverBase(c, anchor, &a.tasks.labelOpen, func(panel ui.Element) {
		th := c.Theme()
		panel.Margin(6, 0, 0, 0).Width(280).Padding(12).Gap(8).Radius(12).Background(th.Surface).Border(1, th.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		ui.Text(c, "Label").Bold()
		in := ui.TextInput(c.Key("label-input"), &a.tasks.labelDraft).Label("Label tugas").Placeholder("Desain, Urgent").FillWidth().AutoFocus()
		muted(c, "Pisahkan beberapa label dengan koma.").FontSize(11)
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for i, l := range taskLabels(a.tasks.labelDraft) {
				labelChip(c, i, l)
			}
		})
		if (in.Submitted() || primarySpaceButton(c, "Simpan label", "check").FillWidth().Disabled(a.taskWriteBlocked() || a.tasks.labelDraft == t.Category).Clicked()) && a.tasks.labelDraft != t.Category {
			category := strings.Join(taskLabels(a.tasks.labelDraft), ", ")
			a.tasks.labelOpen = false
			a.quickUpdateTask(t, "Label", func(r *api.TaskWriteRequest) { r.Category = category }, nil)
		}
	}).AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft)
}

func (a *App) taskPJPopover(c *ui.Context, anchor ui.Element, t api.Task) {
	ui.PopoverBase(c, anchor, &a.tasks.pjOpen, func(panel ui.Element) {
		th := c.Theme()
		panel.Margin(6, 0, 0, 0).Width(290).MaxHeight(420).Padding(12).Gap(8).Radius(12).Background(th.Surface).Border(1, th.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		ui.Text(c, "Ganti PJ utama").Bold()
		ui.TextInput(c.Key("pj-reason"), &a.tasks.pjReason).Label("Alasan perubahan PJ").Placeholder("Alasan perubahan PJ *").FillWidth()
		did := t.OwnerDivisionID
		if a.tasks.membersBusy[did] {
			muted(c, "Memuat anggota…")
		}
		if e := a.tasks.memberErrors[did]; e != "" {
			ui.Text(c, e).FontSize(12).TextColor(th.Danger)
		}
		ui.Scroll(c).MaxHeight(240).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				for _, m := range a.tasks.members[did] {
					m := m
					b := spaceButton(c.Key(m.UserID), m.DisplayName, "user", m.UserID == t.Assignee.ID).FillWidth().Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.pjReason) == "" || m.UserID == t.Assignee.ID)
					if b.Clicked() {
						reason := strings.TrimSpace(a.tasks.pjReason)
						a.tasks.pjOpen = false
						a.quickUpdateTask(t, "PJ", func(r *api.TaskWriteRequest) { r.AssigneeID, r.Reason = m.UserID, reason }, nil)
					}
				}
			})
		})
	}).AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft)
}

// taskFieldGrid is the canvas's PJ · Label · Tenggat · Hasil yang diminta.
func (a *App) taskFieldGrid(c *ui.Context, t api.Task) {
	th := c.Theme()
	cell := func(key, label string, body func()) {
		ui.Column(c.Key(key)).MinWidth(140).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(th.TextMuted)
			body()
		})
	}
	ui.Row(c.Key("fields")).FillWidth().Wrap().Gap(28).Children(func() {
		cell("pj", "PJ", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if t.Assignee.DisplayName == "" {
					muted(c, "Belum ada PJ")
					return
				}
				avatar(c, t.Assignee.DisplayName, 24)
				ui.Text(c, t.Assignee.DisplayName).FontSize(13)
			})
		})
		cell("label", "Label", func() {
			ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				labels := taskLabels(t.Category)
				if len(labels) == 0 {
					muted(c, "—")
				}
				for i, l := range labels {
					labelChip(c, i, l)
				}
			})
		})
		cell("due", "Tenggat", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 10).Radius(7).Background(th.SurfaceHover).Children(func() {
				ui.Text(c, taskDate(t.DeadlineDate, t.DeadlineAt, t.TimeZone)).FontSize(13)
				switch {
				case taskLate(t, c.Now()):
					ui.Text(c, "Lewat").FontSize(11).Bold().Padding(2, 8).Radius(5).Background(th.Danger).TextColor(ui.Hex("#ffffff"))
				case t.Progress == api.TaskDone:
					ui.Text(c, "Selesai").FontSize(11).Bold().Padding(2, 8).Radius(5).Background(th.Success).TextColor(ui.Hex("#24221c"))
				}
			})
		})
		cell("output", "Hasil yang diminta", func() {
			out := t.ExpectedOutput
			if out == "" {
				out = "—"
			}
			ui.Text(c, out).FontSize(13).MaxWidth(320)
		})
	})
}

// taskReopenView reopens a finished or cancelled task with a reason.
func (a *App) taskReopenView(c *ui.Context, t api.Task) {
	if !t.Permissions.Manage || (t.Progress != api.TaskDone && t.Progress != api.TaskCancelled) || a.offlineMode() {
		return
	}
	ui.Column(c.Key("reopen")).FillWidth().Gap(8).Disabled(a.taskWriteBlocked()).Children(func() {
		taskInput(c, "task-reason", "Alasan membuka kembali *", &a.tasks.reason)
		if spaceButton(c, "Buka kembali tugas", "refresh", false).Disabled(strings.TrimSpace(a.tasks.reason) == "").Clicked() {
			reason := a.tasks.reason
			a.mutateTask("Pembukaan tugas", func(s TasksAPI) error {
				_, e := s.MoveTask(context.Background(), t.ID, api.TaskMoveRequest{Progress: api.TaskTodo, ExpectedVersion: t.Version, Reason: reason})
				return e
			}, nil)
		}
	})
}

func (a *App) taskEvidenceView(c *ui.Context, t api.Task) {
	th := c.Theme()
	canSubmit := (t.Permissions.Progress || t.Permissions.Manage) && t.Progress != api.TaskCancelled && t.Progress != api.TaskDone && t.MatchStatus != api.MatchPending && !a.offlineMode()
	ui.Column(c.Key("evidence")).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "clip", "Lampiran dan bukti", "")
			ui.Box(c).Grow(1)
			if canSubmit && !a.tasks.evidenceOpen && spaceButton(c, "Tambah", "", false).Border(1, th.Border).Clicked() {
				a.tasks.evidenceOpen = true
			}
		})
		if len(t.Evidence) == 0 && !a.tasks.evidenceOpen {
			muted(c, "Belum ada. PJ menyerahkan tautan atau catatan hasil lewat “Serahkan hasil”.").FontSize(13)
		}
		if t.Permissions.Review && len(t.Evidence) > 0 {
			taskInput(c, "review-reason", "Alasan pencocokan / koreksi *", &a.tasks.reason)
		}
		for i, e := range t.Evidence {
			ui.Row(c.Key(e.ID)).FillWidth().Gap(12).Padding(10).Radius(8).Border(1, th.Border).AlignItems(ui.Start).Children(func() {
				kind := "CATATAN"
				if e.URL != "" {
					kind = "TAUTAN"
				}
				ui.Text(c, kind).FontSize(9).Bold().TextColor(th.Accent).Padding(10, 6).Radius(6).Background(th.SurfaceHover).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					if e.URL != "" {
						ui.Link(c, e.URL, e.URL).FontSize(13).TextColor(th.Text).MaxLines(1)
					}
					if e.Note != "" {
						ui.Text(c, e.Note).FontSize(13)
					}
					status := ui.Text(c, matchLabel(e.MatchStatus)).FontSize(11)
					switch e.MatchStatus {
					case api.MatchPending:
						status.TextColor(th.Warning)
					case api.MatchVerified:
						status.TextColor(th.Success)
					case api.MatchRejected:
						status.TextColor(th.Danger)
					}
					muted(c, "Diserahkan "+e.SubmittedBy.DisplayName+" · "+taskTimestamp(e.SubmittedAt, t.TimeZone)).FontSize(11)
					if e.ReviewReason != "" {
						muted(c, "Alasan: "+e.ReviewReason).FontSize(11)
					}
					if t.Permissions.Review && i == len(t.Evidence)-1 && t.Progress != api.TaskCancelled {
						ui.Row(c).Gap(6).Wrap().Children(func() {
							if primarySpaceButton(c, "Bukti sesuai", "check").Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.reason) == "" || e.MatchStatus == api.MatchVerified).Clicked() {
								reason := a.tasks.reason
								a.mutateTask("Pencocokan bukti", func(s TasksAPI) error {
									_, err := s.ReviewTaskEvidence(context.Background(), t.ID, e.ID, api.TaskEvidenceReviewRequest{Matched: true, Reason: reason, ExpectedVersion: t.Version})
									return err
								}, nil)
							}
							if spaceButton(c, "Bukti tidak sesuai", "close", false).Border(1, th.Border).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.reason) == "" || e.MatchStatus == api.MatchRejected).Clicked() {
								reason := a.tasks.reason
								a.mutateTask("Pencocokan bukti", func(s TasksAPI) error {
									_, err := s.ReviewTaskEvidence(context.Background(), t.ID, e.ID, api.TaskEvidenceReviewRequest{Matched: false, Reason: reason, ExpectedVersion: t.Version})
									return err
								}, nil)
							}
						})
					}
				})
			})
		}
		if canSubmit && a.tasks.evidenceOpen {
			ui.Column(c.Key("evidence-form")).FillWidth().Gap(10).Padding(12).Radius(8).Border(1, th.Accent).Children(func() {
				muted(c, "Waktu penyerahan asli tetap tersimpan. Tinjauan mencocokkan hasil dengan tugas.").FontSize(12)
				taskInput(c, "evidence-url", "Tautan hasil (https://…)", &a.tasks.evidenceURL)
				taskLongInput(c, "evidence-note", "Catatan hasil", &a.tasks.evidenceNote)
				ui.Row(c).Gap(8).Children(func() {
					if primarySpaceButton(c, "Serahkan hasil", "send").Disabled(a.taskWriteBlocked() || (strings.TrimSpace(a.tasks.evidenceURL) == "" && strings.TrimSpace(a.tasks.evidenceNote) == "")).Clicked() {
						req := api.TaskEvidenceRequest{URL: a.tasks.evidenceURL, Note: a.tasks.evidenceNote, ExpectedVersion: t.Version}
						a.mutateTask("Penyerahan hasil", func(s TasksAPI) error { _, err := s.SubmitTaskEvidence(context.Background(), t.ID, req); return err }, func() {
							a.tasks.evidenceURL, a.tasks.evidenceNote, a.tasks.evidenceOpen = "", "", false
						})
					}
					if spaceButton(c, "Batal", "", false).Clicked() {
						a.tasks.evidenceOpen = false
					}
				})
			})
		}
	})
}

func (a *App) taskHelpView(c *ui.Context, t api.Task) {
	if len(t.HelpRequests) == 0 && !(t.Permissions.Help && a.tasks.helpOpen) {
		return
	}
	ui.Column(c.Key("help")).FillWidth().Gap(8).Children(func() {
		sectionTitle(c, "help", "Bantuan", "")
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
		if t.Permissions.Help && a.tasks.helpOpen {
			taskInput(c, "task-help", "Pesan pengajuan bantuan", &a.tasks.help)
			if primarySpaceButton(c, "Kirim pengajuan", "users").Disabled(a.taskWriteBlocked()).Clicked() {
				message := a.tasks.help
				a.mutateTask("Pengajuan bantuan", func(s TasksAPI) error {
					_, err := s.RequestTaskHelp(context.Background(), t.ID, api.TaskHelpCreateRequest{Message: message})
					return err
				}, func() { a.tasks.help, a.tasks.helpOpen = "", false })
			}
		}
	})
}

// activity is one line of the "Komentar dan aktivitas" timeline.
type activity struct {
	key       string
	at        time.Time
	who, text string
	note      string
	comment   bool
}

// taskActivity merges comments with what happened to the task, newest
// first. Before the full history is loaded it is built from data already in
// the task (creation, evidence, reviews, help); afterwards from history.
func (a *App) taskActivity(t api.Task) []activity {
	var out []activity
	for _, cm := range t.Comments {
		out = append(out, activity{key: "c" + cm.ID.String(), at: cm.CreatedAt, who: cm.Author.DisplayName, text: cm.Body, comment: true})
	}
	if a.tasks.historyLoaded && a.tasks.inspected == t.ID {
		for _, e := range a.tasks.history {
			if e.Action == "task.comment" {
				continue
			}
			out = append(out, activity{key: "h" + e.ID.String(), at: e.CreatedAt, who: e.Actor.DisplayName, text: strings.ToLower(taskEventLabel(e.Action)[:1]) + taskEventLabel(e.Action)[1:], note: e.Reason})
		}
	} else {
		if t.CreatedBy.DisplayName != "" {
			text := "membuat tugas"
			if t.Assignee.DisplayName != "" {
				text += " dan menetapkan " + t.Assignee.DisplayName + " sebagai PJ"
			}
			out = append(out, activity{key: "created", at: t.CreatedAt, who: t.CreatedBy.DisplayName, text: text})
		}
		for _, e := range t.Evidence {
			out = append(out, activity{key: "e" + e.ID.String(), at: e.SubmittedAt, who: e.SubmittedBy.DisplayName, text: "menyerahkan hasil"})
			if e.ReviewedAt != nil && e.ReviewedBy != nil {
				out = append(out, activity{key: "r" + e.ID.String(), at: *e.ReviewedAt, who: e.ReviewedBy.DisplayName, text: "mencocokkan bukti: " + strings.ToLower(matchLabel(e.MatchStatus)), note: e.ReviewReason})
			}
		}
		for _, h := range t.HelpRequests {
			out = append(out, activity{key: "b" + h.ID.String(), at: h.CreatedAt, who: h.User.DisplayName, text: "mengajukan bantuan · " + strings.ToLower(taskHelpLabel(h.Status)), note: h.Message})
		}
	}
	slices.SortStableFunc(out, func(x, y activity) int { return y.at.Compare(x.at) })
	return out
}

// taskDetailSide is the canvas's "Komentar dan aktivitas" column.
func (a *App) taskDetailSide(c *ui.Context, t *api.Task) {
	th := c.Theme()
	tid := t.ID
	sectionTitle(c, "comment", "Komentar dan aktivitas", "")
	if t.Permissions.Comment {
		in := ui.TextInput(c.Key("task-comment"), &a.tasks.comment).Label("Komentar").Placeholder("Tulis komentar lalu Enter…").FillWidth().Disabled(a.taskWriteBlocked())
		if in.Submitted() && strings.TrimSpace(a.tasks.comment) != "" {
			body := a.tasks.comment
			a.mutateTask("Komentar", func(s TasksAPI) error {
				_, err := s.AddTaskComment(context.Background(), tid, api.TaskCommentRequest{Body: body})
				return err
			}, func() { a.tasks.comment = "" })
		}
	}
	for _, item := range a.taskActivity(*t) {
		ui.Row(c.Key(item.key)).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
			avatar(c, item.who, 24)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, item.who).FontSize(13).Bold()
				if item.comment {
					ui.Text(c, item.text).FontSize(13).Padding(8, 10).Radius(8).Background(th.Surface).Border(1, th.Border).FillWidth()
				} else {
					ui.Text(c, item.text).FontSize(13)
				}
				if item.note != "" {
					muted(c, item.note).FontSize(12)
				}
				muted(c, taskTimestamp(item.at, t.TimeZone)).FontSize(11)
			})
		})
	}
	if !a.tasks.historyLoaded {
		if spaceButton(c, "Muat riwayat lengkap", "refresh", false).Disabled(a.tasks.historyBusy).Clicked() {
			a.loadTaskHistory(tid)
		}
	}
	if a.tasks.historyErr != "" {
		ui.Text(c, a.tasks.historyErr).FontSize(12).TextColor(th.Danger)
	}
}

// taskDetailModal shows the selected task over the board on tablets and
// desktops, as canvas A: a header with the status pill, short code and
// breadcrumb; the task on the left; comments and activity on the right.
func (a *App) taskDetailModal(c *ui.Context) {
	tid := a.workspace.SelectedID()
	t := a.taskByID(tid)
	if t == nil {
		return
	}
	th := c.Theme()
	w, h := c.Size()
	open := true
	wide := w >= 960
	ui.DialogBase(c.Key("task-detail"), &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		panel.Width(min(float32(1080), w-48)).Height(min(float32(820), h-48)).Column().Radius(14).Background(th.Surface).Border(1, th.Border).Children(func() {
			ui.Row(c).FillWidth().Padding(12, 16).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(th.Border).Children(func() {
				a.taskStatusPill(c, *t)
				ui.Text(c, a.taskCode(*t)).FontSize(12).TextColor(th.TextMuted).Tooltip("Nomor tugas di Space ini")
				crumb := "Board tugas"
				if s := a.taskSpaceNames(*t); s != "" {
					crumb = s + " · " + crumb
				}
				ui.Text(c, crumb).FontSize(12).TextColor(th.TextMuted).MaxLines(1).MinWidth(0)
				ui.Box(c).Grow(1)
				if t.Permissions.Manage {
					more := spaceIconButton(c.Key("detail-more"), "Menu tugas", "more")
					if more.Clicked() {
						a.tasks.moreOpen = !a.tasks.moreOpen
					}
					popover(c, more, &a.tasks.moreOpen, 230, func() {
						if spaceButton(c.Key("m-edit"), "Ubah tugas / delegasi", "edit", false).FillWidth().Disabled(a.taskWriteBlocked()).Clicked() {
							a.tasks.moreOpen = false
							a.editTask(*t)
						}
						if spaceButton(c.Key("m-dup"), "Duplikat", "plus", false).FillWidth().Disabled(a.taskWriteBlocked()).Clicked() {
							a.tasks.moreOpen = false
							a.taskAction(tid, "duplicate")
						}
						if t.Progress != api.TaskCancelled && spaceButton(c.Key("m-cancel"), "Batalkan tugas", "archive", false).FillWidth().TextColor(th.Danger).Disabled(a.taskWriteBlocked()).Clicked() {
							a.tasks.moreOpen = false
							a.taskAction(tid, "cancel")
						}
					})
				}
				if spaceIconButton(c, "Tutup detail tugas", "close").Clicked() {
					open = false
				}
			})
			body := ui.Row(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Stretch)
			if !wide {
				body.Column()
			}
			body.Children(func() {
				if !wide {
					ui.Scroll(c.Key("detail-all")).Grow(1).MinHeight(0).FillWidth().Children(func() {
						ui.Column(c).FillWidth().Padding(22).Gap(18).Children(func() {
							a.tasks.inModal = true
							a.taskDetails(c, tid)
							a.tasks.inModal = false
						})
					})
					return
				}
				ui.Scroll(c.Key("detail-main")).Grow(1).MinWidth(0).FillHeight().Label("Isi tugas").Children(func() {
					ui.Column(c).FillWidth().Padding(24, 26).Gap(20).Children(func() {
						a.tasks.detailSplit, a.tasks.inModal = true, true
						a.taskDetails(c, tid)
						a.tasks.detailSplit, a.tasks.inModal = false, false
					})
				})
				ui.Scroll(c.Key("detail-side")).Width(360).Shrink(0).FillHeight().Background(th.Background).BorderWidth(0, 0, 0, 1).BorderColor(th.Border).Label("Komentar dan aktivitas").Children(func() {
					ui.Column(c).FillWidth().Padding(20).Gap(14).Children(func() {
						if d := a.tasks.draft; d == nil || d.taskID != tid {
							a.taskDetailSide(c, t)
						}
					})
				})
			})
		})
	})
	if !open {
		a.workspace.Close()
	}
}

// taskStatusPill is the header's status with a menu to move the task. Moves
// go through moveTask, which keeps its rules (submitting needs evidence,
// finishing goes through review, reopening needs a reason).
func (a *App) taskStatusPill(c *ui.Context, t api.Task) {
	th := c.Theme()
	lane := taskLane(t)
	color := th.TextMuted
	switch lane {
	case workspaceui.Doing:
		color = th.Accent
	case workspaceui.Review:
		color = th.Warning
	case workspaceui.Done:
		color = th.Success
	}
	if custom := a.chrome.preferences.LaneColors[lane]; custom != "" {
		color = spaceAccent(custom)
	}
	canMove := (t.Permissions.Progress || t.Permissions.Manage) && !(t.Progress == api.TaskDone || t.Progress == api.TaskCancelled) || t.Permissions.Manage
	pill := ui.ButtonBase(c.Key("status-pill")).Label("Status: "+progressLabel(t.Progress)).Row().AlignItems(ui.Center).Gap(7).Height(30).Padding(0, 10).Radius(8).Background(th.SurfaceHover).Disabled(!canMove || a.tasks.busy || a.tasks.loading)
	pill.FocusRing(pill.FocusVisible()).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Background(color)
		ui.Text(c, progressLabel(t.Progress)).FontSize(13).FontWeight(500)
		if canMove {
			ui.Icon(c, chromeIcon("down")).Size(14, 14).TextColor(th.TextMuted)
		}
	})
	if pill.Clicked() {
		a.tasks.statusOpen = !a.tasks.statusOpen
	}
	ui.PopoverBase(c, pill, &a.tasks.statusOpen, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Width(220).Padding(8).Gap(3).Radius(10).Background(th.Surface).Border(1, th.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		ui.Text(c, "Pindahkan ke").FontSize(11).TextColor(th.TextMuted).Padding(6, 8)
		for l := workspaceui.Todo; l <= workspaceui.Done; l++ {
			if spaceButton(c.Key(l), l.String(), "", l == lane).FillWidth().Clicked() {
				a.tasks.statusOpen = false
				if l != lane {
					a.moveTask(workspaceui.MoveRequest{TaskID: t.ID, Destination: l})
				}
			}
		}
	}).AttachTo(pill, ui.AnchorBottomLeft, ui.AnchorTopLeft)
}
