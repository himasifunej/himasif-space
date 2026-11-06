package appui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
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
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("check")).Size(16, 16)
			ui.Text(c, "Checklist").Bold()
			if len(t.Checklist) > 0 {
				muted(c, fmt.Sprintf("%d/%d", done, len(t.Checklist)))
			}
		})
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
			ui.Row(c.Key(item.ID)).FillWidth().Gap(10).AlignItems(ui.Center).MinHeight(36).Children(func() {
				box := ui.ButtonBase(c.Key("tick")).Label("Centang "+item.Body).Size(22, 22).Radius(6).Border(1, th.Border).Center().Disabled(!edit || a.taskWriteBlocked())
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
					label.TextColor(th.TextMuted)
				}
				if edit && spaceIconButton(c, "Hapus "+item.Body, "close").Disabled(a.taskWriteBlocked()).Clicked() {
					a.mutateTask("Checklist", func(TasksAPI) error {
						return cl.DeleteTaskChecklist(context.Background(), t.ID, item.ID)
					}, nil)
				}
			})
		}
		if edit {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c.Key("checklist-new"), &a.tasks.checklistNew).Label("Item checklist baru").Placeholder("Tambah item lalu Enter").Grow(1).MinWidth(0)
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

// taskDetailModal shows the selected task over the board on tablets and
// desktops: content on the left, conversation and history on the right.
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
		panel.Width(min(float32(1080), w-48)).Height(min(float32(820), h-48)).Column().Radius(16).Background(th.Surface).Border(1, th.Border).Children(func() {
			cover := a.taskCardColor(t.Category, taskLane(*t))
			if cover.A != 0 {
				ui.Box(c).FillWidth().Height(6).Background(cover)
			}
			ui.Row(c).FillWidth().Padding(12, 16).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(th.Border).Children(func() {
				ui.Text(c, "Detail tugas").FontSize(12).TextColor(th.TextMuted)
				ui.Text(c, progressLabel(t.Progress)).FontSize(12).Padding(4, 10).Radius(7).Background(th.SurfaceHover)
				if s := a.taskSpaceNames(*t); s != "" {
					muted(c, s).MaxLines(1)
				}
				ui.Box(c).Grow(1)
				if t.Permissions.Manage {
					if spaceButton(c, "Duplikat", "plus", false).Disabled(a.taskWriteBlocked()).Clicked() {
						a.taskAction(tid, "duplicate")
					}
					if spaceButton(c, "Batalkan", "archive", false).Disabled(a.taskWriteBlocked() || t.Progress == api.TaskCancelled).Clicked() {
						a.taskAction(tid, "cancel")
					}
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
						ui.Column(c).FillWidth().Padding(22).Gap(16).Children(func() { a.taskDetails(c, tid) })
					})
					return
				}
				ui.Scroll(c.Key("detail-main")).Grow(1).MinWidth(0).FillHeight().Label("Isi tugas").Children(func() {
					ui.Column(c).FillWidth().Padding(24).Gap(16).Children(func() {
						a.tasks.detailSplit = true
						a.taskDetails(c, tid)
						a.tasks.detailSplit = false
					})
				})
				ui.Scroll(c.Key("detail-side")).Width(360).Shrink(0).FillHeight().Background(th.Background).BorderWidth(0, 0, 0, 1).BorderColor(th.Border).Label("Komentar dan aktivitas").Children(func() {
					ui.Column(c).FillWidth().Padding(18).Gap(12).Children(func() {
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
