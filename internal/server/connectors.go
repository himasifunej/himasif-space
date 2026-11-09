package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"strings"
	"time"
)

func decodeConnectorSettings(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return badRequest("Pengaturan konektor tidak valid atau memuat bidang yang tidak didukung.")
	}
	return nil
}
func validateConnector(req *api.ConnectorRequest) error {
	var err error
	req.Name, err = text(req.Name, "nama konektor", 1, 120)
	if err != nil {
		return err
	}
	if req.Mode == "" {
		req.Mode = "read_only"
	}
	if req.Mode != "read_only" && req.Mode != "mirror" {
		return badRequest("Mode konektor tidak valid.")
	}
	if req.IntervalSeconds == 0 {
		req.IntervalSeconds = 900
	}
	if req.IntervalSeconds < 300 || req.IntervalSeconds > 86400 {
		return badRequest("Interval fetch harus 5 menit–24 jam.")
	}
	switch req.Kind {
	case "sheets":
		var settings api.SheetsConfig
		if err := decodeConnectorSettings(req.Config, &settings); err != nil {
			return err
		}
		if err := integrations.ValidateSheets(&settings); err != nil {
			return badRequest(err.Error())
		}
		if req.Mode == "mirror" && (settings.KeyColumn == 0 || len(settings.MirrorColumns) == 0) {
			return badRequest("Mirror membutuhkan ID stabil dan pemetaan kolom khusus Space.")
		}
		req.Config, _ = json.Marshal(settings)
	case "trello":
		var settings api.TrelloConfig
		if err := decodeConnectorSettings(req.Config, &settings); err != nil {
			return err
		}
		if len(settings.CardIDs) == 0 || len(settings.CardIDs) > 100 {
			return badRequest("Pilih 1–100 kartu Trello.")
		}
		for _, card := range settings.CardIDs {
			if len(card) > 200 || strings.ContainsAny(card, "\r\n\x00") {
				return badRequest("Identitas kartu Trello tidak valid.")
			}
		}
		if req.Mode != "read_only" {
			return badRequest("Trello hanya dibaca pada transisi ini.")
		}
	default:
		return badRequest("Pilih konektor Sheets atau Trello.")
	}
	return nil
}
func (s *Server) withConnector(r *http.Request, p *principal, write bool, fn func(store.Querier, *scope, *api.Connector) error) error {
	return s.withConnectorAccounts(r, p, write, nil, fn)
}
func (s *Server) withConnectorAccounts(r *http.Request, p *principal, write bool, extra []id.ID, fn func(store.Querier, *scope, *api.Connector) error) error {
	cid, err := pathID(r, "connectorId")
	if err != nil {
		return err
	}
	peek, err := s.store.Connector(r.Context(), s.store.DB(), cid, store.LockNone)
	if err != nil {
		return errNotFound
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		rr := r.Clone(r.Context())
		rr.SetPathValue("periodId", peek.PeriodID.String())
		rr.SetPathValue("divisionId", peek.DivisionID.String())
		lock, rowLock := store.LockShare, store.LockNone
		if write {
			lock, rowLock = store.LockUpdate, store.LockUpdate
		}
		sc, err := s.loadScope(r.Context(), q, rr, p, lock)
		if err != nil {
			return err
		}
		cap := api.CapSpaceRead
		if write {
			cap = api.CapTasksManage
		}
		if err := sc.require(cap); err != nil {
			return err
		}
		if write {
			if p.admin() {
				return errForbidden
			}
			if err := s.lockTaskAccounts(r.Context(), q, append(extra, p.User.ID)); err != nil {
				return err
			}
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		c, err := s.store.Connector(r.Context(), q, cid, rowLock)
		if err != nil {
			return err
		}
		return fn(q, sc, &c)
	})
}
func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.Connector{}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		if err := sc.require(api.CapSpaceRead); err != nil {
			return err
		}
		out, err = s.store.Connectors(r.Context(), q, sc.period.ID, sc.division.ID)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.Connector]{Items: out})
	return nil
}
func (s *Server) createConnector(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.ConnectorRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateConnector(&req); err != nil {
		return err
	}
	var out api.Connector
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockUpdate)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksManage); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		sp, humas, _, err := s.medhumDivisions(r, q, req.SpaceID)
		if err != nil {
			return err
		}
		if sp.PeriodID != sc.period.ID || humas != sc.division.ID {
			return errForbidden
		}
		now := s.now().UTC()
		out = api.Connector{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, SpaceID: req.SpaceID, Kind: req.Kind, Name: req.Name, Config: req.Config, Enabled: req.Enabled, Mode: req.Mode, IntervalSeconds: req.IntervalSeconds, NextRunAt: now, Version: 1, CreatedBy: p.User.ID, UpdatedAt: now}
		if err := s.store.SaveConnector(r.Context(), q, out, true); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "connector.create", TargetType: "connector", TargetID: &out.ID, PeriodID: &out.PeriodID, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func (s *Server) updateConnector(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.ConnectorRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateConnector(&req); err != nil {
		return err
	}
	var out api.Connector
	err := s.withConnector(r, p, true, func(q store.Querier, sc *scope, c *api.Connector) error {
		if req.ExpectedVersion != c.Version {
			return conflict("Pengaturan konektor berubah. Muat ulang sebelum menyimpan.")
		}
		if req.Kind != c.Kind || req.SpaceID != c.SpaceID {
			return badRequest("Jenis dan ruang konektor tetap. Buat konektor terpisah untuk sumber lain.")
		}
		before := *c
		c.Name = req.Name
		c.Config = req.Config
		c.Enabled = req.Enabled
		c.Mode = req.Mode
		c.IntervalSeconds = req.IntervalSeconds
		c.NextRunAt = s.now()
		c.Version++
		c.UpdatedAt = s.now().UTC()
		if err := s.store.SaveConnector(r.Context(), q, *c, false); err != nil {
			return err
		}
		out = *c
		return s.record(r, q, p, store.AuditEvent{Action: "connector.update", TargetType: "connector", TargetID: &c.ID, PeriodID: &c.PeriodID, Before: before, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) connectorItems(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out []api.ConnectorItem
	err := s.withConnector(r, p, false, func(q store.Querier, sc *scope, c *api.Connector) error {
		var err error
		out, err = s.store.ConnectorItems(r.Context(), q, c.ID)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.ConnectorItem]{Items: out})
	return nil
}
func (s *Server) connectorRuns(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out []api.ConnectorRun
	err := s.withConnector(r, p, false, func(q store.Querier, sc *scope, c *api.Connector) error {
		var err error
		out, err = s.store.ConnectorRuns(r.Context(), q, c.ID)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.ConnectorRun]{Items: out})
	return nil
}
func (s *Server) fetchConnector(w http.ResponseWriter, r *http.Request, p *principal) error {
	var c api.Connector
	run := api.ConnectorRun{ID: id.New(), Status: "running", StartedAt: s.now().UTC()}
	err := s.withConnector(r, p, true, func(q store.Querier, sc *scope, value *api.Connector) error {
		c = *value
		run.ConnectorID = c.ID
		var active bool
		if err := q.QueryRow(r.Context(), `select exists(select 1 from connector_runs where connector_id=$1 and status='running')`, c.ID).Scan(&active); err != nil {
			return err
		}
		if active {
			return conflict("Fetch konektor ini masih berjalan.")
		}
		_, err := q.Exec(r.Context(), `insert into connector_runs(id,connector_id,started_at,status) values($1,$2,$3,'running')`, run.ID, c.ID, run.StartedAt)
		return err
	})
	if err != nil {
		return err
	}
	fetchContext, cancelFetch := context.WithTimeout(r.Context(), 45*time.Second)
	items, fetchErr := s.readConnector(fetchContext, c)
	cancelFetch()
	err = s.withConnector(r, p, true, func(q store.Querier, sc *scope, current *api.Connector) error {
		if current.Version != c.Version {
			fetchErr = errors.New("konfigurasi berubah selama fetch; snapshot tidak diterapkan")
		}
		return s.finishConnectorFetch(r.Context(), q, *current, &run, items, fetchErr)
	})
	if err != nil {
		return err
	}
	if fetchErr == nil {
		if err := s.applyConnectorBindings(r.Context(), c, run.ID); err != nil {
			run.Error = "Snapshot tersedia; penerapan otomatis membutuhkan tinjauan."
		}
	}
	writeJSON(w, 200, run)
	return nil
}
func (s *Server) readConnector(ctx context.Context, c api.Connector) ([]api.ConnectorSourceItem, error) {
	if c.Kind == "sheets" {
		var settings api.SheetsConfig
		if err := json.Unmarshal(c.Config, &settings); err != nil {
			return nil, err
		}
		rows, err := s.sources.SheetValues(ctx, settings, false)
		if err != nil {
			return nil, err
		}
		return integrations.ParseRows(settings, rows)
	}
	var settings api.TrelloConfig
	if err := json.Unmarshal(c.Config, &settings); err != nil {
		return nil, err
	}
	items := []api.ConnectorSourceItem{}
	for _, cardID := range settings.CardIDs {
		card, err := s.sources.Trello(ctx, cardID)
		if err != nil {
			return nil, err
		}
		item := api.ConnectorSourceItem{Key: card.ID, Title: card.Name, TrelloCardID: card.ID, ConceptURL: card.URL, Caption: card.Description}
		if card.Closed {
			item.Issue = "Kartu diarsipkan; perlu tinjauan, konten tidak dihapus."
		}
		item.Fingerprint = integrations.Hash(item)
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, errors.New("tidak ada kartu untuk diproses")
	}
	return items, nil
}
func (s *Server) finishConnectorFetch(ctx context.Context, q store.Querier, c api.Connector, run *api.ConnectorRun, items []api.ConnectorSourceItem, fetchErr error) error {
	now := s.now().UTC()
	run.FinishedAt = &now
	run.Status = "needs_review"
	run.ItemCount = len(items)
	if fetchErr != nil {
		run.Status = "failed"
		run.Error = fetchErr.Error()
		if _, err := q.Exec(ctx, `update connector_configs set last_error=$2,next_run_at=$3 where id=$1`, c.ID, run.Error, now.Add(time.Duration(c.IntervalSeconds)*time.Second)); err != nil {
			return err
		}
	} else {
		run.SnapshotHash = integrations.Hash(items)
		var previous string
		_ = q.QueryRow(ctx, `select snapshot_hash from connector_runs where connector_id=$1 and status in ('needs_review','succeeded','unchanged') order by started_at desc,id desc limit 1`, c.ID).Scan(&previous)
		if previous == run.SnapshotHash {
			run.Status = "unchanged"
		} else {
			for _, item := range items {
				data, err := json.Marshal(item)
				if err != nil {
					return err
				}
				_, err = q.Exec(ctx, `insert into connector_items(id,connector_id,external_key,fingerprint,source_row,data,state,issue,first_seen_at,last_seen_at) values($1,$2,$3,$4,$5,$6,'pending',$7,$8,$8)
 on conflict(connector_id,external_key) do update set fingerprint=excluded.fingerprint,source_row=excluded.source_row,data=excluded.data,issue=excluded.issue,last_seen_at=excluded.last_seen_at,state=case when connector_items.state='missing' then 'pending' when connector_items.fingerprint=excluded.fingerprint then connector_items.state else 'pending' end`, id.New(), c.ID, item.Key, item.Fingerprint, item.Row, data, item.Issue, now)
				if err != nil {
					return err
				}
			}
			if _, err := q.Exec(ctx, `update connector_items set state='missing',issue='Baris tidak ada pada snapshot terbaru; tidak membatalkan konten otomatis.' where connector_id=$1 and last_seen_at<$2 and state<>'ignored'`, c.ID, run.StartedAt); err != nil {
				return err
			}
		}
		if _, err := q.Exec(ctx, `update connector_configs set last_success_at=$2,last_error='',next_run_at=$3 where id=$1`, c.ID, now, now.Add(time.Duration(c.IntervalSeconds)*time.Second)); err != nil {
			return err
		}
	}
	_, err := q.Exec(ctx, `update connector_runs set finished_at=$2,status=$3,snapshot_hash=$4,item_count=$5,error=$6 where id=$1`, run.ID, now, run.Status, run.SnapshotHash, run.ItemCount, run.Error)
	return err
}
func (s *Server) connectorPeople(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.ConnectorPersonMapping
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name := strings.ToLower(strings.Join(strings.Fields(req.SourceName), " "))
	if name == "" || len(name) > 240 {
		return badRequest("Nama sumber tidak valid.")
	}
	err := s.withConnectorAccounts(r, p, true, []id.ID{req.UserID}, func(q store.Querier, sc *scope, c *api.Connector) error {
		sp, err := s.store.SpaceByID(r.Context(), q, c.SpaceID)
		if err != nil {
			return err
		}
		if !sp.HasDivision(req.DivisionID) {
			return errForbidden
		}
		if req.DivisionID != sc.division.ID {
			return errForbidden
		} // design identity mapping remains Mediatek's authority
		if _, err := s.taskAssignee(r.Context(), q, c.PeriodID, req.DivisionID, req.UserID); err != nil {
			return err
		}
		_, err = q.Exec(r.Context(), `insert into connector_people(connector_id,source_name,division_id,user_id) values($1,$2,$3,$4) on conflict(connector_id,source_name,division_id) do update set user_id=excluded.user_id`, c.ID, name, req.DivisionID, req.UserID)
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func (s *Server) decideConnectorItem(w http.ResponseWriter, r *http.Request, p *principal) error {
	iid, err := pathID(r, "itemId")
	if err != nil {
		return err
	}
	var req api.ConnectorDecision
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.Action != "ignore" && req.Action != "create" && req.Action != "link" && req.Action != "apply" {
		return badRequest("Pilih abaikan, buat, tautkan atau terapkan.")
	}
	extra := []id.ID{}
	if req.HumasPJID != nil {
		extra = append(extra, *req.HumasPJID)
	}
	if req.ContentID != nil {
		if current, err := s.store.Medhum(r.Context(), s.store.DB(), *req.ContentID, store.LockNone); err == nil {
			extra = append(extra, current.HumasPJID)
			if current.DesignPJID != nil {
				extra = append(extra, *current.DesignPJID)
			}
		}
	}
	var out api.MedhumContent
	err = s.withConnectorAccounts(r, p, true, extra, func(q store.Querier, sc *scope, connector *api.Connector) error {
		var data []byte
		var fingerprint, state string
		err := q.QueryRow(r.Context(), `select data,fingerprint,state from connector_items where id=$1 and connector_id=$2 for update`, iid, connector.ID).Scan(&data, &fingerprint, &state)
		if err != nil {
			return errNotFound
		}
		if fingerprint != req.ExpectedFingerprint {
			return conflict("Data sumber berubah setelah pratinjau. Tinjau snapshot terbaru.")
		}
		var item api.ConnectorSourceItem
		if json.Unmarshal(data, &item) != nil {
			return badRequest("Snapshot sumber tidak valid.")
		}
		if req.Action == "ignore" {
			if _, err := q.Exec(r.Context(), `update connector_items set state='ignored',issue=$2 where id=$1`, iid, req.Reason); err != nil {
				return err
			}
			return s.record(r, q, p, store.AuditEvent{Action: "connector.item.ignore", TargetType: "connector_item", TargetID: &iid, PeriodID: &connector.PeriodID, After: map[string]any{"sourceKey": item.Key, "reason": req.Reason}})
		}
		if req.FollowUpdates && connector.Kind == "sheets" {
			var cfg api.SheetsConfig
			_ = json.Unmarshal(connector.Config, &cfg)
			if cfg.KeyColumn == 0 || strings.HasPrefix(item.Key, "derived:") {
				return badRequest("Ikuti perubahan otomatis memerlukan kolom identitas stabil. Gunakan tinjauan manual untuk sumber tanpa ID.")
			}
		}
		if item.Issue != "" || state == "missing" {
			return conflict("Perbaiki masalah sumber sebelum menerapkan item.")
		}
		now := s.now().UTC()
		create := req.Action == "create"
		if create {
			if connector.Kind != "sheets" || item.PostingDate == nil {
				return badRequest("Konten baru membutuhkan jadwal kalender Sheets; kartu Trello ditautkan ke konten yang sudah ada.")
			}
			if req.ContentID != nil {
				return badRequest("Konten baru tidak menerima tujuan yang sudah ada.")
			}
			var duplicate bool
			if err := q.QueryRow(r.Context(), `select exists(select 1 from medhum_contents where space_id=$1 and lower(title)=lower($2) and posting_date=$3)`, connector.SpaceID, item.Title, item.PostingDate).Scan(&duplicate); err != nil {
				return err
			}
			if duplicate {
				return conflict("Konten dengan judul/jadwal yang sama sudah ada. Pilih tautkan agar identitas tugas tidak berganda.")
			}
			_, humas, design, err := s.medhumDivisions(r, q, connector.SpaceID)
			if err != nil {
				return err
			}
			out = api.MedhumContent{ID: id.New(), PeriodID: connector.PeriodID, SpaceID: connector.SpaceID, HumasDivisionID: humas, DesignDivisionID: design, Title: item.Title, PostingDate: item.PostingDate, Version: 1, CreatedBy: p.User.ID, CreatedAt: now, UpdatedAt: now}
		} else {
			if req.ContentID == nil {
				return badRequest("Pilih konten tujuan secara eksplisit.")
			}
			out, err = s.store.Medhum(r.Context(), q, *req.ContentID, store.LockUpdate)
			if err != nil {
				return errNotFound
			}
			if out.SpaceID != connector.SpaceID || out.PeriodID != connector.PeriodID {
				return errForbidden
			}
		}
		if create {
			if req.HumasPJID == nil || *req.HumasPJID == id.Nil {
				return badRequest("Pilih akun PJ Humas untuk konten baru.")
			}
			out.HumasPJID = *req.HumasPJID
		}
		if err := s.medhumPermissions(r, q, p, &out); err != nil {
			return err
		}
		if !out.CanEditCalendar {
			return errForbidden
		}
		before := out
		if connector.Kind == "sheets" && req.Action != "link" {
			if req.HumasPJID == nil || *req.HumasPJID == id.Nil {
				return badRequest("Pilih akun PJ Humas untuk menerapkan nama sumber; nama teks tidak menjadi identitas akun otomatis.")
			}
			out.HumasPJID = *req.HumasPJID
			out.Title = item.Title
			out.PostingDate = item.PostingDate
			out.PostingAt = nil
			// PJ desain from the sheet is informational until Mediatek assigns it.
			if item.TrelloCardID != "" && out.TrelloCardID == "" {
				out.TrelloCardID = item.TrelloCardID
			}
		}
		if connector.Kind == "trello" && req.Action != "link" {
			out.TrelloCardID = item.TrelloCardID
			out.ConceptURL = item.ConceptURL
			out.Caption = item.Caption
		}
		if create || req.Action != "link" {
			if err := validateMedhum(&out, sc.period.TimeZone); err != nil {
				return err
			}
			if err := s.syncMedhumTasks(r, q, p, sc.period, &out); err != nil {
				return err
			}
			if !create {
				out.Version++
			}
			out.UpdatedAt = now
			if err := s.store.SaveMedhum(r.Context(), q, out, create); err != nil {
				return err
			}
		}
		if _, err := q.Exec(r.Context(), `insert into connector_bindings(connector_id,external_key,content_id,accepted_fingerprint,decided_by,decided_at) values($1,$2,$3,$4,$5,$6) on conflict(connector_id,external_key) do update set content_id=excluded.content_id,accepted_fingerprint=excluded.accepted_fingerprint,decided_by=excluded.decided_by,decided_at=excluded.decided_at`, connector.ID, item.Key, out.ID, fingerprint, p.User.ID, now); err != nil {
			return err
		}
		baseline, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update connector_bindings set follow_updates=$3,accepted_data=$4,accepted_content=$5 where connector_id=$1 and external_key=$2`, connector.ID, item.Key, req.FollowUpdates, data, baseline); err != nil {
			return err
		}
		if connector.Kind == "sheets" && req.HumasPJID != nil && req.Action != "link" {
			name := strings.ToLower(strings.Join(strings.Fields(item.HumasName), " "))
			if _, err := q.Exec(r.Context(), `insert into connector_people(connector_id,source_name,division_id,user_id) values($1,$2,$3,$4) on conflict(connector_id,source_name,division_id) do update set user_id=excluded.user_id`, connector.ID, name, connector.DivisionID, *req.HumasPJID); err != nil {
				return err
			}
		}
		if err := s.connectorEvent(r.Context(), q, connector.ID, &p.User.ID, "binding."+req.Action, nil, map[string]any{"source": item, "content": out, "followUpdates": req.FollowUpdates}); err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update connector_items set content_id=$2,state='applied',issue='' where id=$1`, iid, out.ID); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "connector.item." + req.Action, TargetType: "medhum_content", TargetID: &out.ID, PeriodID: &out.PeriodID, Before: before, After: map[string]any{"content": out, "sourceKey": item.Key, "fingerprint": fingerprint, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
