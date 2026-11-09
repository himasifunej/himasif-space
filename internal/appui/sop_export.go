package appui

import (
	"context"
	"fmt"
	"net/url"

	"github.com/egoist/mygo/ui"
)

func (a *App) sopExportView(c *ui.Context, disabled bool) {
	st := &a.evaluation
	if st.report == nil {
		return
	}
	if st.exportStatus != "" {
		muted(c, st.exportStatus)
	}
	cl := a.notificationClient()
	if len(st.exportData) == 0 {
		if spaceButton(c, "Siapkan CSV rentang laporan ini", "archive", false).Disabled(disabled || cl == nil).Clicked() {
			report := *st.report
			query := url.Values{"from": {report.StartsOn.String()}, "to": {report.EndsOn.String()}}
			gen, seq, pid, did := a.gen, st.seq, st.period, st.division
			st.busy = true
			st.err = ""
			a.async(func() func() {
				data, err := cl.SOPReportCSV(context.Background(), pid, did, query)
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
					st.exportData = data
					st.exportName = fmt.Sprintf("space-sop-%s-%s-%s.csv", did.String(), report.StartsOn.String(), report.EndsOn.String())
					st.exportStatus = "CSV siap disimpan. Ekspor membaca keadaan terbaru pada rentang ini; data dapat berbeda dari ringkasan yang dibuat sebelumnya."
				}
			})
		}
		return
	}
	if a.ExportFile == nil {
		muted(c, "Penyimpanan berkas belum tersedia pada klien ini.")
		return
	}
	if spaceButton(c, "Simpan CSV", "archive", false).Disabled(disabled).Clicked() {
		gen, seq := a.gen, st.seq
		data := append([]byte(nil), st.exportData...)
		name := st.exportName
		save := a.ExportFile
		st.busy = true
		st.err = ""
		a.async(func() func() {
			path, err := save(name, "text/csv;charset=utf-8", data)
			return func() {
				if !a.sopCurrent(gen, seq) {
					return
				}
				st := &a.evaluation
				st.busy = false
				if err != nil {
					st.err = err.Error()
					return
				}
				if path == "" {
					st.exportStatus = "Penyimpanan dibatalkan; CSV masih tersedia untuk disimpan."
					return
				}
				st.exportData = nil
				st.exportStatus = path
			}
		})
	}
}
