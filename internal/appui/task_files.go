package appui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// TaskFilesAPI attaches files to tasks (Tahap 6b). It is apart from
// TasksAPI so older services and fakes without it still open tasks.
type TaskFilesAPI interface {
	UploadTaskAttachment(ctx context.Context, task, attachment id.ID, name string, data []byte) (api.TaskAttachment, error)
	DownloadTaskAttachment(ctx context.Context, task id.ID, a api.TaskAttachment) ([]byte, error)
	RemoveTaskAttachment(ctx context.Context, task, attachment id.ID) error
}

// Approved limits (9 Nov 2026); the server enforces them, the client only
// avoids sending what would be refused.
const (
	taskFileMaxBytes = int64(20) << 20
	taskFileMaxCount = 20
)

// taskFileState is the detail's file work: a picked file waiting for (or
// retrying) its upload, and a downloaded file waiting to be saved.
type taskFileState struct {
	upload   *taskFileUpload
	download *taskFileDownload
	busy     bool
	err      string
}

type taskFileUpload struct {
	task, id id.ID
	name     string
	data     []byte
}

type taskFileDownload struct {
	task id.ID
	file api.TaskAttachment
	data []byte
}

func (a *App) taskFilesService() (TaskFilesAPI, bool) {
	s, ok := a.taskService()
	if !ok {
		return nil, false
	}
	files, ok := s.(TaskFilesAPI)
	return files, ok
}

// canAttachTaskFiles: the PJ, a participant or the manager, on a live task,
// online, with a file picker.
func (a *App) canAttachTaskFiles(t api.Task) bool {
	_, ok := a.taskFilesService()
	return ok && a.PickFile != nil && (t.Permissions.Manage || t.Permissions.Progress) && t.Progress != api.TaskCancelled && !a.offlineMode() && !a.isAdmin()
}

func (a *App) pickTaskFile(t api.Task) {
	st := &a.tasks.files
	if st.busy || st.upload != nil {
		return
	}
	gen, tid := a.gen, t.ID
	st.busy, st.err = true, ""
	a.PickFile(taskFileMaxBytes, func(file SelectedFile, err error) {
		a.update(func() {
			if gen != a.gen {
				return
			}
			st := &a.tasks.files
			st.busy = false
			if err != nil {
				st.err = err.Error()
				return
			}
			if file.Name == "" {
				return
			}
			st.upload = &taskFileUpload{task: tid, id: id.New(), name: file.Name, data: file.Data}
			a.uploadTaskFile()
		})
	})
}

// uploadTaskFile sends the picked file; a failure keeps it, with the same
// upload ID, for "Unggah ulang".
func (a *App) uploadTaskFile() {
	up := a.tasks.files.upload
	files, ok := a.taskFilesService()
	if up == nil || !ok {
		return
	}
	a.tasks.files.err = ""
	// A failure shows in the task's feedback and leaves the file pending,
	// with the same upload ID, for "Unggah ulang". The pending file is
	// cleared once the reloaded task lists it (see taskFilesView).
	a.mutateTask("Lampiran", func(TasksAPI) error {
		_, err := files.UploadTaskAttachment(context.Background(), up.task, up.id, up.name, up.data)
		return err
	}, nil)
}

func (a *App) downloadTaskFile(t api.Task, f api.TaskAttachment) {
	files, ok := a.taskFilesService()
	st := &a.tasks.files
	if !ok || st.busy || a.offlineMode() {
		return
	}
	gen, tid := a.gen, t.ID
	st.busy, st.err = true, ""
	a.async(func() func() {
		data, err := files.DownloadTaskAttachment(context.Background(), tid, f)
		return func() {
			if gen != a.gen {
				return
			}
			st := &a.tasks.files
			st.busy = false
			if err != nil {
				st.err = "Berkas belum terunduh: " + a.taskFailure(err)
				return
			}
			st.download = &taskFileDownload{task: tid, file: f, data: data}
		}
	})
}

func (a *App) removeTaskFile(t api.Task, f api.TaskAttachment) {
	files, ok := a.taskFilesService()
	if !ok {
		return
	}
	tid := t.ID
	a.mutateTask("Hapus lampiran", func(TasksAPI) error {
		return files.RemoveTaskAttachment(context.Background(), tid, f.ID)
	}, nil)
}

// taskFileSize is "820 B", "14 KB" or "3,2 MB".
func taskFileSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%d KB", (n+512)/1024)
	}
	s := fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	for i := range s {
		if s[i] == '.' {
			return s[:i] + "," + s[i+1:]
		}
	}
	return s
}

// taskFilesView is the detail's Berkas list: download, remove (uploader or
// manager), attach, and the pending upload or download.
func (a *App) taskFilesView(c *ui.Context, t api.Task) {
	th := c.Theme()
	st := &a.tasks.files
	if up := st.upload; up != nil && up.task == t.ID {
		for _, f := range t.Attachments {
			if f.ID == up.id {
				st.upload = nil
				break
			}
		}
	}
	can := a.canAttachTaskFiles(t)
	if len(t.Attachments) == 0 && !can {
		return
	}
	ui.Column(c.Key("task-files")).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("Berkas · %d", len(t.Attachments))).FontSize(12).Bold().Grow(1)
			if can && st.upload == nil && len(t.Attachments) < taskFileMaxCount {
				if spaceButton(c.Key("attach-file"), "Lampirkan berkas", "plus", false).Border(1, th.Border).Disabled(st.busy || a.taskWriteBlocked()).Clicked() {
					a.pickTaskFile(t)
				}
			}
		})
		for _, f := range t.Attachments {
			ui.Row(c.Key(f.ID)).FillWidth().MinWidth(0).Gap(10).Padding(8, 10).Radius(8).Border(1, th.Border).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, detailIcon("clip")).Size(16, 16).TextColor(th.TextMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, f.Name).FontSize(13).MaxLines(1)
					muted(c, taskFileSize(f.SizeBytes)+" · "+f.AddedBy.DisplayName+" · "+taskTimestamp(f.CreatedAt, t.TimeZone)).FontSize(11).MaxLines(1)
				})
				if detailIconButton(c.Key("dl"), "Unduh "+f.Name, "download").Disabled(st.busy || a.offlineMode()).Clicked() {
					a.downloadTaskFile(t, f)
				}
				if (f.AddedBy.ID == a.user.ID || t.Permissions.Manage) && t.Progress != api.TaskCancelled {
					if spaceIconButton(c.Key("rm"), "Hapus "+f.Name, "close").Disabled(a.taskWriteBlocked()).Clicked() {
						a.removeTaskFile(t, f)
					}
				}
			})
		}
		if up := st.upload; up != nil && up.task == t.ID {
			ui.Row(c.Key("pending-file")).FillWidth().Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				state := "Mengunggah "
				if !a.tasks.busy {
					state = "Belum terunggah: "
				}
				muted(c, state+up.name+" · "+taskFileSize(int64(len(up.data)))).FontSize(12).Grow(1)
				if !a.tasks.busy {
					if spaceButton(c.Key("retry-file"), "Unggah ulang", "refresh", false).Disabled(a.taskWriteBlocked()).Clicked() {
						a.uploadTaskFile()
					}
					if spaceButton(c.Key("drop-file"), "Batal", "close", false).Clicked() {
						st.upload, st.err = nil, ""
					}
				}
			})
		}
		if st.err != "" {
			ui.Text(c, st.err).FontSize(12).TextColor(th.Danger)
		}
		if d := st.download; d != nil && d.task == t.ID && a.ExportFile != nil {
			if spaceButton(c.Key("save-file"), "Simpan berkas · "+d.file.Name, "archive", true).Disabled(st.busy).Clicked() {
				save, gen := a.ExportFile, a.gen
				st.busy = true
				a.async(func() func() {
					path, err := save(d.file.Name, d.file.ContentType, d.data)
					return func() {
						if gen != a.gen {
							return
						}
						st := &a.tasks.files
						st.busy = false
						if err != nil {
							st.err = err.Error()
							return
						}
						st.download = nil
						if path != "" {
							a.tasks.notice = "Berkas disimpan: " + path
						}
					}
				})
			}
		}
		if can {
			muted(c, "Gambar, PDF, teks, dokumen Office atau zip · maks. 20 MB per berkas, 20 berkas per tugas.").FontSize(11)
		}
	})
}

// detailIconButton is an icon button drawn from the task modal's icons.
func detailIconButton(c *ui.Context, label, icon string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(30, 30).Shrink(0).Center().Radius(8).TextColor(t.TextMuted)
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Icon(c, detailIcon(icon)).Size(15, 15) })
	return b
}
