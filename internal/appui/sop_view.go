package appui

import (
	"context"
	"errors"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"net/url"
	"strings"
)

type sopUIState struct {
	readiness                *api.SOPReadiness
	readinessContent         id.ID
	exportData               []byte
	exportName, exportStatus string
	transfer                 *sopTransferDraft
	deliveries               []api.SOPDelivery
	deliveryCursor           string
	deliveryMore             bool
	deliverySelected         *api.SOPDelivery

	sourceURLs                                 map[string]*string
	seq, detailSeq                             uint64
	user, period, division                     id.ID
	scopesLoaded, loading, busy, detailLoading bool
	scopes                                     []api.SOPAccess
	tab, err, from, to, rule, state            string
	filterPerson                               id.ID
	items                                      []api.SOPFinding
	cursor                                     string
	more                                       bool
	policy                                     api.SOPPolicy
	people                                     []api.SOPPerson
	contents                                   []api.SOPContentRef
	grants                                     []api.SOPEvaluatorGrant
	candidates                                 []api.SOPEvaluatorCandidate
	selected                                   *api.SOPFinding
	history                                    api.SOPHistory
	reason, outcome                            string
	person                                     id.ID
	report                                     *api.SOPReport
	reportFrom, reportTo, quarter              string
	policyDraft                                *sopPolicyDraft
	contextDraft                               *sopContextDraft
	manualDraft                                *sopManualDraft
	grantUser                                  id.ID
	grantReview                                bool
	grantReason                                string
}

func (a *App) sopCurrent(gen int, seq uint64) bool {
	return gen == a.gen && seq == a.evaluation.seq && a.evaluation.user == a.user.ID && a.evaluation.period == a.periodID
}
func (a *App) sopAccess() *api.SOPAccess {
	for index := range a.evaluation.scopes {
		if a.evaluation.scopes[index].DivisionID == a.evaluation.division {
			return &a.evaluation.scopes[index]
		}
	}
	return nil
}
func (a *App) sopError(gen int, seq uint64, err error) {
	message := a.failed(err)
	if !a.sopCurrent(gen, seq) {
		return
	}
	st := &a.evaluation
	st.err = message
	var remote *client.Error
	if errors.As(err, &remote) && (remote.Status == 403 || remote.Status == 404) {
		st.items = nil
		st.people = nil
		st.contents = nil
		st.policy = api.SOPPolicy{}
		st.selected = nil
		st.transfer = nil
		st.history = api.SOPHistory{}
		st.report = nil
		st.exportData = nil
		st.exportStatus = ""
		st.deliveries = nil
		st.readiness = nil
		st.deliverySelected = nil
		st.contextDraft = nil
		st.manualDraft = nil
		st.policyDraft = nil
		st.sourceURLs = nil
		st.grants = nil
		st.candidates = nil
	}
}
func (a *App) ensureSOP() {
	if a.period() == nil {
		return
	}
	st := &a.evaluation
	if st.user != a.user.ID || st.period != a.periodID {
		*st = sopUIState{seq: st.seq + 1, user: a.user.ID, period: a.periodID, tab: "Temuan"}
	}
	if !st.scopesLoaded && !st.loading && st.err == "" {
		a.loadSOPScopes()
	}
}
func (a *App) loadSOPScopes() {
	cl := a.notificationClient()
	st := &a.evaluation
	if cl == nil {
		st.err = "Layanan evaluasi SOP belum tersedia."
		return
	}
	if st.loading || st.busy {
		return
	}
	gen, seq, pid := a.gen, st.seq, st.period
	st.loading = true
	st.err = ""
	a.async(func() func() {
		scopes, err := cl.SOPScopes(context.Background(), pid)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.loading = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			st.scopes = scopes
			st.scopesLoaded = true
			if len(scopes) == 0 {
				a.selectSOPScope(id.Nil)
				return
			}
			if a.sopAccess() == nil {
				a.selectSOPScope(scopes[0].DivisionID)
				return
			}
			a.loadSOPScope()
		}
	})
}
func (a *App) selectSOPScope(did id.ID) {
	st := &a.evaluation
	scopes := st.scopes
	seq, user, period := st.seq+1, st.user, st.period
	*st = sopUIState{seq: seq, user: user, period: period, division: did, scopes: scopes, scopesLoaded: true, tab: "Temuan"}
	a.loadSOPScope()
}
func (a *App) sopFilter() url.Values {
	st := &a.evaluation
	values := url.Values{}
	if st.from != "" {
		values.Set("from", st.from)
	}
	if st.to != "" {
		values.Set("to", st.to)
	}
	if st.rule != "" {
		values.Set("rule", st.rule)
	}
	if st.state != "" {
		values.Set("state", st.state)
	}
	if st.filterPerson != id.Nil {
		values.Set("personId", st.filterPerson.String())
	}
	return values
}
func (a *App) loadSOPScope() {
	cl := a.notificationClient()
	st := &a.evaluation
	ac := a.sopAccess()
	if cl == nil || ac == nil || st.loading || st.busy {
		return
	}
	st.detailSeq++
	st.selected = nil
	st.transfer = nil
	st.detailLoading = false
	st.history = api.SOPHistory{}
	gen, seq, pid, did, configure := a.gen, st.seq, st.period, st.division, ac.CanConfigure
	filter := a.sopFilter()
	st.loading = true
	st.err = ""
	a.async(func() func() {
		policy, err := cl.SOPPolicy(context.Background(), pid, did)
		var people []api.SOPPerson
		var contents []api.SOPContentRef
		var grants []api.SOPEvaluatorGrant
		var candidates []api.SOPEvaluatorCandidate
		var page api.SOPFindingPage
		if err == nil {
			people, err = cl.SOPPeople(context.Background(), pid, did)
		}
		if err == nil {
			contents, err = cl.SOPContents(context.Background(), pid, did)
		}
		if err == nil {
			page, err = cl.SOPFindingPage(context.Background(), pid, did, filter)
		}
		if err == nil && configure {
			grants, err = cl.SOPEvaluatorGrants(context.Background(), pid, did)
		}
		if err == nil && configure {
			candidates, err = cl.SOPEvaluatorCandidates(context.Background(), pid, did)
		}
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.loading = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			st.policy = policy
			st.people = people
			st.contents = contents
			st.grants = grants
			st.candidates = candidates
			st.items = page.Items
			st.cursor = page.NextCursor
			st.more = page.HasMore
			st.selected = nil
			st.transfer = nil
			st.detailSeq++
			st.history = api.SOPHistory{}
			st.policyDraft = newSOPPolicyDraft(policy)
			st.contextDraft = nil
			st.manualDraft = nil
			if st.reportFrom == "" {
				st.reportFrom = a.period().StartsOn.String()
				st.reportTo = a.period().EndsOn.String()
			}
		}
	})
}
func (a *App) loadSOPPage(more bool) {
	cl := a.notificationClient()
	st := &a.evaluation
	if cl == nil || st.loading || st.busy {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	filter := a.sopFilter()
	if more {
		filter.Set("cursor", st.cursor)
	}
	st.loading = true
	st.err = ""
	a.async(func() func() {
		page, err := cl.SOPFindingPage(context.Background(), pid, did, filter)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.loading = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			if more {
				for _, item := range page.Items {
					found := false
					for index := range st.items {
						if st.items[index].ID == item.ID {
							st.items[index] = item
							found = true
							break
						}
					}
					if !found {
						st.items = append(st.items, item)
					}
				}
			} else {
				st.items = page.Items
				st.selected = nil
				st.transfer = nil
				st.detailSeq++
			}
			st.more = page.HasMore
			st.cursor = page.NextCursor
		}
	})
}
func (a *App) selectSOPFinding(fid id.ID) {
	cl := a.notificationClient()
	st := &a.evaluation
	if cl == nil || st.busy {
		return
	}
	st.detailSeq++
	gen, seq, detail, pid, did := a.gen, st.seq, st.detailSeq, st.period, st.division
	st.detailLoading = true
	st.err = ""
	a.async(func() func() {
		finding, err := cl.SOPFinding(context.Background(), pid, did, fid)
		var history api.SOPHistory
		if err == nil {
			history, err = cl.SOPFindingHistory(context.Background(), pid, did, fid)
		}
		return func() {
			if !a.sopCurrent(gen, seq) || detail != a.evaluation.detailSeq {
				return
			}
			st := &a.evaluation
			st.detailLoading = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			st.selected = &finding
			st.transfer = nil
			st.sourceURLs = map[string]*string{}
			st.history = history
			st.reason = ""
			st.person = id.Nil
			if finding.Latest.ProposedPersonID != nil {
				st.person = *finding.Latest.ProposedPersonID
			}
			st.outcome = finding.State
			if st.outcome == "corrected" {
				st.outcome = "needs_review"
			}
		}
	})
}
func (a *App) sopWrite(work func(*client.Client, id.ID, id.ID) error) {
	cl := a.notificationClient()
	st := &a.evaluation
	if cl == nil || st.busy || st.loading || a.offlineMode() {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.busy = true
	st.err = ""
	a.async(func() func() {
		err := work(cl, pid, did)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			a.evaluation.busy = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			a.loadSOPScope()
		}
	})
}
func (a *App) sopDecision(action string) {
	st := &a.evaluation
	if st.selected == nil {
		return
	}
	req := api.SOPDecisionRequest{Action: action, Outcome: st.outcome, ExpectedVersion: st.selected.Version, ObservationID: st.selected.Latest.ID, Reason: st.reason}
	person := st.person
	req.PersonID = &person
	fid := st.selected.ID
	a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
		_, err := cl.DecideSOPFinding(context.Background(), pid, did, fid, req)
		return err
	})
}
func (a *App) sopView(c *ui.Context) {
	a.ensureSOP()
	pageBody(c, "Evaluasi SOP", func() {
		a.periodLine(c)
		st := &a.evaluation
		if a.period() == nil {
			muted(c, "Pilih periode terlebih dahulu.")
			return
		}
		if st.err != "" {
			pageNotice(c, "alert", "Permintaan belum selesai", st.err)
		}
		if a.offlineMode() {
			pageNotice(c, "info", "Sedang offline", "Evaluasi dan keputusan memerlukan koneksi untuk memeriksa bukti dan izin terbaru.")
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, scope := range st.scopes {
				if spaceButton(c.Key(scope.DivisionID.String()), scope.DivisionName, "shield", scope.DivisionID == st.division).Disabled(st.loading || st.busy).Clicked() {
					a.selectSOPScope(scope.DivisionID)
				}
			}
			if spaceButton(c, "Muat ulang izin dan data", "refresh", false).Disabled(st.loading || st.busy || a.offlineMode()).Clicked() {
				a.loadSOPScopes()
			}
		})
		if st.loading {
			muted(c, "Memuat evaluasi…")
		}
		ac := a.sopAccess()
		if ac == nil {
			if st.scopesLoaded {
				pageNotice(c, "shield", "Akses evaluasi belum tersedia", "Kadiv dapat memberikan izin SOP sesuai divisi. Izin ini terpisah dari kecocokan hasil dan akses Notes.")
			}
			return
		}
		if st.policy.Version == 0 && !st.loading {
			pageNotice(c, "info", "Kebijakan belum ditetapkan", "Kadiv perlu menyimpan versi dokumen dan konfigurasi sebelum evaluasi/laporan digunakan.")
		}
		if ac.ReadOnly {
			pageNotice(c, "lock", "Periode hanya baca", "Riwayat tetap tersedia; keputusan dan konfigurasi tidak dapat diubah.")
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, tab := range []string{"Temuan", "Konteks", "Aturan", "Laporan", "Pengiriman", "Izin"} {
				if tab == "Izin" && !ac.CanConfigure {
					continue
				}
				if spaceButton(c.Key(tab), tab, "", st.tab == tab).Clicked() {
					st.tab = tab
				}
			}
		})
		switch st.tab {
		case "Aturan":
			a.sopPolicyForm(c, *ac)
		case "Konteks":
			a.sopContextForm(c, *ac)
		case "Izin":
			a.sopGrantsForm(c, *ac)
		case "Laporan":
			a.sopReportView(c)
		case "Pengiriman":
			a.sopDeliveriesView(c)
		default:
			a.sopFindingsView(c, *ac)
		}
	})
}
func sopStateLabel(state string) string {
	if label := map[string]string{"needs_review": "Perlu tinjauan", "confirmed": "Disahkan", "dismissed": "Dibatalkan", "corrected": "Diralat"}[state]; label != "" {
		return label
	}
	return state
}
func sopMeasurementLabel(value string) string {
	if label := map[string]string{"late": "Melewati tenggat", "insufficient": "Bukti belum cukup", "on_time": "Tepat waktu", "exempt": "Kesepakatan/pengecualian", "within_limit": "Dalam batas", "exceeds_limit": "Melewati batas", "not_applicable": "Tidak berlaku", "manual": "Bukti manual"}[value]; label != "" {
		return label
	}
	return value
}
func sopDeadlineText(d sop.Deadline, zone string) string {
	if d.At == nil && d.Date == nil {
		return "Belum dapat ditetapkan"
	}
	return taskDate(d.Date, d.At, zone)
}
func (a *App) sopPersonName(uid id.ID) string {
	for _, person := range a.evaluation.people {
		if person.ID == uid {
			return person.DisplayName
		}
	}
	return "Tanpa atribusi individu"
}
func (a *App) sopFindingsView(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	disabled := st.loading || st.busy || a.offlineMode()
	pageNotice(c, "shield", "Temuan awal memerlukan tinjauan", "Pencocokan hasil, persetujuan mutu dan pengesahan SOP merupakan tindakan terpisah.")
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.TextInput(c, &st.from).Label("Mulai · YYYY-MM-DD").Width(200)
		ui.TextInput(c, &st.to).Label("Sampai · YYYY-MM-DD").Width(200)
		ui.TextInput(c, &st.rule).Label("Poin SOP · opsional").Width(180)
	})
	ui.Row(c).Wrap().Gap(5).Children(func() {
		for _, state := range []string{"", "needs_review", "confirmed", "dismissed", "corrected"} {
			label := sopStateLabel(state)
			if state == "" {
				label = "Semua status"
			}
			if spaceButton(c.Key("status"+state), label, "", st.state == state).Clicked() {
				st.state = state
			}
		}
	})
	ui.Row(c).Wrap().Gap(5).Children(func() {
		if spaceButton(c, "Semua penanggung jawab", "users", st.filterPerson == id.Nil).Clicked() {
			st.filterPerson = id.Nil
		}
		for _, person := range st.people {
			if spaceButton(c.Key("person"+person.ID.String()), person.DisplayName, "", st.filterPerson == person.ID).Clicked() {
				st.filterPerson = person.ID
			}
		}
	})
	if spaceButton(c, "Terapkan filter", "search", false).Disabled(disabled).Clicked() {
		a.loadSOPPage(false)
	}
	if ac.CanReview {
		if spaceButton(c, "Catat bukti/temuan manual", "plus", false).Disabled(disabled || st.policy.Version == 0).Clicked() {
			st.manualDraft = newSOPManualDraft(st.policy.Version)
			st.tab = "Konteks"
		}
	}
	if st.detailLoading {
		muted(c, "Memuat bukti dan riwayat terbaru…")
	}
	if st.selected != nil {
		a.sopFindingDetail(c, ac, disabled || st.detailLoading)
	}
	for _, finding := range st.items {
		finding := finding
		ui.Column(c.Key(finding.ID.String())).FillWidth().Padding(14).Gap(6).Border(1, c.Theme().Border).Radius(10).Children(func() {
			ui.Text(c, finding.ContentTitle+" · "+finding.Rule).Bold()
			muted(c, finding.EventOn.String()+" · "+finding.PersonName+" · "+sopStateLabel(finding.State))
			ui.Text(c, sopMeasurementLabel(finding.Latest.Observation.Measurement))
			if finding.NeedsRecheck {
				muted(c, "Sumber berubah; keputusan perlu diperiksa ulang.")
			}
			if spaceButton(c, "Periksa bukti dan keputusan", "search", false).Disabled(disabled).Clicked() {
				a.selectSOPFinding(finding.ID)
			}
		})
	}
	if st.more {
		if spaceButton(c, "Muat halaman berikutnya", "plus", false).Disabled(disabled).Clicked() {
			a.loadSOPPage(true)
		}
	} else if !st.loading && len(st.items) == 0 {
		muted(c, "Tidak ada temuan pada filter ini.")
	}
}
func (a *App) sopObservationView(c *ui.Context, label string, o api.SOPObservation) {
	pageSection(c, label, o.DocumentVersion, func() {
		ui.Text(c, sopMeasurementLabel(o.Observation.Measurement)).Bold()
		muted(c, "Tenggat: "+sopDeadlineText(o.Observation.Deadline, a.period().TimeZone))
		ui.Text(c, o.Observation.Reason)
		for index, e := range o.Observation.Evidence {
			ui.Column(c.Key(index)).FillWidth().Gap(3).Children(func() {
				when := "Waktu belum tersedia"
				if e.At != nil || e.Date != nil {
					when = taskDate(e.Date, e.At, a.period().TimeZone)
				}
				muted(c, when)
				if e.Verified {
					muted(c, "Bukti dikonfirmasi pada sumber terkait")
				} else {
					muted(c, "Bukti belum terverifikasi")
				}
				if e.URL != "" {
					st := &a.evaluation
					if st.sourceURLs == nil {
						st.sourceURLs = map[string]*string{}
					}
					key := fmt.Sprintf("%s:%d", o.ID, index)
					if st.sourceURLs[key] == nil {
						copy := e.URL
						st.sourceURLs[key] = &copy
					}
					ui.TextInput(c.Key(key), st.sourceURLs[key]).Label("Salin tautan sumber").FillWidth()
					if ui.SafeToOpen(e.URL) && spaceButton(c.Key(key+"open"), "Buka sumber", "link", false).Clicked() {
						c.OpenURL(e.URL)
					}
				}
				if e.Note != "" {
					ui.Text(c, e.Note)
				}
			})
		}
	})
}
func (a *App) sopFindingDetail(c *ui.Context, ac api.SOPAccess, disabled bool) {
	st := &a.evaluation
	f := st.selected
	pageSection(c, "Tinjauan: "+f.ContentTitle, "Periksa bukti terbaru dan keputusan sebelumnya sebelum menetapkan hasil.", func() {
		a.sopObservationView(c, "Observasi terbaru", f.Latest)
		if f.Decision != nil {
			a.sopObservationView(c, "Dasar keputusan terakhir", *f.Decision)
			ui.Text(c, sopStateLabel(f.State)+" · "+f.DecidedBy)
			ui.Text(c, f.DecisionReason)
		}
		if f.NeedsRecheck {
			pageNotice(c, "alert", "Sumber berubah", "Keputusan sebelumnya tetap tercatat. Periksa perubahan sumber pada riwayat.")
		}
		if ac.CanReview && !ac.ReadOnly {
			ui.Text(c, "Atribusi yang akan berlaku").Bold()
			ui.Row(c).Wrap().Gap(5).Children(func() {
				if spaceButton(c, "Divisi tanpa individu", "users", st.person == id.Nil).Disabled(disabled).Clicked() {
					st.person = id.Nil
				}
				for _, person := range st.people {
					label := person.DisplayName
					if !person.Active {
						label += " · arsip"
					}
					if spaceButton(c.Key(person.ID.String()), label, "", st.person == person.ID).Disabled(disabled).Clicked() {
						st.person = person.ID
					}
				}
			})
			ui.TextArea(c, &st.reason).Label("Alasan keputusan / ralat").FillWidth().Height(90)
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, action := range []struct{ code, label string }{{"confirm", "Sahkan temuan"}, {"dismiss", "Batalkan dugaan"}, {"reopen", "Buka tinjauan ulang"}} {
					if spaceButton(c.Key(action.code), action.label, "check", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
						a.sopDecision(action.code)
					}
				}
			})
			ui.Row(c).Wrap().Gap(5).Children(func() {
				for _, outcome := range []string{"confirmed", "dismissed", "needs_review"} {
					if spaceButton(c.Key("outcome"+outcome), sopStateLabel(outcome), "", st.outcome == outcome).Disabled(disabled).Clicked() {
						st.outcome = outcome
					}
				}
			})
			if spaceButton(c, "Simpan ralat dengan hasil terpilih", "edit", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
				a.sopDecision("correct")
			}
			if spaceButton(c, "Tambahkan bukti manual ke temuan ini", "plus", false).Disabled(disabled).Clicked() {
				st.manualDraft = newSOPManualDraft(st.policy.Version)
				st.manualDraft.content = f.ContentID
				st.manualDraft.rule = f.Rule
				st.manualDraft.findingVersion = f.Version
				st.manualDraft.person = st.person
				st.tab = "Konteks"
			}
		}
		if ac.CanReview && !ac.ReadOnly {
			a.sopTransferForm(c, disabled)
		}
		pageSection(c, "Riwayat", "Keputusan dan perubahan bukti tetap tersimpan.", func() {
			for _, transfer := range st.history.Transfers {
				ui.Text(c, "Ralat lintas divisi · "+transfer.ActorName+" · "+taskTimestamp(transfer.CreatedAt, a.period().TimeZone))
				muted(c, transfer.SourceRule+" → "+transfer.TargetRule+" · "+transfer.Reason)
				muted(c, "Temuan sumber: "+transfer.SourceFindingID.String()+" · tujuan: "+transfer.TargetFindingID.String())
			}
			for _, d := range st.history.Decisions {
				ui.Text(c, d.ActorName+" · "+d.Action+" · "+taskTimestamp(d.CreatedAt, a.period().TimeZone))
				muted(c, d.Reason)
			}
			for _, change := range st.history.SourceChanges {
				muted(c, "Perubahan sumber "+change.SourceType+" · "+taskTimestamp(change.RecordedAt, a.period().TimeZone))
			}
			for _, o := range st.history.Observations {
				muted(c, o.DocumentVersion+" · "+sopMeasurementLabel(o.Observation.Measurement)+" · "+taskTimestamp(o.RecordedAt, a.period().TimeZone))
			}
		})
		if spaceButton(c, "Muat ulang bukti ini", "refresh", false).Disabled(disabled).Clicked() {
			a.selectSOPFinding(f.ID)
		}
		if spaceButton(c, "Tutup tinjauan", "close", false).Disabled(disabled).Clicked() {
			st.selected = nil
			st.transfer = nil
			st.history = api.SOPHistory{}
			st.detailSeq++
		}
	})
}
func (a *App) sopReportView(c *ui.Context) {
	st := &a.evaluation
	disabled := st.busy || st.loading || a.offlineMode()
	cl := a.notificationClient()
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, quarter := range st.policy.Settings.Quarters {
			if spaceButton(c.Key(quarter.Label), quarter.Label, "calendar", st.quarter == quarter.Label).Disabled(disabled).Clicked() {
				st.quarter = quarter.Label
				st.reportFrom = quarter.StartsOn.String()
				st.reportTo = quarter.EndsOn.String()
			}
		}
	})
	if ui.TextInput(c, &st.reportFrom).Label("Awal rentang · YYYY-MM-DD").FillWidth().Changed() {
		st.quarter = ""
	}
	if ui.TextInput(c, &st.reportTo).Label("Akhir rentang · YYYY-MM-DD").FillWidth().Changed() {
		st.quarter = ""
	}
	if spaceButton(c, "Buat laporan", "archive", false).Disabled(disabled || cl == nil || st.policy.Version == 0).Clicked() {
		gen, seq, pid, did := a.gen, st.seq, st.period, st.division
		query := url.Values{"from": {st.reportFrom}, "to": {st.reportTo}}
		if st.quarter != "" {
			query.Set("quarter", st.quarter)
		}
		st.busy = true
		st.err = ""
		a.async(func() func() {
			report, err := cl.SOPReport(context.Background(), pid, did, query)
			return func() {
				if !a.sopCurrent(gen, seq) {
					return
				}
				a.evaluation.busy = false
				if err != nil {
					a.sopError(gen, seq, err)
					return
				}
				a.evaluation.report = &report
				a.evaluation.exportData = nil
				a.evaluation.exportStatus = ""
			}
		})
	}
	if st.report == nil {
		muted(c, "Quarter memakai rentang yang ditetapkan pengelola, bukan kuartal kalender asumsi.")
		return
	}
	report := st.report
	a.sopExportView(c, disabled)
	ui.Text(c, report.DivisionName+" · "+report.Label).Bold()
	muted(c, report.StartsOn.String()+" — "+report.EndsOn.String())
	summary := report.Summary
	info(c, [][2]string{{"Total temuan", fmt.Sprint(summary.Total)}, {"Perlu tinjauan", fmt.Sprint(summary.NeedsReview)}, {"Disahkan", fmt.Sprint(summary.Confirmed)}, {"Dibatalkan", fmt.Sprint(summary.Dismissed)}, {"Perlu diperiksa ulang", fmt.Sprint(summary.NeedsRecheck)}, {"Bukti belum cukup", fmt.Sprint(summary.InsufficientEvidence)}})
	muted(c, "Laporan memakai keputusan manusia terakhir. Keputusan yang sumbernya berubah ditandai untuk diperiksa ulang.")
	if spaceButton(c, "Lihat daftar pada rentang ini", "search", false).Disabled(disabled).Clicked() {
		st.from = report.StartsOn.String()
		st.to = report.EndsOn.String()
		st.rule = ""
		st.state = ""
		st.filterPerson = id.Nil
		st.tab = "Temuan"
		a.loadSOPPage(false)
	}
}
