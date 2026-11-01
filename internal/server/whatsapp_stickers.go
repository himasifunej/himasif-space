package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"net/http"
)

func (s *Server) waStickerCommand(r *http.Request, q store.Querier, p *principal, conn api.WAConnection, group api.WAGroup, in waInbound, command string, args []string) (whatsapp.SendRequest, error) {
	reply := whatsapp.SendRequest{}
	req := api.WACustomCommandRequest{}
	if command == "setsticker" {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockNone)
		if err != nil {
			return reply, err
		}
		if err = sc.require(api.CapWhatsAppManage); err != nil {
			return reply, err
		}
		if len(args) != 1 {
			return reply, badRequest("Balas media dengan !setsticker <pemicu>, misalnya !setsticker damn.")
		}
		req.Trigger = args[0]
		req.Response = "placeholder"
		if err = validateWACommand(&req); err != nil {
			return reply, err
		}
	}
	prepared, ok := r.Context().Value(waStickerKey{}).(waPreparedSticker)
	if !ok {
		return reply, fail(503, api.CodeUnavailable, "Media belum disiapkan; command belum disimpan.")
	}
	if prepared.Err != nil {
		return reply, prepared.Err
	}
	asset := prepared.Asset
	var err error
	if _, err = q.Exec(r.Context(), `insert into wa_assets(id,connection_id,bridge_asset_id,name,mime,size,source_message_id,created_by,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict(connection_id,bridge_asset_id) do nothing`, id.New(), conn.ID, asset.AssetID, "Stiker WhatsApp", asset.MIME, asset.Size, in.ID, p.User.ID, s.now()); err != nil {
		return reply, err
	}
	if command == "setsticker" {
		req.Response = ""
		req.AssetID = asset.AssetID
		req.MediaType = "sticker"
		if err = validateWACommand(&req); err != nil {
			return reply, err
		}
		// A chat command changes only this connection's named slot. Existing group
		// restrictions remain intact, so replacement never widens its audience.
		_, err = q.Exec(r.Context(), `insert into wa_custom_commands(id,connection_id,trigger,asset_id,media_type,group_ids,updated_by,updated_at) values($1,$2,$3,$4,'sticker',$5,$6,$7) on conflict(connection_id,trigger) do update set response='',asset_id=excluded.asset_id,media_type='sticker',version=wa_custom_commands.version+1,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, id.New(), conn.ID, req.Trigger, asset.AssetID, []id.ID{group.ID}, p.User.ID, s.now())
		if err != nil {
			return reply, err
		}
		if err = s.waAudit(r, q, p, conn, "whatsapp.command.sticker", map[string]any{"trigger": req.Trigger, "assetId": asset.AssetID, "inboundId": in.ID}); err != nil {
			return reply, err
		}
		reply.Text = waText(r.Context(), "sticker.saved", req.Trigger)
	} else {
		reply.AssetID = asset.AssetID
		reply.MediaType = "sticker"
	}
	return reply, nil
}
