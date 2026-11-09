package server

import (
	"context"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
	"sync"
	"time"
)

type realtimeHub struct {
	mu    sync.Mutex
	ready bool
	subs  map[id.ID]map[chan struct{}]struct{}
}

func (h *realtimeHub) publish(period id.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for pid, subs := range h.subs {
		if period != id.Nil && pid != period {
			continue
		}
		for ch := range subs {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}
func (h *realtimeHub) setReady(ready bool) {
	h.mu.Lock()
	h.ready = ready
	h.mu.Unlock()
	h.publish(id.Nil)
}
func (h *realtimeHub) subscribe(period id.ID) (chan struct{}, func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.ready {
		return nil, nil, false
	}
	if h.subs == nil {
		h.subs = map[id.ID]map[chan struct{}]struct{}{}
	}
	if h.subs[period] == nil {
		h.subs[period] = map[chan struct{}]struct{}{}
	}
	// Bound memory and database authorization work, even with many abandoned tabs.
	n := 0
	for _, subs := range h.subs {
		n += len(subs)
	}
	if n >= 256 {
		return nil, nil, false
	}
	ch := make(chan struct{}, 1)
	h.subs[period][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[period], ch)
		if len(h.subs[period]) == 0 {
			delete(h.subs, period)
		}
		h.mu.Unlock()
	}, true
}

func (s *Server) RunRealtime(ctx context.Context) {
	defer s.realtime.setReady(false)
	for ctx.Err() == nil {
		_ = s.store.ListenChanges(ctx, func() { s.realtime.setReady(true) }, func(payload string) {
			if period, err := id.Parse(payload); err == nil {
				s.realtime.publish(period)
			}
		})
		s.realtime.setReady(false)
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// Check both session and current access before every notification. Invalidations
// carry no content; refetches still pass through the canonical resource APIs.
func (s *Server) realtimeAccess(r *http.Request, pid, sid id.ID) error {
	p, err := s.principal(r)
	if err != nil {
		return err
	}
	if p.admin() {
		return errForbidden
	}
	if sid != id.Nil {
		sp, err := s.spaceView(r.Context(), s.store.DB(), p, sid)
		if err != nil {
			return err
		}
		if sp.PeriodID != pid {
			return errNotFound
		}
		return nil
	}
	var allowed bool
	err = s.store.DB().QueryRow(r.Context(), `select exists(select 1 from memberships where user_id=$1 and period_id=$2 and removed_at is null)`, p.User.ID, pid).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return errForbidden
	}
	return nil
}

func (s *Server) streamChanges(w http.ResponseWriter, r *http.Request, p *principal) error {
	pid, err := id.Parse(r.URL.Query().Get("periodId"))
	if err != nil {
		return errNotFound
	}
	sid := id.Nil
	if value := r.URL.Query().Get("spaceId"); value != "" {
		sid, err = id.Parse(value)
		if err != nil {
			return errNotFound
		}
	}
	if err = s.realtimeAccess(r, pid, sid); err != nil {
		return err
	}
	ch, unsubscribe, ok := s.realtime.subscribe(pid)
	if !ok {
		return fail(http.StatusServiceUnavailable, "unavailable", "Pembaruan langsung sedang menyambungkan kembali.")
	}
	defer unsubscribe()
	controller := http.NewResponseController(w)
	// Replace the ordinary 60s response deadline for this stream. Each write is
	// bounded, so a slow/disconnected peer cannot retain the goroutine forever.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(event string) error {
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event); err != nil {
			return err
		}
		if err := controller.Flush(); err != nil {
			return err
		}
		_ = controller.SetWriteDeadline(time.Time{})
		return nil
	}
	if err = send("ready"); err != nil {
		return nil
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		event := "heartbeat"
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
		case <-ch:
			event = "change"
		}
		s.realtime.mu.Lock()
		ready := s.realtime.ready
		s.realtime.mu.Unlock()
		if !ready {
			return nil
		}
		if err = s.realtimeAccess(r, pid, sid); err != nil {
			return nil
		}
		if err = send(event); err != nil {
			return nil
		}
	}
}
