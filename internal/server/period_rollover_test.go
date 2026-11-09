package server

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// TestPeriodRollover20262027 walks the change of period planned for early
// December 2026, end to end, on a pilot-like organization: the old
// period keeps its work and history read-only, the new one starts empty of
// people and work, and only explicit steps open the old work to the new
// board (PER-02..PER-09).
func TestPeriodRollover20262027(t *testing.T) {
	o := newTaskOrg(t) // 2025/2026 active; kadiv + PJ Mediatek, a Humas member
	ctx := context.Background()
	p1 := o.period
	spacesOf := func(pid id.ID, tok string) map[string]api.Space {
		t.Helper()
		var l api.List[api.Space]
		if c := o.call("GET", "/api/periods/"+pid.String()+"/spaces", tok, nil, &l); c != 200 {
			t.Fatalf("spaces %s = %d", pid, c)
		}
		out := map[string]api.Space{}
		for _, s := range l.Items {
			out[s.Code] = s
		}
		return out
	}

	// Work left in the old period: an open task for a PJ who stays, one
	// for a PJ who leaves, and a Medhum task.
	stay := o.create(false)
	leaverTok, leaver := o.join(o.kadiv, p1.ID, "mediatek", "keluar@himasif.test", "Anggota Keluar")
	req := o.taskRequest()
	req.AssigneeID = leaver.User.ID
	req.Title = "Dokumentasi akhir periode"
	var gone api.Task
	if c := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/tasks", o.kadiv, req, &gone); c != 201 {
		t.Fatalf("leaver task = %d", c)
	}
	shared := o.create(true)
	oldSpaces := spacesOf(p1.ID, o.admin)

	// 1. Preparing the new period copies the structure, not people or work.
	var p2 api.Period
	if c := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2026/2027",
		StartsOn: p1.EndsOn.AddDays(1), EndsOn: p1.EndsOn.AddDays(365)}, &p2); c != http.StatusCreated || p2.Status != api.PeriodPreparing {
		t.Fatalf("create p2 = %d %+v", c, p2)
	}
	newSpaces := spacesOf(p2.ID, o.admin)
	for _, code := range []string{"division-mediatek", "division-humas", "medhum"} {
		if _, ok := newSpaces[code]; !ok {
			t.Fatalf("p2 lacks %s: %v", code, newSpaces)
		}
	}
	if len(spacesOf(p2.ID, o.kadiv)) != 0 || len(spacesOf(p2.ID, o.member)) != 0 {
		t.Fatal("old people see the new period before being invited")
	}
	var members api.List[api.Member]
	o.call("GET", o.scope(p2.ID, "mediatek")+"/members", o.admin, nil, &members)
	if len(members.Items) != 0 {
		t.Fatalf("members copied into p2: %+v", members.Items)
	}

	// 2. Activating the new one leaves the old one active too (PER-09):
	// both can be worked in until the old one is closed.
	o.wantStatus(200, "POST", "/api/periods/"+p2.ID.String()+"/activate", o.admin, nil)
	var ps api.List[api.Period]
	o.call("GET", "/api/periods", o.admin, nil, &ps)
	active := 0
	for _, p := range ps.Items {
		if p.Status == api.PeriodActive {
			active++
		}
	}
	t.Logf("after activate: %d active periods, list order %s, %s", active, ps.Items[0].Label, ps.Items[1].Label)
	if ps.Items[0].ID != p2.ID {
		t.Fatalf("clients pick the first active period; p2 must sort first: %+v", ps.Items)
	}
	extra := o.taskRequest()
	extra.Title = "Masih bisa ditulis selama tumpang tindih"
	o.wantStatus(201, "POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/tasks", o.kadiv, extra)

	// 3. People of the new period: the PJ who stays becomes kadiv (same
	// account, ACC-01), a new member joins.
	var inv api.Invitation
	if c := o.call("POST", o.scope(p2.ID, "mediatek")+"/invitations", o.admin, api.InviteRequest{Email: "task-member@himasif.test"}, &inv); c != 201 {
		t.Fatalf("invite existing = %d", c)
	}
	tok := param(t, o.mail.last(t, "task-member@himasif.test").Text, "token")
	var acc api.InvitationAcceptResponse
	if c := o.call("POST", "/api/auth/invitations/accept", o.member, api.InvitationAcceptRequest{Token: tok}, &acc); c != 200 {
		t.Fatalf("existing account accept = %d", c)
	}
	o.wantStatus(200, "PUT", o.scope(p2.ID, "mediatek")+"/kadiv", o.admin, api.SetKadivRequest{UserID: o.memberID})
	newKadiv := o.member
	newTok, newMember := o.join(newKadiv, p2.ID, "mediatek", "baru@himasif.test", "Anggota Baru")

	// The new board numbers from #1 and starts empty.
	var first api.Task
	nreq := o.taskRequest()
	nreq.AssigneeID = newMember.User.ID
	nreq.DeadlineDate = taskTestDate(t, "2026-12-10")
	nreq.PostingDate = taskTestDate(t, "2026-12-12")
	if c := o.call("POST", "/api/spaces/"+newSpaces["division-mediatek"].ID.String()+"/tasks", newKadiv, nreq, &first); c != 201 {
		t.Fatalf("p2 task = %d", c)
	}
	t.Logf("first p2 task numbers %v (p1 task numbers %v)", first.Numbers, stay.Numbers)
	if first.Numbers[newSpaces["division-mediatek"].ID.String()] != 1 {
		t.Errorf("p2 numbering does not restart: %v", first.Numbers)
	}
	var mine api.List[api.Task]
	o.call("GET", "/api/periods/"+p2.ID.String()+"/tasks/mine", newKadiv, nil, &mine)
	for _, tk := range mine.Items {
		if tk.ID == stay.ID {
			t.Error("old-period task in the new period's Jobdesk")
		}
	}

	// 4. Handover, then close the old period.
	h := api.HandoverRequest{SuccessorPeriodID: &p2.ID, IncomingLeadership: "Mediatek: PJ lama jadi kadiv",
		ArchivePlan: "Arsip Mediatek untuk kadiv baru", RemainingWork: "2 tugas Mediatek + 1 Medhum masih todo"}
	o.wantStatus(200, "PUT", "/api/periods/"+p1.ID.String()+"/handover", o.admin, h)
	// Reminders of the old work exist before the close...
	if err := o.srv.schedule(ctx); err != nil {
		t.Fatal(err)
	}
	pending := func() int {
		var n int
		if err := o.st.DB().QueryRow(ctx, `select count(*) from delivery_events where source_kind='task' and source_id = any($1) and cancelled_at is null`, []id.ID{stay.ID, gone.ID, shared.ID}).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := pending()
	o.wantStatus(200, "POST", "/api/periods/"+p1.ID.String()+"/close", o.admin, nil)
	if err := o.srv.schedule(ctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("old-task reminders queued before close %d, after %d", before, pending())
	if pending() != 0 {
		t.Error("reminders of the closed period's tasks keep running (PER-08)")
	}
	// Closing is final.
	if c, code := o.errCode("POST", "/api/periods/"+p1.ID.String()+"/activate", o.admin, nil); c != 409 || code != api.CodePeriodClosed {
		t.Errorf("reopen closed period = %d %s", c, code)
	}

	// 5. The old work stays as it was, read-only, with its PJ unchanged.
	left := o.get(stay, o.kadiv)
	if left.Progress != "todo" || left.Assignee.ID != o.memberID || left.Permissions != (api.TaskPermissions{}) {
		t.Errorf("leftover task changed by the close: %+v", left)
	}

	// 6. Access afterwards.
	// The old kadiv, not in the new period: old spaces read-only, nothing new.
	if s := spacesOf(p1.ID, o.kadiv); !s["division-mediatek"].ReadOnly {
		t.Errorf("old kadiv's old space %+v", s["division-mediatek"])
	}
	if len(spacesOf(p2.ID, o.kadiv)) != 0 {
		t.Error("old kadiv sees the new period")
	}
	// The member who left still reads the space they were in (PER-03).
	ls := spacesOf(p1.ID, leaverTok)
	if sp, ok := ls["division-mediatek"]; !ok || !sp.ReadOnly {
		t.Errorf("leaver's archive %+v", ls)
	}
	if len(spacesOf(p2.ID, leaverTok)) != 0 {
		t.Error("leaver sees the new period")
	}
	// The new member sees no archive until the new kadiv grants it (PER-04).
	if len(spacesOf(p1.ID, newTok)) != 0 {
		t.Error("new member sees the archive without a grant")
	}
	if c, _ := o.errCode("GET", taskPath(stay), newTok, nil); c != 403 && c != 404 {
		t.Errorf("new member reads old task without grant = %d", c)
	}
	var g api.ArchiveGrant
	if c := o.call("POST", o.scope(p2.ID, "mediatek")+"/archives", newKadiv, api.ArchiveRequest{UserID: newMember.User.ID, SpaceID: oldSpaces["division-mediatek"].ID}, &g); c != 201 {
		t.Fatalf("archive grant = %d", c)
	}
	if c := o.call("GET", taskPath(stay), newTok, nil, nil); c != 200 {
		t.Errorf("granted archive task = %d", c)
	}
	if got := spacesOf(p1.ID, newTok); len(got) != 1 || !got["division-mediatek"].ReadOnly {
		t.Errorf("grant opened %v", got)
	}

	// 7. Leftover work does not move on its own (PER-06): it is not on the
	// new board and nobody can write to it.
	var board api.List[api.Task]
	o.call("GET", "/api/spaces/"+newSpaces["division-mediatek"].ID.String()+"/tasks", newKadiv, nil, &board)
	if len(board.Items) != 1 {
		t.Errorf("new board has %d tasks, want only the new one", len(board.Items))
	}
	if slices.ContainsFunc(board.Items, func(t api.Task) bool { return t.ID == stay.ID || t.ID == gone.ID }) {
		t.Error("old tasks leaked into the new board")
	}
}

// GET /api/periods marks the periods the caller belongs to, so the app
// opens the caller's own period, not an empty new one, during and after
// the change of period.
func TestPeriodsMarkMembership(t *testing.T) {
	o := newOrg(t)
	old, _ := o.kadiv("mediatek", "anggota-lama@himasif.test")
	var p2 api.Period
	if c := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2026/2027", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365)}, &p2); c != 201 {
		t.Fatalf("create = %d", c)
	}
	o.call("POST", "/api/periods/"+p2.ID.String()+"/activate", o.admin, nil, nil)
	fresh, _ := o.join(o.admin, p2.ID, "humas", "anggota-baru@himasif.test", "Anggota Baru")
	member := func(tok string) map[string]bool {
		t.Helper()
		var ps api.List[api.Period]
		if c := o.call("GET", "/api/periods", tok, nil, &ps); c != 200 || len(ps.Items) != 2 {
			t.Fatalf("periods = %d %+v", c, ps)
		}
		out := map[string]bool{}
		for _, p := range ps.Items {
			out[p.Label] = p.Member
		}
		return out
	}
	if m := member(old); !m["2025/2026"] || m["2026/2027"] {
		t.Errorf("old kadiv's periods %v", m)
	}
	if m := member(fresh); m["2025/2026"] || !m["2026/2027"] {
		t.Errorf("new member's periods %v", m)
	}
	if m := member(o.admin); m["2025/2026"] || m["2026/2027"] {
		t.Errorf("admin belongs to no division: %v", m)
	}
	// A closed period still counts: it is the one its people read.
	o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil)
	if m := member(old); !m["2025/2026"] {
		t.Errorf("closed period lost its member mark %v", m)
	}
}
