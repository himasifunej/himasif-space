package appui

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (a *App) loadSOPDeliveries(more bool) {
	st := &a.evaluation
	cl := a.notificationClient()
	if cl == nil || a.sopAccess() == nil || st.busy || st.loading {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	cursor := ""
	if more {
		cursor = st.deliveryCursor
	}
	st.busy = true
	st.err = ""
	a.async(func() func() {
		page, err := cl.SOPDeliveries(context.Background(), pid, did, cursor)
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
			if !more {
				st.deliveries = nil
				st.deliverySelected = nil
			}
			seen := map[id.ID]bool{}
			for _, item := range st.deliveries {
				seen[item.ID] = true
			}
			for _, item := range page.Items {
				if !seen[item.ID] {
					st.deliveries = append(st.deliveries, item)
					seen[item.ID] = true
				}
			}
			st.deliveryCursor = page.NextCursor
			st.deliveryMore = page.HasMore
		}
	})
}
func (a *App) selectSOPDelivery(rid id.ID) {
	st := &a.evaluation
	cl := a.notificationClient()
	if cl == nil || st.busy || st.loading {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.busy = true
	st.err = ""
	a.async(func() func() {
		item, err := cl.SOPDelivery(context.Background(), pid, did, rid)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			a.evaluation.busy = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			a.evaluation.deliverySelected = &item
		}
	})
}
func sopDeliveryStatus(status string) string {
	if text := map[string]string{"queued": "Dalam antrean", "sending": "Sedang dikirim", "accepted": "Diterima layanan WA", "delivered": "Terkirim ke perangkat", "read": "Receipt dibaca", "failed": "Gagal", "unknown": "Hasil belum pasti", "cancelled": "Dibatalkan", "unavailable": "Antrean belum tersedia"}[status]; text != "" {
		return text
	}
	return status
}
func (a *App) sopDeliveriesView(c *ui.Context) {
	st := &a.evaluation
	disabled := st.busy || st.loading || a.offlineMode()
	a.sopReadinessView(c, disabled)
	pageNotice(c, "info", "Pengiriman SOP", "Status accepted berarti layanan WA menerima pesan. Status ini tidak membuktikan penerima sudah membaca atau SOP terpenuhi. Snapshot mempertahankan keadaan saat pesan disiapkan.")
	if spaceButton(c, "Muat ulang pengiriman", "refresh", false).Disabled(disabled).Clicked() {
		a.loadSOPDeliveries(false)
	}
	if len(st.deliveries) == 0 {
		muted(c, "Muat riwayat untuk melihat follow-up H-4 dan rekap mingguan yang sudah dijadwalkan. Pesan yang belum dibuat tidak muncul di riwayat.")
	}
	for _, item := range st.deliveries {
		label := "Follow-up H-4"
		if item.Kind == "weekly" {
			label = "Rekap mingguan"
		}
		ui.Column(c.Key(item.ID.String())).FillWidth().Gap(4).Children(func() {
			ui.Text(c, label+" · "+item.Title).Bold()
			muted(c, taskTimestamp(item.ScheduledAt, a.period().TimeZone)+" · "+sopDeliveryStatus(item.Status))
			muted(c, "Tujuan saat antrean dibuat: "+item.TargetName)
			if item.SuppressedAt != nil {
				muted(c, "Ditahan setelah restore: "+item.SuppressionReason)
			}
			if item.Error != "" {
				ui.Text(c, item.Error)
			}
			if spaceButton(c, "Lihat snapshot dan status", "search", false).Disabled(disabled).Clicked() {
				a.selectSOPDelivery(item.ID)
			}
		})
	}
	if st.deliveryMore {
		if spaceButton(c, "Muat pengiriman berikutnya", "plus", false).Disabled(disabled).Clicked() {
			a.loadSOPDeliveries(true)
		}
	}
	if st.deliverySelected == nil {
		return
	}
	item := st.deliverySelected
	pageSection(c, "Snapshot pengiriman", item.Title, func() {
		info(c, [][2]string{{"Status", sopDeliveryStatus(item.Status)}, {"Versi kebijakan", fmt.Sprint(item.PolicyVersion)}, {"Percobaan", fmt.Sprint(item.Attempts)}, {"Tujuan", item.TargetName}, {"Dibuat", taskTimestamp(item.CreatedAt, a.period().TimeZone)}, {"Dijadwalkan", taskTimestamp(item.ScheduledAt, a.period().TimeZone)}})
		if item.AcceptedAt != nil {
			muted(c, "Diterima layanan: "+taskTimestamp(*item.AcceptedAt, a.period().TimeZone))
		}
		if item.SuppressedAt != nil {
			muted(c, "Ditahan setelah restore: "+item.SuppressionReason)
		}
		if item.Error != "" {
			ui.Text(c, item.Error)
		}
		if item.Kind == "weekly" {
			var snapshot struct {
				Report     api.SOPReport `json:"report"`
				References []struct {
					FindingID    id.ID  `json:"findingId"`
					State        string `json:"state"`
					NeedsRecheck bool   `json:"needsRecheck"`
				} `json:"references"`
			}
			if err := json.Unmarshal(item.Snapshot, &snapshot); err != nil {
				muted(c, "Snapshot tidak dapat dibaca.")
			} else {
				report := snapshot.Report
				muted(c, report.StartsOn.String()+" — "+report.EndsOn.String()+" · "+report.DocumentVersion)
				info(c, [][2]string{{"Total", fmt.Sprint(report.Summary.Total)}, {"Perlu tinjauan", fmt.Sprint(report.Summary.NeedsReview)}, {"Disahkan", fmt.Sprint(report.Summary.Confirmed)}, {"Dibatalkan", fmt.Sprint(report.Summary.Dismissed)}, {"Perlu diperiksa ulang", fmt.Sprint(report.Summary.NeedsRecheck)}})
				for _, ref := range snapshot.References {
					label := sopStateLabel(ref.State)
					if ref.NeedsRecheck {
						label += " · sumber berubah"
					}
					if spaceButton(c.Key(ref.FindingID.String()), "Buka temuan · "+label, "search", false).Disabled(disabled).Clicked() {
						st.tab = "Temuan"
						a.selectSOPFinding(ref.FindingID)
					}
				}
			}
		} else {
			var snapshot struct {
				ConceptURL         string `json:"conceptUrl"`
				BotCountsRequested bool   `json:"botCountsRequested"`
				AgreementID        *id.ID `json:"agreementId"`
			}
			if err := json.Unmarshal(item.Snapshot, &snapshot); err != nil {
				muted(c, "Snapshot tidak dapat dibaca.")
			} else {
				ui.Text(c, "Konsep saat pesan disiapkan: "+snapshot.ConceptURL)
				if snapshot.BotCountsRequested && snapshot.AgreementID != nil {
					muted(c, "Snapshot mencatat usulan pemenuhan melalui bot dan persetujuan evaluator. Keberlakuan saat accepted diperiksa oleh evaluator SOP.")
				} else {
					muted(c, "Pesan koordinasi ini belum menjadi bukti pemenuhan SOP melalui bot.")
				}
			}
		}
		if spaceButton(c, "Perbarui status ini", "refresh", false).Disabled(disabled).Clicked() {
			a.selectSOPDelivery(item.ID)
		}
		if spaceButton(c, "Tutup snapshot", "close", false).Disabled(disabled).Clicked() {
			st.deliverySelected = nil
		}
	})
}
