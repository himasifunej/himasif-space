package server

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func taskTestDate(t *testing.T, s string) *timeutil.Date {
	t.Helper()
	d, err := timeutil.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}
func TestTaskMetadataValidation(t *testing.T) {
	valid := func() api.TaskWriteRequest {
		return api.TaskWriteRequest{OwnerDivisionID: id.New(), Title: "Poster", ExpectedOutput: "Tautan desain", AssigneeID: id.New(), DeadlineDate: taskTestDate(t, "2026-11-05")}
	}
	for _, tc := range []struct {
		name   string
		change func(*api.TaskWriteRequest)
	}{
		{"no PJ", func(r *api.TaskWriteRequest) { r.AssigneeID = id.Nil }},
		{"no owner", func(r *api.TaskWriteRequest) { r.OwnerDivisionID = id.Nil }},
		{"no title", func(r *api.TaskWriteRequest) { r.Title = "  " }},
		{"no output", func(r *api.TaskWriteRequest) { r.ExpectedOutput = "" }},
		{"no deadline", func(r *api.TaskWriteRequest) { r.DeadlineDate = nil }},
		{"both deadlines", func(r *api.TaskWriteRequest) { at := time.Now(); r.DeadlineAt = &at }},
		{"invalid date", func(r *api.TaskWriteRequest) { r.DeadlineDate = &timeutil.Date{Year: 2026, Month: 2, Day: 30} }},
		{"both posting fields", func(r *api.TaskWriteRequest) { r.PostingDate = r.DeadlineDate; at := time.Now(); r.PostingAt = &at }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.change(&r)
			if err := validateTaskWrite(&r); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
	r := valid()
	if err := validateTaskWrite(&r); err != nil {
		t.Fatal(err)
	}
	if r.DeadlineAt != nil || r.DeadlineDate.String() != "2026-11-05" {
		t.Fatal("date-only got an invented time")
	}
	at := time.Date(2026, 11, 5, 10, 30, 0, 0, time.FixedZone("WIB", 7*3600))
	r.DeadlineDate = nil
	r.DeadlineAt = &at
	if err := validateTaskWrite(&r); err != nil {
		t.Fatal(err)
	}
	if r.DeadlineAt.Location() != time.UTC || r.DeadlineAt.Hour() != 3 {
		t.Fatal("instant not normalized to UTC")
	}
}

func TestTaskFilterPeriodTimezoneAndVersion(t *testing.T) {
	at := time.Date(2026, 11, 4, 19, 30, 0, 0, time.UTC)
	task := store.TaskRow{Task: api.Task{DeadlineAt: &at, TimeZone: "Asia/Jakarta", Version: 3}}
	before := taskFilter{due: taskTestDate(t, "2026-11-05")}
	if before.matches(&task) {
		t.Fatal("UTC date used instead of period calendar date")
	}
	before.due = taskTestDate(t, "2026-11-06")
	if !before.matches(&task) {
		t.Fatal("dueBefore excluded earlier local date")
	}
	task.DeadlineAt = nil
	task.DeadlineDate = taskTestDate(t, "2026-11-05")
	if !before.matches(&task) {
		t.Fatal("date-only filter failed")
	}
	if taskVersion(&task, 0) == nil || taskVersion(&task, 2) == nil || taskVersion(&task, 3) != nil {
		t.Fatal("optimistic version check failed")
	}
	for _, q := range []string{"?dueBefore=tomorrow", "?assigneeId=nope", "?progress=revisions"} {
		if _, err := parseTaskFilter(httptest.NewRequest("GET", "/"+q, nil)); err == nil {
			t.Fatalf("invalid query accepted %s", q)
		}
	}
}

type taskOrg struct {
	*org
	kadiv, member, peer string
	memberID, peerID    id.ID
	spaces              map[string]id.ID
}

func newTaskOrg(t *testing.T) *taskOrg {
	o := newOrg(t)
	k, _ := o.kadiv("mediatek", "task-kadiv@himasif.test")
	m, ma := o.join(k, o.period.ID, "mediatek", "task-member@himasif.test", "PJ Mediatek")
	peer, pa := o.join(o.admin, o.period.ID, "humas", "task-peer@himasif.test", "Anggota Humas")
	var spaces api.List[api.Space]
	if c := o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", o.admin, nil, &spaces); c != 200 {
		t.Fatal(c)
	}
	out := &taskOrg{org: o, kadiv: k, member: m, peer: peer, memberID: ma.User.ID, peerID: pa.User.ID, spaces: map[string]id.ID{}}
	for _, sp := range spaces.Items {
		out.spaces[sp.Code] = sp.ID
	}
	return out
}
func (o *taskOrg) taskRequest() api.TaskWriteRequest {
	return api.TaskWriteRequest{OwnerDivisionID: o.div["mediatek"].ID, Title: "Poster kegiatan", Description: "Dari rapat divisi", ExpectedOutput: "Tautan Drive desain final", Category: "Desain", AssigneeID: o.memberID, DeadlineDate: taskTestDate(o.t, "2026-11-05"), PostingDate: taskTestDate(o.t, "2026-11-07")}
}
func (o *taskOrg) create(shared bool) api.Task {
	o.t.Helper()
	source := "division-mediatek"
	if shared {
		source = "medhum"
	}
	var task api.Task
	if c := o.call("POST", "/api/spaces/"+o.spaces[source].String()+"/tasks", o.kadiv, o.taskRequest(), &task); c != 201 {
		o.t.Fatalf("create task = %d %+v", c, task)
	}
	return task
}
func taskPath(t api.Task) string { return "/api/tasks/" + t.ID.String() }
func (o *taskOrg) get(task api.Task, token string) api.Task {
	o.t.Helper()
	var out api.Task
	if c := o.call("GET", taskPath(task), token, nil, &out); c != 200 {
		o.t.Fatalf("get task = %d", c)
	}
	return out
}
func (o *taskOrg) wantStatus(want int, method, path, token string, body any) {
	o.t.Helper()
	if c := o.call(method, path, token, body, nil); c != want {
		o.t.Fatalf("%s %s = %d want %d", method, path, c, want)
	}
}

func TestTasksCanonicalSpacesOwnerPermissionsAndHelp(t *testing.T) {
	o := newTaskOrg(t)
	internal := o.create(false)
	shared := o.create(true)
	if len(internal.SpaceIDs) != 1 || len(shared.SpaceIDs) != 2 {
		t.Fatalf("task links internal=%v shared=%v", internal.SpaceIDs, shared.SpaceIDs)
	}
	o.wantStatus(403, "GET", taskPath(internal), o.peer, nil)
	var div, medhum, mine api.List[api.Task]
	o.call("GET", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/tasks", o.member, nil, &div)
	o.call("GET", "/api/spaces/"+o.spaces["medhum"].String()+"/tasks", o.peer, nil, &medhum)
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/tasks/mine", o.member, nil, &mine)
	if len(div.Items) != 2 || len(medhum.Items) != 1 || len(mine.Items) != 2 || medhum.Items[0].ID != shared.ID {
		t.Fatalf("noncanonical projections div=%+v medhum=%+v mine=%+v", div, medhum, mine)
	}
	for _, list := range [][]api.Task{div.Items, mine.Items} {
		i := slices.IndexFunc(list, func(x api.Task) bool { return x.ID == shared.ID })
		if i < 0 || list[i].Version != shared.Version || list[i].Assignee.ID != shared.Assignee.ID || list[i].DeadlineDate.String() != shared.DeadlineDate.String() {
			t.Fatal("canonical metadata disagrees")
		}
	}
	peerView := o.get(shared, o.peer)
	if peerView.Permissions.Manage || peerView.Permissions.Progress || !peerView.Permissions.Comment || !peerView.Permissions.Help {
		t.Fatalf("peer permissions %+v", peerView.Permissions)
	}
	adminView := o.get(shared, o.admin)
	if adminView.Permissions != (api.TaskPermissions{}) {
		t.Fatalf("admin got task work rights %+v", adminView.Permissions)
	}
	humkadiv, _ := o.org.kadiv("humas", "task-humas-kadiv@himasif.test")
	for _, tok := range []string{o.peer, o.admin, humkadiv} {
		o.wantStatus(403, "POST", taskPath(shared)+"/move", tok, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: 1})
		o.wantStatus(403, "PATCH", taskPath(shared), tok, o.taskRequest())
	}
	var comment api.TaskComment
	if c := o.call("POST", taskPath(shared)+"/comments", o.peer, api.TaskCommentRequest{Body: "Saya bisa bantu."}, &comment); c != 201 || comment.Author.ID != o.peerID {
		t.Fatalf("comment %d %+v", c, comment)
	}
	var help api.TaskHelpRequest
	if c := o.call("POST", taskPath(shared)+"/help", o.peer, api.TaskHelpCreateRequest{Message: "Bantu ilustrasi"}, &help); c != 201 {
		t.Fatalf("help = %d", c)
	}
	o.wantStatus(409, "POST", taskPath(shared)+"/help", o.peer, api.TaskHelpCreateRequest{})
	o.wantStatus(403, "POST", taskPath(shared)+"/help/"+help.ID.String()+"/decide", humkadiv, api.TaskHelpDecisionRequest{Approved: true, ExpectedVersion: 1})
	if c := o.call("POST", taskPath(shared)+"/help/"+help.ID.String()+"/decide", o.kadiv, api.TaskHelpDecisionRequest{Approved: true, ExpectedVersion: 1}, &help); c != 200 || help.Status != "approved" {
		t.Fatalf("decision %d %+v", c, help)
	}
	got := o.get(shared, o.peer)
	if got.Assignee.ID != o.memberID || len(got.Participants) != 1 || got.Participants[0].ID != o.peerID || !got.Permissions.Progress || got.Permissions.Help {
		t.Fatalf("help replaced PJ or failed rights %+v", got)
	}
	var moved api.Task
	if c := o.call("POST", taskPath(shared)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: got.Version}, &moved); c != 200 || moved.Version != 3 {
		t.Fatalf("helper move %d %+v", c, moved)
	}
	// Membership revocation defeats participant IDs, direct links, personal lists,
	// notifications and history alike.
	o.wantStatus(204, "DELETE", o.scope(o.period.ID, "humas")+"/members/"+o.peerID.String(), humkadiv, nil)
	for _, suffix := range []string{"", "/history"} {
		o.wantStatus(403, "GET", taskPath(shared)+suffix, o.peer, nil)
	}
	o.wantStatus(403, "POST", taskPath(shared)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskTodo, ExpectedVersion: 3})
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/tasks/mine", o.peer, nil, &mine)
	if len(mine.Items) != 0 {
		t.Fatal("revoked helper still sees mine")
	}
	var ns api.List[api.Notification]
	o.call("GET", "/api/notifications", o.peer, nil, &ns)
	if len(ns.Items) != 0 {
		t.Fatal("revoked helper notification leaked")
	}
}

func TestTasksEvidenceMatchingHistoryAndVersions(t *testing.T) {
	o := newTaskOrg(t)
	task := o.create(true)
	path := taskPath(task)
	// Neither the PJ nor an owner manager may bypass matching through the
	// generic drag/progress endpoint, including tasks with no evidence yet.
	for _, token := range []string{o.member, o.kadiv} {
		o.wantStatus(409, "POST", path+"/move", token, api.TaskMoveRequest{Progress: api.TaskDone, ExpectedVersion: 1})
	}
	o.wantStatus(400, "POST", path+"/move", o.member, api.TaskMoveRequest{Progress: api.TaskSubmitted, ExpectedVersion: 1})
	o.wantStatus(400, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{URL: "file:///etc/passwd", ExpectedVersion: 1})
	o.wantStatus(400, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{ExpectedVersion: 1})
	o.wantStatus(403, "POST", path+"/evidence", o.peer, api.TaskEvidenceRequest{Note: "Tidak punya tugas", ExpectedVersion: 1})
	if c := o.call("POST", path+"/move", o.member, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: 1}, &task); c != 200 {
		t.Fatal(c)
	}
	var evidence api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{URL: "https://drive.google.com/file/d/example", Note: "Hasil desain", ExpectedVersion: task.Version}, &evidence); c != 201 || evidence.MatchStatus != api.MatchPending {
		t.Fatalf("submit %d %+v", c, evidence)
	}
	originalTime := evidence.SubmittedAt
	task = o.get(task, o.member)
	if task.Progress != api.TaskSubmitted || task.MatchStatus != api.MatchPending || task.Version != 3 {
		t.Fatalf("pending %+v", task)
	}
	o.wantStatus(409, "POST", path+"/move", o.member, api.TaskMoveRequest{Progress: api.TaskDone, ExpectedVersion: task.Version})
	o.wantStatus(409, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{Note: "Duplicate", ExpectedVersion: task.Version})
	o.wantStatus(403, "POST", path+"/evidence/"+evidence.ID.String()+"/review", o.member, api.TaskEvidenceReviewRequest{Matched: true, Reason: "Cocok", ExpectedVersion: task.Version})
	if c := o.call("POST", path+"/evidence/"+evidence.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: false, Reason: "Tautan pekerjaan berbeda", ExpectedVersion: task.Version}, &evidence); c != 200 || !evidence.SubmittedAt.Equal(originalTime) || evidence.MatchStatus != api.MatchRejected {
		t.Fatalf("reject %d %+v", c, evidence)
	}
	task = o.get(task, o.member)
	if task.Progress != api.TaskDoing || task.MatchStatus != api.MatchRejected {
		t.Fatalf("rejection did not restore work %+v", task)
	}
	o.wantStatus(409, "POST", path+"/move", o.kadiv, api.TaskMoveRequest{Progress: api.TaskDone, ExpectedVersion: task.Version})
	var second api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{Note: "Hasil yang benar", ExpectedVersion: task.Version}, &second); c != 201 || second.ID == evidence.ID {
		t.Fatalf("resubmit %d %+v", c, second)
	}
	task = o.get(task, o.member)
	if len(task.Evidence) != 2 || task.Evidence[0].ID != evidence.ID || !task.Evidence[0].SubmittedAt.Equal(originalTime) {
		t.Fatal("original evidence was overwritten")
	}
	o.wantStatus(409, "POST", path+"/evidence/"+evidence.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: true, Reason: "Bukti lama", ExpectedVersion: task.Version})
	if c := o.call("POST", path+"/evidence/"+second.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: true, Reason: "PJ dan pekerjaan cocok", ExpectedVersion: task.Version}, &second); c != 200 {
		t.Fatal(c)
	}
	task = o.get(task, o.member)
	if task.Progress != api.TaskDone || task.MatchStatus != api.MatchVerified {
		t.Fatalf("verified %+v", task)
	}
	o.wantStatus(403, "POST", path+"/move", o.member, api.TaskMoveRequest{Progress: api.TaskDoing, Reason: "Coba", ExpectedVersion: task.Version})
	o.wantStatus(400, "POST", path+"/move", o.kadiv, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: task.Version})
	if c := o.call("POST", path+"/move", o.kadiv, api.TaskMoveRequest{Progress: api.TaskDoing, Reason: "Koreksi pencocokan", ExpectedVersion: task.Version}, &task); c != 200 || task.MatchStatus != api.MatchNone {
		t.Fatalf("reopen %d %+v", c, task)
	}
	o.wantStatus(409, "POST", path+"/move", o.kadiv, api.TaskMoveRequest{Progress: api.TaskTodo, ExpectedVersion: 1})
	var history api.List[api.TaskEvent]
	if c := o.call("GET", path+"/history", o.member, nil, &history); c != 200 || len(history.Items) != 7 {
		t.Fatalf("history %d length=%d %+v", c, len(history.Items), history.Items)
	}
	var eventCount, auditCount int
	if err := o.st.DB().QueryRow(context.Background(), `select (select count(*) from task_events where task_id=$1),(select count(*) from audit_events where target_type='task' and target_id=$1)`, task.ID).Scan(&eventCount, &auditCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != auditCount {
		t.Fatalf("history/audit not transactional %d/%d", eventCount, auditCount)
	}
	if _, err := o.st.DB().Exec(context.Background(), `update task_events set reason='tamper' where task_id=$1`, task.ID); err == nil {
		t.Fatal("history update was permitted")
	}
	if _, err := o.st.DB().Exec(context.Background(), `delete from task_events where task_id=$1`, task.ID); err == nil {
		t.Fatal("history deletion was permitted")
	}
}

func TestTasksMetadataNotificationsAndClosedPeriod(t *testing.T) {
	o := newTaskOrg(t)
	task := o.create(false)
	path := taskPath(task)
	req := o.taskRequest()
	req.ExpectedVersion = 1
	req.DeadlineDate = taskTestDate(t, "2026-11-06")
	o.wantStatus(400, "PATCH", path, o.kadiv, req)
	req.Reason = "Penyesuaian rapat"
	if c := o.call("PATCH", path, o.kadiv, req, &task); c != 200 || task.Version != 2 || task.DeadlineDate.String() != "2026-11-06" || task.DeadlineAt != nil {
		t.Fatalf("metadata update %d %+v", c, task)
	}
	bad := req
	bad.ExpectedVersion = task.Version
	bad.AssigneeID = o.peerID
	o.wantStatus(400, "PATCH", path, o.kadiv, bad)
	bad = req
	bad.ExpectedVersion = task.Version
	bad.SpaceIDs = []id.ID{o.spaces["division-humas"]}
	o.wantStatus(400, "PATCH", path, o.kadiv, bad)
	bad = req
	bad.ExpectedVersion = task.Version
	bad.OwnerDivisionID = o.div["humas"].ID
	o.wantStatus(400, "PATCH", path, o.kadiv, bad)
	var ns api.List[api.Notification]
	if c := o.call("GET", "/api/notifications", o.member, nil, &ns); c != 200 || len(ns.Items) != 2 {
		t.Fatalf("notifications %d %+v", c, ns)
	}
	n := ns.Items[0]
	o.wantStatus(404, "POST", "/api/notifications/"+n.ID.String()+"/read", o.peer, nil)
	o.wantStatus(204, "POST", "/api/notifications/"+n.ID.String()+"/read", o.member, nil)
	if o.audits("notification.read") != 1 {
		t.Fatal("notification receipt was not audited")
	}
	o.call("GET", "/api/notifications", o.member, nil, &ns)
	if ns.Items[0].ReadAt == nil {
		t.Fatal("read receipt not persisted")
	}
	var tasks api.List[api.Task]
	o.call("GET", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/tasks?dueBefore=2026-11-07&progress=todo&assigneeId="+o.memberID.String(), o.member, nil, &tasks)
	if len(tasks.Items) != 1 || tasks.Items[0].ID != task.ID {
		t.Fatal("task filters failed")
	}
	o.wantStatus(200, "POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil)
	task = o.get(task, o.member)
	if task.Permissions != (api.TaskPermissions{}) {
		t.Fatalf("closed task work rights %+v", task.Permissions)
	}
	for _, tc := range []struct {
		suffix string
		body   any
	}{{"/move", api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: task.Version}}, {"/comments", api.TaskCommentRequest{Body: "x"}}, {"/help", api.TaskHelpCreateRequest{}}, {"/evidence", api.TaskEvidenceRequest{Note: "x", ExpectedVersion: task.Version}}} {
		if c, code := o.errCode("POST", path+tc.suffix, o.member, tc.body); c != 409 || code != api.CodePeriodClosed {
			t.Fatalf("closed %s = %d %s", tc.suffix, c, code)
		}
	}
	// Read receipts remain possible without mutating the closed task itself.
	o.wantStatus(204, "POST", "/api/notifications/"+n.ID.String()+"/read", o.member, nil)
}

func TestTasksOrderingRejectsHiddenTargetsAndStaleMoves(t *testing.T) {
	o := newTaskOrg(t)
	a, b, c := o.create(true), o.create(true), o.create(true)
	hidden := o.create(false)
	var help api.TaskHelpRequest
	o.call("POST", taskPath(c)+"/help", o.peer, api.TaskHelpCreateRequest{}, &help)
	o.wantStatus(200, "POST", taskPath(c)+"/help/"+help.ID.String()+"/decide", o.kadiv, api.TaskHelpDecisionRequest{Approved: true, ExpectedVersion: c.Version})
	c = o.get(c, o.peer)
	var ranksBefore, ranksAfter []int64
	readRanks := func() []int64 {
		ids := []id.ID{a.ID, b.ID, c.ID, hidden.ID}
		out := []int64{}
		for _, tid := range ids {
			var rank int64
			if err := o.st.DB().QueryRow(context.Background(), `select rank from tasks where id=$1`, tid).Scan(&rank); err != nil {
				t.Fatal(err)
			}
			out = append(out, rank)
		}
		return out
	}
	ranksBefore = readRanks()
	o.wantStatus(403, "POST", taskPath(c)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskTodo, BeforeID: hidden.ID, ExpectedVersion: c.Version})
	ranksAfter = readRanks()
	if !slices.Equal(ranksBefore, ranksAfter) {
		t.Fatal("rejected reorder changed task ranks")
	}
	o.wantStatus(400, "POST", taskPath(c)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskDoing, BeforeID: b.ID, ExpectedVersion: c.Version})
	o.wantStatus(200, "POST", taskPath(c)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskTodo, BeforeID: a.ID, ExpectedVersion: c.Version})
	c = o.get(c, o.peer)
	var list api.List[api.Task]
	o.call("GET", "/api/spaces/"+o.spaces["medhum"].String()+"/tasks", o.member, nil, &list)
	if len(list.Items) != 3 || list.Items[0].ID != c.ID || list.Items[1].ID != a.ID || list.Items[2].ID != b.ID {
		t.Fatalf("order not persisted %+v", list.Items)
	}
	if o.get(a, o.member).Version != a.Version || o.get(b, o.member).Version != b.Version {
		t.Fatal("rank maintenance changed unrelated task versions")
	}
	// Two requests with the same version race: precisely one must win.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	one := accessCallAsync(ctx, o.env, "POST", taskPath(c)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: c.Version})
	two := accessCallAsync(ctx, o.env, "POST", taskPath(c)+"/move", o.peer, api.TaskMoveRequest{Progress: api.TaskTodo, ExpectedVersion: c.Version})
	statuses := []int{}
	for _, ch := range []<-chan accessHTTPResult{one, two} {
		select {
		case res := <-ch:
			if res.err != nil {
				t.Fatal(res.err)
			}
			statuses = append(statuses, res.status)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{200, 409}) {
		t.Fatalf("concurrent lost update %v", statuses)
	}
}

func TestTasksArchiveExactSpaceAndRevokedNotifications(t *testing.T) {
	o := newTaskOrg(t)
	internal, shared := o.create(false), o.create(true)
	// Seed a task delivery for a future-period member. It must not disclose a
	// task until the precise linked archived space is granted.
	var next api.Period
	o.wantStatus(201, "POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2027/2028", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365)})
	var ps api.List[api.Period]
	o.call("GET", "/api/periods", o.admin, nil, &ps)
	for _, p := range ps.Items {
		if p.ID != o.period.ID {
			next = p
			break
		}
	}
	reader, accepted := o.join(o.admin, next.ID, "humas", "archive-task@himasif.test", "Arsip")
	for _, task := range []api.Task{internal, shared} {
		if err := o.st.AddTaskNotification(context.Background(), o.st.DB(), accepted.User.ID, api.Notification{ID: id.New(), TaskID: task.ID, Title: task.Title, Body: "Rahasia tugas", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	o.wantStatus(200, "POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil)
	grantPath := o.scope(next.ID, "humas") + "/archives"
	var grant api.ArchiveGrant
	if c := o.call("POST", grantPath, o.admin, api.ArchiveRequest{UserID: accepted.User.ID, SpaceID: o.spaces["division-humas"]}, &grant); c != 201 {
		t.Fatal(c)
	}
	o.wantStatus(403, "GET", taskPath(shared), reader, nil)
	var ns api.List[api.Notification]
	o.call("GET", "/api/notifications", reader, nil, &ns)
	if len(ns.Items) != 0 {
		t.Fatal("wrong archive grant exposed notifications")
	}
	if c := o.call("POST", grantPath, o.admin, api.ArchiveRequest{UserID: accepted.User.ID, SpaceID: o.spaces["medhum"]}, &grant); c != 201 {
		t.Fatal(c)
	}
	task := o.get(shared, reader)
	if task.Permissions != (api.TaskPermissions{}) {
		t.Fatal("archive gave work rights")
	}
	o.wantStatus(403, "GET", taskPath(internal), reader, nil)
	o.call("GET", "/api/notifications", reader, nil, &ns)
	if len(ns.Items) != 1 || ns.Items[0].TaskID != shared.ID {
		t.Fatalf("archive notification filtering %+v", ns)
	}
	nid := ns.Items[0].ID
	o.wantStatus(204, "DELETE", grantPath+"/"+grant.ID.String(), o.admin, nil)
	o.wantStatus(403, "GET", taskPath(shared)+"/history", reader, nil)
	o.wantStatus(403, "POST", "/api/notifications/"+nid.String()+"/read", reader, nil)
	o.call("GET", "/api/notifications", reader, nil, &ns)
	if len(ns.Items) != 0 {
		t.Fatal("revoked archive delivery still visible")
	}
}

func TestTasksWriteRechecksAuthorityAfterPeriodLock(t *testing.T) {
	o := newTaskOrg(t)
	task := o.create(false)
	var appointment api.Appointment45
	if c := o.call("PUT", o.scope(o.period.ID, "mediatek")+"/45", o.kadiv, api.Appoint45Request{UserID: o.memberID}, &appointment); c != 201 {
		t.Fatal(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gate, err := regressionConnection(ctx, o.st)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close(context.Background())
	tx, err := gate.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `select id from periods where id=$1 for update`, o.period.ID); err != nil {
		t.Fatal(err)
	}
	req := o.taskRequest()
	req.ExpectedVersion = task.Version
	req.Title = "Queued before 45 revocation"
	done := accessCallAsync(ctx, o.env, "PATCH", taskPath(task), o.member, req)
	waitAccessQueryBlocked(ctx, t, o.st, "from periods where id = $1", "", done)
	if _, err = tx.Exec(ctx, `update acting45 set removed_at=now(),removed_by=$2 where id=$1`, appointment.ID, task.CreatedBy.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case res := <-done:
		if res.err != nil || res.status != 403 {
			t.Fatalf("revoked 45 changed task %+v", res)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	got := o.get(task, o.member)
	if got.Title != task.Title || got.Version != task.Version {
		t.Fatal("revoked queued write committed")
	}
}
