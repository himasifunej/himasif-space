package server

import (
	"context"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// MON-05: a monitor approves a submission where a kadiv could and pings the
// kadiv, never the members; both are logged as the monitor's. Without the
// grant, neither.
func TestMonitorActions(t *testing.T) {
	o := newMonitorOrg(t)
	path := taskPath(o.task)
	var task api.Task
	if c := o.call("POST", path+"/move", o.member, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: o.task.Version}, &task); c != 200 {
		t.Fatalf("move = %d", c)
	}
	var ev api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{URL: "https://drive.google.com/file/d/hasil", Note: "Hasil", ExpectedVersion: task.Version}, &ev); c != 201 {
		t.Fatalf("evidence = %d", c)
	}
	task = o.get(task, o.member)
	review := api.TaskEvidenceReviewRequest{Matched: true, Reason: "Cocok", ExpectedVersion: task.Version}
	sid := o.spaces["division-mediatek"]
	kadivID := o.kadivID(t)
	ping := func(to id.ID) api.PingRequest {
		return api.PingRequest{Message: "Mohon cek tugas yang terlambat.", Recipients: []id.ID{to}, Channels: []string{"device"}, SourceKind: "task", SourceID: &o.task.ID}
	}

	// A non-monitor is refused both.
	o.refused("POST", path+"/evidence/"+ev.ID.String()+"/review", o.bph, review)
	o.refused("POST", "/api/spaces/"+sid.String()+"/pings", o.bph, ping(kadivID))

	o.grant()
	got := o.get(task, o.bph)
	if !got.Permissions.Review || got.Permissions.SelfReview || got.Permissions.Manage {
		t.Fatalf("monitor permissions %+v", got.Permissions)
	}
	// Pings go to the kadiv only.
	if c, code := o.errCode("POST", "/api/spaces/"+sid.String()+"/pings", o.bph, ping(o.memberID)); c != 403 || code != api.CodeForbidden {
		t.Fatalf("monitor pings a member = %d %s", c, code)
	}
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/pings", o.bph, ping(kadivID), nil); c != 201 {
		t.Fatalf("monitor pings the kadiv = %d", c)
	}
	if c := o.call("POST", path+"/evidence/"+ev.ID.String()+"/review", o.bph, review, &ev); c != 200 || ev.MatchStatus != api.MatchVerified {
		t.Fatalf("monitor approves = %d %+v", c, ev)
	}
	if got := o.get(task, o.member); got.Progress != api.TaskDone {
		t.Fatalf("task after approval %+v", got)
	}
	// Both actions are the monitor's in the log, with the division.
	var acts int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from audit_events where action = 'monitor.act' and actor_user_id = $1 and scope_id = $2
		and after ->> 'action' in ('ping.created', 'task.evidence.review', 'task.evidence.reviewed', 'task.review')`, o.bphID, o.div["mediatek"].ID).Scan(&acts); err != nil {
		t.Fatal(err)
	}
	var all int
	o.st.DB().QueryRow(context.Background(), `select count(*) from audit_events where action = 'monitor.act' and actor_user_id = $1 and scope_id = $2`, o.bphID, o.div["mediatek"].ID).Scan(&all)
	if all < 2 {
		t.Fatalf("monitor actions logged = %d (named %d)", all, acts)
	}
	// Still no comment, no management.
	o.refused("POST", path+"/comments", o.bph, api.TaskCommentRequest{Body: "x"})
}

// A monitor's ping never goes to the division's WhatsApp group.
func TestMonitorPingIsDeviceOnly(t *testing.T) {
	o := newMonitorOrg(t)
	o.grant()
	kadivID := o.kadivID(t)
	sid := o.spaces["division-mediatek"]
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/pings", o.bph, api.PingRequest{Message: "Cek progres", Recipients: []id.ID{kadivID}, Channels: []string{"device", "whatsapp"}}, nil); c != 201 {
		t.Fatalf("ping = %d", c)
	}
	var wa int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from delivery_recipients d join delivery_events e on e.id = d.event_id
		where e.source_kind = 'ping' and e.space_id = $1 and e.body = 'Cek progres' and d.channel = 'whatsapp'`, sid).Scan(&wa); err != nil || wa != 0 {
		t.Fatalf("whatsapp recipients = %d %v", wa, err)
	}
	var ov api.MonitorOverview
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/monitor", o.bph, nil, &ov)
	for _, s := range ov.Spaces {
		if s.SpaceID == sid && (len(s.Kadiv) != 1 || s.Kadiv[0].ID != kadivID) {
			t.Fatalf("dashboard kadiv %+v", s.Kadiv)
		}
	}
}
