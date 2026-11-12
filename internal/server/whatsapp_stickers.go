package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"net/http"
	"regexp"
	"time"
)

// A GIF is a silent mp4 WhatsApp plays looping. The bridge cuts it at ten
// seconds; the half second more allows for the container's rounding.
const (
	waGIFMaxBytes    = 4 << 20
	waGIFMaxDuration = 10_500 * time.Millisecond
)

// waMediaKind is a command reply made from WhatsApp media: a sticker or a GIF.
// Set is the command that saves one under a trigger; Direct, when not empty,
// converts and replies at once without saving.
type waMediaKind struct {
	Type, Set, Direct, Name, Usage, Unusable, Unavailable, Saved, Audit string
	// Timeout bounds the download and conversion, outside the transaction.
	Timeout time.Duration
}

var (
	waStickerMedia = waMediaKind{Type: "sticker", Set: "setsticker", Direct: "sticker", Name: "Stiker WhatsApp",
		Usage:       "Balas media dengan !setsticker <pemicu>, misalnya !setsticker damn.",
		Unusable:    "Media tidak tersedia atau tidak dapat dijadikan stiker. Lampirkan gambar/video singkat atau balas pesan medianya.",
		Unavailable: "Konversi stiker belum tersedia. Command belum disimpan.",
		Saved:       "sticker.saved", Audit: "whatsapp.command.sticker", Timeout: 10 * time.Second}
	waGIFMedia = waMediaKind{Type: "gif", Set: "setgif", Name: "GIF WhatsApp",
		Usage:       "Balas GIF, video, atau gambar dengan !setgif <pemicu>, misalnya !setgif damn.",
		Unusable:    "Media tidak tersedia atau tidak dapat dijadikan GIF. Balas pesan GIF, video, atau gambarnya.",
		Unavailable: "Konversi GIF belum tersedia. Command belum disimpan.",
		Saved:       "gif.saved", Audit: "whatsapp.command.gif", Timeout: 20 * time.Second}
)

// waMediaCommand returns the media kind a command (without "!") converts.
func waMediaCommand(command string) (waMediaKind, bool) {
	for _, kind := range []waMediaKind{waStickerMedia, waGIFMedia} {
		if command == kind.Set || (kind.Direct != "" && command == kind.Direct) {
			return kind, true
		}
	}
	return waMediaKind{}, false
}

// validateWAGIF checks what the bridge made before Space saves or sends it.
func validateWAGIF(asset whatsapp.Asset) error {
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(asset.AssetID) || asset.MIME != "video/mp4" {
		return badRequest("GIF harus berupa video mp4.")
	}
	if asset.Size <= 0 || asset.Size > waGIFMaxBytes {
		return badRequest("GIF melebihi 4 MiB.")
	}
	if asset.DurationMS <= 0 || time.Duration(asset.DurationMS)*time.Millisecond > waGIFMaxDuration {
		return badRequest("GIF harus berdurasi paling lama 10 detik.")
	}
	return nil
}

func (s *Server) waMediaCommand(r *http.Request, q store.Querier, p *principal, conn api.WAConnection, group api.WAGroup, in waInbound, kind waMediaKind, command string, args []string) (whatsapp.SendRequest, error) {
	reply := whatsapp.SendRequest{}
	req := api.WACustomCommandRequest{}
	set := command == kind.Set
	if set {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockNone)
		if err != nil {
			return reply, err
		}
		if err = sc.require(api.CapWhatsAppManage); err != nil {
			return reply, err
		}
		if len(args) != 1 {
			return reply, badRequest(kind.Usage)
		}
		req.Trigger = args[0]
		req.Response = "placeholder"
		if err = validateWACommand(&req); err != nil {
			return reply, err
		}
	}
	prepared, ok := r.Context().Value(waMediaKey{}).(waPreparedMedia)
	if !ok || prepared.Kind != kind.Type {
		return reply, fail(503, api.CodeUnavailable, "Media belum disiapkan; command belum disimpan.")
	}
	if prepared.Err != nil {
		return reply, prepared.Err
	}
	asset := prepared.Asset
	var err error
	if kind.Type == "gif" {
		if err = validateWAGIF(asset); err != nil {
			return reply, err
		}
	}
	if _, err = q.Exec(r.Context(), `insert into wa_assets(id,connection_id,bridge_asset_id,name,mime,size,source_message_id,created_by,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict(connection_id,bridge_asset_id) do nothing`, id.New(), conn.ID, asset.AssetID, kind.Name, asset.MIME, asset.Size, in.ID, p.User.ID, s.now()); err != nil {
		return reply, err
	}
	if set {
		req.Response = ""
		req.AssetID = asset.AssetID
		req.MediaType = kind.Type
		if err = validateWACommand(&req); err != nil {
			return reply, err
		}
		// A chat command changes only this connection's named slot. Existing group
		// restrictions remain intact, so replacement never widens its audience.
		_, err = q.Exec(r.Context(), `insert into wa_custom_commands(id,connection_id,trigger,asset_id,media_type,group_ids,updated_by,updated_at) values($1,$2,$3,$4,$8,$5,$6,$7) on conflict(connection_id,trigger) do update set response='',asset_id=excluded.asset_id,media_type=excluded.media_type,version=wa_custom_commands.version+1,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, id.New(), conn.ID, req.Trigger, asset.AssetID, []id.ID{group.ID}, p.User.ID, s.now(), kind.Type)
		if err != nil {
			return reply, err
		}
		if err = s.waAudit(r, q, p, conn, kind.Audit, map[string]any{"trigger": req.Trigger, "assetId": asset.AssetID, "inboundId": in.ID}); err != nil {
			return reply, err
		}
		reply.Text = waText(r.Context(), kind.Saved, req.Trigger)
	} else {
		reply.AssetID = asset.AssetID
		reply.MediaType = kind.Type
	}
	return reply, nil
}
