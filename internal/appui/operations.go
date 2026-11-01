package appui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

type operationsUIState struct {
	seq                    uint64
	user, period, division id.ID
	loading                bool
	err, divisionErr       string
	system                 *api.SystemOperations
	scope                  *api.DivisionOperations
}

func (a *App) operationsCurrent(gen int, seq uint64) bool {
	return gen == a.gen && seq == a.operations.seq && a.operations.user == a.user.ID && a.operations.period == a.periodID
}
func (a *App) loadOperations() {
	st := &a.operations
	cl := a.notificationClient()
	if cl == nil || st.loading || a.offlineMode() {
		return
	}
	gen, seq, pid, did, admin := a.gen, st.seq, st.period, st.division, a.isAdmin()
	st.loading = true
	st.err = ""
	st.divisionErr = ""
	a.async(func() func() {
		system, err := cl.SystemOperations(context.Background())
		var scope api.DivisionOperations
		var scopeErr error
		if err == nil && did != id.Nil && !admin {
			scope, scopeErr = cl.DivisionOperations(context.Background(), pid, did)
		}
		return func() {
			if !a.operationsCurrent(gen, seq) {
				return
			}
			st := &a.operations
			st.loading = false
			if err != nil {
				message := a.failed(err)
				if !a.operationsCurrent(gen, seq) {
					return
				}
				st.err = message
				st.system = nil
				st.scope = nil
				return
			}
			st.system = &system
			st.scope = nil
			if scopeErr != nil {
				message := a.failed(scopeErr)
				if !a.operationsCurrent(gen, seq) {
					return
				}
				st.divisionErr = message
			} else if did != id.Nil && !admin {
				st.scope = &scope
			}
		}
	})
}
func operationState(value string) string {
	if label := map[string]string{"ok": "Baik", "connected": "Tersambung", "connecting": "Menyambungkan", "pairing": "Menunggu pairing", "needs_pairing": "Perlu pairing ulang", "disconnected": "Terputus", "waiting": "Menunggu siklus berikutnya", "running": "Sedang bekerja", "failed": "Ada kegagalan", "degraded": "Perlu diperiksa", "stale": "Status/progres sudah lama", "disabled": "Nonaktif", "not_observed": "Belum terpantau", "not_synced": "Belum pernah berhasil sinkron", "not_started": "Belum dimulai", "stopped": "Berhenti"}[value]; label != "" {
		return label
	}
	return value
}
func workerName(name string) string {
	if label := map[string]string{"scheduler": "Penjadwal pengingat", "connectors": "Polling dan mirror sumber", "wa.poll": "Polling sesi WhatsApp", "wa.commands": "Command WhatsApp", "wa.content": "Parser konsep/bukti WhatsApp", "wa.outbox": "Pengiriman WhatsApp", "sop": "Observasi dan penjadwal SOP"}[name]; label != "" {
		return label
	}
	return name
}
func operationTime(value *time.Time, zone string) string {
	if value == nil {
		return "Belum tercatat"
	}
	return taskTimestamp(*value, zone)
}
func (a *App) queueHealthView(c *ui.Context, q api.QueueHealth) {
	zone := "Asia/Jakarta"
	if a.period() != nil {
		zone = a.period().TimeZone
	}
	info(c, [][2]string{{"Antrean", fmt.Sprint(q.Queued)}, {"Sudah waktunya diproses", fmt.Sprint(q.Due)}, {"Sedang dikirim", fmt.Sprint(q.Sending)}, {"Hasil belum pasti", fmt.Sprint(q.Unknown)}, {"Gagal", fmt.Sprint(q.Failed)}, {"Diterima layanan/perangkat", fmt.Sprint(q.Accepted)}, {"Receipt terkirim", fmt.Sprint(q.Delivered)}, {"Receipt dibaca", fmt.Sprint(q.Read)}, {"Dibatalkan", fmt.Sprint(q.Cancelled)}, {"Ditahan setelah restore", fmt.Sprint(q.Suppressed)}})
	if q.OldestDueAt != nil {
		muted(c, "Antrean jatuh tempo tertua: "+taskTimestamp(*q.OldestDueAt, zone))
	}
}
func (a *App) operationsView(c *ui.Context) {
	st := &a.operations
	if st.user != a.user.ID || st.period != a.periodID {
		*st = operationsUIState{seq: st.seq + 1, user: a.user.ID, period: a.periodID}
	}
	zone := "Asia/Jakarta"
	if a.period() != nil {
		zone = a.period().TimeZone
	}
	pageBody(c, "Status layanan", func() {
		if st.err != "" {
			pageNotice(c, "alert", "Status belum dapat dimuat", st.err)
		}
		if spaceButton(c, "Muat ulang status", "refresh", false).Disabled(st.loading || a.offlineMode()).Clicked() {
			a.loadOperations()
		}
		muted(c, "Status diambil saat dimuat ulang. Heartbeat menunjukkan progres worker, bukan bukti pesan sudah terkirim atau terbaca.")
		if st.system != nil {
			system := st.system
			info(c, [][2]string{{"Versi server", system.Version}, {"Revisi sumber", system.Build.Revision}, {"Hash snapshot", system.Build.SourceDigest}, {"Mode", system.RunMode}, {"Status layanan", operationState(system.Status)}, {"Diperiksa", taskTimestamp(system.GeneratedAt, zone)}})
			if system.RunMode != "live" {
				pageNotice(c, "info", "Pengiriman dinonaktifkan", "Mode pemeliharaan/restore menonaktifkan worker dan transport eksternal. Status nonaktif pada mode ini memang diharapkan.")
			}
			for _, worker := range system.Workers {
				pageSection(c.Key(worker.Name), workerName(worker.Name), operationState(worker.State), func() {
					info(c, [][2]string{{"Instance aktif terpantau", fmt.Sprint(worker.ActiveInstances)}, {"Progres terakhir", operationTime(worker.LastProgressAt, zone)}, {"Siklus berhasil terakhir", operationTime(worker.LastSuccessAt, zone)}, {"Kegagalan terakhir", operationTime(worker.LastFailureAt, zone)}})
				})
			}
		}
		if a.isAdmin() {
			muted(c, "Akun admin organisasi hanya melihat status infrastruktur. Rincian antrean/sumber divisi memerlukan akun pribadi berizin.")
			return
		}
		pageSection(c, "Status divisi", "Kewenangan tugas membuka statistik notifikasi anggota/sumber. Kewenangan WhatsApp membuka status bot dan antreannya.", func() {
			ui.Row(c).Wrap().Gap(5).Children(func() {
				for _, division := range a.divisions {
					if spaceButton(c.Key(division.ID.String()), division.Name, "", st.division == division.ID).Disabled(st.loading).Clicked() {
						st.division = division.ID
						st.seq++
						st.scope = nil
						st.divisionErr = ""
						a.loadOperations()
					}
				}
			})
			if st.divisionErr != "" {
				muted(c, st.divisionErr)
			}
			if st.scope == nil {
				return
			}
			scope := st.scope
			ui.Text(c, scope.DivisionName).Bold()
			muted(c, "Diperiksa: "+taskTimestamp(scope.GeneratedAt, zone))
			if scope.MemberNotifications != nil {
				pageSection(c, "Notifikasi perangkat anggota", "Jumlah record notifikasi kerja pada Space terkait; reminder pribadi tidak masuk laporan divisi ini.", func() { a.queueHealthView(c, *scope.MemberNotifications) })
			}
			if scope.CanReadSources {
				for _, source := range scope.Connectors {
					pageSection(c.Key(source.ID.String()), source.Name, operationState(source.State), func() {
						info(c, [][2]string{{"Jenis", source.Kind}, {"Berhasil terakhir", operationTime(source.LastSuccessAt, zone)}, {"Polling berikutnya", taskTimestamp(source.NextRunAt, zone)}, {"Item perlu ditinjau", fmt.Sprint(source.ReviewItems)}})
					})
				}
			}
			if scope.CanReadWhatsApp {
				if scope.WhatsApp == nil {
					muted(c, "Bot divisi belum dikonfigurasi.")
				} else {
					bot := scope.WhatsApp
					pageSection(c, bot.Name, operationState(bot.State), func() {
						info(c, [][2]string{{"Polling bridge terakhir", operationTime(bot.LastPolledAt, zone)}, {"Event terakhir", operationTime(bot.LastEventAt, zone)}, {"Grup dipetakan", fmt.Sprint(bot.MappedGroups)}, {"Pesan masuk mengantre", fmt.Sprint(bot.InboundQueued)}})
						if bot.HasError {
							muted(c, "Ada kegagalan sesi. Buka pengaturan WhatsApp untuk tindakan sesuai izin.")
						}
						a.queueHealthView(c, bot.Queue)
						if bot.Queue.Unknown > 0 {
							muted(c, "Hasil belum pasti harus direkonsiliasi dengan jurnal. Jangan mengirim ulang hanya karena belum ada receipt.")
						}
					})
				}
			}
		})
	})
}
