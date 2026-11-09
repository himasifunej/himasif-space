package appui

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type changesAPI interface {
	WatchChanges(context.Context, id.ID, id.ID, func(string)) error
}
type realtimeUIState struct {
	cancel                          context.CancelFunc
	period, space                   id.ID
	gen, seq                        int
	connected, dirty                bool
	retryAt, lastEvent, lastRefresh time.Time
	failures                        int
	activity                        []string
}

func (a *App) stopRealtime() {
	if a.realtime.cancel != nil {
		a.realtime.cancel()
	}
	seq := a.realtime.seq + 1
	a.realtime = realtimeUIState{seq: seq}
}
func (a *App) tickRealtime(c *ui.Context) {
	var service changesAPI
	if actual, ok := a.api.(*client.Client); ok {
		service = actual.AccountSession()
	} else {
		service, _ = a.api.(changesAPI)
	}
	if service == nil || a.isAdmin() {
		return
	}
	period, space := a.periodID, id.Nil
	if a.router.Path() == PathSpaces {
		space = a.selectedSpaceID()
	}
	if period == id.Nil || (a.router.Path() == PathSpaces && space == id.Nil) {
		a.stopRealtime()
		return
	}
	r := &a.realtime
	if r.period != period || r.space != space || r.gen != a.gen {
		a.stopRealtime()
		r = &a.realtime
		r.period = period
		r.space = space
		r.gen = a.gen
	}
	if r.cancel != nil && !r.lastEvent.IsZero() && c.Now().Sub(r.lastEvent) > 50*time.Second {
		r.cancel()
		r.cancel = nil
		r.seq++
		r.connected = false
		r.retryAt = c.Now().Add(3 * time.Second)
	}
	if r.cancel == nil && !c.Now().Before(r.retryAt) {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		r.lastEvent = c.Now()
		r.seq++
		seq, gen := r.seq, a.gen
		go func() {
			err := service.WatchChanges(ctx, period, space, func(event string) {
				a.update(func() {
					if ctx.Err() != nil || gen != a.gen || a.realtime.seq != seq {
						return
					}
					r := &a.realtime
					r.connected = true
					r.failures = 0
					r.lastEvent = time.Now()
					if event == "change" || event == "ready" {
						r.dirty = true
					}
				})
			})
			a.update(func() {
				if ctx.Err() != nil || gen != a.gen || a.realtime.seq != seq {
					return
				}
				r := &a.realtime
				r.cancel = nil
				r.connected = false
				r.failures++
				delay := min(time.Duration(1<<min(r.failures, 5))*time.Second, 30*time.Second)
				r.retryAt = time.Now().Add(delay)
				if client.IsUnauthenticated(err) {
					a.expire()
				}
			})
		}()
	}
	// A broken proxy or unavailable stream still refreshes. Never call this
	// healthy/realtime: the status text explicitly identifies the fallback.
	if !r.connected && (r.lastRefresh.IsZero() || c.Now().Sub(r.lastRefresh) >= 30*time.Second) {
		r.dirty = true
	}
	if r.dirty && !a.tasks.busy && !a.tasks.loading && !a.meetings.busy && !a.meetings.loading && c.Now().Sub(r.lastRefresh) >= 300*time.Millisecond {
		r.dirty = false
		r.lastRefresh = c.Now()
		if a.tasks.loaded {
			a.loadTasks()
		}
		// Comments/evidence arrive with the task snapshot; history and inbox
		// have separate caches. Refresh them without clearing typed replies.
		if a.tasks.historyLoaded && !a.tasks.historyBusy {
			a.tasks.historyLoaded = false
			if a.tasks.inspected != id.Nil {
				a.loadTaskHistory(a.tasks.inspected)
			}
		}
		if a.tasks.notificationsLoaded || a.chrome.notificationsOpen {
			a.loadTaskNotifications()
		}
		a.PollDeliveries()
		if a.meetings.loaded && a.meetings.space == space {
			a.loadMeetings()
		}
		if a.notes.dirty || a.notes.editing {
			// The detail loader keeps a dirty editor's baseline and content,
			// while updating comments, attachments and conflict information.
			if !a.notes.busy {
				a.loadNoteDetail(true)
			}
		} else if a.notes.loaded {
			a.loadNotes()
		}
		if space != id.Nil {
			a.chrome.members.loaded = false
			a.chrome.members.err = ""
		}
		r.activity = append([]string{c.Now().Format("15:04") + " · Memeriksa pembaruan ruang kerja"}, r.activity...)
		if len(r.activity) > 8 {
			r.activity = r.activity[:8]
		}
	}
	if r.dirty {
		c.After(max(time.Millisecond, 300*time.Millisecond-c.Now().Sub(r.lastRefresh)))
	}
	c.After(3 * time.Second)
}
