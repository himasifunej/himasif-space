package server

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// withWATransport serializes only one bot's external operations. It holds no
// transaction/period/account locks while waiting for the bridge. Its dedicated
// physical connection closes on every exit, releasing all session locks.
func (s *Server) withWATransport(ctx context.Context, cid id.ID, fn func() error) (bool, error) {
	// Keep advisory ownership off the request pool: even a pool of size one
	// must be able to perform the short authorization/receipt transactions.
	select {
	case s.waTransportSlots <- struct{}{}:
		defer func() { <-s.waTransportSlots }()
	default:
		return false, nil
	}
	return s.store.WithSessionLock(ctx, "wa-transport:"+cid.String(), fn)
}

type waTransportSnapshot struct {
	Connection api.WAConnection
	Closed     bool
	Cursor     int64
}

func (s *Server) waTransportSnapshot(ctx context.Context, cid id.ID) (waTransportSnapshot, error) {
	var snap waTransportSnapshot
	peek, err := s.store.WAConnection(ctx, s.store.DB(), cid, store.LockNone)
	if err != nil {
		return snap, err
	}
	err = s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, peek.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		snap.Closed = period.Status == api.PeriodClosed
		snap.Connection, err = s.store.WAConnection(ctx, q, cid, store.LockShare)
		if err != nil {
			return err
		}
		snap.Connection.Groups, err = s.store.WAGroups(ctx, q, cid)
		if err != nil {
			return err
		}
		return q.QueryRow(ctx, `select event_cursor from wa_connections where id=$1`, cid).Scan(&snap.Cursor)
	})
	return snap, err
}
func (snap waTransportSnapshot) allowedGroups() []string {
	out := []string{}
	if snap.Connection.Enabled && !snap.Closed {
		for _, g := range snap.Connection.Groups {
			out = append(out, g.JID)
		}
	}
	return out
}
