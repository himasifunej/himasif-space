package server

import (
	"context"
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// monitorOrg is the seeded organization with a Mediatek division at work
// and a BPH member who may be given org.monitor.
type monitorOrg struct {
	*taskOrg
	bph   string // the BPH member's session
	bphID id.ID
	task  api.Task
	note  api.Note
}

func newMonitorOrg(t *testing.T) *monitorOrg {
	o := newTaskOrg(t)
	bph, acc := o.join(o.admin, o.period.ID, "bph", "ketum@himasif.test", "Ketua Umum")
	m := &monitorOrg{taskOrg: o, bph: bph, bphID: acc.User.ID}
	m.task = o.create(false)
	if c := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/notes", o.kadiv,
		api.NoteCreateRequest{Title: "Notulen rapat divisi", Content: []api.NoteBlock{para("Keputusan rapat")}}, &m.note); c != 201 {
		t.Fatalf("create note = %d", c)
	}
	return m
}

func (o *monitorOrg) monitorsPath() string { return "/api/periods/" + o.period.ID.String() + "/monitors" }

func (o *monitorOrg) grant() api.MonitorGrant {
	o.t.Helper()
	var g api.MonitorGrant
	if c := o.call("POST", o.monitorsPath(), o.admin, api.MonitorGrantRequest{UserID: o.bphID}, &g); c != 201 {
		o.t.Fatalf("grant = %d", c)
	}
	return g
}

// refused fails unless the request is refused: 403, or 404 where the
// server hides that the object exists.
func (o *monitorOrg) refused(method, path, token string, body any) {
	o.t.Helper()
	if c := o.call(method, path, token, body, nil); c != 403 && c != 404 {
		o.t.Fatalf("%s %s = %d, want refused", method, path, c)
	}
}

func (o *monitorOrg) monitorAudits(action string, division id.ID) int {
	o.t.Helper()
	var n int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from audit_events where action = $1 and actor_user_id = $2 and scope_type = 'division' and scope_id = $3`,
		action, o.bphID, division).Scan(&n); err != nil {
		o.t.Fatal(err)
	}
	return n
}

// Only the admin grants, only to BPH, once per period; grant and revoke
// are logged; a non-monitor is refused everywhere.
func TestMonitorGrantIsAdminsAndBPHOnly(t *testing.T) {
	o := newMonitorOrg(t)
	mediatek := "/api/spaces/" + o.spaces["division-mediatek"].String()

	// Without a grant, a BPH member is a non-monitor: refused.
	o.refused("GET", mediatek, o.bph, nil)
	o.refused("GET", mediatek+"/tasks", o.bph, nil)
	o.refused("GET", taskPath(o.task), o.bph, nil)
	o.refused("GET", "/api/notes/"+o.note.ID.String(), o.bph, nil)
	if c := o.call("GET", o.scope(o.period.ID, "mediatek")+"/members", o.bph, nil, nil); c != 403 {
		t.Fatalf("members without grant = %d", c)
	}

	// Kadiv and members do not grant; nor read the grants or the log.
	for _, tok := range []string{o.kadiv, o.bph, o.member} {
		if c := o.call("POST", o.monitorsPath(), tok, api.MonitorGrantRequest{UserID: o.bphID}, nil); c != 403 {
			t.Fatalf("non-admin grant = %d", c)
		}
		if c := o.call("GET", o.monitorsPath(), tok, nil, nil); c != 403 {
			t.Fatalf("non-admin list = %d", c)
		}
		if c := o.call("GET", o.monitorsPath()+"/audit", tok, nil, nil); c != 403 {
			t.Fatalf("non-admin audit = %d", c)
		}
	}
	// Only BPH members.
	if c := o.call("POST", o.monitorsPath(), o.admin, api.MonitorGrantRequest{UserID: o.memberID}, nil); c != 400 {
		t.Fatalf("grant to a Mediatek member = %d", c)
	}
	g := o.grant()
	if c := o.call("POST", o.monitorsPath(), o.admin, api.MonitorGrantRequest{UserID: o.bphID}, nil); c != 409 {
		t.Fatalf("second grant = %d", c)
	}
	var list api.MonitorGrants
	if c := o.call("GET", o.monitorsPath(), o.admin, nil, &list); c != 200 || len(list.Grants) != 1 || len(list.Candidates) != 1 || list.Candidates[0].Grant == nil {
		t.Fatalf("grants = %d %+v", c, list)
	}
	var ps api.List[api.Period]
	o.call("GET", "/api/periods", o.bph, nil, &ps)
	if !ps.Items[0].Monitor {
		t.Fatalf("period does not say monitor: %+v", ps.Items[0])
	}
	if o.audits("monitor.grant") != 1 {
		t.Fatal("grant not audited")
	}

	// Revoke: access ends, and it is logged.
	if c := o.call("DELETE", o.monitorsPath()+"/"+g.ID.String(), o.kadiv, nil, nil); c != 403 {
		t.Fatalf("kadiv revoke = %d", c)
	}
	if c := o.call("GET", mediatek, o.bph, nil, nil); c != 200 {
		t.Fatalf("monitor opens mediatek = %d", c)
	}
	if c := o.call("DELETE", o.monitorsPath()+"/"+g.ID.String(), o.admin, nil, nil); c != 204 {
		t.Fatalf("revoke = %d", c)
	}
	o.refused("GET", mediatek, o.bph, nil)
	o.refused("GET", taskPath(o.task), o.bph, nil)
	if o.audits("monitor.revoke") != 1 {
		t.Fatal("revoke not audited")
	}
	var log api.List[api.MonitorAuditEvent]
	if c := o.call("GET", o.monitorsPath()+"/audit", o.admin, nil, &log); c != 200 || len(log.Items) < 3 {
		t.Fatalf("audit log = %d %+v", c, log.Items)
	}
	if !slices.ContainsFunc(log.Items, func(e api.MonitorAuditEvent) bool {
		return e.Action == "monitor.read" && e.DivisionName == "Mediatek" && e.ActorName == "Ketua Umum"
	}) {
		t.Fatalf("no read of Mediatek in the log: %+v", log.Items)
	}
}

// A monitor reads a division's work in full, read-only, and every read is
// audited; the division's own sidebar view is unchanged.
func TestMonitorReadsDivisionWorkAndIsAudited(t *testing.T) {
	o := newMonitorOrg(t)
	o.grant()
	sid := o.spaces["division-mediatek"]
	mediatek := o.div["mediatek"].ID

	var before api.List[api.Notification]
	o.call("GET", "/api/notifications", o.kadiv, nil, &before)
	var sp api.Space
	if c := o.call("GET", "/api/spaces/"+sid.String(), o.bph, nil, &sp); c != 200 || sp.Access != api.AccessMonitor {
		t.Fatalf("space = %d %+v", c, sp)
	}
	for _, c := range sp.Capabilities {
		if !slices.Contains([]string{api.CapSpaceRead, api.CapMembersRead, api.CapNotesRead, api.CapSOPRead}, c) {
			t.Fatalf("monitor holds %s", c)
		}
	}
	var tasks api.List[api.Task]
	if c := o.call("GET", "/api/spaces/"+sid.String()+"/tasks", o.bph, nil, &tasks); c != 200 || len(tasks.Items) != 1 {
		t.Fatalf("tasks = %d %+v", c, tasks.Items)
	}
	got := o.get(o.task, o.bph)
	if got.Description == "" || got.Permissions.Manage || got.Permissions.Comment || got.Permissions.Progress || got.Permissions.Help {
		t.Fatalf("monitor task view %+v", got)
	}
	var note api.Note
	if c := o.call("GET", "/api/notes/"+o.note.ID.String(), o.bph, nil, &note); c != 200 || len(note.Content) == 0 || note.Permissions.Manage {
		t.Fatalf("note = %d %+v", c, note)
	}
	// Writes stay closed.
	edit := o.taskRequest()
	edit.Title, edit.ExpectedVersion = "Diubah pemantau", got.Version
	o.refused("PATCH", taskPath(o.task), o.bph, edit)
	o.refused("POST", taskPath(o.task)+"/comments", o.bph, api.TaskCommentRequest{Body: "Komentar pemantau"})
	o.refused("POST", "/api/spaces/"+sid.String()+"/notes", o.bph, api.NoteCreateRequest{Title: "x", Content: []api.NoteBlock{para("y")}})
	o.refused("POST", "/api/spaces/"+sid.String()+"/emergency-snapshots", o.bph, map[string]any{})

	// The sidebar lists only the monitor's own spaces; the monitoring page
	// asks for the others.
	var own, all api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", o.bph, nil, &own)
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces?monitor=1", o.bph, nil, &all)
	if slices.ContainsFunc(own.Items, func(s api.Space) bool { return s.ID == sid }) || !slices.ContainsFunc(all.Items, func(s api.Space) bool { return s.ID == sid }) {
		t.Fatalf("own %d all %d", len(own.Items), len(all.Items))
	}
	// Every read was written to the log, with the division; the division's
	// members got no notification of it.
	if n := o.monitorAudits("monitor.read", mediatek); n < 4 {
		t.Fatalf("monitor reads audited = %d", n)
	}
	var after api.List[api.Notification]
	o.call("GET", "/api/notifications", o.kadiv, nil, &after)
	if len(after.Items) != len(before.Items) {
		t.Fatalf("division notified: %d → %d", len(before.Items), len(after.Items))
	}
}

// Personal reminders and drafts are never returned to a monitor.
func TestMonitorNeverSeesPrivateRemindersOrDrafts(t *testing.T) {
	o := newMonitorOrg(t)
	o.grant()
	d, _ := timeutil.ParseDate("2026-11-20")
	var rem api.Reminder
	if c := o.call("POST", "/api/reminders", o.kadiv, api.ReminderRequest{Title: "Reminder pribadi kadiv", DueKind: api.DueDate, DueOn: &d}, &rem); c != 201 {
		t.Fatalf("reminder = %d", c)
	}
	nid := o.note.ID.String()
	if c := o.call("POST", "/api/notes/"+nid+"/links", o.kadiv, api.NoteLinkRequest{TargetType: api.NoteLinkReminder, TargetID: rem.ID}, nil); c != 201 {
		t.Fatalf("link reminder = %d", c)
	}
	if c := o.call("PUT", "/api/notes/"+nid+"/draft", o.kadiv, api.NoteDraftRequest{Title: "Draf", Content: []api.NoteBlock{para("draf kadiv")}, BaseVersion: 1}, nil); c != 200 && c != 201 {
		t.Fatalf("draft = %d", c)
	}

	o.refused("GET", "/api/reminders/"+rem.ID.String(), o.bph, nil)
	o.refused("GET", "/api/reminders/"+rem.ID.String()+"/history", o.bph, nil)
	var mine api.List[api.Reminder]
	if c := o.call("GET", "/api/reminders", o.bph, nil, &mine); c != 200 || slices.ContainsFunc(mine.Items, func(r api.Reminder) bool { return r.ID == rem.ID }) {
		t.Fatalf("monitor lists kadiv's reminder: %d %+v", c, mine.Items)
	}
	var links api.List[api.NoteLink]
	if c := o.call("GET", "/api/notes/"+nid+"/links", o.bph, nil, &links); c != 200 {
		t.Fatalf("links = %d", c)
	}
	for _, l := range links.Items {
		if l.TargetType == api.NoteLinkReminder {
			t.Fatalf("monitor sees a private reminder link: %+v", l)
		}
	}
	if c := o.call("GET", "/api/notes/"+nid+"/draft", o.bph, nil, nil); c != 404 {
		t.Fatalf("monitor draft = %d", c)
	}
	var drafts api.List[api.NoteDraft]
	if c := o.call("GET", "/api/notes/"+nid+"/drafts", o.bph, nil, &drafts); c != 200 || len(drafts.Items) != 0 {
		t.Fatalf("monitor drafts = %d %+v", c, drafts.Items)
	}
}

// Closing the period revokes every grant (decision 6) and logs it.
func TestMonitorRevokedWhenPeriodCloses(t *testing.T) {
	o := newMonitorOrg(t)
	o.grant()
	sid := "/api/spaces/" + o.spaces["division-mediatek"].String()
	if c := o.call("GET", sid, o.bph, nil, nil); c != 200 {
		t.Fatalf("before close = %d", c)
	}
	if c := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); c != 200 {
		t.Fatalf("close = %d", c)
	}
	o.refused("GET", sid, o.bph, nil)
	o.refused("GET", taskPath(o.task), o.bph, nil)
	var active int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from monitor_grants where revoked_at is null`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("active grants after close = %d %v", active, err)
	}
	var reason string
	if err := o.st.DB().QueryRow(context.Background(), `select after ->> 'reason' from audit_events where action = 'monitor.revoke'`).Scan(&reason); err != nil || reason != api.MonitorRevokedClosed {
		t.Fatalf("revoke audit = %q %v", reason, err)
	}
	var list api.MonitorGrants
	o.call("GET", o.monitorsPath(), o.admin, nil, &list)
	if len(list.Grants) != 0 {
		t.Fatalf("grants after close %+v", list.Grants)
	}
	// A closed period takes no new grant.
	if c := o.call("POST", o.monitorsPath(), o.admin, api.MonitorGrantRequest{UserID: o.bphID}, nil); c != 409 {
		t.Fatalf("grant in closed period = %d", c)
	}
}
