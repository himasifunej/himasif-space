package appui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/emergency"
	"github.com/himasifunej/himasif-space/internal/id"
)

type emergencyUIState struct {
	seq                              uint64
	user, period, space              id.ID
	busy, mine, completed, confirmed bool
	err, notice, fileName, reason    string
	snapshot                         *api.EmergencySnapshot
	csv                              []byte
	changes                          []api.EmergencyChange
	results                          []api.EmergencyReconcileResult
	selected                         map[id.ID]bool
}

func (a *App) emergencyCurrent(gen int, seq uint64) bool {
	return gen == a.gen && seq == a.emergency.seq && a.emergency.user == a.user.ID && a.emergency.period == a.periodID
}
func (a *App) emergencyError(gen int, seq uint64, err error) {
	message := a.failed(err)
	if !a.emergencyCurrent(gen, seq) {
		return
	}
	st := &a.emergency
	st.err = message
	var remote *client.Error
	if errors.As(err, &remote) && (remote.Status == 403 || remote.Status == 404) {
		st.snapshot = nil
		st.csv = nil
		st.results = nil
		st.selected = nil
		st.confirmed = false
	}
}
func (a *App) createEmergencyExport() {
	st := &a.emergency
	cl := a.notificationClient()
	if cl == nil || st.busy || st.space == id.Nil || a.offlineMode() {
		return
	}
	gen, seq, sid := a.gen, st.seq, st.space
	req := api.EmergencySnapshotRequest{IncludeCompleted: st.completed}
	if st.mine {
		uid := a.user.ID
		req.PersonID = &uid
	}
	st.busy = true
	st.err = ""
	st.csv = nil
	st.snapshot = nil
	a.async(func() func() {
		snapshot, err := cl.CreateEmergencySnapshot(context.Background(), sid, req)
		var csv []byte
		if err == nil {
			csv, err = cl.EmergencySnapshotCSV(context.Background(), snapshot.ID)
		}
		return func() {
			if !a.emergencyCurrent(gen, seq) {
				return
			}
			st := &a.emergency
			st.busy = false
			if snapshot.ID != id.Nil {
				st.snapshot = &snapshot
			}
			if err != nil {
				a.emergencyError(gen, seq, err)
				return
			}
			st.csv = csv
			st.notice = "Snapshot siap disimpan. CSV tetap merupakan salinan pada waktu yang tercantum."
		}
	})
}
func (a *App) saveEmergencyCSV() {
	st := &a.emergency
	if st.busy || st.snapshot == nil || len(st.csv) == 0 || a.ExportFile == nil {
		return
	}
	gen, seq := a.gen, st.seq
	data := st.csv
	save := a.ExportFile
	name := "space-emergency-" + st.snapshot.ID.String() + ".csv"
	st.busy = true
	a.async(func() func() {
		path, err := save(name, "text/csv;charset=utf-8", data)
		return func() {
			if !a.emergencyCurrent(gen, seq) {
				return
			}
			st := &a.emergency
			st.busy = false
			if err != nil {
				st.err = err.Error()
				return
			}
			if path != "" {
				st.notice = path
			}
		}
	})
}
func (a *App) pickEmergencyCSV() {
	st := &a.emergency
	if st.busy || a.PickFile == nil {
		return
	}
	gen, seq := a.gen, st.seq
	st.busy = true
	st.err = ""
	a.PickFile(32<<20, func(file SelectedFile, err error) {
		a.update(func() {
			if !a.emergencyCurrent(gen, seq) {
				return
			}
			st := &a.emergency
			if err != nil {
				st.busy = false
				st.err = err.Error()
				return
			}
			if file.Name == "" {
				st.busy = false
				return
			}
			if !strings.HasSuffix(strings.ToLower(file.Name), ".csv") {
				st.busy = false
				st.err = "Ekspor spreadsheet sebagai CSV UTF-8 sebelum dipilih."
				return
			}
			a.async(func() func() {
				changes, err := emergency.ParseChanges(file.Data)
				return func() {
					if !a.emergencyCurrent(gen, seq) {
						return
					}
					st := &a.emergency
					st.busy = false
					if err != nil {
						st.err = err.Error()
						return
					}
					st.changes = changes
					st.results = nil
					st.selected = map[id.ID]bool{}
					st.confirmed = false
					st.fileName = file.Name
					st.notice = "CSV dibaca pada perangkat. Muat pratinjau untuk memeriksa baseline dan izin server terbaru."
				}
			})
		})
	})
}
func (a *App) reconcileEmergency(action string) {
	st := &a.emergency
	cl := a.notificationClient()
	if cl == nil || st.busy || a.offlineMode() {
		return
	}
	apply := action == "apply"
	resolving := action == "manual_reviewed" || action == "ignored"
	changes := append([]api.EmergencyChange{}, st.changes...)
	if apply || resolving {
		if !st.confirmed || strings.TrimSpace(st.reason) == "" {
			return
		}
		selected := []api.EmergencyChange{}
		for _, change := range changes {
			for _, result := range st.results {
				if result.RowID == change.RowID && (apply && result.Status == "ready" || resolving && emergencySelectable(result)) && st.selected[change.RowID] && result.Current != nil {
					change.ExpectedCurrentVersion = result.Current.Version
					selected = append(selected, change)
					break
				}
			}
		}
		changes = selected
	}
	if len(changes) == 0 {
		return
	}
	gen, seq, reason := a.gen, st.seq, st.reason
	st.busy = true
	st.err = ""
	a.async(func() func() {
		var results []api.EmergencyReconcileResult
		var err error
		if resolving {
			results, err = cl.ResolveEmergency(context.Background(), api.EmergencyReconcileRequest{Resolution: action, Changes: changes, Reason: reason})
		} else if apply {
			results, err = cl.ApplyEmergency(context.Background(), api.EmergencyReconcileRequest{Changes: changes, Reason: reason})
		} else {
			results, err = cl.PreviewEmergency(context.Background(), changes)
		}
		return func() {
			if !a.emergencyCurrent(gen, seq) {
				return
			}
			st := &a.emergency
			st.busy = false
			st.confirmed = false
			if err != nil {
				st.results = nil
				st.selected = nil
				a.emergencyError(gen, seq, err)
				return
			}
			if apply || resolving {
				updates := map[id.ID]api.EmergencyReconcileResult{}
				for _, result := range results {
					updates[result.RowID] = result
				}
				for i, result := range st.results {
					if update, ok := updates[result.RowID]; ok {
						st.results[i] = update
					}
				}
				st.notice = "Hasil per baris ditampilkan. Baris konflik/manual tidak menimpa tugas server."
				a.tasks.loaded = false
			} else {
				st.results = results
				st.notice = "Periksa tiap baris sebelum memilih perubahan yang akan diterapkan."
			}
			st.selected = map[id.ID]bool{}
		}
	})
}
func emergencyStatus(status string) string {
	if label := map[string]string{"ready": "Siap dipilih", "applied": "Sudah dicatat", "unchanged": "Sudah sesuai", "conflict": "Konflik", "manual": "Tinjauan manual", "forbidden": "Tidak berwenang", "missing": "Tidak tersedia", "manual_reviewed": "Tinjauan manual dicatat", "ignored": "Diabaikan dengan alasan"}[status]; label != "" {
		return label
	}
	return status
}
func (a *App) emergencyView(c *ui.Context) {
	st := &a.emergency
	if st.user != a.user.ID || st.period != a.periodID {
		*st = emergencyUIState{seq: st.seq + 1, user: a.user.ID, period: a.periodID, space: a.selectedSpaceID()}
	}
	pageBody(c, "Salinan darurat dan rekonsiliasi", func() {
		if a.period() == nil {
			muted(c, "Pilih periode terlebih dahulu.")
			return
		}
		if a.isAdmin() {
			muted(c, "Gunakan akun pribadi dengan akses tugas untuk menyiapkan atau merekonsiliasi salinan darurat.")
			return
		}
		if st.err != "" {
			pageNotice(c, "alert", "Permintaan belum selesai", st.err)
		}
		if st.notice != "" {
			muted(c, st.notice)
		}
		pageSection(c, "Salinan tugas", "Pilih Space yang berizin. CSV mencatat waktu snapshot, ID tugas, hasil yang diminta, PJ, deadline dan status terakhir.", func() {
			ui.Row(c).Wrap().Gap(5).Children(func() {
				for _, space := range a.access.spaces {
					if spaceButton(c.Key(space.ID.String()), space.Name, "", st.space == space.ID).Disabled(st.busy).Clicked() {
						st.space = space.ID
						st.snapshot = nil
						st.csv = nil
					}
				}
			})
			ui.Checkbox(c, &st.mine, "Hanya tugas dengan saya sebagai PJ utama").Disabled(st.busy)
			ui.Checkbox(c, &st.completed, "Sertakan tugas selesai/dibatalkan").Disabled(st.busy)
			if spaceButton(c, "Buat snapshot terbaru", "archive", false).Disabled(st.busy || st.space == id.Nil || a.offlineMode()).Clicked() {
				a.createEmergencyExport()
			}
			if st.snapshot != nil {
				snapshot := st.snapshot
				info(c, [][2]string{{"Space", snapshot.SpaceName}, {"Waktu snapshot", taskTimestamp(snapshot.CapturedAt, a.period().TimeZone)}, {"Dibuat oleh", snapshot.CapturedBy.DisplayName}, {"Jumlah tugas", fmt.Sprint(snapshot.RowCount)}})
				if len(st.csv) == 0 && spaceButton(c, "Ambil CSV snapshot ini", "refresh", false).Disabled(st.busy || a.offlineMode()).Clicked() {
					a.reloadEmergencyCSV()
				}
				if len(st.csv) > 0 && a.ExportFile != nil {
					if spaceButton(c, "Simpan CSV darurat", "archive", false).Disabled(st.busy).Clicked() {
						a.saveEmergencyCSV()
					}
				}
			}
			muted(c, "Saat gangguan, isi new_progress (todo/doing), reported_at, reported_by dan change_note. Catat perubahan lain pada change_note. Pertahankan seluruh ID dan kolom lain. Salinan ini tidak memperbarui dirinya sendiri.")
		})
		pageSection(c, "Rekonsiliasi setelah pulih", "Pilih CSV yang sudah dicatat selama gangguan. Server memakai baseline tersimpan dan izin terkini; perubahan tidak langsung diterapkan saat impor.", func() {
			if a.PickFile != nil {
				if spaceButton(c, "Pilih CSV perubahan darurat", "plus", false).Disabled(st.busy).Clicked() {
					a.pickEmergencyCSV()
				}
			}
			if len(st.changes) == 0 {
				return
			}
			muted(c, st.fileName+" · "+fmt.Sprint(len(st.changes))+" baris perubahan")
			if spaceButton(c, "Muat ulang pratinjau server", "refresh", false).Disabled(st.busy || a.offlineMode()).Clicked() {
				a.reconcileEmergency("preview")
			}
			for _, result := range st.results {
				change := api.EmergencyChange{}
				for _, item := range st.changes {
					if item.RowID == result.RowID {
						change = item
						break
					}
				}
				pageSection(c.Key(result.RowID.String()), emergencyStatus(result.Status), result.Message, func() {
					if result.Snapshot != nil {
						muted(c, result.Snapshot.SpaceName+" · snapshot "+taskTimestamp(result.Snapshot.CapturedAt, a.period().TimeZone))
					}
					if result.Baseline != nil {
						base := result.Baseline
						ui.Text(c, base.Title).Bold()
						muted(c, "Snapshot: "+base.Progress+" · "+base.Assignee.DisplayName+" · "+base.DeadlineValue+" · versi "+fmt.Sprint(base.Version))
					}
					if result.Current != nil {
						now := result.Current
						muted(c, "Server: "+now.Progress+" · "+now.Assignee.DisplayName+" · "+now.DeadlineValue+" · versi "+fmt.Sprint(now.Version))
					}
					ui.Text(c, "Usulan: "+change.Progress+" · pelapor: "+change.ReportedBy+" · waktu laporan: "+change.ReportedAt)
					if change.Note != "" {
						ui.Text(c, change.Note)
					}
					if result.AppliedAt != nil {
						muted(c, "Dicatat oleh "+result.AppliedBy+" · "+taskTimestamp(*result.AppliedAt, a.period().TimeZone))
					}
					if emergencySelectable(result) {
						checked := st.selected[result.RowID]
						if ui.Checkbox(c, &checked, "Pilih perubahan ini").Disabled(st.busy).Changed() {
							if st.selected == nil {
								st.selected = map[id.ID]bool{}
							}
							st.selected[result.RowID] = checked
							st.confirmed = false
						}
					}
					if result.Current != nil && spaceButton(c, "Buka tugas untuk tinjauan", "task", false).Disabled(st.busy).Clicked() {
						a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: result.Current.TaskID})
					}
				})
			}
			selected, ready := 0, 0
			for _, result := range st.results {
				if emergencySelectable(result) && st.selected[result.RowID] {
					selected++
					if result.Status == "ready" {
						ready++
					}
				}
			}
			if selected == 0 {
				return
			}
			if ui.TextArea(c, &st.reason).Label("Alasan menerapkan rekonsiliasi").FillWidth().Height(80).Changed() {
				st.confirmed = false
			}
			ui.Checkbox(c, &st.confirmed, "Saya sudah memeriksa baris terpilih, baseline dan keadaan server terbaru.").Disabled(st.busy)
			if spaceButton(c, fmt.Sprintf("Terapkan %d progres yang siap", ready), "check", false).Disabled(st.busy || a.offlineMode() || ready == 0 || !st.confirmed || strings.TrimSpace(st.reason) == "").Clicked() {
				a.reconcileEmergency("apply")
			}
			for _, resolution := range []struct{ action, label string }{{"manual_reviewed", "Catat tinjauan manual untuk baris terpilih"}, {"ignored", "Abaikan baris terpilih dengan alasan"}} {
				if spaceButton(c.Key(resolution.action), resolution.label, "check", false).Disabled(st.busy || a.offlineMode() || !st.confirmed || strings.TrimSpace(st.reason) == "").Clicked() {
					a.reconcileEmergency(resolution.action)
				}
			}
			muted(c, "Penyerahan hasil, approval, perubahan PJ/deadline dan catatan lain ditangani pada tugas masing-masing. Waktu laporan sheet tidak mengganti waktu bukti atau pencatatan server.")
		})
	})
}

func emergencySelectable(result api.EmergencyReconcileResult) bool {
	return result.Current != nil && (result.Status == "ready" || result.Status == "conflict" || result.Status == "manual" || result.Status == "unchanged")
}

func (a *App) reloadEmergencyCSV() {
	st := &a.emergency
	cl := a.notificationClient()
	if st.busy || st.snapshot == nil || cl == nil || a.offlineMode() {
		return
	}
	gen, seq, snapshotID := a.gen, st.seq, st.snapshot.ID
	st.busy = true
	st.err = ""
	a.async(func() func() {
		data, err := cl.EmergencySnapshotCSV(context.Background(), snapshotID)
		return func() {
			if !a.emergencyCurrent(gen, seq) {
				return
			}
			st := &a.emergency
			st.busy = false
			if err != nil {
				a.emergencyError(gen, seq, err)
				return
			}
			st.csv = data
		}
	})
}
