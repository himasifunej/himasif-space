package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"slices"
	"strings"
	"time"
)

type sopQuarterDraft struct{ label, from, to string }
type sopPolicyDraft struct {
	original                                                            api.SOPPolicy
	enabled                                                             bool
	document, followup, weekly, urgentFrom, urgentTo, agreement, reason string
	weekday                                                             int
	rules                                                               []string
	quarters                                                            []*sopQuarterDraft
	botCounts                                                           bool
}

func newSOPPolicyDraft(p api.SOPPolicy) *sopPolicyDraft {
	d := &sopPolicyDraft{original: p, enabled: p.Enabled, document: p.DocumentVersion, followup: p.Settings.FollowupClock, weekly: p.Settings.WeeklyClock, weekday: -1, rules: slices.Clone(p.Settings.Rules), botCounts: p.Settings.BotFollowupCountsRequested, agreement: p.Settings.BotAgreementNote}
	if d.document == "" {
		d.document = "SOP Medhum revisi 28 Maret 2026"
	}
	if p.Settings.WeeklyWeekday != nil {
		d.weekday = *p.Settings.WeeklyWeekday
	}
	if p.Settings.UrgentWindowStart != nil {
		d.urgentFrom = p.Settings.UrgentWindowStart.String()
	}
	if p.Settings.UrgentWindowEnd != nil {
		d.urgentTo = p.Settings.UrgentWindowEnd.String()
	}
	for _, q := range p.Settings.Quarters {
		d.quarters = append(d.quarters, &sopQuarterDraft{label: q.Label, from: q.StartsOn.String(), to: q.EndsOn.String()})
	}
	return d
}
func parseOptionalSOPDate(raw string) (*timeutil.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	date, err := timeutil.ParseDate(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("tanggal harus YYYY-MM-DD")
	}
	return &date, nil
}
func (d *sopPolicyDraft) request() (api.SOPPolicyRequest, error) {
	req := api.SOPPolicyRequest{SpaceID: d.original.SpaceID, DocumentVersion: d.document, Enabled: d.enabled, ExpectedVersion: d.original.Version, Reason: d.reason, Settings: api.SOPSettings{Rules: slices.Clone(d.rules), FollowupClock: strings.TrimSpace(d.followup), WeeklyClock: strings.TrimSpace(d.weekly), BotFollowupCountsRequested: d.botCounts, BotAgreementNote: d.agreement, Quarters: []api.SOPQuarter{}}}
	if d.weekday >= 0 {
		day := d.weekday
		req.Settings.WeeklyWeekday = &day
	}
	var err error
	req.Settings.UrgentWindowStart, err = parseOptionalSOPDate(d.urgentFrom)
	if err != nil {
		return req, err
	}
	req.Settings.UrgentWindowEnd, err = parseOptionalSOPDate(d.urgentTo)
	if err != nil {
		return req, err
	}
	for _, q := range d.quarters {
		if strings.TrimSpace(q.label+q.from+q.to) == "" {
			continue
		}
		from, err := parseOptionalSOPDate(q.from)
		if err != nil {
			return req, err
		}
		to, err := parseOptionalSOPDate(q.to)
		if err != nil {
			return req, err
		}
		if from == nil || to == nil {
			return req, fmt.Errorf("awal dan akhir setiap quarter harus diisi")
		}
		req.Settings.Quarters = append(req.Settings.Quarters, api.SOPQuarter{Label: q.label, StartsOn: *from, EndsOn: *to})
	}
	return req, nil
}

var sopRuleChoices = []struct{ code, label, owner string }{{sop.ConceptH7, "7.5 · Permintaan matang H-7", "humas"}, {sop.DesignH2, "7.11 · Penyerahan desain H-2", "mediatek"}, {sop.Urgent24, "7.12 · Konfirmasi dadakan 24 jam", "humas"}, {sop.FollowupH4, "7.14 · Follow-up H-4", "humas"}, {sop.FinalH12, "7.19 · Final H-12", "mediatek"}, {sop.UrgentLimit, "7.6 · Batas lima dadakan", "humas"}}

func (a *App) sopPolicyForm(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	d := st.policyDraft
	if d == nil {
		muted(c, "Kebijakan belum dimuat.")
		return
	}
	disabled := st.busy || st.loading || ac.ReadOnly || a.offlineMode()
	info(c, [][2]string{{"Dokumen", st.policy.DocumentVersion}, {"Otomatisasi aktif", waYesNo(st.policy.Enabled)}, {"Pemenuhan lewat bot diminta", waYesNo(st.policy.Settings.BotFollowupCountsRequested)}, {"Disetujui Litbang pada versi ini", waYesNo(st.policy.BotFollowupCountsApproved)}})
	if ac.CanConfigure {
		ui.TextInput(c, &d.document).Label("Versi dokumen SOP").Placeholder("Versi dokumen SOP").FillWidth()
		muted(c, "Jam posting/cutoff untuk aturan berbasis jam ditetapkan melalui kalender konten sesuai kewenangan Humas dan Mediatek.")
		if spaceButton(c, "Otomatisasi aktif: "+waYesNo(d.enabled), "check", d.enabled).Disabled(disabled).Clicked() {
			d.enabled = !d.enabled
		}
		for _, choice := range sopRuleChoices {
			if choice.owner != ac.DivisionSlug {
				continue
			}
			if spaceButton(c.Key(choice.code), choice.label, "check", slices.Contains(d.rules, choice.code)).Disabled(disabled).Clicked() {
				d.rules = toggleString(d.rules, choice.code)
			}
		}
		if ac.DivisionSlug == "humas" {
			ui.TextInput(c, &d.followup).Label("Jam follow-up H-4 · HH:MM; kosong untuk nonaktif").Placeholder("Jam follow-up H-4 · HH:MM; kosong untuk nonaktif").FillWidth()
			ui.TextInput(c, &d.urgentFrom).Label("Awal rentang hitung dadakan · YYYY-MM-DD").Placeholder("Awal rentang hitung dadakan · YYYY-MM-DD").FillWidth()
			ui.TextInput(c, &d.urgentTo).Label("Akhir rentang hitung dadakan · YYYY-MM-DD").Placeholder("Akhir rentang hitung dadakan · YYYY-MM-DD").FillWidth()
			muted(c, "Batas SOP: lima konten nonpengecualian. Rentang tidak ditentukan otomatis.")
			if spaceButton(c, "Ajukan pemenuhan follow-up lewat bot", "check", d.botCounts).Disabled(disabled).Clicked() {
				d.botCounts = !d.botCounts
			}
			ui.TextArea(c, &d.agreement).Label("Dasar kesepakatan untuk ditinjau Litbang").FillWidth().Height(80)
		}
		pageSection(c, "Rekap mingguan", "Rekap hanya ke grup divisi ini; tidak ke Medhum.", func() {
			ui.Row(c).Wrap().Gap(4).Children(func() {
				for day, label := range []string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"} {
					if spaceButton(c.Key(day), label, "", d.weekday == day).Disabled(disabled).Clicked() {
						d.weekday = day
					}
				}
			})
			ui.TextInput(c, &d.weekly).Label("Jam rekap · HH:MM").Placeholder("Jam rekap · HH:MM").FillWidth()
			if spaceButton(c, "Nonaktifkan rekap terjadwal", "close", false).Disabled(disabled).Clicked() {
				d.weekday = -1
				d.weekly = ""
			}
		})
		pageSection(c, "Rentang quarter", "Tetapkan batas yang disepakati; tidak memakai kuartal kalender asumsi.", func() {
			remove := -1
			for index, q := range d.quarters {
				ui.Row(c.Key(index)).Wrap().Gap(6).Children(func() {
					ui.TextInput(c, &q.label).Label("Label").Placeholder("Label").Width(160)
					ui.TextInput(c, &q.from).Label("Mulai · YYYY-MM-DD").Placeholder("Mulai · YYYY-MM-DD").Width(200)
					ui.TextInput(c, &q.to).Label("Sampai · YYYY-MM-DD").Placeholder("Sampai · YYYY-MM-DD").Width(200)
					if spaceButton(c, "Hapus rentang", "close", false).Disabled(disabled).Clicked() {
						remove = index
					}
				})
			}
			if remove >= 0 {
				d.quarters = append(d.quarters[:remove], d.quarters[remove+1:]...)
			}
			if spaceButton(c, "Tambah quarter", "plus", false).Disabled(disabled || len(d.quarters) >= 12).Clicked() {
				d.quarters = append(d.quarters, &sopQuarterDraft{})
			}
		})
		ui.TextArea(c, &d.reason).Label("Alasan perubahan kebijakan").FillWidth().Height(70)
		if spaceButton(c, "Simpan kebijakan", "check", true).Disabled(disabled || strings.TrimSpace(d.reason) == "").Clicked() {
			req, err := d.request()
			if err != nil {
				st.err = err.Error()
				return
			}
			a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
				_, err := cl.SetSOPPolicy(context.Background(), pid, did, req)
				return err
			})
		}
	}
	if ac.CanApproveBot && st.policy.Version > 0 {
		pageSection(c, "Persetujuan Litbang", "Persetujuan berlaku untuk versi kebijakan ini. Perubahan kebijakan memerlukan tinjauan baru.", func() {
			ui.Text(c, st.policy.Settings.BotAgreementNote)
			ui.TextArea(c, &st.grantReason).Label("Alasan persetujuan / pencabutan").FillWidth().Height(80)
			for _, option := range []struct {
				approved bool
				label    string
			}{{true, "Setujui pemenuhan melalui bot"}, {false, "Cabut persetujuan versi ini"}} {
				if spaceButton(c.Key(option.label), option.label, "check", false).Disabled(disabled || strings.TrimSpace(st.grantReason) == "").Clicked() {
					approved, version, reason := option.approved, st.policy.Version, st.grantReason
					a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
						return cl.SOPBotAgreement(context.Background(), pid, did, approved, version, reason)
					})
				}
			}
		})
	}
}

type sopEvidenceDraft struct {
	original                                          sop.Evidence
	date, clock, initialDate, initialClock, url, note string
	verified                                          bool
}

func newSOPEvidenceDraft(e sop.Evidence, zone string) *sopEvidenceDraft {
	d := &sopEvidenceDraft{original: e, url: e.URL, note: e.Note, verified: e.Verified}
	if e.Date != nil {
		d.date = e.Date.String()
	}
	if e.At != nil {
		loc, err := timeutil.LoadZone(zone)
		if err != nil {
			loc = time.UTC
		}
		t := e.At.In(loc)
		d.date = t.Format("2006-01-02")
		d.clock = t.Format("15:04:05")
	}
	d.initialDate = d.date
	d.initialClock = d.clock
	return d
}
func (d *sopEvidenceDraft) value(zone string) (sop.Evidence, error) {
	e := d.original
	e.URL = d.url
	e.Note = d.note
	e.Verified = d.verified
	if d.date == d.initialDate && d.clock == d.initialClock {
		return e, nil
	}
	e.At = nil
	e.Date = nil
	date, err := parseOptionalSOPDate(d.date)
	if err != nil {
		return e, err
	}
	if date == nil {
		if strings.TrimSpace(d.clock) != "" {
			return e, fmt.Errorf("jam bukti memerlukan tanggal kejadian")
		}
		return e, nil
	}
	if strings.TrimSpace(d.clock) == "" {
		e.Date = date
		return e, nil
	}
	clock, err := time.Parse("15:04:05", strings.TrimSpace(d.clock))
	if err != nil {
		clock, err = time.Parse("15:04", strings.TrimSpace(d.clock))
	}
	if err != nil {
		return e, fmt.Errorf("jam kejadian harus HH:MM atau HH:MM:SS")
	}
	loc, err := timeutil.LoadZone(zone)
	if err != nil {
		return e, err
	}
	at := date.At(clock.Hour(), clock.Minute(), loc).Add(time.Duration(clock.Second()) * time.Second)
	e.At = &at
	return e, nil
}
func sopEvidenceForm(c *ui.Context, label string, d *sopEvidenceDraft, disabled bool) {
	pageSection(c, label, "Catat kejadian yang benar-benar terjadi. Kosongkan jam untuk bukti tanggal saja.", func() {
		if spaceButton(c, "Bukti sudah saya periksa", "check", d.verified).Disabled(disabled).Clicked() {
			d.verified = !d.verified
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			ui.TextInput(c, &d.date).Label("Tanggal · YYYY-MM-DD").Placeholder("Tanggal · YYYY-MM-DD").Width(210)
			ui.TextInput(c, &d.clock).Label("Jam · opsional").Placeholder("Jam · opsional").Width(180)
		})
		ui.TextInput(c, &d.url).Label("Tautan bukti · bila tersedia").Placeholder("Tautan bukti · bila tersedia").FillWidth()
		ui.TextArea(c, &d.note).Label("Keterangan bukti / kesepakatan").FillWidth().Height(80)
	})
}

type sopContextDraft struct {
	content           id.ID
	version           int64
	fields            map[string]*sopEvidenceDraft
	urgent            int
	specialRules      []string
	reason            string
	exceptionCategory string
}

func newSOPContextDraft(x api.SOPContext, zone string) *sopContextDraft {
	d := &sopContextDraft{content: x.ContentID, version: x.Version, fields: map[string]*sopEvidenceDraft{}, urgent: -1, specialRules: slices.Clone(x.SpecialAgreementRules), exceptionCategory: x.UrgentExceptionCategory}
	if x.Urgent != nil {
		d.urgent = 0
		if *x.Urgent {
			d.urgent = 1
		}
	}
	for _, f := range []struct {
		key string
		e   sop.Evidence
	}{{"mature", x.ConceptMature}, {"followup", x.Followup}, {"final", x.FinalApproved}, {"urgent", x.UrgentConfirmed}, {"exception", x.UrgentException}, {"agreement", x.SpecialAgreement}} {
		d.fields[f.key] = newSOPEvidenceDraft(f.e, zone)
	}
	return d
}
func (a *App) loadSOPContext(cid id.ID) {
	cl := a.notificationClient()
	st := &a.evaluation
	if cl == nil || st.busy || st.loading {
		return
	}
	gen, seq, pid, did, zone := a.gen, st.seq, st.period, st.division, a.period().TimeZone
	st.busy = true
	st.err = ""
	a.async(func() func() {
		value, err := cl.SOPContext(context.Background(), pid, did, cid)
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			a.evaluation.busy = false
			if err != nil {
				a.sopError(gen, seq, err)
				return
			}
			a.evaluation.contextDraft = newSOPContextDraft(value, zone)
			a.evaluation.manualDraft = nil
		}
	})
}
func (a *App) sopContextForm(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	disabled := st.busy || st.loading || ac.ReadOnly || !ac.CanReview || a.offlineMode()
	if st.manualDraft != nil {
		a.sopManualForm(c, ac)
		return
	}
	ui.Text(c, "Pilih konten untuk memeriksa konteks SOP").Bold()
	for _, content := range st.contents {
		label := content.Title + " · " + taskDate(content.PostingDate, content.PostingAt, a.period().TimeZone)
		if spaceButton(c.Key(content.ID.String()), label, "calendar", st.contextDraft != nil && st.contextDraft.content == content.ID).Disabled(st.busy || st.loading).Clicked() {
			a.loadSOPContext(content.ID)
		}
	}
	d := st.contextDraft
	if d == nil {
		return
	}
	specs := []struct{ key, label string }{}
	if ac.DivisionSlug == "humas" {
		specs = append(specs, struct{ key, label string }{"mature", "Permintaan matang"}, struct{ key, label string }{"followup", "Follow-up manusia"}, struct{ key, label string }{"urgent", "Konfirmasi dadakan"})
	} else {
		specs = append(specs, struct{ key, label string }{"final", "Persetujuan final desain"})
	}
	if ac.DivisionSlug == "humas" {
		ui.Row(c).Wrap().Gap(5).Children(func() {
			for index, label := range []string{"Belum diklasifikasikan", "Bukan dadakan", "Dadakan"} {
				value := index - 1
				if spaceButton(c.Key(label), label, "", d.urgent == value).Disabled(disabled).Clicked() {
					d.urgent = value
				}
			}
		})
		specs = append(specs, struct{ key, label string }{"exception", "Bukti pengecualian dadakan"})
		ui.Row(c).Wrap().Gap(5).Children(func() {
			for _, category := range []struct{ key, label string }{{"sempro", "Seminar proposal"}, {"semhas_magang", "Seminar hasil magang"}, {"berita_duka", "Berita duka"}} {
				if spaceButton(c.Key(category.key), category.label, "", d.exceptionCategory == category.key).Disabled(disabled).Clicked() {
					d.exceptionCategory = category.key
				}
			}
		})
	}
	specs = append(specs, struct{ key, label string }{"agreement", "Kesepakatan khusus"})
	for _, f := range specs {
		sopEvidenceForm(c.Key(f.key), f.label, d.fields[f.key], disabled)
	}
	ui.Text(c, "Aturan yang dicakup kesepakatan khusus").Bold()
	for _, choice := range sopRuleChoices {
		if choice.owner != ac.DivisionSlug {
			continue
		}
		if spaceButton(c.Key("agreement"+choice.code), choice.label, "check", slices.Contains(d.specialRules, choice.code)).Disabled(disabled).Clicked() {
			d.specialRules = toggleString(d.specialRules, choice.code)
		}
	}
	ui.TextArea(c, &d.reason).Label("Alasan pencatatan / koreksi konteks").FillWidth().Height(80)
	if spaceButton(c, "Simpan konteks", "check", true).Disabled(disabled || strings.TrimSpace(d.reason) == "").Clicked() {
		value := api.SOPContext{ContentID: d.content, DivisionID: st.division, SpecialAgreementRules: slices.Clone(d.specialRules), UrgentExceptionCategory: d.exceptionCategory}
		var err error
		values := map[string]sop.Evidence{}
		for _, field := range specs {
			values[field.key], err = d.fields[field.key].value(a.period().TimeZone)
			if err != nil {
				st.err = err.Error()
				return
			}
		}
		value.ConceptMature = values["mature"]
		value.Followup = values["followup"]
		value.FinalApproved = values["final"]
		value.UrgentConfirmed = values["urgent"]
		value.UrgentException = values["exception"]
		value.SpecialAgreement = values["agreement"]
		if ac.DivisionSlug == "humas" && d.urgent >= 0 {
			urgent := d.urgent == 1
			value.Urgent = &urgent
		}
		req := api.SOPContextRequest{Context: value, ExpectedVersion: d.version, Reason: d.reason}
		cid := d.content
		a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
			_, err := cl.SetSOPContext(context.Background(), pid, did, cid, req)
			return err
		})
	}
}

type sopManualDraft struct {
	content, person               id.ID
	rule, reason                  string
	evidence                      *sopEvidenceDraft
	policyVersion, findingVersion int64
}

func newSOPManualDraft(version int64) *sopManualDraft {
	return &sopManualDraft{policyVersion: version, evidence: newSOPEvidenceDraft(sop.Evidence{}, timeutil.DefaultZoneName)}
}
func (a *App) sopManualForm(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	d := st.manualDraft
	if d == nil {
		return
	}
	disabled := st.busy || st.loading || !ac.CanReview || ac.ReadOnly || a.offlineMode()
	pageNotice(c, "shield", "Bukti manual untuk ditinjau", "Bagian visual/isi tidak dinilai otomatis. Pencatatan ini belum mengesahkan pelanggaran.")
	for _, content := range st.contents {
		if spaceButton(c.Key(content.ID.String()), content.Title, "calendar", d.content == content.ID).Disabled(disabled).Clicked() {
			if d.content != content.ID {
				d.findingVersion = 0
			}
			d.content = content.ID
		}
	}
	if ui.TextInput(c, &d.rule).Label("Poin SOP · 7.1 sampai 7.25").Placeholder("Poin SOP · 7.1 sampai 7.25").FillWidth().Changed() {
		d.findingVersion = 0
	}
	ui.Row(c).Wrap().Gap(5).Children(func() {
		if spaceButton(c, "Tanpa individu", "users", d.person == id.Nil).Disabled(disabled).Clicked() {
			d.person = id.Nil
		}
		for _, person := range st.people {
			if spaceButton(c.Key(person.ID.String()), person.DisplayName, "", d.person == person.ID).Disabled(disabled).Clicked() {
				d.person = person.ID
			}
		}
	})
	sopEvidenceForm(c, "Bukti yang saya periksa", d.evidence, disabled)
	ui.TextArea(c, &d.reason).Label("Alasan pencatatan temuan / bukti").FillWidth().Height(90)
	if spaceButton(c, "Catat untuk tinjauan", "check", true).Disabled(disabled || d.content == id.Nil || !d.evidence.verified || strings.TrimSpace(d.reason) == "").Clicked() {
		evidence, err := d.evidence.value(a.period().TimeZone)
		if err != nil {
			st.err = err.Error()
			return
		}
		person := d.person
		req := api.SOPManualFindingRequest{ContentID: d.content, Rule: d.rule, Evidence: evidence, PersonID: &person, ExpectedPolicyVersion: d.policyVersion, ExpectedFindingVersion: d.findingVersion, Reason: d.reason}
		a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
			_, err := cl.ManualSOPFinding(context.Background(), pid, did, req)
			return err
		})
	}
	if spaceButton(c, "Tutup form manual", "close", false).Disabled(disabled).Clicked() {
		st.manualDraft = nil
	}
}
func (a *App) sopGrantsForm(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	if !ac.CanConfigure {
		return
	}
	disabled := st.busy || st.loading || ac.ReadOnly || a.offlineMode()
	muted(c, "Izin evaluasi tidak membuka Notes atau seluruh Space divisi lain. BPH dapat menerima baca; review evaluator hanya untuk Litbang.")
	for _, candidate := range st.candidates {
		candidate := candidate
		label := candidate.DisplayName + " · " + candidate.DivisionName
		if spaceButton(c.Key(candidate.ID.String()+candidate.DivisionName), label, "users", st.grantUser == candidate.ID).Disabled(disabled).Clicked() {
			st.grantUser = candidate.ID
			st.grantReview = false
		}
	}
	canReview := slices.ContainsFunc(st.candidates, func(candidate api.SOPEvaluatorCandidate) bool {
		return candidate.ID == st.grantUser && candidate.CanReview
	})
	if spaceButton(c, "Izinkan tinjauan SOP", "check", st.grantReview).Disabled(disabled || !canReview).Clicked() {
		st.grantReview = !st.grantReview
	}
	ui.TextArea(c, &st.grantReason).Label("Alasan pemberian / pencabutan izin").FillWidth().Height(80)
	if spaceButton(c, "Berikan izin evaluator", "check", true).Disabled(disabled || st.grantUser == id.Nil || strings.TrimSpace(st.grantReason) == "").Clicked() {
		req := api.SOPEvaluatorGrantRequest{UserID: st.grantUser, CanReview: st.grantReview && canReview, Reason: st.grantReason}
		a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
			return cl.GrantSOPEvaluator(context.Background(), pid, did, req)
		})
	}
	for _, grant := range st.grants {
		grant := grant
		label := "Baca"
		if grant.CanReview {
			label = "Baca dan tinjau"
		}
		ui.Text(c, grant.DisplayName+" · "+label)
		if grant.RevokedAt != nil {
			muted(c, "Dicabut: "+taskTimestamp(*grant.RevokedAt, a.period().TimeZone))
		} else if spaceButton(c.Key(grant.ID.String()), "Cabut izin", "close", false).Disabled(disabled || strings.TrimSpace(st.grantReason) == "").Clicked() {
			gid, reason := grant.ID, st.grantReason
			a.sopWrite(func(cl *client.Client, pid, did id.ID) error {
				return cl.RevokeSOPEvaluator(context.Background(), pid, did, gid, reason)
			})
		}
	}
}
