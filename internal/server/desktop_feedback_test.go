package server

import (
	"context"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/store"
	"testing"
	"time"
)

func TestMeetingUnknownEndRoundTrip(t *testing.T) {
	o := newTaskOrg(t)
	req := api.MeetingRequest{Title: "Koordinasi sampai selesai", StartsAt: time.Now().UTC().Add(time.Hour).Truncate(time.Second), TimeZone: "Asia/Jakarta"}
	var m api.Meeting
	path := "/api/spaces/" + o.spaces["division-mediatek"].String() + "/meetings"
	if code := o.call("POST", path, o.kadiv, req, &m); code != 201 {
		t.Fatalf("create %d", code)
	}
	if m.EndsAt != nil {
		t.Fatal("invented meeting end")
	}
	path = "/api/meetings/" + m.ID.String()
	var read api.Meeting
	if code := o.call("GET", path, o.member, nil, &read); code != 200 || read.EndsAt != nil {
		t.Fatalf("read unknown end %d %+v", code, read)
	}
	end := req.StartsAt.Add(time.Hour)
	req.EndsAt = &end
	req.ExpectedVersion = m.Version
	if code := o.call("PUT", path, o.kadiv, req, &m); code != 200 || m.EndsAt == nil || !m.EndsAt.Equal(end) {
		t.Fatalf("set end %d %+v", code, m)
	}
	req.ExpectedVersion = m.Version
	req.EndsAt = nil
	if code := o.call("PUT", path, o.kadiv, req, &m); code != 200 || m.EndsAt != nil {
		t.Fatalf("clear end %d %+v", code, m)
	}
	req.ExpectedVersion = m.Version
	end = req.StartsAt
	req.EndsAt = &end
	if code := o.call("PUT", path, o.kadiv, req, nil); code != 400 {
		t.Fatalf("invalid end %d", code)
	}
}

func TestTaskPlannedStartIndependentOfPosting(t *testing.T) {
	o := newTaskOrg(t)
	req := o.taskRequest()
	req.PlannedStartDate = taskTestDate(t, "2026-11-02")
	var task api.Task
	path := "/api/spaces/" + o.spaces["division-mediatek"].String() + "/tasks"
	if code := o.call("POST", path, o.kadiv, req, &task); code != 201 {
		t.Fatalf("create %d", code)
	}
	read := o.get(task, o.member)
	if read.PlannedStartDate == nil || read.PlannedStartDate.String() != "2026-11-02" || read.PlannedStartAt != nil || read.PostingDate.String() != "2026-11-07" {
		t.Fatalf("date fields conflated %+v", read)
	}
	req.ExpectedVersion = read.Version
	req.PlannedStartDate = nil
	var cleared api.Task // Omitted JSON fields must not retain the previous decode target.
	if code := o.call("PATCH", taskPath(task), o.kadiv, req, &cleared); code != 200 || cleared.PlannedStartDate != nil {
		t.Fatalf("clear planned start %d %+v", code, cleared)
	}
	if persisted := o.get(cleared, o.member); persisted.PlannedStartDate != nil || persisted.PostingDate.String() != "2026-11-07" {
		t.Fatalf("cleared start did not persist independently: %+v", persisted)
	}
	req.PlannedStartDate = taskTestDate(t, "2026-11-06")
	if code := o.call("POST", path, o.kadiv, req, nil); code != 400 {
		t.Fatalf("reversed range %d", code)
	}
	// 23:30 WIB on the deadline day is valid, despite a different UTC date.
	req.PlannedStartDate = nil
	at := time.Date(2026, 11, 5, 16, 30, 0, 0, time.UTC)
	req.PlannedStartAt = &at
	if code := o.call("POST", path, o.kadiv, req, &task); code != 201 || task.PlannedStartAt == nil || !task.PlannedStartAt.Equal(at) {
		t.Fatalf("mixed range %d %+v", code, task)
	}
}

func TestRealtimeCommittedChangeAndRevokedAccess(t *testing.T) {
	for _, revokeSession := range []bool{false, true} {
		name := "membership"
		if revokeSession {
			name = "session"
		}
		t.Run(name, func(t *testing.T) { realtimeCommittedChangeAndRevokedAccess(t, revokeSession) })
	}
}

func realtimeCommittedChangeAndRevokedAccess(t *testing.T, revokeSession bool) {
	o := newTaskOrg(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); o.srv.RunRealtime(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(5 * time.Second)
	for {
		o.srv.realtime.mu.Lock()
		ready := o.srv.realtime.ready
		o.srv.realtime.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("listener never became ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	path := "/api/realtime?periodId=" + o.period.ID.String() + "&spaceId=" + o.spaces["division-mediatek"].String()
	if code := o.call("GET", path, o.peer, nil, nil); code != 403 && code != 404 {
		t.Fatalf("foreign stream %d", code)
	}
	if code := o.call("GET", path, "", nil, nil); code != 401 {
		t.Fatalf("anonymous stream %d", code)
	}
	c := client.New(o.url)
	c.SetToken(o.member)
	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()
	ready, changed := make(chan struct{}, 8), make(chan struct{}, 8)
	closed := make(chan error, 1)
	go func() {
		closed <- c.WatchChanges(wctx, o.period.ID, o.spaces["division-mediatek"], func(event string) {
			switch event {
			case "ready":
				ready <- struct{}{}
			case "change":
				changed <- struct{}{}
			}
		})
	}()
	select {
	case <-ready:
	case err := <-closed:
		t.Fatalf("stream rejected %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("stream not flushed")
	}
	task := o.create(false)
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("committed task did not invalidate stream")
	}
	// A rolled-back write must not notify remote viewers.
	rollback := errors.New("rollback fixture")
	err := o.st.Tx(ctx, func(q store.Querier) error {
		_, err := q.Exec(ctx, `update tasks set title='rolled back' where id=$1`, task.ID)
		if err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	select {
	case <-changed:
		t.Fatal("rolled back task emitted change")
	case <-time.After(150 * time.Millisecond):
	}
	// Comments do not bump the parent task. They still need to wake viewers.
	if code := o.call("POST", taskPath(task)+"/comments", o.member, api.TaskCommentRequest{Body: "Komentar dari perangkat lain"}, nil); code != 201 {
		t.Fatalf("comment %d", code)
	}
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("committed comment did not invalidate stream")
	}
	if len(o.get(task, o.member).Comments) != 1 {
		t.Fatal("comment missing from refreshed task snapshot")
	}
	if revokeSession {
		_, err = o.st.DB().Exec(ctx, `update sessions set revoked_at=now() where user_id=$1 and revoked_at is null`, o.memberID)
	} else {
		_, err = o.st.DB().Exec(ctx, `update memberships set removed_at=now() where user_id=$1 and period_id=$2`, o.memberID, o.period.ID)
	}
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("revoked membership retained stream")
	}
}
