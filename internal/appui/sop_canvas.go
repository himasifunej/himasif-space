package appui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Canvas A's Evaluasi SOP (Tahap 7a): segments Temuan · Laporan kuartal ·
// Kebijakan, a table of findings and a chart per rule.

var sopSegments = []string{"Temuan", "Laporan kuartal", "Kebijakan"}

// sopPolicyTabs are what "Kebijakan" holds: the old tabs, as sub-tabs.
var sopPolicyTabs = []string{"Aturan", "Konteks", "Pengiriman", "Izin"}

// sopSegment is the segment an old tab value belongs to.
func sopSegment(tab string) string {
	switch tab {
	case "Laporan":
		return "Laporan kuartal"
	case "Aturan", "Konteks", "Pengiriman", "Izin":
		return "Kebijakan"
	}
	return "Temuan"
}

// sopRuleLabel names a rule as canvas A does ("Desain H-2").
func sopRuleLabel(rule string) string {
	switch rule {
	case sop.ConceptH7:
		return "Konsep H-7"
	case sop.DesignH2:
		return "Desain H-2"
	case sop.Urgent24:
		return "Dadakan 24 jam"
	case sop.FollowupH4:
		return "Follow-up H-4"
	case sop.FinalH12:
		return "Posting H-12"
	case sop.UrgentLimit:
		return "Batas dadakan"
	}
	return "Poin " + rule
}

// sopRuleGroup is the chart's bar a rule adds to.
func sopRuleGroup(rule string) string {
	switch rule {
	case sop.ConceptH7:
		return "Konsep"
	case sop.DesignH2:
		return "Desain"
	case sop.FollowupH4:
		return "Follow-up"
	case sop.FinalH12:
		return "Posting"
	case sop.Urgent24, sop.UrgentLimit:
		return "Dadakan"
	}
	return "Lainnya"
}

var sopGroupOrder = []string{"Konsep", "Desain", "Follow-up", "Posting", "Dadakan", "Lainnya"}

// sopStateColor colours a finding's state as the canvas' dot.
func sopStateColor(t *ui.Theme, state string) ui.Color {
	switch state {
	case "confirmed":
		return t.Danger
	case "dismissed", "corrected":
		return t.Success
	}
	return t.Warning
}

// sopStateCanvasLabel is the state in canvas A's words.
func sopStateCanvasLabel(state string) string {
	switch state {
	case "confirmed":
		return "Pelanggaran disahkan"
	case "dismissed":
		return "Bukan pelanggaran"
	case "corrected":
		return "Diralat"
	}
	return "Perlu tinjauan"
}

// sopBar is one bar of the chart: Sesuai (on time, or judged not a
// violation), Perlu tinjauan, Pelanggaran disahkan.
type sopBar struct {
	group                  string
	ok, review, violations int
}

func sopBars(s *api.SOPSummary) []sopBar {
	by := map[string]*sopBar{}
	for _, r := range s.Rules {
		g := sopRuleGroup(r.Rule)
		b := by[g]
		if b == nil {
			b = &sopBar{group: g}
			by[g] = b
		}
		b.ok += r.Compliant + r.Dismissed + r.Corrected
		b.review += r.NeedsReview
		b.violations += r.Confirmed
	}
	var out []sopBar
	for _, g := range sopGroupOrder {
		if b := by[g]; b != nil {
			out = append(out, *b)
		}
	}
	return out
}

// sopCurrentQuarter is the policy's quarter holding today, else its first.
func sopCurrentQuarter(p api.SOPPolicy, today timeutil.Date) string {
	for _, q := range p.Settings.Quarters {
		if !today.Before(q.StartsOn) && today.Compare(q.EndsOn) <= 0 {
			return q.Label
		}
	}
	if len(p.Settings.Quarters) > 0 {
		return p.Settings.Quarters[0].Label
	}
	return ""
}

// ensureSOPSummary loads the chart of the chosen quarter once per scope.
func (a *App) ensureSOPSummary() {
	st := &a.evaluation
	cl := a.notificationClient()
	if cl == nil || st.policy.Version == 0 || st.loading || st.summaryLoading || a.offlineMode() {
		return
	}
	if st.summaryQuarter == "" {
		st.summaryQuarter = sopCurrentQuarter(st.policy, timeutil.DateOf(time.Now()))
	}
	if st.summaryQuarter == "" {
		return
	}
	key := st.division.String() + "/" + st.summaryQuarter + fmt.Sprint(st.policy.Version)
	if st.summaryKey == key {
		return
	}
	st.summaryKey = key
	st.summaryLoading, st.summaryErr = true, ""
	gen, seq, pid, did, quarter := a.gen, st.seq, st.period, st.division, st.summaryQuarter
	a.async(func() func() {
		sum, err := cl.SOPSummary(context.Background(), pid, did, url.Values{"quarter": {quarter}})
		return func() {
			if !a.sopCurrent(gen, seq) {
				return
			}
			st := &a.evaluation
			st.summaryLoading = false
			var ce *client.Error
			switch {
			case errors.As(err, &ce) && (ce.Status == http.StatusNotFound || ce.Status == http.StatusMethodNotAllowed):
				st.summary, st.summaryErr = nil, "Ringkasan per aturan belum tersedia di server ini."
			case err != nil:
				st.summary, st.summaryErr = nil, a.failed(err)
			default:
				st.summary = &sum
			}
		}
	})
}

// sopHead is the canvas' head: "Evaluasi SOP <scope>", its line and the
// segments on the right.
func (a *App) sopHead(c *ui.Context, ac *api.SOPAccess) {
	st := &a.evaluation
	title := "Evaluasi SOP"
	if ac != nil && ac.DivisionName != "" {
		title += " " + ac.DivisionName
	}
	head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
	wide := head.Bounds().W >= 760
	if wide {
		head.Row().AlignItems(ui.End)
	}
	head.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(20).Bold()
			muted(c, "Temuan awal dari waktu konsep, follow-up, penyerahan dan tenggat. Semua mulai berstatus perlu tinjauan; tidak diumumkan ke grup Medhum.").FontSize(13)
		})
		if ac == nil {
			return
		}
		current := 0
		for i, s := range sopSegments {
			if s == sopSegment(st.tab) {
				current = i
			}
		}
		if chosen := agendaSegmented(c, current, sopSegments...); chosen != current {
			switch sopSegments[chosen] {
			case "Temuan":
				st.tab = "Temuan"
			case "Laporan kuartal":
				st.tab = "Laporan"
			default:
				st.tab = "Aturan"
			}
		}
	})
}

// sopPolicyTabsRow is Kebijakan's sub-tabs (Izin only for who configures).
func (a *App) sopPolicyTabsRow(c *ui.Context, ac api.SOPAccess) {
	st := &a.evaluation
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, tab := range sopPolicyTabs {
			if tab == "Izin" && !ac.CanConfigure {
				continue
			}
			if spaceButton(c.Key(tab), tab, "", st.tab == tab).Clicked() {
				st.tab = tab
			}
		}
	})
}

// sopFindingsTable is the canvas' table: TEMUAN · ATURAN · PIHAK · STATUS,
// cards on a phone; a row opens the finding's detail.
func (a *App) sopFindingsTable(c *ui.Context, disabled bool) {
	st := &a.evaluation
	t := c.Theme()
	division := ""
	if ac := a.sopAccess(); ac != nil {
		division = ac.DivisionName
	}
	panel := ui.Column(c.Key("sop-table")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Clip()
	table := !isMobile(c) && panel.Bounds().W >= 640
	panel.Children(func() {
		if table {
			ui.Row(c).FillWidth().Padding(12, 16).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "TEMUAN").FontSize(11).TextColor(faint(t)).Grow(1).MinWidth(0)
				for _, col := range []struct {
					label string
					w     float32
				}{{"ATURAN", 140}, {"PIHAK", 170}, {"STATUS", 170}} {
					ui.Text(c.Key(col.label), col.label).FontSize(11).TextColor(faint(t)).Width(col.w).Shrink(0)
				}
			})
		}
		if len(st.items) == 0 {
			ui.Column(c).FillWidth().Padding(20, 16).Children(func() {
				msg := "Tidak ada temuan pada filter ini."
				if st.loading {
					msg = "Memuat temuan…"
				}
				muted(c, msg)
			})
		}
		for i, f := range st.items {
			f := f
			row := ui.ButtonBase(c.Key(f.ID.String())).Label("Temuan "+f.ContentTitle+" "+sopRuleLabel(f.Rule)).FillWidth().MinWidth(0).Padding(12, 16).Gap(12)
			if table {
				row.Row().AlignItems(ui.Center)
			} else {
				row.Column().AlignItems(ui.Start)
			}
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			}
			if st.selected != nil && st.selected.ID == f.ID {
				row.Background(t.SurfaceHover)
			} else if row.Hovered() {
				row.Background(spaceMix(t.Surface, t.Text, .04))
			}
			party := strings.TrimSpace(strings.Trim(division+" · "+f.PersonName, " ·"))
			row.Disabled(disabled).FocusRing(row.FocusVisible()).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, f.ContentTitle).FontSize(13).FontWeight(600).MaxLines(2)
					note := f.EventOn.String()
					if m := sopMeasurementLabel(f.Latest.Observation.Measurement); m != "" {
						note += " · " + m
					}
					if f.NeedsRecheck {
						note += " · sumber berubah"
					}
					muted(c, note).FontSize(11).MaxLines(2)
				})
				cell := func(key, text string, w float32) {
					txt := ui.Text(c.Key(key), text).FontSize(13).MaxLines(2)
					if table {
						txt.Width(w).Shrink(0)
					}
				}
				cell("rule", sopRuleLabel(f.Rule), 140)
				cell("party", party, 170)
				status := ui.Row(c.Key("status")).Gap(8).AlignItems(ui.Center)
				if table {
					status.Width(170).Shrink(0)
				}
				status.Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(sopStateColor(t, f.State)).Shrink(0)
					ui.Text(c, sopStateCanvasLabel(f.State)).FontSize(12).FontWeight(600).MaxLines(1)
				})
			})
			if row.Clicked() {
				a.selectSOPFinding(f.ID)
			}
		}
	})
}

// sopChartCard is the canvas' right card: "<quarter> · per aturan", stacked
// bars, the legend and Ekspor CSV.
func (a *App) sopChartCard(c *ui.Context, disabled bool) {
	st := &a.evaluation
	t := c.Theme()
	a.ensureSOPSummary()
	ui.Column(c.Key("sop-chart")).FillWidth().MinWidth(0).Padding(16).Gap(14).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		title := "Ringkasan per aturan"
		if st.summaryQuarter != "" {
			title = st.summaryQuarter + " · per aturan"
		}
		ui.Text(c, title).FontSize(15).Bold()
		if len(st.policy.Settings.Quarters) > 1 {
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, q := range st.policy.Settings.Quarters {
					if spaceButton(c.Key("q"+q.Label), q.Label, "", st.summaryQuarter == q.Label).Padding(4, 10).MinHeight(28).Disabled(st.summaryLoading).Clicked() {
						st.summaryQuarter = q.Label
					}
				}
			})
		}
		switch {
		case st.policy.Version == 0:
			muted(c, "Grafik tampil setelah kadiv menyimpan kebijakan.")
			return
		case len(st.policy.Settings.Quarters) == 0:
			muted(c, "Tetapkan kuartal di Kebijakan → Aturan agar grafik per kuartal tampil.")
			return
		case st.summaryErr != "":
			muted(c, st.summaryErr)
		case st.summary == nil:
			muted(c, "Menghitung…")
		}
		if st.summary != nil {
			bars := sopBars(st.summary)
			maxTotal := 1
			for _, b := range bars {
				maxTotal = max(maxTotal, b.ok+b.review+b.violations)
			}
			const height = float32(120)
			ui.Row(c.Key("bars")).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
				for _, b := range bars {
					b := b
					total := b.ok + b.review + b.violations
					ui.Column(c.Key(b.group)).Grow(1).Basis(0).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprint(total)).FontSize(11).TextColor(t.TextMuted)
						col := ui.Column(c).FillWidth().Height(height).Radius(6).Background(t.Background).Clip().Justify(ui.End)
						col.Label(fmt.Sprintf("%s: %d sesuai, %d perlu tinjauan, %d pelanggaran", b.group, b.ok, b.review, b.violations))
						col.Children(func() {
							for _, seg := range []struct {
								n     int
								color ui.Color
							}{{b.violations, t.Danger}, {b.review, t.Warning}, {b.ok, t.Success}} {
								if seg.n > 0 {
									ui.Box(c).FillWidth().Height(height * float32(seg.n) / float32(maxTotal)).Background(seg.color)
								}
							}
						})
						ui.Text(c, b.group).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
					})
				}
			})
			ui.Row(c).Wrap().Gap(12).Children(func() {
				for _, l := range []struct {
					label string
					color ui.Color
				}{{"Sesuai", t.Success}, {"Perlu tinjauan", t.Warning}, {"Pelanggaran disahkan", t.Danger}} {
					ui.Row(c.Key(l.label)).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(2).Background(l.color)
						ui.Text(c, l.label).FontSize(11).TextColor(t.TextMuted)
					})
				}
			})
			muted(c, fmt.Sprintf("%d konten Medhum dalam rentang. Sesuai = tepat waktu, atau temuan yang dinyatakan bukan pelanggaran.", st.summary.Contents)).FontSize(11)
		}
		if spaceButton(c.Key("sop-export"), "Ekspor CSV", "archive", false).Disabled(disabled || st.summary == nil || a.ExportFile == nil).Clicked() {
			a.exportSOPQuarter(*st.summary)
		}
		if st.exportStatus != "" {
			muted(c, st.exportStatus).FontSize(11)
		}
	})
}

// exportSOPQuarter saves the quarter's findings as CSV in one step.
func (a *App) exportSOPQuarter(s api.SOPSummary) {
	st := &a.evaluation
	cl := a.notificationClient()
	if cl == nil || a.ExportFile == nil {
		return
	}
	gen, seq, pid, did, save := a.gen, st.seq, st.period, st.division, a.ExportFile
	name := fmt.Sprintf("space-sop-%s-%s.csv", strings.ReplaceAll(s.Label, " ", "-"), s.StartsOn.String())
	st.busy, st.exportStatus = true, ""
	a.async(func() func() {
		data, err := cl.SOPReportCSV(context.Background(), pid, did, url.Values{"from": {s.StartsOn.String()}, "to": {s.EndsOn.String()}})
		path := ""
		if err == nil {
			path, err = save(name, "text/csv", data)
		}
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
			if path != "" {
				st.exportStatus = "CSV disimpan: " + path
			}
		}
	})
}
