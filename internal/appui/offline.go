package appui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type offlineUIState struct {
	action   string
	items    []client.PendingMutation
	loading  bool
	last     time.Time
	err      string
	selected id.ID
	confirm  bool
}

func (a *App) offlineClient() *client.Client {
	c, ok := a.api.(*client.Client)
	if !ok || c.Offline == nil {
		return nil
	}
	return c.AccountSession()
}
func (a *App) offlineAvailable() bool { return a.offlineClient() != nil && a.user.ID != id.Nil }
func (a *App) offlineMode() bool {
	c := a.offlineClient()
	if c == nil {
		return false
	}
	disconnected, _, _, _ := c.OfflineStatus()
	return disconnected || a.conn == connOffline
}
func (a *App) tickOffline(now time.Time) {
	if a.screen != screenShell {
		return
	}
	if a.offlineMode() {
		a.conn = connOffline
	}
	if !a.offlineSync.loading && (a.offlineSync.last.IsZero() || now.Sub(a.offlineSync.last) > 15*time.Second) {
		a.refreshOffline(true)
	}
}
func (a *App) refreshOffline(sync bool) {
	c := a.offlineClient()
	if c == nil || a.offlineSync.loading {
		return
	}
	gen := a.gen
	a.offlineSync.loading = true
	a.offlineSync.last = time.Now()
	a.async(func() func() {
		before, _ := c.PendingOffline()
		var syncErr error
		if sync {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := c.Health(ctx)
			if err == nil {
				syncErr = c.SyncPending(ctx)
			}
			cancel()
		}
		items, err := c.PendingOffline()
		if err == nil {
			err = syncErr
		}
		return func() {
			if gen != a.gen {
				return
			}
			a.offlineSync.loading = false
			a.offlineSync.items = items
			if err != nil {
				a.offlineSync.err = a.failed(err)
			} else {
				a.offlineSync.err = ""
			}
			changed := len(before) != len(items)
			if !changed {
				for i := range items {
					if before[i].State != items[i].State {
						changed = true
						break
					}
				}
			}
			if sync && changed && !a.offlineMode() {
				a.loadSpaces()
				a.tasks.loaded = false
				a.notes.loaded = false
				a.meetings.loaded = false
			}
		}
	})
}
func (a *App) queueProgress(t api.Task, progress string) {
	if period := a.period(); period == nil || period.Status == api.PeriodClosed || period.ID != t.PeriodID {
		a.tasks.err = "Periode tugas sudah ditutup atau tidak lagi dipilih."
		return
	}

	c := a.offlineClient()
	if c == nil {
		return
	}
	if progress != api.TaskTodo && progress != api.TaskDoing {
		a.tasks.err = "Saat offline, hanya progres belum/sedang dikerjakan yang dapat diantrekan. Penyerahan, penyelesaian dan perubahan delegasi memerlukan koneksi."
		return
	}
	req := api.OfflineMutation{ID: id.New(), Kind: "task.progress", TargetID: t.ID, BaseProgress: t.Progress, Progress: progress}
	gen, seq := a.gen, a.tasks.seq
	a.tasks.busy = true
	a.async(func() func() {
		err := c.QueueOffline(req)
		return func() {
			if gen != a.gen || seq != a.tasks.seq {
				return
			}
			a.tasks.busy = false
			if err != nil {
				a.tasks.err = err.Error()
				return
			}
			for i := range a.tasks.items {
				if a.tasks.items[i].ID == t.ID {
					a.tasks.items[i].Progress = progress
					a.tasks.items[i].LocalPending = true
				}
			}
			a.tasks.notice = "Tersimpan di perangkat — belum tersinkron."
			a.projectTasks()
			a.refreshOffline(false)
		}
	})
}
func (a *App) queueNote() {
	n := a.notes.note
	c := a.offlineClient()
	if n == nil || c == nil || a.notes.editor == nil {
		return
	}
	if !a.notes.baselineKnown {
		a.notes.err = "Dasar versi draf belum tersedia. Tinjau pemulihan terhadap isi resmi sebelum mengantrekan."
		return
	}
	m := api.OfflineMutation{ID: id.New(), Kind: "note.content", TargetID: n.ID, BaseTitle: a.notes.baselineTitle, BaseContent: cloneNoteDocument(a.notes.baselineContent), Title: a.notes.title, Content: a.notes.editor.Content()}
	if err := api.ValidateNoteContent(m.Content); err != nil {
		a.notes.err = err.Error()
		return
	}
	gen, seq, revision := a.gen, a.notes.seq, a.notes.revision
	a.persistNoteLocal(nil)
	a.notes.busy = true
	a.async(func() func() {
		err := c.QueueOffline(m)
		return func() {
			if gen != a.gen || seq != a.notes.seq {
				return
			}
			a.notes.busy = false
			if err != nil {
				a.notes.err = err.Error()
				return
			}
			a.notes.localStatus = "Tersimpan di perangkat — belum tersinkron."
			if revision == a.notes.revision {
				a.notes.dirty = true
			}
			a.refreshOffline(false)
		}
	})
}
func (a *App) overlayPendingTasks(items []api.Task) []api.Task {
	c := a.offlineClient()
	if c == nil {
		return items
	}
	pending, err := c.PendingOffline()
	if err != nil {
		return items
	}
	for i := range items {
		for _, p := range pending {
			if p.Mutation.Kind == "task.progress" && p.Mutation.TargetID == items[i].ID && (p.State == "local" || p.State == "sending") {
				items[i].Progress = p.Mutation.Progress
				items[i].LocalPending = true
			}
		}
	}
	return items
}
func (a *App) offlineBanner(c *ui.Context) {
	cl := a.offlineClient()
	if cl == nil {
		return
	}
	off, syncing, last, err := cl.OfflineStatus()
	status := "Tersinkron"
	if off || a.conn == connOffline {
		status = "Tidak ada koneksi"
	}
	if len(a.offlineSync.items) > 0 {
		status += " · Tersimpan di perangkat — belum tersinkron"
	}
	if syncing || a.offlineSync.loading {
		status = "Menyinkronkan…"
	}
	if !last.IsZero() {
		status += " · terakhir " + last.Local().Format("02 Jan 15:04")
	}
	ui.Row(c).FillWidth().Padding(6, 16).Gap(8).Children(func() {
		muted(c, status).Grow(1)
		if ui.Button(c, "Sinkronisasi").Clicked() {
			a.chrome.settingsTab = "Sinkronisasi"
			a.navigate(PathPengaturan)
		}
	})
	if err != "" {
		muted(c, err)
	}
}
func (a *App) offlineView(c *ui.Context) {
	pageBody(c, "Sinkronisasi perangkat", func() {
		if a.offlineClient() == nil {
			pageNotice(c, "alert", "Penyimpanan offline tidak tersedia", "Cache dan antrean belum dapat disimpan pada perangkat ini.")
			return
		}
		muted(c, "Hanya data yang sudah diunduh tersedia offline. Delegasi, role, akses, konfigurasi bot dan penyerahan hasil memerlukan koneksi.")
		if spaceButton(c, "Sinkronkan sekarang", "refresh", true).Disabled(a.offlineSync.loading).Clicked() {
			a.refreshOffline(true)
		}
		if a.offlineSync.err != "" {
			ui.Text(c, a.offlineSync.err).TextColor(c.Theme().Danger)
		}
		for _, p := range a.offlineSync.items {
			p := p
			panel := ui.Column(c.Key(p.Mutation.ID)).FillWidth().Gap(8).Padding(12).Border(1, c.Theme().Border).Radius(8)
			panel.Children(func() {
				ui.Text(c, offlineKindLabel(p.Mutation.Kind)+" · "+offlineStateLabel(p.State)).Bold()
				muted(c, p.Error)
				a.offlineMutationPreview(c, p, panel)
				if p.State == "conflict" {
					if spaceButton(c, "Gunakan perubahan perangkat setelah meninjau kedua versi", "check", false).Clicked() {
						a.offlineSync.selected = p.Mutation.ID
						a.offlineSync.confirm = true
						a.offlineSync.action = "local"
					}
				}
				if p.State == "rejected" && spaceButton(c, "Coba lagi setelah akses diperbaiki", "refresh", false).Clicked() {
					a.offlineSync.selected = p.Mutation.ID
					a.offlineSync.action = "retry"
					a.offlineSync.confirm = false
				}
				if a.offlineSync.selected == p.Mutation.ID && a.offlineSync.action == "retry" {
					if spaceButton(c, "Konfirmasi ajukan ulang perubahan", "check", true).Clicked() {
						a.retryRejectedOffline(p.Mutation.ID)
					}
				}
				if a.offlineSync.confirm && a.offlineSync.selected == p.Mutation.ID {
					muted(c, "Konfirmasi: perubahan perangkat akan diajukan terhadap versi server yang ditampilkan. Perubahan server sesudahnya tetap dapat menimbulkan konflik baru.")
					if spaceButton(c, "Konfirmasi gunakan perangkat", "check", true).Clicked() {
						a.resolveOffline(p.Mutation.ID, true)
					}
				}
				if spaceButton(c, "Buang antrean ini dan gunakan server", "close", false).Clicked() {
					a.offlineSync.selected = p.Mutation.ID
					a.offlineSync.confirm = false
					a.offlineSync.action = "discard"
				}
				if a.offlineSync.selected == p.Mutation.ID && !a.offlineSync.confirm && a.offlineSync.action != "retry" {
					if spaceButton(c, "Konfirmasi buang perubahan perangkat", "close", true).Clicked() {
						a.resolveOffline(p.Mutation.ID, false)
					}
				}
			})
		}
		if len(a.offlineSync.items) == 0 {
			muted(c, "Tidak ada perubahan yang menunggu sinkronisasi.")
		}
		muted(c, fmt.Sprintf("%d perubahan tersimpan. Lampiran lokal yang belum diunggah tetap berstatus pending.", len(a.offlineSync.items)))
	})
}
func (a *App) resolveOffline(mid id.ID, local bool) {
	c := a.offlineClient()
	if c == nil {
		return
	}
	gen := a.gen
	a.async(func() func() {
		err := c.ResolveOffline(mid, local)
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.offlineSync.err = err.Error()
				return
			}
			a.offlineSync.selected = id.Nil
			a.refreshOffline(local)
		}
	})
}

func (a *App) canEditOfflineNote() bool {
	sp := a.selectedSpace()
	n := a.notes.note
	return a.offlineMode() && a.screen == screenShell && a.offlineAvailable() && sp != nil && !sp.ReadOnly && n != nil && n.Permissions.Manage && !a.notes.busy && !a.notes.loading && !a.notes.detailLoading
}

func (a *App) retryRejectedOffline(mid id.ID) {
	cl := a.offlineClient()
	if cl == nil {
		return
	}
	gen := a.gen
	a.async(func() func() {
		err := cl.RecoverPending(mid)
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.offlineSync.err = err.Error()
				return
			}
			a.offlineSync.selected = id.Nil
			a.refreshOffline(true)
		}
	})
}

func offlineKindLabel(kind string) string {
	switch kind {
	case "task.progress":
		return "Progres tugas"
	case "note.content":
		return "Catatan"
	case "delivery.response":
		return "Respons pengingat"
	}
	return "Perubahan perangkat"
}
func offlineStateLabel(state string) string {
	switch state {
	case "local":
		return "Tersimpan di perangkat"
	case "sending":
		return "Menunggu konfirmasi server"
	case "conflict":
		return "Perlu memilih versi"
	case "rejected":
		return "Ditolak server"
	}
	return state
}
func (a *App) offlineMutationPreview(c *ui.Context, p client.PendingMutation, panel ui.Element) {
	switch p.Mutation.Kind {
	case "task.progress":
		ui.Text(c, "Perangkat: "+progressLabel(p.Mutation.Progress))
		if len(p.Remote) > 0 {
			var task api.Task
			if json.Unmarshal(p.Remote, &task) == nil && task.ID != id.Nil {
				ui.Text(c, task.Title).Bold()
				ui.Text(c, "Server: "+progressLabel(task.Progress))
				muted(c, "Deadline server: "+taskDate(task.DeadlineDate, task.DeadlineAt, task.TimeZone))
			}
		}
	case "note.content":
		var remote api.Note
		_ = json.Unmarshal(p.Remote, &remote)
		key := p.Mutation.ID.String() + fmt.Sprint(remote.Version)
		readers := ui.Local(panel, key, func() noteComparisonReaders {
			local := newNoteDocumentEditor(p.Mutation.Content)
			official := newNoteDocumentEditor(remote.Content)
			local.SetEditable(false)
			official.SetEditable(false)
			return noteComparisonReaders{official: official, recovered: local}
		})
		pageSection(c, "Perangkat", p.Mutation.Title, func() { readers.recovered.View(c) })
		if remote.ID != id.Nil {
			pageSection(c, "Server", remote.Title, func() { readers.official.View(c) })
		}
	case "delivery.response":
		if p.Mutation.Action == "seen" {
			muted(c, "Sudah lihat pengingat — menunggu sinkronisasi.")
		} else if p.Mutation.SnoozeUntil != nil {
			muted(c, "Ingatkan lagi: "+p.Mutation.SnoozeUntil.Local().Format("02 Jan 2006 15:04"))
		}
	}
}
