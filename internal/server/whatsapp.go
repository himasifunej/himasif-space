package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

var waGroupJID = regexp.MustCompile(`^[0-9]+(?:-[0-9]+)?@g\.us$`)

func (s *Server) withWA(r *http.Request, p *principal, write bool, fn func(store.Querier, *scope, *api.WAConnection) error) error {
	cid, err := pathID(r, "connectionId")
	if err != nil {
		return err
	}
	peek, err := s.store.WAConnection(r.Context(), s.store.DB(), cid, store.LockNone)
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
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapWhatsAppManage); err != nil {
			return err
		}
		if write {
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		c, err := s.store.WAConnection(r.Context(), q, cid, rowLock)
		if err != nil {
			return err
		}
		c.Groups, err = s.store.WAGroups(r.Context(), q, cid)
		if err != nil {
			return err
		}
		return fn(q, sc, &c)
	})
}
func validateWA(req *api.WAConnectionRequest) error {
	var err error
	req.DisplayName, err = text(req.DisplayName, "nama bot", 1, 100)
	if err != nil {
		return err
	}
	req.Greeting, err = text(req.Greeting, "sapaan", 0, 500)
	if err != nil {
		return err
	}
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if req.Language == "" {
		req.Language = "id"
	}
	if !slices.Contains([]string{"id", "en"}, req.Language) {
		return badRequest("Pilih bahasa bot Indonesia (id) atau English (en).")
	}
	if req.Templates == nil {
		req.Templates = map[string]string{}
	}
	for key, value := range req.Templates {
		if !slices.Contains([]string{"reminder", "ping", "reply", "weekly", "followup"}, key) || len(value) > 8000 {
			return badRequest("Template bot tidak valid.")
		}
		if value != "" && !strings.Contains(value, "{body}") {
			return badRequest("Template harus memuat {body} agar isi dan hasil tindakan tetap ditampilkan.")
		}
	}
	return nil
}
func (s *Server) waAudit(r *http.Request, q store.Querier, p *principal, c api.WAConnection, action string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	if _, err := q.Exec(r.Context(), `insert into wa_events(id,connection_id,actor_id,action,detail,occurred_at) values($1,$2,$3,$4,$5,$6)`, id.New(), c.ID, p.User.ID, action, data, s.now()); err != nil {
		return err
	}
	return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "wa_connection", TargetID: &c.ID, PeriodID: &c.PeriodID, After: detail})
}
func (s *Server) listWA(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.WAConnection{}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapWhatsAppManage); err != nil {
			return err
		}
		out, err = s.store.WAConnections(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		for i := range out {
			out[i].Groups, err = s.store.WAGroups(r.Context(), q, out[i].ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.WAConnection]{Items: out})
	return nil
}
func (s *Server) createWA(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.WAConnectionRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateWA(&req); err != nil {
		return err
	}
	var out api.WAConnection
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockUpdate)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapWhatsAppManage); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		var exists bool
		if err := q.QueryRow(r.Context(), `select exists(select 1 from wa_connections where period_id=$1 and division_id=$2)`, sc.period.ID, sc.division.ID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return conflict("Divisi ini sudah memiliki konfigurasi koneksi WhatsApp.")
		}
		now := s.now().UTC()
		out = api.WAConnection{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, DisplayName: req.DisplayName, Language: req.Language, Greeting: req.Greeting, Templates: req.Templates, Status: "disconnected", Groups: []api.WAGroup{}, Version: 1, UpdatedAt: now}
		templates, _ := json.Marshal(out.Templates)
		if _, err := q.Exec(r.Context(), `insert into wa_connections(id,period_id,division_id,display_name,language,greeting,templates,created_by,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, out.ID, out.PeriodID, out.DivisionID, out.DisplayName, out.Language, out.Greeting, templates, p.User.ID, now); err != nil {
			return err
		}
		return s.waAudit(r, q, p, out, "whatsapp.create", out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func (s *Server) updateWA(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.WAConnectionRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateWA(&req); err != nil {
		return err
	}
	var out api.WAConnection
	err := s.withWA(r, p, true, func(q store.Querier, sc *scope, c *api.WAConnection) error {
		if req.ExpectedVersion != c.Version {
			return conflict("Konfigurasi WhatsApp berubah. Muat ulang.")
		}
		data, _ := json.Marshal(req.Templates)
		c.DisplayName = req.DisplayName
		c.Language = req.Language
		c.Greeting = req.Greeting
		c.Templates = req.Templates
		c.Version++
		c.UpdatedAt = s.now().UTC()
		if _, err := q.Exec(r.Context(), `update wa_connections set display_name=$2,language=$3,greeting=$4,templates=$5,version=$6,updated_at=$7 where id=$1`, c.ID, c.DisplayName, c.Language, c.Greeting, data, c.Version, c.UpdatedAt); err != nil {
			return err
		}
		out = *c
		return s.waAudit(r, q, p, *c, "whatsapp.configure", req)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) waSession(w http.ResponseWriter, r *http.Request, p *principal) error {
	var c api.WAConnection
	err := s.withWA(r, p, false, func(q store.Querier, sc *scope, current *api.WAConnection) error { c = *current; return nil })
	if err != nil {
		return err
	}
	state, err := s.wa.State(r.Context(), c.ID)
	if err != nil {
		return fail(503, api.CodeUnavailable, "Status koneksi WhatsApp belum dapat diperiksa.")
	}
	writeJSON(w, 200, api.WASessionState{Status: state.Status, AccountJID: state.AccountJID, QR: state.QR, QRExpiresAt: state.QRExpiresAt, LastError: state.LastError, UpdatedAt: state.UpdatedAt})
	return nil
}
func (s *Server) waAction(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.WAActionRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains([]string{"connect", "disconnect", "logout"}, req.Action) {
		return badRequest("Aksi WhatsApp tidak valid.")
	}
	if req.Action == "logout" && !req.Confirmed {
		return badRequest("Konfirmasi pelepasan sesi WhatsApp diperlukan.")
	}
	cid, err := pathID(r, "connectionId")
	if err != nil {
		return err
	}
	var state whatsapp.State
	acquired, err := s.withWATransport(r.Context(), cid, func() error {
		var version int64
		err := s.withWA(r, p, true, func(q store.Querier, sc *scope, current *api.WAConnection) error {
			if req.ExpectedVersion != current.Version {
				return conflict("Konfigurasi berubah. Muat ulang sebelum mengubah sesi.")
			}
			current.Enabled = req.Action == "connect"
			current.Version++
			current.UpdatedAt = s.now().UTC()
			version = current.Version
			if _, err := q.Exec(r.Context(), `update wa_connections set enabled=$2,version=$3,updated_at=$4 where id=$1`, current.ID, current.Enabled, current.Version, current.UpdatedAt); err != nil {
				return err
			}
			return s.waAudit(r, q, p, *current, "whatsapp."+req.Action, map[string]any{"enabled": current.Enabled})
		})
		if err != nil {
			return err
		}
		snap, err := s.waTransportSnapshot(r.Context(), cid)
		if err != nil {
			return err
		}
		if snap.Connection.Version != version {
			return conflict("Konfigurasi berubah sebelum bridge menerima tindakan. Muat ulang status.")
		}
		if snap.Closed {
			return errPeriodClosed
		}
		if req.Action == "connect" {
			if _, err = s.wa.Configure(r.Context(), cid, snap.allowedGroups()); err != nil {
				return fail(503, api.CodeUnavailable, "Konfigurasi tersimpan; bridge belum menerima perubahan grup.")
			}
		}
		state, err = s.wa.Action(r.Context(), cid, req.Action)
		if err != nil {
			return fail(503, api.CodeUnavailable, "Permintaan sesi dicatat, hasil bridge belum dapat dipastikan. Muat ulang status sebelum mencoba lagi.")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !acquired {
		return conflict("Koneksi sedang menjalankan operasi lain. Muat ulang status sebelum mencoba lagi.")
	}

	writeJSON(w, 200, api.WASessionState{Status: state.Status, AccountJID: state.AccountJID, QR: state.QR, QRExpiresAt: state.QRExpiresAt, LastError: state.LastError, UpdatedAt: state.UpdatedAt})
	return nil
}
func (s *Server) waAvailableGroups(w http.ResponseWriter, r *http.Request, p *principal) error {
	var cid id.ID
	err := s.withWA(r, p, false, func(q store.Querier, sc *scope, c *api.WAConnection) error { cid = c.ID; return nil })
	if err != nil {
		return err
	}
	groups, err := s.wa.Groups(r.Context(), cid)
	if err != nil {
		return fail(503, api.CodeUnavailable, "Daftar grup belum dapat dimuat dari sesi WhatsApp.")
	}
	out := []api.WAAvailableGroup{}
	for _, g := range groups {
		out = append(out, api.WAAvailableGroup{JID: g.JID, Name: g.Name})
	}
	writeJSON(w, 200, api.List[api.WAAvailableGroup]{Items: out})
	return nil
}
func (s *Server) saveWAGroup(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.WAGroupRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !waGroupJID.MatchString(req.JID) {
		return badRequest("Pilih grup WhatsApp yang tersedia.")
	}
	var snapshot api.WAConnection
	err := s.withWA(r, p, false, func(q store.Querier, sc *scope, c *api.WAConnection) error { snapshot = *c; return nil })
	if err != nil {
		return err
	}
	available, err := s.wa.Groups(r.Context(), snapshot.ID)
	if err != nil {
		return fail(503, api.CodeUnavailable, "Grup harus diverifikasi melalui sesi yang terhubung.")
	}
	name := ""
	for _, g := range available {
		if g.JID == req.JID {
			name = g.Name
			break
		}
	}
	if name == "" {
		return badRequest("Akun WhatsApp tidak terkonfirmasi sebagai anggota grup ini.")
	}
	err = s.withWA(r, p, true, func(q store.Querier, sc *scope, c *api.WAConnection) error {
		if req.ExpectedVersion != c.Version {
			return conflict("Konfigurasi grup berubah. Muat ulang.")
		}
		sp, err := s.store.SpaceByID(r.Context(), q, req.SpaceID)
		if err != nil {
			return err
		}
		if sp.PeriodID != c.PeriodID || !sp.HasDivision(c.DivisionID) {
			return errForbidden
		}
		if sp.Kind == api.SpaceShared && (sp.Code != "medhum" || sc.division.Slug != "humas") {
			return badRequest("Grup Medhum hanya dilayani bot Humas.")
		}
		var duplicate bool
		if err := q.QueryRow(r.Context(), `select exists(select 1 from wa_groups where period_id=$1 and (space_id=$2 or jid=$3) and not(connection_id=$4 and space_id=$2))`, c.PeriodID, req.SpaceID, req.JID, c.ID).Scan(&duplicate); err != nil {
			return err
		}
		if duplicate {
			return conflict("Ruang/grup ini sudah dilayani koneksi lain; dua bot tidak ditambahkan ke tujuan yang sama.")
		}
		if _, err := q.Exec(r.Context(), `update wa_outbox set status='cancelled',last_error='Pemetaan grup berubah sebelum pengiriman.' where group_id in(select id from wa_groups where connection_id=$1 and space_id=$2) and status in ('queued','failed')`, c.ID, req.SpaceID); err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `insert into wa_groups(id,connection_id,space_id,period_id,jid,name,commands_enabled) values($1,$2,$3,$4,$5,$6,$7) on conflict(space_id) do update set jid=excluded.jid,name=excluded.name,commands_enabled=excluded.commands_enabled`, id.New(), c.ID, req.SpaceID, c.PeriodID, req.JID, name, req.CommandsEnabled); err != nil {
			return err
		}
		c.Version++
		c.UpdatedAt = s.now()
		if _, err := q.Exec(r.Context(), `update wa_connections set version=$2,updated_at=$3 where id=$1`, c.ID, c.Version, c.UpdatedAt); err != nil {
			return err
		}
		return s.waAudit(r, q, p, *c, "whatsapp.group", req)
	})
	if err != nil {
		return err
	}
	if err := s.syncWABridgeGroups(r.Context(), snapshot.ID); err != nil {
		return fail(503, api.CodeUnavailable, "Pemetaan tersimpan; penerapan allowlist pada bridge belum dikonfirmasi.")
	}
	w.WriteHeader(204)
	return nil
}
func (s *Server) createWALink(w http.ResponseWriter, r *http.Request, p *principal) error {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	code := "SPACE-" + base64.RawURLEncoding.EncodeToString(bytes)
	sum := sha256.Sum256([]byte(code))
	out := api.WALinkChallenge{Code: code, ExpiresAt: s.now().Add(10 * time.Minute), Instructions: "Kirim !link " + code + " di grup divisi/Medhum yang terhubung. Kode sekali pakai ini hanya untuk akunmu."}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockUpdate)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksOwn); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update wa_link_challenges set used_at=$2 where user_id=$1 and used_at is null`, p.User.ID, s.now()); err != nil {
			return err
		}
		_, err = q.Exec(r.Context(), `insert into wa_link_challenges(id,period_id,division_id,user_id,token_hash,expires_at) values($1,$2,$3,$4,$5,$6)`, id.New(), sc.period.ID, sc.division.ID, p.User.ID, sum[:], out.ExpiresAt)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}

var reservedWACommands = []string{"tugas", "jadwal", "cek", "hari", "done", "bantu", "delegasi", "summary", "rekap", "eval", "pending", "link", "sticker", "setsticker", "help", "bantuan"}

func validateWACommand(req *api.WACustomCommandRequest) error {
	req.Trigger = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(req.Trigger), "!"))
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`).MatchString(req.Trigger) || slices.Contains(reservedWACommands, req.Trigger) {
		return badRequest("Pemicu tidak valid atau merupakan command inti.")
	}
	if len(req.Response) > 8000 {
		return badRequest("Respons command terlalu panjang.")
	}
	if req.AssetID != "" && !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(req.AssetID) {
		return badRequest("Aset command tidak valid.")
	}
	if req.AssetID != "" && !slices.Contains([]string{"sticker", "image", "video"}, req.MediaType) {
		return badRequest("Jenis media tidak valid.")
	}
	if req.Response == "" && req.AssetID == "" {
		return badRequest("Isi respons atau pilih aset command.")
	}
	return nil
}

// Always synchronize the latest committed mapping, rather than a stale API
// snapshot that may have been superseded while waiting for the bridge.
func (s *Server) syncWABridgeGroups(ctx context.Context, cid id.ID) error {
	acquired, err := s.withWATransport(ctx, cid, func() error {
		snap, err := s.waTransportSnapshot(ctx, cid)
		if err != nil {
			return err
		}
		_, err = s.wa.Configure(ctx, cid, snap.allowedGroups())
		return err
	})
	if err == nil && !acquired {
		return conflict("Sinkronisasi grup menunggu operasi koneksi yang sedang berjalan.")
	}
	return err
}
