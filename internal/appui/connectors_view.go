package appui

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"strconv"
	"strings"
)

type connectorUIState struct {
	editing                                                                                               *api.Connector
	wantMirror                                                                                            bool
	mirrorProgress, mirrorEvidence, mirrorMatch, mirrorStage                                              string
	events                                                                                                []api.ConnectorEvent
	followUpdates                                                                                         bool
	seq                                                                                                   uint64
	space, division, selected, target, humas                                                              id.ID
	loaded, loading, busy, creating                                                                       bool
	sources                                                                                               []api.Connector
	items                                                                                                 []api.ConnectorItem
	runs                                                                                                  []api.ConnectorRun
	name, kind, sheet, tab, cards, interval, header, key, title, posting, pj, design, trello, reason, err string
	pending                                                                                               *struct {
		item    id.ID
		request api.ConnectorDecision
	}
}

func (a *App) ensureConnectors() {
	sp := a.selectedSpace()
	if sp == nil {
		return
	}
	if a.connectors.space != sp.ID {
		a.connectors = connectorUIState{seq: a.connectors.seq + 1, space: sp.ID, kind: "sheets", tab: "Postingan", interval: "900", header: "1", key: "0", title: "1", posting: "2", pj: "3", design: "4", trello: "0"}
	}
	var did id.ID
	for _, d := range sp.Divisions {
		if d.Slug == "humas" && a.spacePermissionsForDivision(d.ID, api.CapTasksManage) {
			did = d.ID
		}
	}
	a.connectors.division = did
	if did != id.Nil && !a.connectors.loaded && !a.connectors.loading && a.connectors.err == "" {
		a.loadConnectors()
	}
}
func (a *App) loadConnectors() {
	cl := a.notificationClient()
	sp := a.selectedSpace()
	if cl == nil || sp == nil || a.connectors.division == id.Nil || a.connectors.loading {
		return
	}
	gen, seq, pid, did, sid := a.gen, a.connectors.seq, sp.PeriodID, a.connectors.division, sp.ID
	a.connectors.loading = true
	a.async(func() func() {
		items, err := cl.Connectors(context.Background(), pid, did)
		return func() {
			if gen != a.gen || seq != a.connectors.seq || sid != a.selectedSpaceID() {
				return
			}
			a.connectors.loading = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.connectors.sources = items
			a.connectors.loaded = true
			a.connectors.err = ""
		}
	})
}
func (a *App) loadConnectorDetails(cid id.ID) {
	cl := a.notificationClient()
	if cl == nil || a.connectors.busy {
		return
	}
	a.connectors.selected = cid
	a.connectors.busy = true
	a.connectors.pending = nil
	gen, seq := a.gen, a.connectors.seq
	a.async(func() func() {
		items, err := cl.ConnectorItems(context.Background(), cid)
		runs, runErr := cl.ConnectorRuns(context.Background(), cid)
		events, eventErr := cl.ConnectorEvents(context.Background(), cid)
		if runErr == nil {
			runErr = eventErr
		}
		if err == nil {
			err = runErr
		}
		return func() {
			if gen != a.gen || seq != a.connectors.seq || cid != a.connectors.selected {
				return
			}
			a.connectors.busy = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.connectors.items = items
			a.connectors.runs = runs
			a.connectors.events = events
			a.connectors.err = ""
		}
	})
}
func (a *App) createSource() {
	cl := a.notificationClient()
	sp := a.selectedSpace()
	st := &a.connectors
	if cl == nil || sp == nil || a.offlineMode() {
		return
	}
	interval, err := strconv.Atoi(st.interval)
	if err != nil {
		st.err = "Interval diisi detik, minimal 300."
		return
	}
	req := api.ConnectorRequest{Kind: st.kind, Name: st.name, SpaceID: sp.ID, Mode: "read_only", IntervalSeconds: interval}
	if st.wantMirror {
		req.Mode = "mirror"
	}
	if st.editing != nil {
		req.Enabled = st.editing.Enabled
		req.ExpectedVersion = st.editing.Version
	}
	if st.kind == "sheets" {
		mapping := api.SheetsConfig{SpreadsheetID: st.sheet, Tab: st.tab, MirrorStage: st.mirrorStage, MirrorColumns: map[string]int{}}
		for _, f := range []struct{ value, key string }{{st.mirrorProgress, "progress"}, {st.mirrorMatch, "matchStatus"}, {st.mirrorEvidence, "evidenceUrl"}} {
			if strings.TrimSpace(f.value) == "" {
				continue
			}
			col, err := strconv.Atoi(f.value)
			if err != nil || col < 0 {
				st.err = "Kolom mirror diisi nomor kolom atau 0 jika tidak dipakai."
				return
			}
			if col > 0 {
				mapping.MirrorColumns[f.key] = col
			}
		}
		fields := []struct {
			value string
			dest  *int
		}{{st.header, &mapping.HeaderRow}, {st.key, &mapping.KeyColumn}, {st.title, &mapping.TitleColumn}, {st.posting, &mapping.PostingColumn}, {st.pj, &mapping.HumasColumn}, {st.design, &mapping.DesignColumn}, {st.trello, &mapping.TrelloColumn}}
		for _, f := range fields {
			n, err := strconv.Atoi(f.value)
			if err != nil {
				st.err = "Pemetaan kolom harus berupa nomor kolom: A=1, B=2, dan 0 untuk kolom opsional yang tidak dipakai."
				return
			}
			*f.dest = n
		}
		req.Config, _ = json.Marshal(mapping)
	} else {
		cards := strings.FieldsFunc(st.cards, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' })
		for i := range cards {
			cards[i] = strings.TrimSpace(cards[i])
		}
		req.Config, _ = json.Marshal(api.TrelloConfig{CardIDs: cards})
	}
	gen, seq, pid, did := a.gen, st.seq, sp.PeriodID, st.division
	editing := st.editing
	st.busy = true
	a.async(func() func() {
		var err error
		if editing == nil {
			_, err = cl.CreateConnector(context.Background(), pid, did, req)
		} else {
			_, err = cl.UpdateConnector(context.Background(), editing.ID, req)
		}
		return func() {
			if gen != a.gen || seq != a.connectors.seq {
				return
			}
			a.connectors.busy = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.connectors.creating = false
			a.loadConnectors()
		}
	})
}
func (a *App) connectorAction(c api.Connector, fetch bool) {
	cl := a.notificationClient()
	if cl == nil || a.connectors.busy || a.offlineMode() {
		return
	}
	gen, seq := a.gen, a.connectors.seq
	a.connectors.busy = true
	a.async(func() func() {
		var err error
		if fetch {
			_, err = cl.FetchConnector(context.Background(), c.ID)
		} else {
			_, err = cl.UpdateConnector(context.Background(), c.ID, api.ConnectorRequest{Kind: c.Kind, Name: c.Name, SpaceID: c.SpaceID, Config: c.Config, Enabled: !c.Enabled, Mode: c.Mode, IntervalSeconds: c.IntervalSeconds, ExpectedVersion: c.Version})
		}
		return func() {
			if gen != a.gen || seq != a.connectors.seq {
				return
			}
			a.connectors.busy = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.loadConnectors()
			if fetch {
				a.loadConnectorDetails(c.ID)
			}
		}
	})
}
func (a *App) prepareSourceDecision(item api.ConnectorItem, action string) {
	req := api.ConnectorDecision{FollowUpdates: a.connectors.followUpdates, Action: action, ExpectedFingerprint: item.Source.Fingerprint, Reason: a.connectors.reason}
	if action != "create" && action != "ignore" && a.connectors.target != id.Nil {
		target := a.connectors.target
		req.ContentID = &target
	}
	if a.connectors.humas != id.Nil {
		uid := a.connectors.humas
		req.HumasPJID = &uid
	}
	a.connectors.pending = &struct {
		item    id.ID
		request api.ConnectorDecision
	}{item: item.ID, request: req}
}
func (a *App) submitSourceDecision() {
	cl := a.notificationClient()
	st := &a.connectors
	if cl == nil || st.pending == nil || st.busy || a.offlineMode() {
		return
	}
	pending := *st.pending
	cid, gen, seq := st.selected, a.gen, st.seq
	st.busy = true
	a.async(func() func() {
		_, err := cl.DecideConnectorItem(context.Background(), cid, pending.item, pending.request)
		return func() {
			if gen != a.gen || seq != a.connectors.seq {
				return
			}
			a.connectors.busy = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.connectors.pending = nil
			a.loadConnectorDetails(cid)
			a.loadMedhum()
		}
	})
}
func (a *App) connectorsView(c *ui.Context) {
	a.ensureMedhum()
	a.ensureConnectors()
	pageBody(c, "Sumber kalender", func() {
		st := &a.connectors
		muted(c, "Fetch membaca sumber ke pratinjau. Menautkan/menerapkan menentukan identitas konten tanpa menimpa status, bukti atau approval. Kolom PJ desain lama tetap informasi sampai ditetapkan Mediatek.")
		if st.division == id.Nil {
			pageNotice(c, "lock", "Dikelola Humas", "Pengaturan sumber kalender mengikuti kewenangan Humas pada periode aktif.")
			return
		}
		if st.err != "" {
			ui.Text(c, st.err).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if spaceButton(c, "Muat ulang sumber", "refresh", false).Disabled(st.busy || st.loading).Clicked() {
				a.loadConnectors()
			}
			if spaceButton(c, "Tambah sumber", "plus", true).Disabled(st.busy || a.offlineMode()).Clicked() {
				a.beginConnectorForm(nil)
			}
		})
		if st.creating {
			a.connectorCreateForm(c)
		}
		for _, source := range st.sources {
			source := source
			ui.Column(c.Key(source.ID)).FillWidth().Padding(14).Gap(7).Border(1, c.Theme().Border).Radius(8).Children(func() {
				ui.Text(c, source.Name).Bold()
				if spaceButton(c, "Ubah pemetaan / mode", "settings", false).Disabled(st.busy || a.offlineMode()).Clicked() {
					a.beginConnectorForm(&source)
				}
				if source.Mode == "mirror" && spaceButton(c, "Mirror kolom Space sekarang", "refresh", false).Disabled(st.busy || a.offlineMode()).Clicked() {
					a.mirrorSource(source.ID)
				}
				status := "Polling nonaktif"
				if source.Enabled {
					status = "Polling aktif"
				}
				muted(c, status+" · "+strconv.Itoa(source.IntervalSeconds)+" detik · "+source.Kind)
				if source.LastSuccessAt != nil {
					muted(c, "Fetch terakhir: "+taskTimestamp(*source.LastSuccessAt, a.period().TimeZone))
				}
				if source.LastError != "" {
					ui.Text(c, source.LastError).TextColor(c.Theme().Danger)
				}
				ui.Row(c).Wrap().Gap(6).Children(func() {
					if spaceButton(c, "Baca sumber sekarang", "refresh", false).Disabled(st.busy || a.offlineMode()).Clicked() {
						a.connectorAction(source, true)
					}
					toggle := "Aktifkan polling"
					if source.Enabled {
						toggle = "Nonaktifkan polling"
					}
					if spaceButton(c, toggle, "clock", source.Enabled).Disabled(st.busy || a.offlineMode()).Clicked() {
						a.connectorAction(source, false)
					}
					if spaceButton(c, "Pratinjau / riwayat", "note", st.selected == source.ID).Disabled(st.busy).Clicked() {
						a.loadConnectorDetails(source.ID)
					}
				})
			})
		}
		if st.selected != id.Nil {
			a.connectorPreview(c)
		}
	})
}
func (a *App) connectorCreateForm(c *ui.Context) {
	st := &a.connectors
	ui.Column(c).FillWidth().Gap(9).Padding(14).Border(1, c.Theme().Border).Radius(8).Children(func() {
		ui.TextInput(c, &st.name).Label("Nama sumber").FillWidth()
		ui.Row(c).Gap(6).Children(func() {
			for _, kind := range []string{"sheets", "trello"} {
				kind := kind
				if spaceButton(c, map[string]string{"sheets": "Google Sheets", "trello": "Trello"}[kind], "link", st.kind == kind).Clicked() {
					st.kind = kind
				}
			}
		})
		ui.TextInput(c, &st.interval).Label("Interval polling · detik (minimal 300)").FillWidth()
		if st.kind == "sheets" {
			for _, f := range []struct {
				label string
				v     *string
			}{{"Link spreadsheet", &st.sheet}, {"Nama tab", &st.tab}, {"Baris header", &st.header}, {"Kolom ID stabil · opsional", &st.key}, {"Kolom judul", &st.title}, {"Kolom tanggal posting", &st.posting}, {"Kolom PJ Humas", &st.pj}, {"Kolom PJ Mediatek lama · informasi", &st.design}, {"Kolom kartu Trello · opsional", &st.trello}} {
				ui.TextInput(c.Key(f.label), f.v).Label(f.label).FillWidth()
			}
			muted(c, "Nomor kolom: A=1, B=2, C=3. Gunakan 0 untuk kolom opsional yang tidak dipakai. Kredensial dikonfigurasi pada server, tidak dimasukkan ke formulir ini.")
		} else {
			ui.TextArea(c, &st.cards).Label("ID/link kartu Trello · pisahkan baris atau koma").FillWidth().Height(100)
		}
		if st.kind == "sheets" {
			for _, f := range []struct {
				label string
				value *string
			}{{"Kolom space_progress · opsional", &st.mirrorProgress}, {"Kolom space_match_status · opsional", &st.mirrorMatch}, {"Kolom space_evidence_url · opsional", &st.mirrorEvidence}} {
				ui.TextInput(c.Key(f.label), f.value).Label(f.label).FillWidth()
			}
			ui.Row(c).Gap(6).Children(func() {
				for _, stage := range []string{"design", "humas"} {
					stage := stage
					if spaceButton(c, map[string]string{"design": "Tahap desain", "humas": "Tahap Humas"}[stage], "", st.mirrorStage == stage).Clicked() {
						st.mirrorStage = stage
					}
				}
			})
			label := "Mode baca sumber"
			if st.wantMirror {
				label = "Mode mirror kolom Space"
			}
			if spaceButton(c, label, "link", st.wantMirror).Clicked() {
				st.wantMirror = !st.wantMirror
			}
			muted(c, "Mirror hanya ke kolom berheader khusus Space, tanpa formula, setelah penulisan eksternal diaktifkan pada server. Kolom manual/approval tidak diambil alih.")
		}
		if spaceButton(c, "Simpan pengaturan sumber", "check", true).Disabled(st.busy || a.offlineMode()).Clicked() {
			a.createSource()
		}
	})
}
func (a *App) connectorPreview(c *ui.Context) {
	st := &a.connectors
	ui.Text(c, "Pratinjau sumber").FontSize(19).Bold()
	ui.Text(c, "Pilih konten tujuan untuk menautkan atau memperbarui").Bold()
	for _, content := range a.medhum.items {
		content := content
		if spaceButton(c, content.Title, "calendar", st.target == content.ID).Clicked() {
			st.target = content.ID
		}
	}
	ui.Text(c, "Pemetaan PJ Humas yang diterapkan").Bold()
	for _, person := range a.medhum.humans {
		person := person
		if spaceButton(c, person.DisplayName, "user", st.humas == person.UserID).Clicked() {
			st.humas = person.UserID
		}
	}
	ui.TextInput(c, &st.reason).Label("Alasan keputusan / perubahan").FillWidth()
	label := "Perubahan berikutnya ditinjau manual"
	if st.followUpdates {
		label = "Ikuti perubahan pada bidang milik sumber"
	}
	if spaceButton(c, label, "refresh", st.followUpdates).Clicked() {
		st.followUpdates = !st.followUpdates
	}
	for _, item := range st.items {
		item := item
		ui.Column(c.Key(item.ID)).FillWidth().Padding(12).Gap(7).Border(1, c.Theme().Border).Radius(8).Children(func() {
			ui.Text(c, item.Source.Title).Bold()
			if item.Source.PostingDate != nil {
				muted(c, "Posting: "+item.Source.PostingDate.String())
			}
			muted(c, "PJ sumber: "+item.Source.HumasName+" · desain lama: "+item.Source.DesignName)
			muted(c, "Status pratinjau: "+item.State)
			if item.Issue != "" {
				ui.Text(c, item.Issue).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Wrap().Gap(5).Children(func() {
				for _, choice := range []struct{ action, label string }{{"create", "Buat konten baru"}, {"link", "Tautkan ke pilihan"}, {"apply", "Terapkan ke pilihan"}, {"ignore", "Abaikan item"}} {
					choice := choice
					disabled := st.busy || a.offlineMode()
					if choice.action != "ignore" {
						disabled = disabled || item.Source.Issue != "" || item.State == "missing"
					}
					if choice.action == "link" || choice.action == "apply" {
						disabled = disabled || st.target == id.Nil
					}
					if spaceButton(c, choice.label, "", false).Disabled(disabled).Clicked() {
						a.prepareSourceDecision(item, choice.action)
					}
				}
			})
			if st.pending != nil && st.pending.item == item.ID {
				muted(c, "Konfirmasi keputusan ini terhadap snapshot yang sedang ditampilkan. Status/bukti tugas yang sudah ada dipertahankan.")
				if spaceButton(c, "Konfirmasi keputusan", "check", true).Disabled(st.busy || a.offlineMode()).Clicked() {
					a.submitSourceDecision()
				}
			}
		})
	}
	if len(st.events) > 0 {
		ui.Text(c, "Riwayat perubahan sumber").Bold()
		for _, event := range st.events {
			muted(c, taskTimestamp(event.OccurredAt, a.period().TimeZone)+" · "+event.Action)
		}
	}
	if len(st.runs) > 0 {
		ui.Text(c, "Riwayat pembacaan sumber").Bold()
		for _, run := range st.runs {
			muted(c, taskTimestamp(run.StartedAt, a.period().TimeZone)+" · "+run.Status+" · "+strconv.Itoa(run.ItemCount)+" item")
			if run.Error != "" {
				muted(c, run.Error)
			}
		}
	}
}

func (a *App) beginConnectorForm(source *api.Connector) {
	st := &a.connectors
	st.creating = true
	st.editing = source
	st.wantMirror = false
	st.mirrorProgress = "0"
	st.mirrorEvidence = "0"
	st.mirrorMatch = "0"
	st.mirrorStage = "design"
	if source == nil {
		st.name = ""
		st.kind = "sheets"
		st.sheet = ""
		st.tab = "Postingan"
		st.cards = ""
		st.interval = "900"
		st.header = "1"
		st.key = "0"
		st.title = "1"
		st.posting = "2"
		st.pj = "3"
		st.design = "4"
		st.trello = "0"
		return
	}
	st.name = source.Name
	st.kind = source.Kind
	st.interval = strconv.Itoa(source.IntervalSeconds)
	st.wantMirror = source.Mode == "mirror"
	if source.Kind == "sheets" {
		var cfg api.SheetsConfig
		if json.Unmarshal(source.Config, &cfg) != nil {
			return
		}
		st.sheet = cfg.SpreadsheetID
		st.tab = cfg.Tab
		st.header = strconv.Itoa(cfg.HeaderRow)
		st.key = strconv.Itoa(cfg.KeyColumn)
		st.title = strconv.Itoa(cfg.TitleColumn)
		st.posting = strconv.Itoa(cfg.PostingColumn)
		st.pj = strconv.Itoa(cfg.HumasColumn)
		st.design = strconv.Itoa(cfg.DesignColumn)
		st.trello = strconv.Itoa(cfg.TrelloColumn)
		st.mirrorStage = cfg.MirrorStage
		if st.mirrorStage == "" {
			st.mirrorStage = "design"
		}
		st.mirrorProgress = strconv.Itoa(cfg.MirrorColumns["progress"])
		st.mirrorMatch = strconv.Itoa(cfg.MirrorColumns["matchStatus"])
		st.mirrorEvidence = strconv.Itoa(cfg.MirrorColumns["evidenceUrl"])
	} else {
		var cfg api.TrelloConfig
		_ = json.Unmarshal(source.Config, &cfg)
		st.cards = strings.Join(cfg.CardIDs, "\n")
	}
}
func (a *App) mirrorSource(cid id.ID) {
	cl := a.notificationClient()
	if cl == nil || a.connectors.busy {
		return
	}
	gen, seq := a.gen, a.connectors.seq
	a.connectors.busy = true
	a.async(func() func() {
		err := cl.MirrorConnector(context.Background(), cid)
		return func() {
			if gen != a.gen || seq != a.connectors.seq {
				return
			}
			a.connectors.busy = false
			if err != nil {
				a.connectors.err = a.failed(err)
				return
			}
			a.loadConnectorDetails(cid)
		}
	})
}
