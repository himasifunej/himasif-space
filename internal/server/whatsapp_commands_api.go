package server

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"slices"
)

func (s *Server) listWACommands(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.WACustomCommand{}
	err := s.withWA(r, p, false, func(q store.Querier, _ *scope, c *api.WAConnection) error {
		rows, err := q.Query(r.Context(), `select id,trigger,response,asset_id,media_type,group_ids,version from wa_custom_commands where connection_id=$1 order by trigger`, c.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var command api.WACustomCommand
			if err := rows.Scan(&command.ID, &command.Trigger, &command.Response, &command.AssetID, &command.MediaType, &command.GroupIDs, &command.Version); err != nil {
				return err
			}
			out = append(out, command)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) saveWACommand(w http.ResponseWriter, r *http.Request, p *principal) error {
	commandID, err := pathID(r, "commandId")
	if err != nil {
		return err
	}
	var req api.WACustomCommandRequest
	if err = readJSON(r, &req); err != nil {
		return err
	}
	if err = validateWACommand(&req); err != nil {
		return err
	}
	if req.AssetID == "" {
		req.MediaType = ""
	}
	if req.GroupIDs == nil {
		req.GroupIDs = []id.ID{}
	}
	slices.SortFunc(req.GroupIDs, func(a, b id.ID) int { return slices.Compare(a[:], b[:]) })
	req.GroupIDs = slices.Compact(req.GroupIDs)
	var out api.WACustomCommand
	err = s.withWA(r, p, true, func(q store.Querier, _ *scope, c *api.WAConnection) error {
		if len(req.GroupIDs) > 20 {
			return badRequest("Terlalu banyak cakupan grup.")
		}
		for _, gid := range req.GroupIDs {
			if !slices.ContainsFunc(c.Groups, func(g api.WAGroup) bool { return g.ID == gid }) {
				return badRequest("Grup command harus milik koneksi ini.")
			}
		}
		if req.AssetID != "" {
			var exists bool
			if err := q.QueryRow(r.Context(), `select exists(select 1 from wa_assets where connection_id=$1 and bridge_asset_id=$2)`, c.ID, req.AssetID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return badRequest("Aset belum tersimpan pada koneksi ini.")
			}
		}
		var version int64
		err := q.QueryRow(r.Context(), `select version from wa_custom_commands where id=$1 and connection_id=$2 for update`, commandID, c.ID).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			if req.ExpectedVersion != 0 {
				return errNotFound
			}
			_, err = q.Exec(r.Context(), `insert into wa_custom_commands(id,connection_id,trigger,response,asset_id,media_type,group_ids,updated_by,updated_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, commandID, c.ID, req.Trigger, req.Response, req.AssetID, req.MediaType, req.GroupIDs, p.User.ID, s.now())
			version = 1
		} else if err == nil {
			if version != req.ExpectedVersion {
				return conflict("Command berubah. Muat ulang sebelum menyimpan.")
			}
			version++
			_, err = q.Exec(r.Context(), `update wa_custom_commands set trigger=$2,response=$3,asset_id=$4,media_type=$5,group_ids=$6,version=$7,updated_by=$8,updated_at=$9 where id=$1`, commandID, req.Trigger, req.Response, req.AssetID, req.MediaType, req.GroupIDs, version, p.User.ID, s.now())
		}
		if err != nil {
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				return conflict("Pemicu atau identitas command sudah dipakai.")
			}
			return err
		}
		out = api.WACustomCommand{ID: commandID, Trigger: req.Trigger, Response: req.Response, AssetID: req.AssetID, MediaType: req.MediaType, GroupIDs: req.GroupIDs, Version: version}
		return s.waAudit(r, q, p, *c, "whatsapp.command.save", out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) deleteWACommand(w http.ResponseWriter, r *http.Request, p *principal) error {
	commandID, err := pathID(r, "commandId")
	if err != nil {
		return err
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err = readJSON(r, &req); err != nil {
		return err
	}
	err = s.withWA(r, p, true, func(q store.Querier, _ *scope, c *api.WAConnection) error {
		var version int64
		if err := q.QueryRow(r.Context(), `select version from wa_custom_commands where id=$1 and connection_id=$2 for update`, commandID, c.ID).Scan(&version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errNotFound
			}
			return err
		}
		if version != req.ExpectedVersion {
			return conflict("Command berubah. Muat ulang sebelum menghapus.")
		}
		if _, err := q.Exec(r.Context(), `delete from wa_custom_commands where id=$1`, commandID); err != nil {
			return err
		}
		return s.waAudit(r, q, p, *c, "whatsapp.command.delete", map[string]any{"id": commandID, "version": version})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func (s *Server) waOutbox(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.WAOutboxItem{}
	err := s.withWA(r, p, false, func(q store.Querier, _ *scope, c *api.WAConnection) error {
		rows, err := q.Query(r.Context(), `select o.id,coalesce(nullif(o.target_name,''),g.name),o.status,o.attempts,o.message_id,o.last_error,o.created_at,o.updated_at,o.suppressed_at,o.suppression_reason from wa_outbox o join wa_groups g on g.id=o.group_id where o.connection_id=$1 order by o.created_at desc,o.id desc limit 100`, c.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item api.WAOutboxItem
			if err := rows.Scan(&item.ID, &item.GroupName, &item.Status, &item.Attempts, &item.MessageID, &item.LastError, &item.CreatedAt, &item.UpdatedAt, &item.SuppressedAt, &item.SuppressionReason); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
