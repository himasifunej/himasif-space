package appui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

type sopTransferDraft struct {
	division, person id.ID
	rule, reason     string
	preview          *api.SOPTransferPreview
	accepted         bool
}

func (a *App) loadSOPTransferPreview() {
	st := &a.evaluation
	d := st.transfer
	cl := a.notificationClient()
	if d == nil || st.selected == nil || cl == nil || st.busy || st.loading {
		return
	}
	gen, seq, detail, pid, did, fid := a.gen, st.seq, st.detailSeq, st.period, st.division, st.selected.ID
	target, rule := d.division, d.rule
	st.busy = true
	st.err = ""
	a.async(func() func() {
		preview, err := cl.SOPTransferPreview(context.Background(), pid, did, fid, target, rule)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.busy = false
			if detail != st.detailSeq || st.transfer != d || d.division != target || d.rule != rule {
				return
			}
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			d.preview = &preview
			d.accepted = false
			d.person = id.Nil
			if preview.Finding != nil && preview.Finding.PersonID != nil {
				d.person = *preview.Finding.PersonID
			}
		}
	})
}
func (a *App) sopTransferForm(c *ui.Context, disabled bool) {
	st := &a.evaluation
	targets := []api.SOPAccess{}
	for _, scope := range st.scopes {
		if scope.DivisionID != st.division && scope.CanReview && !scope.ReadOnly {
			targets = append(targets, scope)
		}
	}
	if len(targets) == 0 {
		muted(c, "Ralat lintas divisi memerlukan izin tinjauan di kedua divisi. Kadiv dapat memberikan izin evaluator sesuai kebutuhan.")
		return
	}
	pageSection(c, "Ralat atribusi lintas divisi", "Temuan sumber dibatalkan dan tujuan dibuka untuk tinjauan. Riwayat kedua divisi tetap tersimpan.", func() {
		if st.transfer == nil {
			if spaceButton(c, "Siapkan ralat lintas divisi", "edit", false).Disabled(disabled).Clicked() {
				st.transfer = &sopTransferDraft{division: targets[0].DivisionID, rule: st.selected.Rule}
			}
			return
		}
		d := st.transfer
		ui.Row(c).Wrap().Gap(5).Children(func() {
			for _, scope := range targets {
				if spaceButton(c.Key(scope.DivisionID.String()), scope.DivisionName, "", d.division == scope.DivisionID).Disabled(disabled).Clicked() {
					d.division = scope.DivisionID
					d.preview = nil
					d.accepted = false
				}
			}
		})
		if ui.TextInput(c, &d.rule).Label("Nomor aturan tujuan · 7.1 sampai 7.25").Placeholder("Nomor aturan tujuan · 7.1 sampai 7.25").FillWidth().Changed() {
			d.preview = nil
			d.accepted = false
		}
		if spaceButton(c, "Periksa temuan dan kebijakan tujuan", "search", false).Disabled(disabled).Clicked() {
			a.loadSOPTransferPreview()
		}
		if d.preview != nil {
			preview := d.preview
			muted(c, "Dokumen tujuan: "+preview.DocumentVersion+" · versi "+fmt.Sprint(preview.PolicyVersion))
			if preview.Finding == nil {
				muted(c, "Belum ada temuan untuk konten/aturan ini pada divisi tujuan.")
			} else {
				f := preview.Finding
				pageNotice(c, "alert", "Temuan tujuan sudah ada", sopStateLabel(f.State)+" · "+f.PersonName+". Ralat ini akan membuka tinjauan kembali; keputusan sebelumnya tetap ada pada riwayat.")
				a.sopObservationView(c, "Bukti terbaru pada tujuan", f.Latest)
				if f.Decision != nil {
					a.sopObservationView(c, "Dasar keputusan tujuan sebelumnya", *f.Decision)
					ui.Text(c, f.DecisionReason)
				}
			}
			ui.Text(c, "Atribusi yang diusulkan pada tujuan").Bold()
			ui.Row(c).Wrap().Gap(5).Children(func() {
				if spaceButton(c, "Divisi tanpa individu", "users", d.person == id.Nil).Disabled(disabled).Clicked() {
					d.person = id.Nil
					d.accepted = false
				}
				for _, person := range preview.People {
					label := person.DisplayName
					if !person.Active {
						label += " · arsip"
					}
					if spaceButton(c.Key(person.ID.String()), label, "", d.person == person.ID).Disabled(disabled).Clicked() {
						d.person = person.ID
						d.accepted = false
					}
				}
			})
			if ui.TextArea(c, &d.reason).Label("Alasan ralat dan hubungan bukti dengan divisi tujuan").FillWidth().Height(90).Changed() {
				d.accepted = false
			}
			ui.Checkbox(c, &d.accepted, "Saya telah memeriksa sumber dan tujuan; batalkan atribusi sumber dan buka tinjauan tujuan.").Disabled(disabled)
			if spaceButton(c, "Simpan ralat lintas divisi", "check", false).Disabled(disabled || !d.accepted || strings.TrimSpace(d.reason) == "").Clicked() {
				f := st.selected
				person := d.person
				req := api.SOPTransferRequest{ExpectedVersion: f.Version, ObservationID: f.Latest.ID, TargetDivisionID: d.division, TargetRule: d.rule, TargetPersonID: &person, TargetPolicyVersion: preview.PolicyVersion, Reason: d.reason}
				if preview.Finding != nil {
					req.TargetFindingVersion = preview.Finding.Version
				}
				a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
					_, err := cl.TransferSOPFinding(context.Background(), pid, did, f.ID, req)
					return err
				})
			}
		}
		if spaceButton(c, "Batalkan pengisian ralat", "close", false).Disabled(disabled).Clicked() {
			st.transfer = nil
		}
	})
}
