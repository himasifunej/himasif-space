package appui

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (a *App) sopReadinessView(c *ui.Context, disabled bool) {
	st := &a.evaluation
	ac := a.sopAccess()
	pageSection(c, "Prasyarat penjadwalan", "Pemeriksaan ini membaca konfigurasi terkini. Pengiriman aktual dan snapshot ada pada riwayat di bawah.", func() {
		if ac != nil && ac.DivisionSlug == "humas" {
			ui.Row(c).Wrap().Gap(5).Children(func() {
				if spaceButton(c, "Rekap saja", "", st.readinessContent == id.Nil).Disabled(disabled).Clicked() {
					st.readinessContent = id.Nil
					st.readiness = nil
				}
				for _, content := range st.contents {
					if spaceButton(c.Key(content.ID.String()), content.Title, "", st.readinessContent == content.ID).Disabled(disabled).Clicked() {
						st.readinessContent = content.ID
						st.readiness = nil
					}
				}
			})
		}
		if spaceButton(c, "Periksa prasyarat", "search", false).Disabled(disabled).Clicked() {
			a.loadSOPReadiness()
		}
		if st.readiness == nil {
			return
		}
		readiness := st.readiness
		muted(c, "Diperiksa: "+taskTimestamp(readiness.GeneratedAt, a.period().TimeZone))
		show := func(title string, item api.SOPScheduleReadiness) {
			ui.Text(c, title).Bold()
			if item.ScheduledAt != nil {
				muted(c, "Slot: "+taskTimestamp(*item.ScheduledAt, a.period().TimeZone))
			}
			if item.Ready {
				muted(c, "Prasyarat antrean tersedia. Scheduler tetap memeriksa jadwal, duplikasi dan izin saat memproses.")
			}
			for _, reason := range item.Blockers {
				ui.Text(c, reason)
			}
			for _, warning := range item.Warnings {
				muted(c, warning)
			}
		}
		show("Rekap mingguan", readiness.Weekly)
		if readiness.Followup != nil {
			show("Follow-up H-4", *readiness.Followup)
		}
	})
}
func (a *App) loadSOPReadiness() {
	st := &a.evaluation
	cl := a.notificationClient()
	if cl == nil || st.busy || st.loading {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	var cid *id.ID
	if st.readinessContent != id.Nil {
		value := st.readinessContent
		cid = &value
	}
	st.busy = true
	st.err = ""
	a.async(func() func() {
		out, err := cl.SOPReadiness(context.Background(), pid, did, cid)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.busy = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			st.readiness = &out
		}
	})
}
