package appui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// MonitorAdminAPI is the organization admin's side of org.monitor (OPEN-15):
// granting it per account and period, revoking it, and reading its log.
// The server decides; the app only offers what it may ask for.
type MonitorAdminAPI interface {
	MonitorGrants(context.Context, id.ID) (api.MonitorGrants, error)
	GrantMonitor(context.Context, id.ID, api.MonitorGrantRequest) (api.MonitorGrant, error)
	RevokeMonitor(context.Context, id.ID, id.ID) error
	MonitorAudit(context.Context, id.ID) ([]api.MonitorAuditEvent, error)
}

var _ MonitorAdminAPI = (*client.Client)(nil)

type monitorAdminUIState struct {
	seq                   uint64
	period                id.ID
	loaded, loading, busy bool
	err, notice           string
	grants                api.MonitorGrants
	audit                 []api.MonitorAuditEvent
	confirmRevoke         id.ID
}

func (a *App) monitorAdminService() (MonitorAdminAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(MonitorAdminAPI)
	return s, ok
}

func (a *App) loadMonitorAdmin() {
	s, ok := a.monitorAdminService()
	if !ok {
		a.monitorAdmin.err = "Layanan izin pemantauan belum tersedia."
		return
	}
	if !a.isAdmin() || a.periodID == id.Nil || a.monitorAdmin.loading {
		return
	}
	a.monitorAdmin.seq++
	gen, seq, p := a.gen, a.monitorAdmin.seq, a.periodID
	a.monitorAdmin.period, a.monitorAdmin.loading, a.monitorAdmin.err = p, true, ""
	a.async(func() func() {
		grants, err := s.MonitorGrants(context.Background(), p)
		var audit []api.MonitorAuditEvent
		if err == nil {
			audit, err = s.MonitorAudit(context.Background(), p)
		}
		return func() {
			if gen != a.gen || seq != a.monitorAdmin.seq || p != a.periodID {
				return
			}
			a.monitorAdmin.loading = false
			if err != nil {
				a.monitorAdmin.err = a.failed(err)
				return
			}
			a.conn = connOnline
			a.monitorAdmin.grants, a.monitorAdmin.audit, a.monitorAdmin.loaded = grants, audit, true
		}
	})
}

func (a *App) writeMonitorAdmin(notice string, work func(MonitorAdminAPI, id.ID) error) {
	s, ok := a.monitorAdminService()
	if !ok || a.monitorAdmin.busy || a.conn != connOnline || a.periodID == id.Nil {
		return
	}
	gen, p := a.gen, a.periodID
	a.monitorAdmin.busy, a.monitorAdmin.err, a.monitorAdmin.notice = true, "", ""
	a.async(func() func() {
		err := work(s, p)
		return func() {
			if gen != a.gen || p != a.periodID {
				return
			}
			a.monitorAdmin.busy, a.monitorAdmin.confirmRevoke = false, id.Nil
			if err != nil {
				a.monitorAdmin.err = a.failed(err)
				return
			}
			a.monitorAdmin.notice = notice
			a.monitorAdmin.loaded = false
			a.loadMonitorAdmin()
		}
	})
}

func monitorAuditAction(e api.MonitorAuditEvent) string {
	switch e.Action {
	case "monitor.grant":
		return "Admin memberi izin pemantauan kepada " + e.SubjectName
	case "monitor.revoke":
		if e.Detail != "" {
			return e.Detail + " · " + e.SubjectName
		}
		return "Izin pemantauan dicabut · " + e.SubjectName
	case "monitor.read":
		return e.Detail + " · " + e.DivisionName + " (hanya-baca)"
	case "monitor.act":
		return e.Detail + " · " + e.DivisionName
	}
	return e.Action
}

// organizationMonitorView is Pengaturan organisasi → Pemantauan: org.monitor
// per BPH account of the selected period, and its log (canvas "Privasi dan
// izin pemantauan").
func (a *App) organizationMonitorView(c *ui.Context) {
	if a.monitorAdmin.period != a.periodID {
		a.monitorAdmin = monitorAdminUIState{seq: a.monitorAdmin.seq + 1}
	}
	if !a.monitorAdmin.loaded && !a.monitorAdmin.loading && a.monitorAdmin.err == "" {
		a.loadMonitorAdmin()
	}
	p := a.period()
	closed := p != nil && p.Status == api.PeriodClosed
	pageSection(c, "Privasi dan izin pemantauan", "Izin diberikan admin ke akun tertentu anggota BPH, bukan ke jabatan. Pengganti jabatan tidak mewarisi izin, dan izin dicabut otomatis saat periode ditutup.", func() {
		info(c, [][2]string{
			{"Yang terbuka", "Tugas semua divisi lengkap dengan deskripsi, bukti, lampiran dan komentar; notulensi rapat; rekap SOP; beban kerja."},
			{"Tetap tertutup", "Reminder pribadi dan draf catatan siapa pun. Divisi tidak diberi tahu saat dibuka, tetapi setiap pembukaan tercatat di log di bawah."},
		})
	})
	if a.monitorAdmin.err != "" {
		pageNotice(c, "alert", "Izin pemantauan belum dapat dimuat", a.monitorAdmin.err)
	}
	if a.monitorAdmin.notice != "" {
		pageNotice(c, "check", "Perubahan tersimpan", a.monitorAdmin.notice)
	}
	if a.monitorAdmin.loading && !a.monitorAdmin.loaded {
		muted(c, "Memuat izin pemantauan…")
		return
	}
	if closed {
		pageNotice(c, "lock", "Periode ditutup", "Semua izin pemantauan periode ini sudah dicabut otomatis. Log tetap dapat dibaca.")
	}
	pageSection(c, "Anggota BPH", "Ketua Umum, Sekretaris dan Bendahara periode ini.", func() {
		if len(a.monitorAdmin.grants.Candidates) == 0 {
			pageEmpty(c, "users", "Belum ada anggota BPH", "Undang pengurus BPH ke divisi BPH periode ini terlebih dahulu.")
		}
		for _, cand := range a.monitorAdmin.grants.Candidates {
			cand := cand
			row := ui.Row(c.Key(cand.UserID)).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Padding(10, 0)
			row.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, cand.DisplayName).FontSize(13).Bold()
					if cand.Grant != nil {
						at := cand.Grant.GrantedAt.Local()
						muted(c, fmt.Sprintf("BPH · %s · izin pemantauan organisasi diberikan admin, %d %s %d", cand.Role, at.Day(), monthName(at.Month())[:3], at.Year())).FontSize(12)
					} else {
						muted(c, "BPH · "+cand.Role+" · belum memegang izin pemantauan").FontSize(12)
					}
				})
				blocked := a.monitorAdmin.busy || a.conn != connOnline || closed
				switch {
				case cand.Grant == nil:
					if spaceButton(c, "Beri izin "+cand.DisplayName, "eye", false).Disabled(blocked).Clicked() {
						uid := cand.UserID
						a.writeMonitorAdmin("Izin pemantauan diberikan. Pembukaan divisi tercatat di log.", func(s MonitorAdminAPI, p id.ID) error {
							_, err := s.GrantMonitor(context.Background(), p, api.MonitorGrantRequest{UserID: uid})
							return err
						})
					}
				case a.monitorAdmin.confirmRevoke == cand.Grant.ID:
					if spaceButton(c, "Konfirmasi cabut izin", "check", true).Disabled(blocked).Clicked() {
						gid := cand.Grant.ID
						a.writeMonitorAdmin("Izin pemantauan dicabut.", func(s MonitorAdminAPI, p id.ID) error {
							return s.RevokeMonitor(context.Background(), p, gid)
						})
					}
					if spaceButton(c, "Batal", "close", false).Disabled(a.monitorAdmin.busy).Clicked() {
						a.monitorAdmin.confirmRevoke = id.Nil
					}
				default:
					if spaceButton(c, "Cabut izin "+cand.DisplayName, "lock", false).Disabled(blocked).Clicked() {
						a.monitorAdmin.confirmRevoke = cand.Grant.ID
					}
				}
			})
		}
	})
	pageSection(c, "Log audit", "Setiap pemberian, pencabutan, pembukaan divisi dan tindakan pemantau tercatat.", func() {
		if spaceButton(c, "Muat ulang log", "refresh", false).Disabled(a.monitorAdmin.loading).Clicked() {
			a.monitorAdmin.loaded = false
			a.loadMonitorAdmin()
		}
		if len(a.monitorAdmin.audit) == 0 {
			muted(c, "Belum ada kejadian pemantauan pada periode ini.")
		}
		for _, e := range a.monitorAdmin.audit {
			at := e.OccurredAt.Local()
			ui.Row(c.Key(e.ID)).FillWidth().MinWidth(0).Gap(12).Padding(6, 0).AlignItems(ui.Start).Children(func() {
				ui.Text(c, fmt.Sprintf("%d %s %02d.%02d", at.Day(), monthName(at.Month())[:3], at.Hour(), at.Minute())).FontSize(12).Width(96).Shrink(0).TextColor(c.Theme().TextMuted)
				ui.Text(c, monitorAuditAction(e)).FontSize(12).Grow(1).MinWidth(0)
				ui.Text(c, e.ActorName).FontSize(12).MaxLines(1).TextColor(c.Theme().TextMuted)
			})
		}
	})
}
