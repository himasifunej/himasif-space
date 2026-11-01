package server

import (
	"context"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"net/http"
	"strings"
)

type waStickerKey struct{}
type waPreparedSticker struct {
	Asset whatsapp.Asset
	Err   error
}

// Conversion reads the cached media outside the mutation transaction. The
// command revalidates sender, group, role and period before saving its result.
func (s *Server) prepareWASticker(ctx context.Context, iid, pid, cid id.ID) waPreparedSticker {
	var in waInbound
	var group api.WAGroup
	err := s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, pid, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		conn, err := s.store.WAConnection(ctx, q, cid, store.LockShare)
		if err != nil {
			return err
		}
		if !conn.Enabled {
			return errForbidden
		}
		err = q.QueryRow(ctx, `select i.id,i.message_id,i.quoted_message_id,i.body,i.sender_jid,i.sender_alt_jid,g.id,g.space_id,g.jid,g.commands_enabled from wa_inbound i join wa_groups g on g.id=i.group_id and g.jid=i.group_jid where i.id=$1 and i.state='queued' and g.connection_id=$2`, iid, cid).Scan(&in.ID, &in.MessageID, &in.QuotedMessageID, &in.Body, &in.SenderJID, &in.SenderAltJID, &group.ID, &group.SpaceID, &group.JID, &group.CommandsEnabled)
		if err != nil {
			return err
		}
		if !group.CommandsEnabled {
			return errForbidden
		}
		r := (&http.Request{}).WithContext(ctx)
		r.SetPathValue("periodId", pid.String())
		r.SetPathValue("divisionId", conn.DivisionID.String())
		p, err := s.waSender(r, q, period, group, in)
		if err != nil {
			return err
		}
		fields := strings.Fields(in.Body)
		if len(fields) > 0 && strings.EqualFold(fields[0], "!setsticker") {
			sc, err := s.loadScope(ctx, q, r, p, store.LockNone)
			if err != nil {
				return err
			}
			if err = sc.require(api.CapWhatsAppManage); err != nil {
				return err
			}
			if len(fields) != 2 {
				return badRequest("Balas media dengan !setsticker <pemicu>, misalnya !setsticker damn.")
			}
			req := api.WACustomCommandRequest{Trigger: fields[1], Response: "placeholder"}
			return validateWACommand(&req)
		}
		return nil
	})
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) {
			return waPreparedSticker{Err: err}
		}
		return waPreparedSticker{Err: fail(503, api.CodeUnavailable, "Media belum dapat disiapkan; command belum disimpan.")}
	}
	message := in.MessageID
	if in.QuotedMessageID != "" {
		message = in.QuotedMessageID
	}
	var asset whatsapp.Asset
	acquired, err := s.withWATransport(ctx, cid, func() error { var err error; asset, err = s.wa.Sticker(ctx, cid, group.JID, message); return err })
	if err == nil && !acquired {
		err = fail(503, api.CodeUnavailable, "Koneksi sedang sibuk; konversi belum dicoba.")
	}
	if err != nil {
		var failure *whatsapp.Error
		if errors.As(err, &failure) && (failure.Status == 400 || failure.Status == 404) {
			err = badRequest("Media tidak tersedia atau tidak dapat dijadikan stiker. Lampirkan gambar/video singkat atau balas pesan medianya.")
		} else {
			err = fail(503, api.CodeUnavailable, "Konversi stiker belum tersedia. Command belum disimpan.")
		}
	}
	return waPreparedSticker{Asset: asset, Err: err}
}
